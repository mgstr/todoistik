// The keyboard layer. Everything it triggers is a plain link or form that
// exists on the page — the server stays the single source of truth.
(function () {
  "use strict";

  let gPending = false;

  const jumps = {
    i: "/inbox", t: "/today", n: "/next", p: "/projects", k: "/tasks",
    w: "/waiting", c: "/calendar", s: "/someday", h: "/scheduler",
    r: "/review", a: "/archive", u: "/audit", e: "/settings",
    // not a view: g z is g i then z, which is the pair pressed most often
    z: "/process",
  };

  // Vimium-style hints: holding "g" pins the jump key onto each nav link, so
  // the letter is visible on the destination itself. Built fresh from the
  // nav's own title="g i" attributes, so it can never drift from the links.
  function hintKey(el) {
    if (el.dataset.ghint) return el.dataset.ghint;
    const m = /^g (\S)$/.exec(el.getAttribute("title") || "");
    return m && jumps[m[1]] ? m[1] : null;
  }

  function showHints() {
    clearHints();
    document.querySelectorAll("nav a[title], [data-ghint]").forEach(function (el) {
      const key = hintKey(el);
      if (!key) return;
      const hint = document.createElement("span");
      hint.className = "ghint";
      hint.textContent = key;
      hint.setAttribute("aria-hidden", "true");
      el.appendChild(hint);
    });
  }

  function clearHints() {
    document.querySelectorAll(".ghint").forEach(function (h) { h.remove(); });
  }

  // The capture dialog. A native <dialog> so the centring, the backdrop, the
  // focus trap and Escape-to-cancel all come from the browser. Enter submits
  // to /capture, which drops a text already sitting in the inbox without
  // complaint — the item is there, which is what matters.
  function captureDialog() {
    return document.getElementById("capture-dialog");
  }

  function openCapture() {
    const d = captureDialog();
    if (!d || d.open) return;
    const box = d.querySelector("input[name=text]");
    if (box) box.value = "";
    d.showModal();
    if (box) box.focus();
    renderKeybar();
  }

  function closeCapture(d) {
    d.close();
    renderKeybar();
  }

  // Enter adds. Nothing typed means nothing to add, so just close: the empty
  // box is not a mistake worth a complaint, same as a duplicate is not.
  function submitCapture(d) {
    const box = d.querySelector("input[name=text]");
    if (!box || box.value.trim() === "") { closeCapture(d); return; }
    d.querySelector("form").requestSubmit();
  }

  function setPending(on) {
    gPending = on;
    if (on) showHints(); else clearHints();
    renderKeybar();
  }

  // The key bar, in two groups: what this view offers and what is always
  // there. They are held apart — view keys left, global keys right — so the
  // right half becomes fixed furniture and only the left half has to be
  // re-read when the view or the selection changes.
  // ? is offered only where there is a panel to open: a detail page sits under
  // no view, so it has no view help and the key would do nothing.
  function globalKeys() {
    const keys = [["q", "add to inbox"], ["g", "go to"]];
    if (document.getElementById("help")) keys.push(["?", "help"]);
    // a key marked data-global belongs to the app rather than to this view, so
    // it is read here and lands in the right half of the bar. Last, so a flag
    // whose label changes sits in the corner and does not shift the keys
    // beside it when it does
    declaredKeys("[data-key][data-global]").forEach(function (k) { keys.push(k); });
    return keys;
  }

  // Every entry is derived from what is actually on the page and what is
  // actually selected, so the bar can only ever offer a key that will do
  // something. A mode fills the view group and empties the global one:
  // while a dialog or an overlay is up, none of the global keys are live.
  function keybarGroups() {
    // doing mode is a mode like a dialog is: it owns the keyboard, so the bar
    // says the two keys that are live in it and nothing else
    if (doingBox()) {
      const timer = document.querySelector("#doing .timer");
      const keys = [["c", "done"], ["esc", "back"]];
      if (timer) keys.push(["^t", "timer " + (timer.hidden ? "hidden" : "shown")]);
      return { view: keys, global: [] };
    }
    const dlg = captureDialog();
    if (dlg && dlg.open) return { view: [["\u21b5", "add"], ["esc", "cancel"]], global: [] };
    const help = document.getElementById("help");
    if (help && !help.hidden) return { view: [["esc", "close help"]], global: [] };
    const np = document.getElementById("newproject-dialog");
    if (np && np.open) return { view: makeKeys(np).concat([["esc", "cancel"]]), global: [] };
    const dd = draftDialog();
    if (dd && dd.open) return { view: makeKeys(dd).concat([["esc", "cancel"]]), global: [] };
    const picker = document.querySelector("[data-picker] .pickerlist:not([hidden])");
    if (picker) {
      return { view: [["\u2193\u2191", "move"], ["^j ^k", "move"], ["\u21b5", "take"],
        ["type", "filter"], ["esc", "back"]], global: [] };
    }
    const closed = document.querySelector("[data-picker] .pickerbox");
    if (closed && closed === document.activeElement) {
      const empty = !document.querySelector("[data-picker] [name=projectid]").value &&
        !document.querySelector("[data-picker] [name=newproject]").value;
      return { view: [["\u2193", "projects"], ["c", "new project"],
        ["\u21b5", empty ? "new project" : "change"], ["esc", "standalone"]], global: [] };
    }
    if (gPending) {
      // z is the one jump with nothing on screen to mark: the processing
      // screen has no nav entry of its own (see implementation.md,
      // "Navigation"), so the bar is where it can be offered — and only while
      // there is an inbox to work down, or it would be a key that does nothing
      const view = [["\u2026", "press a marked key"]];
      if (document.querySelector("nav a.alert")) view.push(["z", "inbox zero"]);
      view.push(["esc", "cancel"]);
      return { view: view, global: [] };
    }

    const view = [];
    const typed = document.activeElement;
    if (typed && typed.closest && typing({ target: typed })) {
      const form = typed.closest("form");
      if (form) makeKeys(form).forEach(function (k) { view.push(k); });
    }
    const row = selected();
    const zero = document.querySelector("[data-inbox-zero]");

    // Where the view has a run to work down, the run leads and acting on one
    // picked item trails it, with the movement keys in between. The order is
    // the bar saying which is the default way through and which is the
    // exception — see design.md, "Inbox Zero". Everywhere else there is no
    // run, so acting on the selection leads.
    if (zero) view.push(["z", "inbox zero"]);
    if (!zero) pushRowKeys(view, row);
    if (rows().length) view.push(["j k", "move"]);
    if (zero) pushRowKeys(view, row);
    branchKeys().forEach(function (k) { view.push(k); });
    if (document.querySelector("[data-cancel]")) view.push(["esc", "cancel"]);
    if (document.querySelector(".namebox")) view.push(["/", "filter"]);
    return { view: view, global: globalKeys() };
  }

  // A screen can give its own controls keys, by declaring them on the control:
  // data-key is the key, data-key-label what the bar calls it. Both the bar and
  // the handler read the page, so — like the row keys above — a key can only
  // exist here if the thing it presses exists, and it can never be advertised
  // without working. Document order is the bar's order, which lets the template
  // decide how the answers read rather than this file.
  function branchKeys() {
    return declaredKeys("[data-key]:not([data-global])");
  }

  function declaredKeys(sel) {
    return Array.from(document.querySelectorAll(sel)).map(function (el) {
      return [el.dataset.key, el.dataset.keyLabel || ""];
    });
  }

  // A declared key may ask for ctrl, written "^a" — the same notation the bar
  // already uses for ctrl-enter. Ctrl and not cmd: cmd-a is select-all in every
  // text box on this machine, and a screen key must not take that away.
  function branchFor(e) {
    if (e.key.length !== 1 || e.altKey || e.metaKey) return null;
    const want = (e.ctrlKey ? "^" : "") + e.key.toLowerCase();
    return document.querySelector('[data-key="' + CSS.escape(want) + '"]');
  }

  // Pressing the key does exactly what clicking the control does: submit the
  // form, or follow the link. Nothing here knows what a branch means.
  function press(el) {
    if (el.tagName === "FORM") {
      if (el.requestSubmit) el.requestSubmit(); else el.submit();
      return;
    }
    // a button is pressed by pressing it, so the key and the mouse arrive at
    // the same handler and cannot come to differ
    if (el.tagName === "BUTTON") { el.click(); return; }
    const href = el.getAttribute("href");
    if (href) window.location.href = href;
  }

  function pushRowKeys(into, row) {
    if (!row) return;
    // an action written before its project exists is not on the list yet, so
    // none of the list's keys mean anything to it. What it has instead is
    // where it sits, and the keys for that are offered only where they would
    // do something: no "up" on the first row, no "down" on the last.
    if (row.hasAttribute("data-draft")) {
      into.push(["\u21b5", "edit"]);
      if (row.previousElementSibling) into.push(["u", "up"]);
      if (row.nextElementSibling) into.push(["d", "down"]);
      into.push(["r", "remove"]);
      return;
    }
    // opening an inbox item is processing it, so the two keys share a label
    if (row.hasAttribute("data-process")) into.push(["\u21b5 p", "process"]);
    else if (row.dataset.href) into.push(["\u21b5", "open"]);
    else if (row.querySelector("input[type=radio]")) into.push(["\u21b5", "pick"]);
    if (row.querySelector("form.kb-complete")) into.push(["c", "done"]);
    if (canDo(row)) into.push(["d", "doing"]);
    if (row.querySelector("form.kb-pick")) into.push(["t", "today"]);
  }

  // What the create button in this scope offers, said the way the screen says
  // it: the button's own words when it can be pressed, and what is still blank
  // when it cannot.
  function makeKeys(scope) {
    const btn = makeButton(scope);
    if (!btn) return [];
    if (!btn.disabled) return [["^\u21b5", btn.textContent.trim().toLowerCase()]];
    const need = missing(scope);
    if (!need.length) return [];
    return [["\u2026", "needs " + need.join(" and ")]];
  }

  function keygroup(cls, items) {
    const box = document.createElement("div");
    box.className = cls;
    items.forEach(function (pair) {
      const item = document.createElement("span");
      const key = document.createElement("b");
      key.textContent = pair[0];
      item.appendChild(key);
      item.append(pair[1]);
      box.appendChild(item);
    });
    return box;
  }

  function renderKeybar() {
    const bar = document.getElementById("keybar");
    if (!bar) return;
    const groups = keybarGroups();
    bar.textContent = "";
    bar.appendChild(keygroup("kb-view", groups.view));
    if (groups.global.length) bar.appendChild(keygroup("kb-global", groups.global));
  }

  // the button a form would submit with, if it has one
  function submitButton(form) {
    return form.querySelector("button[type=submit], button:not([type]):not([type=button])");
  }

  // The button that makes the thing, whether the thing is made by submitting a
  // form or by confirming a dialog.
  function makeButton(scope) {
    return submitButton(scope) || scope.querySelector("button.primary");
  }

  // What is still blank, named the way the screen names it. Reading the label
  // rather than the field means the bar says "needs a definition of done", in
  // the same words as the thing being pointed at.
  function missing(scope) {
    return Array.from(scope.querySelectorAll("[required]"))
      .filter(function (el) { return el.value.trim() === ""; })
      .map(function (el) {
        // a requirement with no box of its own says what to call it, since
        // there is no label to read it off — see the project form's action
        if (el.dataset.label) return el.dataset.label;
        const label = el.closest("label");
        if (!label) return el.name;
        return label.childNodes[0].textContent.trim().toLowerCase() || el.name;
      });
  }

  // A create button whose prerequisites are unmet is disabled, not hidden. It
  // still says that creating is what happens here and where the control is;
  // hiding it moves everything under it and leaves no clue the thing is
  // possible at all. Disabled promises nothing false — it says "not yet".
  function gate(scope) {
    const btn = makeButton(scope);
    if (!btn) return;
    if (!scope.querySelector("[required]")) return;
    btn.hidden = false;
    btn.disabled = missing(scope).length > 0;
  }

  function gateAll() {
    document.querySelectorAll("form, dialog").forEach(gate);
    renderKeybar();
  }
  document.addEventListener("input", function (e) {
    const scope = e.target.closest && e.target.closest("form, dialog");
    if (scope) { gate(scope); renderKeybar(); }
  });

  // Doing mode: the selected action alone, in the middle of an otherwise empty
  // screen. Built here out of the row rather than served as a page of its own,
  // because it is not a view (design.md, "Views") and there is nothing in it
  // the row does not already hold — the title it shows is the row's title, and
  // the one thing it can do is press the row's own complete form.
  function canDo(row) {
    return !!(row && row.hasAttribute("data-doing"));
  }

  function doingBox() { return document.getElementById("doing"); }

  // The nav slot you came in through reads "Doing…" while the mode is up, the
  // way it reads "Processing…" during a run (implementation.md, "Navigation").
  // The mode has no entry of its own and wants none: it borrows the one that
  // already answers "where am I", and the view's name is not news while you
  // are in the middle of one of its items. The markup is put back exactly as
  // it was taken, badge and all — unless the page it came from is gone, in
  // which case the nav on the new page is already right.
  let navHeld = null;

  function takeNavSlot() {
    const on = document.querySelector("nav a.on");
    if (!on) return;
    navHeld = { el: on, html: on.innerHTML };
    on.textContent = "Doing…";
  }

  function releaseNavSlot() {
    if (navHeld && navHeld.el.isConnected) navHeld.el.innerHTML = navHeld.html;
    navHeld = null;
  }

  function enterDoing(row) {
    if (!canDo(row) || doingBox()) return;
    const title = row.querySelector(".title");
    const pane = document.querySelector(".pane");
    const bar = document.getElementById("keybar");
    if (!title || !pane) return;
    const box = document.createElement("section");
    box.id = "doing";
    const p = document.createElement("p");
    p.textContent = title.textContent;
    box.appendChild(p);
    box.appendChild(startTimer(pane));
    pane.insertBefore(box, bar);
    // what the settings file said, read off the pane. The classes go on the
    // body because the rail is not inside the pane, and they come off again
    // in exitDoing — no other state is kept anywhere. The classes hide, the
    // settings show, so an absent attribute is what turns one on
    document.body.classList.add("doing");
    if (!pane.hasAttribute("data-doing-shows-nav")) document.body.classList.add("doing-no-nav");
    if (!pane.hasAttribute("data-doing-shows-keybar")) document.body.classList.add("doing-no-keybar");
    takeNavSlot();
    renderKeybar();
  }

  // The timer: the minutes since this action went on the screen. It is never
  // written down and never sent anywhere — it exists to give a feel for how
  // long things take, and a second `d` on the same action starts it again from
  // zero (design.md, "Doing one action").
  //
  // It is always built, whatever the settings file said: that decides whether
  // it starts visible, and `ctrl-t` decides after that. A timer that only
  // existed when it was on would start counting from the moment it was asked
  // for, which is not the number anyone means.
  let doingTick = null;
  let timerOn = null; // null until the settings file has been read once

  function pad2(n) { return String(n).padStart(2, "0"); }

  // "auto" is the app's own rendering: minutes while there are only minutes,
  // hours and minutes once there is an hour. Anything else is a pattern, where
  // H/HH is the hours and M/MM the minutes — within the hour when the pattern
  // asks for hours, and the whole elapsed time when it does not, since `M`
  // alone can only mean "how long has this been up". Everything else in the
  // pattern is literal (implementation.md, "Settings file").
  function elapsed(ms, format) {
    const mins = Math.max(0, Math.floor(ms / 60000));
    if (!format || format.toLowerCase() === "auto") {
      return mins < 60 ? pad2(mins) : Math.floor(mins / 60) + ":" + pad2(mins % 60);
    }
    const hours = Math.floor(mins / 60);
    const rest = /H/.test(format) ? mins % 60 : mins;
    return format.replace(/HH|H|MM|M/g, function (field) {
      if (field === "HH") return pad2(hours);
      if (field === "H") return String(hours);
      if (field === "MM") return pad2(rest);
      return String(rest);
    });
  }

  function startTimer(pane) {
    const started = Date.now();
    const format = pane.dataset.doingTimerFormat || "auto";
    if (timerOn === null) timerOn = pane.hasAttribute("data-doing-shows-timer");
    const el = document.createElement("span");
    el.className = "timer";
    el.hidden = !timerOn;
    el.textContent = elapsed(0, format);
    // once a second, written only when the minute has actually turned: the
    // clock has to be right the moment it is looked at, and a redraw that
    // changes nothing is one the eye can catch out of the corner
    doingTick = setInterval(function () {
      const now = elapsed(Date.now() - started, format);
      if (now !== el.textContent) el.textContent = now;
    }, 1000);
    return el;
  }

  function stopTimer() {
    if (doingTick !== null) { clearInterval(doingTick); doingTick = null; }
  }

  // ctrl-t means "show me the time" wherever it is pressed: the ages on a list
  // (see the layout's own toggle), the timer in here. The choice outlives the
  // mode but not the page — the settings file says how doing mode opens, and
  // the key says how it is going to be for the rest of this sitting.
  function toggleTimer() {
    const el = document.querySelector("#doing .timer");
    if (!el) return;
    timerOn = el.hidden;
    el.hidden = !timerOn;
    renderKeybar();
  }

  function exitDoing() {
    stopTimer();
    const box = doingBox();
    if (box) box.remove();
    document.body.classList.remove("doing", "doing-no-nav", "doing-no-keybar");
    releaseNavSlot();
    renderKeybar();
  }

  function rows() {
    return Array.from(document.querySelectorAll("[data-kb-row]"));
  }

  function selected() {
    return document.querySelector("[data-kb-row].kb-selected");
  }

  function select(row) {
    const cur = selected();
    if (cur) cur.classList.remove("kb-selected");
    if (row) {
      row.classList.add("kb-selected");
      row.scrollIntoView({ block: "nearest" });
      // a row that is a choice keeps focus with the selection, so tabbing and
      // j/k end up in the same place rather than disagreeing about where you are
      const radio = row.querySelector("input[type=radio]");
      if (radio && radio !== document.activeElement) radio.focus();
    }
    renderKeybar();
  }

  function move(delta) {
    const all = rows();
    if (!all.length) return;
    const cur = selected();
    let i = cur ? all.indexOf(cur) + delta : (delta > 0 ? 0 : all.length - 1);
    i = Math.max(0, Math.min(all.length - 1, i));
    select(all[i]);
  }

  function submitIn(row, cls) {
    const form = row.querySelector("form." + cls);
    if (form) form.submit();
  }

  // Only a field you can put text into counts as typing. A radio or a
  // checkbox is an <input> too, and treating those as typing meant that
  // tabbing onto one silently killed j/k/c/t — a bug everywhere, and fatal to
  // a list whose rows carry radios.
  const TEXTISH = /^(text|search|url|tel|email|password|number|date|month|week|time|datetime-local)$/;
  function typing(e) {
    const t = e.target;
    if (!t) return false;
    if (t.tagName === "TEXTAREA" || t.tagName === "SELECT" || t.isContentEditable) return true;
    return t.tagName === "INPUT" && TEXTISH.test(t.type || "text");
  }

  document.addEventListener("keydown", function (e) {
    // Doing mode answers to two keys and swallows the rest — including the
    // ctrl ones below and the global ones further down. A mode whose whole
    // point is that there is nothing else on the screen cannot leave the rest
    // of the app pressable behind it.
    if (doingBox()) {
      if (e.key === "c") {
        e.preventDefault();
        const row = selected();
        exitDoing();
        if (row) submitIn(row, "kb-complete");
        return;
      }
      if (e.key === "Escape") { e.preventDefault(); exitDoing(); return; }
      if (e.key === "t" && e.ctrlKey && !e.metaKey && !e.altKey) {
        e.preventDefault();
        toggleTimer();
        return;
      }
      // modified keys are the browser's (reload, address bar, a new tab), and
      // taking those would be taking more than this mode is entitled to
      if (!e.ctrlKey && !e.metaKey && !e.altKey) e.preventDefault();
      return;
    }

    // A screen key that asks for ctrl is live wherever the screen is, text
    // boxes included — reaching it without leaving the field is the whole
    // point of the modifier, and the reason a screen would choose one. Not
    // while a dialog is up: a dialog owns the keyboard, and the control the
    // key presses is on the page behind it.
    if (e.ctrlKey && !e.metaKey && !e.altKey && !document.querySelector("dialog[open]")) {
      const ctrlBranch = branchFor(e);
      if (ctrlBranch) { e.preventDefault(); press(ctrlBranch); return; }
    }

    const dlg = captureDialog();
    if (dlg && dlg.open) {
      // The dialog owns the keyboard while it is up, and both of its keys are
      // handled here rather than left to the browser: a modal <dialog> closes
      // itself on Escape and a lone text field submits itself on Enter, but
      // both are UA behaviours with edge cases, and these two keys are the
      // whole interaction.
      if (e.key === "Escape") { e.preventDefault(); closeCapture(dlg); }
      if (e.key === "Enter") { e.preventDefault(); submitCapture(dlg); }
      return;
    }
    if (typing(e)) {
      // ctrl-enter (cmd on a mac) submits the form being typed in. Plain Enter
      // cannot: in a textarea it makes a newline, and the description box is a
      // textarea — so the one key that finishes a form has to be reachable
      // from inside it. Derived from the page like everything else: it does
      // what the form's own submit button does, or nothing.
      if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) {
        e.preventDefault();
        submitScope(e.target);
        return;
      }
      if (e.key === "Escape") { e.target.blur(); return; }
      // j/k cannot live in a text box — the box has to be typeable — so a box
      // with a list under it says so with data-kb-into, and the arrow drops
      // into the list, where j/k do live
      if (e.key === "ArrowDown" && e.target.hasAttribute("data-kb-into")) {
        const first = rows()[0];
        if (first) { e.preventDefault(); select(first); }
      }
      return;
    }
    // ctrl-enter finishes the form being written, and that has to hold once
    // your hands have left its boxes: with a row of the project's action list
    // selected, plain enter opens that action and ctrl-enter still means "done
    // with this form". The scope is whatever form the selection or the focus
    // is inside — on every other screen a selected row is a link row sitting
    // in no form at all, so there is nothing there for this to reach.
    if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) {
      const row = selected();
      const here = (row && row.closest("form")) ||
        (document.activeElement && document.activeElement.closest &&
          document.activeElement.closest("form"));
      if (here) { e.preventDefault(); submitScope(here); }
      return;
    }
    if (e.metaKey || e.ctrlKey || e.altKey) return;

    if (gPending) {
      setPending(false);
      if (e.key === "g") { e.preventDefault(); openCapture(); return; }
      const dest = jumps[e.key];
      if (dest) {
        e.preventDefault();
        window.location.href = dest;
      }
      return;
    }

    // a key the page declares beats the standing map: on a screen that has
    // its own answers, those are what the letters mean there
    const branch = branchFor(e);
    if (branch) { e.preventDefault(); press(branch); return; }

    const row = selected();
    switch (e.key) {
      case "g": e.preventDefault(); setPending(true); break;
      case "j": e.preventDefault(); move(1); break;
      case "k": e.preventDefault(); move(-1); break;
      case "Enter":
      case "o": {
        if (row && row.hasAttribute("data-draft")) { e.preventDefault(); openDraft(row); break; }
        const radio = row && row.querySelector("input[type=radio]");
        if (radio) { e.preventDefault(); radio.checked = true; renderKeybar(); break; }
        if (row && row.dataset.href) { e.preventDefault(); window.location.href = row.dataset.href; }
        break;
      }
      case "p":
        if (row && row.hasAttribute("data-process") && row.dataset.href) {
          e.preventDefault();
          window.location.href = row.dataset.href;
        }
        break;
      case "z": {
        // Inbox Zero is only p over and over: the same screen, fed the oldest
        // item each time instead of the selected one.
        const list = document.querySelector("[data-inbox-zero]");
        if (list) { e.preventDefault(); window.location.href = list.dataset.inboxZero; }
        break;
      }
      case "u": if (row && row.hasAttribute("data-draft")) { e.preventDefault(); moveDraft(row, -1); } break;
      // d moves a draft down and starts doing an action. The two never meet:
      // a draft is an action that does not exist yet, so it has nothing to do
      case "d":
        if (row && row.hasAttribute("data-draft")) { e.preventDefault(); moveDraft(row, 1); break; }
        if (canDo(row)) { e.preventDefault(); enterDoing(row); }
        break;
      case "r": if (row && row.hasAttribute("data-draft")) { e.preventDefault(); removeDraft(row); } break;
      case "c": if (row) { e.preventDefault(); submitIn(row, "kb-complete"); } break;
      case "t": if (row) { e.preventDefault(); submitIn(row, "kb-pick"); } break;
      case "q": e.preventDefault(); openCapture(); break;
      case "/": {
        e.preventDefault();
        const box = document.querySelector(".namebox");
        if (box) box.focus();
        break;
      }
      case "?": {
        e.preventDefault();
        const help = document.getElementById("help");
        if (help) help.hidden = !help.hidden;
        renderKeybar();
        break;
      }
      case "Escape": {
        const help = document.getElementById("help");
        if (help && !help.hidden) { help.hidden = true; renderKeybar(); break; }
        // a screen that can be abandoned says so with data-cancel, and says
        // where leaving it goes. Nothing is written on the way out.
        const cancel = document.querySelector("[data-cancel]");
        if (cancel) { e.preventDefault(); window.location.href = cancel.dataset.cancel; break; }
        select(null);
        break;
      }
    }
  });

  // A row responds to the mouse the way a list row is expected to: one click
  // selects it, two open it. Selection was reachable only from j/k before, so
  // the row keys the bar was offering were unreachable without the keyboard.
  // Controls inside the row keep their own jobs — the title link navigates,
  // the complete and pick buttons submit — so none of them select instead.
  function rowFromEvent(e) {
    const row = e.target.closest("[data-kb-row]");
    if (!row || e.target.closest("a, button, input, select, textarea, label")) return null;
    return row;
  }

  // reaching a choice with Tab selects its row, so focus and selection say the
  // same thing however you got there
  document.addEventListener("focusin", function (e) {
    if (e.target.classList && e.target.classList.contains("pickerbox")) renderKeybar();
    else if (e.target.closest && e.target.closest("form")) renderKeybar();
    if (e.target.closest && !e.target.closest("[data-picker]")) {
      const openList = document.querySelector("[data-picker] .pickerlist:not([hidden])");
      if (openList) { openList.hidden = true; renderKeybar(); }
    }
    const row = e.target.closest && e.target.closest("[data-kb-row]");
    if (row && e.target.type === "radio" && !row.classList.contains("kb-selected")) select(row);
  });

  // a click or a lost window abandons a half-typed "g" sequence
  document.addEventListener("click", function (e) {
    setPending(false);
    if (e.target.closest("[data-capture-open]")) { e.preventDefault(); openCapture(); return; }
    if (e.target.closest("[data-draft-add]")) { e.preventDefault(); openDraft(null); return; }
    const row = rowFromEvent(e);
    if (row) select(row);
  });
  window.addEventListener("blur", function () { setPending(false); });

  // double click is the mouse's Enter: on the Inbox that is processing the
  // item, everywhere else it is opening it — the same data-href either way
  document.addEventListener("dblclick", function (e) {
    const row = rowFromEvent(e);
    if (row && row.dataset.href) { e.preventDefault(); window.location.href = row.dataset.href; }
  });

  // Actions written before their project exists (the project branch of
  // processing). They are held in the form as rows of hidden fields rather
  // than saved as they are written, because there is nothing yet to save them
  // to: design.md will not make a project without an action, so the project
  // and its actions are created in the one submit. A refused form therefore
  // has to carry them back, which is why they are form fields and not state
  // kept in here — see implementation.md, "Writing a project".
  function draftDialog() { return document.getElementById("draft-dialog"); }

  function draftValues(row) {
    return {
      title: row.querySelector("[name=atitle]").value,
      meta: row.querySelector("[name=ameta]").value,
      description: row.querySelector("[name=adescription]").value,
    };
  }

  function writeDraft(row, v) {
    row.querySelector(".title").textContent = v.title;
    row.querySelector(".draftmeta").textContent = v.meta;
    row.querySelector(".draftnote").textContent = v.description ? "note" : "";
    row.querySelector("[name=atitle]").value = v.title;
    row.querySelector("[name=ameta]").value = v.meta;
    row.querySelector("[name=adescription]").value = v.description;
  }

  // The create button is gated on there being an action, the same way it is
  // gated on a title and a DOD: a hidden required field the list keeps in step
  // with itself, so the one gate still reads the whole form.
  function syncDrafts(form) {
    if (!form) return;
    const flag = form.querySelector("[name=hasaction]");
    if (flag) flag.value = form.querySelector("[data-draft]") ? "1" : "";
    gate(form);
    renderKeybar();
  }

  function moveDraft(row, delta) {
    const other = delta < 0 ? row.previousElementSibling : row.nextElementSibling;
    if (!other) return;
    if (delta < 0) row.parentNode.insertBefore(row, other);
    else row.parentNode.insertBefore(other, row);
    row.scrollIntoView({ block: "nearest" });
    renderKeybar();
  }

  function removeDraft(row) {
    const form = row.closest("form");
    const next = row.nextElementSibling || row.previousElementSibling;
    row.remove();
    select(next || null);
    syncDrafts(form);
  }

  // One dialog, opened blank to add and filled to edit — the same fields in
  // both cases, and the same fields the processing screen writes an action in.
  function openDraft(row) {
    const dlg = draftDialog();
    const list = document.querySelector("[data-draft-list]");
    const tpl = document.getElementById("draftrow-template");
    if (!dlg || !list || !tpl) return;
    const form = list.closest("form");
    const title = dlg.querySelector("[name=title]");
    const meta = dlg.querySelector("[name=meta]");
    const desc = dlg.querySelector("[name=description]");
    const ok = dlg.querySelector("[data-draft-ok]");
    const v = row ? draftValues(row) : { title: "", meta: "", description: "" };
    title.value = v.title; meta.value = v.meta; desc.value = v.description;
    dlg.querySelector("h2").textContent = row ? "Edit action" : "Add an action";
    ok.textContent = row ? "Save action" : "Create action";

    gate(dlg);
    dlg.showModal();
    title.focus();
    renderKeybar();

    function done(confirmed) {
      if (confirmed) {
        if (ok.disabled) { title.focus(); return; }
        let target = row;
        if (!target) {
          target = tpl.content.firstElementChild.cloneNode(true);
          list.appendChild(target);
        }
        writeDraft(target, {
          title: title.value.trim(),
          meta: meta.value.trim(),
          description: desc.value.trim(),
        });
        select(target);
      }
      dlg.close();
      syncDrafts(form);
    }
    ok.onclick = function () { done(true); };
    dlg.querySelector("[data-draft-cancel]").onclick = function () { done(false); };
    // the dialog owns both keys outright, for the reason the new-project one
    // does: Escape reaching the screen would close the dialog and leave the
    // whole form in the same press
    dlg.onkeydown = function (e) {
      if (e.key === "Escape") { e.preventDefault(); e.stopPropagation(); done(false); return; }
      if (e.key !== "Enter") return;
      if (e.ctrlKey || e.metaKey) { e.preventDefault(); e.stopPropagation(); done(true); return; }
      if (e.target !== desc) { e.preventDefault(); e.stopPropagation(); done(true); }
    };
  }

  // The project picker. A project is chosen, never typed, so what is submitted
  // is an id or a pending new project — never a name to be resolved. It owns
  // the keyboard while it has focus, which is why its handler stops the event
  // before the layer above sees it: j and k are letters here.
  //
  // Letters filter and the arrows move, rather than the other way round. j/k
  // cannot filter and move at the same time, and with a long list typing is
  // the fast path — so movement takes the form vim itself uses when the
  // letters are spoken for: the arrows, and ctrl-j / ctrl-k.
  function setupPicker(root) {
    const box = root.querySelector(".pickerbox");
    const list = root.querySelector(".pickerlist");
    const idField = root.querySelector("[name=projectid]");
    const newField = root.querySelector("[name=newproject]");
    const dodField = root.querySelector("[name=newdod]");
    const rows = function () {
      return Array.from(list.querySelectorAll(".pickrow")).filter(function (r) { return !r.hidden; });
    };

    function label() {
      if (newField.value) return "+ " + newField.value;
      if (!idField.value) return "<standalone>";
      const row = list.querySelector('[data-pick="' + idField.value + '"]');
      return row ? row.dataset.title : "<standalone>";
    }

    function close() {
      list.hidden = true;
      box.readOnly = true;
      box.value = label();
      renderKeybar();
    }

    function open() {
      if (!list.hidden) return;
      box.readOnly = false;
      box.value = "";
      list.hidden = false;
      filter();
      highlight(rows()[0]);
      renderKeybar();
    }

    function filter() {
      const q = box.value.trim().toLowerCase();
      let shown = null;
      list.querySelectorAll(".pickrow").forEach(function (row) {
        const hay = (row.dataset.title || "").toLowerCase();
        // every word must appear, in any order — the rule the name filter uses
        const hit = q === "" || q.split(/\s+/).every(function (w) { return hay.includes(w); });
        row.hidden = !hit && !row.classList.contains("pnew");
        if (!row.hidden && !shown) shown = row;
      });
      if (!list.querySelector(".pickrow.on:not([hidden])")) highlight(shown);
    }

    function highlight(row) {
      list.querySelectorAll(".pickrow.on").forEach(function (r) { r.classList.remove("on"); });
      if (row) { row.classList.add("on"); row.scrollIntoView({ block: "nearest" }); }
    }

    function move(delta) {
      const all = rows();
      if (!all.length) return;
      const cur = list.querySelector(".pickrow.on");
      let i = cur ? all.indexOf(cur) + delta : 0;
      i = Math.max(0, Math.min(all.length - 1, i));
      highlight(all[i]);
    }

    function take(row) {
      if (!row) return;
      if (row.dataset.pick === "new") { close(); newProjectDialog(root); return; }
      newField.value = ""; dodField.value = "";
      idField.value = row.dataset.pick;
      close();
    }

    function standalone() {
      idField.value = ""; newField.value = ""; dodField.value = "";
      close();
    }

    box.addEventListener("input", filter);
    box.addEventListener("mousedown", function () { if (list.hidden) open(); });
    list.addEventListener("click", function (e) {
      const row = e.target.closest(".pickrow");
      if (row) take(row);
    });

    root.addEventListener("keydown", function (e) {
      if (e.target !== box) return;
      const openList = !list.hidden;
      const ctrl = e.ctrlKey && !e.metaKey && !e.altKey;
      switch (true) {
        case e.key === "ArrowDown" || (ctrl && e.key === "j"):
          e.preventDefault(); e.stopPropagation();
          if (openList) move(1); else open();
          return;
        case e.key === "ArrowUp" || (ctrl && e.key === "k"):
          e.preventDefault(); e.stopPropagation();
          if (openList) move(-1);
          return;
        case e.key === "Enter":
          e.preventDefault(); e.stopPropagation();
          // ctrl-enter is the form's, everywhere and always. Here it takes
          // whatever the list is showing as chosen first, so what is submitted
          // is what is on screen, and then finishes.
          if (e.ctrlKey || e.metaKey) {
            if (openList) take(list.querySelector(".pickrow.on"));
            submitScope(root);
            return;
          }
          if (openList) { take(list.querySelector(".pickrow.on")); return; }
          if (!idField.value && !newField.value) newProjectDialog(root); else open();
          return;
        // a letter can only be a command while the list is shut, because an
        // open list is being filtered and every letter belongs to the filter
        case e.key === "c" && !openList && !ctrl:
          e.preventDefault(); e.stopPropagation();
          newProjectDialog(root);
          return;
        case e.key === "Escape":
          // one step at a time: the filter first, then the choice. Once there
          // is nothing of ours left to undo the key is not ours either — it
          // has to reach the screen, or the form could not be left from here
          if (openList && box.value !== "") {
            e.preventDefault(); e.stopPropagation();
            box.value = ""; filter(); return;
          }
          if (openList || idField.value || newField.value) {
            e.preventDefault(); e.stopPropagation();
            standalone(); return;
          }
          return;
      }
    });

    close();
  }

  // The new project is held, not created: until the action form is submitted
  // there is no action to be its first, and a project without one is a thing
  // design.md will not make.
  function newProjectDialog(root) {
    const dlg = document.getElementById("newproject-dialog");
    if (!dlg) return;
    const title = dlg.querySelector("#np-title");
    const dod = dlg.querySelector("#np-dod");
    const ok = dlg.querySelector("#np-ok");
    title.value = ""; dod.value = "";

    // A project needs both a title and a definition of done — design.md will
    // not make one without them — so until it has both the button is disabled.
    // It stays on screen: see gate().
    function ready() { return !ok.disabled; }
    gate(dlg);

    dlg.showModal();
    title.focus();
    renderKeybar();

    function done(confirmed) {
      if (confirmed) {
        if (!ready()) { (title.value.trim() === "" ? title : dod).focus(); return; }
        const name = title.value.trim();
        root.querySelector("[name=newproject]").value = name;
        root.querySelector("[name=newdod]").value = dod.value.trim();
        root.querySelector("[name=projectid]").value = "";
        const box = root.querySelector(".pickerbox");
        box.value = "+ " + name;
      }
      dlg.close();
      root.querySelector(".pickerbox").focus();
      renderKeybar();
    }
    ok.onclick = function () { done(true); };
    dlg.querySelector("#np-cancel").onclick = function () { done(false); };
    // the dialog owns both keys outright: letting Escape reach the screen
    // would close the dialog and leave the form in the same press
    dlg.onkeydown = function (e) {
      if (e.key === "Escape") { e.preventDefault(); e.stopPropagation(); done(false); return; }
      if (e.key !== "Enter") return;
      // ctrl-enter finishes from anywhere in the dialog, including the
      // textarea where a plain Enter has to keep meaning "newline"
      if (e.ctrlKey || e.metaKey) { e.preventDefault(); e.stopPropagation(); done(true); return; }
      if (e.target !== dod) { e.preventDefault(); e.stopPropagation(); done(true); }
    };
  }

  // submitScope finishes whatever is being written around el — the dialog if
  // there is one, otherwise the form. Refuses when the create button is
  // disabled, so the key and the button can never disagree.
  function submitScope(el) {
    const dlg = el.closest("dialog");
    const scope = dlg || el.closest("form");
    if (!scope) return false;
    const btn = makeButton(scope);
    if (!btn || btn.disabled) return false;
    if (dlg) { btn.click(); return true; }
    if (scope.requestSubmit) scope.requestSubmit(); else scope.submit();
    return true;
  }

  function setupPickers() {
    document.querySelectorAll("[data-picker]").forEach(setupPicker);
    document.querySelectorAll("[data-drafts]").forEach(syncDrafts);
  }
  setupPickers();
  gateAll();

  // hx-boost swaps the body, taking the rendered bar with it
  document.addEventListener("htmx:afterSwap", function () {
    // whatever was being done is gone with the old page: the mode is the row
    // and the row has just been replaced
    exitDoing();
    renderKeybar(); setupPickers(); gateAll();
  });

  // A field that opens focused with the caret at position 0 means the first
  // thing typed lands in front of the text already there, which is never what
  // was meant. Neither the browser nor htmx places the caret when it honours
  // autofocus, so it is placed here. This is the one thing in this file that
  // is not the keyboard layer, and it is allowed the same way the layer is:
  // it decides nothing and stores nothing — the rule it must not break is that
  // the server is the single source of truth (see implementation.md, "Stack").
  function caretToEnd() {
    const el = document.querySelector("[autofocus]");
    if (!el || el !== document.activeElement) return;
    if (typeof el.selectionStart !== "number") return;
    el.setSelectionRange(el.value.length, el.value.length);
  }
  // afterSettle, not afterSwap: htmx honours autofocus in the settle step, so
  // on a boosted navigation the field is not focused yet when the swap fires
  document.addEventListener("htmx:afterSettle", caretToEnd);
  caretToEnd();

  renderKeybar();

  // filter forms apply themselves on any change
  document.addEventListener("change", function (e) {
    const form = e.target.closest("form[data-autosubmit]");
    if (form) form.submit();
  });
})();
