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
    const pd = panelsDialog();
    // the panel chooser is a list of keys and nothing else, so the bar is that
    // list — read off the dialog's own controls, like every other declared key
    if (pd && pd.open) {
      return { view: declaredKeys("[data-key]").concat([["esc", "close"]]), global: [] };
    }
    const unknown = unknownDialog();
    if (unknown && unknown.open && topDialog() === unknown) {
      const keys = declaredKeys("[data-key]");
      if (rows().length > 1) keys.push(["j k", "move"]);
      keys.push(["\u21b5", "take"], ["esc", "cancel"]);
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
    if (openSuggest()) {
      return { view: [["\u2193\u2191", "move"], ["\u21b5", "take"], ["esc", "back"]], global: [] };
    }
    if (filterBox() && document.activeElement === filterBox()) {
      return { view: [["\u21b5", "apply"], ["^f", "no filter"], ["esc", "leave the box"]], global: globalKeys() };
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
      if (document.querySelector(".pane[data-inbox-full]")) view.push(["z", "inbox zero"]);
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
    const cancel = document.querySelector("[data-cancel]");
    if (cancel) view.push(["esc", cancel.dataset.cancelLabel || "cancel"]);
    if (document.querySelector(".namebox")) view.push(["/", "filter"]);
    const bar = filterBar();
    if (bar) view.push(["^f", bar.hidden ? "filter" : "no filter"]);
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
    return Array.from(document.querySelectorAll(sel)).filter(keyUsable).map(function (el) {
      return [el.dataset.key, el.dataset.keyLabel || ""];
    });
  }

  // A key a dialog declares is live exactly while that dialog is open, and
  // while one is open no key outside it is: a dialog owns the keyboard, and
  // the control a page key would press is on the page behind it. Without this
  // the chooser's t/n/k/z would mean its four panels on every screen in the
  // app, since the dialog is in the layout and therefore always on the page.
  function keyLive(el) {
    const open = topDialog();
    const own = el.closest("dialog");
    return open ? own === open : !own;
  }

  // The dialog on top, when more than one is up: the unknown-name dialog can
  // open over the one that is writing a draft action, and the keys belong to
  // the newer question. Document order answers it, because the dialogs that
  // interrupt another one live in the layout, after everything a page holds.
  function topDialog() {
    const all = document.querySelectorAll("dialog[open]");
    return all.length ? all[all.length - 1] : null;
  }

  // and a key on a control that cannot be pressed is not a key: the bar must
  // never offer one that does nothing (see implementation.md, "Keyboard")
  function keyUsable(el) {
    return keyLive(el) && !el.disabled;
  }

  // A declared key may ask for ctrl, written "^a" — the same notation the bar
  // already uses for ctrl-enter. Ctrl and not cmd: cmd-a is select-all in every
  // text box on this machine, and a screen key must not take that away.
  function branchFor(e) {
    if (e.key.length !== 1 || e.altKey || e.metaKey) return null;
    const want = (e.ctrlKey ? "^" : "") + e.key.toLowerCase();
    const all = document.querySelectorAll('[data-key="' + CSS.escape(want) + '"]');
    for (let i = 0; i < all.length; i++) {
      if (keyUsable(all[i])) return all[i];
    }
    return null;
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

  // A dialog that is a step rather than a question says so with data-crumb,
  // and the title bar carries it while it is up: the trail is where you are,
  // and being in the new-project dialog is a place. The server wrote the steps
  // before it and this is the one that only exists in the browser, which is
  // why it is added here rather than rendered — there is no request to render
  // it on.
  function syncCrumb() {
    const bar = document.getElementById("titlebar");
    if (!bar) return;
    const open = document.querySelector("dialog[open][data-crumb]");
    const here = bar.querySelector(".crumb.added");
    if (open && !here) {
      const el = document.createElement("span");
      el.className = "crumb added";
      el.textContent = open.dataset.crumb;
      bar.appendChild(el);
    } else if (!open && here) {
      here.remove();
    }
  }

  function renderKeybar() {
    syncCrumb();
    const bar = document.getElementById("keybar");
    if (!bar) return;
    const groups = keybarGroups();
    bar.textContent = "";
    bar.appendChild(keygroup("kb-view", groups.view));
    if (groups.global.length) bar.appendChild(keygroup("kb-global", groups.global));
  }

  // the button a form would submit with, if it has one. A button may sit
  // outside the form and point at it with the form attribute — which is how
  // Save gets to stand in one row with Complete and Delete, each of which is
  // a form of its own (see implementation.md, "Writing an action")
  function submitButton(form) {
    const inside = form.querySelector("button[type=submit], button:not([type]):not([type=button])");
    if (inside) return inside;
    return form.id ? document.querySelector('button[form="' + CSS.escape(form.id) + '"]') : null;
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
        // the name is its own element on a label that puts it in a gutter, and
        // a bare text node in front of the box everywhere else
        const word = label.querySelector(".lb") || label.childNodes[0];
        return word.textContent.trim().toLowerCase() || el.name;
      });
  }

  // A create button whose prerequisites are unmet is disabled, not hidden. It
  // still says that creating is what happens here and where the control is;
  // hiding it moves everything under it and leaves no clue the thing is
  // possible at all. Disabled promises nothing false — it says "not yet".
  function gate(scope) {
    const btn = makeButton(scope);
    if (!btn) return;
    const needs = !!scope.querySelector("[required]");
    const dirty = scope.hasAttribute && scope.hasAttribute("data-dirty-save");
    if (!needs && !dirty) return;
    btn.hidden = false;
    btn.disabled = (needs && missing(scope).length > 0) || (dirty && !changed(scope));
  }

  // A form that says data-dirty-save has a button meaning "keep this", and
  // there is nothing to keep until something differs from what the server
  // sent. Each field's own defaultValue is that very thing, so nothing has to
  // be remembered in here — the same trick the filter box's Apply uses.
  function changed(scope) {
    return Array.from(scope.querySelectorAll("input, textarea, select")).some(function (el) {
      if (el.type === "checkbox" || el.type === "radio") return el.checked !== el.defaultChecked;
      if (el.tagName === "SELECT") {
        return Array.from(el.options).some(function (o) { return o.selected !== o.defaultSelected; });
      }
      return el.value !== el.defaultValue;
    });
  }

  function gateAll() {
    document.querySelectorAll("form, dialog").forEach(gate);
    renderKeybar();
  }
  document.addEventListener("input", function (e) {
    // a token box paints itself and offers what you may be typing — and then
    // falls through, because it is a field in a form like any other and the
    // form's own button has to know that something changed
    if (e.target.matches && e.target.matches("[data-tokenbox]")) {
      paintBox(e.target);
      showSuggest(e.target);
    }
    const scope = e.target.closest && e.target.closest("form, dialog");
    if (scope) { gate(scope); renderKeybar(); }
  });
  // the mirror is only right while it is scrolled exactly as far as the box
  document.addEventListener("scroll", function (e) {
    if (e.target.matches && e.target.matches("[data-tokenbox]")) {
      const wrap = boxWrap(e.target);
      if (wrap) wrap.querySelector(".fmirror").scrollLeft = e.target.scrollLeft;
    }
  }, true);

  // Doing is a view of its own now — /doing/<id>, opened from a row with d —
  // so nothing here builds it. What is left is the timer, because the timer is
  // the one thing on that screen the server does not decide: it is never
  // written down (design.md, "Doing one action"), which means it can only live
  // in here, for as long as the tab does.
  function canDo(row) {
    return !!(row && row.dataset.doing);
  }

  // d carries the view it was pressed in on the URL. The screen has to know
  // where "back" goes after a reload, and the URL is the only part of how you
  // got there that survives one.
  function doingHref(row) {
    return row.dataset.doing + "?from=" + encodeURIComponent(location.pathname);
  }

  // The timer: the minutes since this action went on the screen. It is never
  // written down and never sent anywhere — it exists to give a feel for how
  // long things take, and opening the screen again starts it at zero
  // (design.md, "Doing one action").
  //
  // It is always ticking, whatever the settings file said: that decides
  // whether it starts visible, and `ctrl-t` decides after that. A timer that
  // only existed when it was on would start counting from the moment it was
  // asked for, which is not the number anyone means.
  let doingTick = null;
  let timerOn = null; // null until a doing screen has shown what the file said

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

  function timerEl() { return document.querySelector("#doing .timer"); }

  function startTimer() {
    stopTimer();
    const el = timerEl();
    if (!el) return;
    const pane = document.querySelector(".pane");
    const format = (pane && pane.dataset.zenTimerFormat) || "auto";
    // the server renders the settings file's answer; the key's answer, once
    // given, outlives every boosted navigation and is gone on a reload
    if (timerOn === null) timerOn = !el.hidden;
    el.hidden = !timerOn;
    labelTimer();
    const started = Date.now();
    el.textContent = elapsed(0, format);
    // once a second, written only when the minute has actually turned: the
    // clock has to be right the moment it is looked at, and a redraw that
    // changes nothing is one the eye can catch out of the corner
    doingTick = setInterval(function () {
      const now = elapsed(Date.now() - started, format);
      if (now !== el.textContent) el.textContent = now;
    }, 1000);
  }

  function stopTimer() {
    if (doingTick !== null) { clearInterval(doingTick); doingTick = null; }
  }

  // the bar reads the flag off the control, the way it reads the ages flag off
  // the layout's form — so the label is written where the key is declared and
  // nothing in the bar knows what a timer is
  function labelTimer() {
    const btn = document.querySelector("[data-timer]");
    const el = timerEl();
    if (btn && el) btn.dataset.keyLabel = "timer " + (el.hidden ? "hidden" : "shown");
  }

  // ctrl-t means "show me the time" wherever it is pressed: the ages on a list
  // (see the layout's own toggle), the timer here. The screen with a timer on
  // it renders no ages control at all, so the key has exactly one meaning
  // wherever it is pressed.
  function toggleTimer() {
    const el = timerEl();
    if (!el) return;
    timerOn = el.hidden;
    el.hidden = !timerOn;
    labelTimer();
    renderKeybar();
  }

  // The panel chooser: ctrl-v, four forms, each one both a key and a click.
  function panelsDialog() { return document.getElementById("panels-dialog"); }

  // --- token boxes --------------------------------------------------------
  //
  // The filter line and the two meta lines are the same box: a line of @names
  // and #names, completed as it is typed and marked where the app does not
  // know one. What differs is which notation each accepts, and that is the
  // whole of `BOX_RULES` — the rest of this section does not know which box it
  // is looking at.
  //
  // The lines are parsed properly in Go (ParseQuery, ParseMeta). What happens
  // here is the two things that have to happen before a line is submitted:
  // asking about a name the app cannot use, and saving you from typing the
  // ones it can. The rules are therefore stated twice, and deliberately —
  // here as questions, there as answers.
  const FILTER_OPEN = "kb-filter-open";
  const TOKEN_RE = /(^|\s)([@#])([\p{L}\p{N}_-]+)(\(([^)]*)\))?/gu;
  const DATE_RE = /(^|\s)([a-z]+):(\S+)/g;
  // what is being typed right now, which is a token that may still be empty.
  // A date key is a sigil like any other here — it is what has been committed
  // to and the rest is still open — except that its sigil is `due:` or
  // `snooze:` rather than one character
  const TYPING_RE = /(^|\s)([@#])([\p{L}\p{N}_-]*)$/u;
  const TYPING_DATE_RE = /(^|\s)([a-z]+:)([\p{L}\p{N}-]*)$/u;
  const DATE_OK = /^\d{4}-\d{2}-\d{2}$/;
  const NDAYS_OK = /^\d+(d|days?)$/;
  const NDAYS_ZERO = /^0+(d|days?)$/;

  // Each box says what it takes. contexts is how many an item can have, fields
  // are the #names that stand for fields rather than tags, dates are the two
  // date notations, and prose says whether a word that is not notation is
  // allowed — the filter line matches titles by its leftover words, a meta
  // line refuses them (design.md, "Writing an action").
  // `dates` says which `key:value` notations this line takes and what the value
  // may be: a date, one of a fixed set of words, or a number of days. The
  // windows are words because "what is coming at me" moves with the day
  // (design.md, "Calendar"); a meta line's words are a date counted off from
  // today, resolved by the server on the day it is saved. `today` is a real
  // answer to a deadline and no answer at all to a snooze, which is the one
  // difference between the two lists (design.md, "Time fields").
  const DUE_WINDOWS = ["today", "tomorrow", "thisweek", "nextweek"];
  const DAY_NAMES = ["monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"];
  const WHEN_DUE = { date: true, days: true, words: ["today", "tomorrow"].concat(DAY_NAMES) };
  const WHEN_SNOOZE = { date: true, days: true, ahead: true, words: ["tomorrow"].concat(DAY_NAMES) };
  const WHEN_WINDOW = { date: false, days: false, words: DUE_WINDOWS };
  const BOX_RULES = {
    filter: { contexts: 1, fields: ["short", "medium", "long", "focus", "today"], dates: {}, prose: true },
    // some views filter by tag and by name and by nothing else — design.md
    // gives each view the subset it offers, and a line that quietly ignored
    // the rest would be the app pretending to have narrowed something
    "filter-tags": { contexts: 0, fields: [], dates: {}, prose: true },
    "filter-due": { contexts: 0, fields: [], dates: { due: WHEN_WINDOW }, prose: true },
    "filter-name": { contexts: 0, fields: [], tags: false, dates: {}, prose: true },
    action: { contexts: 1, fields: ["short", "medium", "long", "focus", "parked", "today"], dates: { due: WHEN_DUE, snooze: WHEN_SNOOZE }, prose: false, waiting: true },
    project: { contexts: 0, fields: [], dates: { snooze: WHEN_SNOOZE }, prose: false },
  };

  function tokenBoxes() { return Array.from(document.querySelectorAll("[data-tokenbox]")); }
  function rulesFor(box) { return BOX_RULES[box.dataset.tokenbox] || BOX_RULES.filter; }
  function boxWrap(box) { return box.closest(".fbox"); }
  function suggestList(box) { const w = boxWrap(box); return w && w.querySelector(".fsuggest"); }
  function openSuggest() { return document.querySelector(".fsuggest:not([hidden])"); }

  function filterBar() { return document.querySelector("[data-filterbar]"); }
  function filterBox() { const bar = filterBar(); return bar && bar.querySelector("[data-tokenbox]"); }

  // the remembered lists, off the pane: every page carries them because any
  // page may hold a box (implementation.md, "Token boxes")
  function vocabNames(which) {
    const pane = document.querySelector(".pane");
    if (!pane) return [];
    return (pane.dataset[which] || "").trim().split(/\s+/).filter(Boolean);
  }

  function knownNames(box, sigil) {
    const rules = rulesFor(box);
    if (sigil === "@") {
      return vocabNames("contexts").concat(rules.waiting ? ["waitingFor"] : []);
    }
    return vocabNames("tags").concat(rules.fields);
  }

  function tokensIn(text) {
    const out = [];
    TOKEN_RE.lastIndex = 0;
    let m;
    while ((m = TOKEN_RE.exec(text)) !== null) {
      const start = m.index + m[1].length;
      out.push({
        start: start, end: m.index + m[0].length,
        sigil: m[2], name: m[3], param: m[5] || "",
        text: text.slice(start, m.index + m[0].length),
      });
    }
    return out;
  }

  // Everything wrong with a line, in the order it is written. A problem is a
  // span of the text and what to do about it; only the ones that are simply
  // names the app has never been told about can be answered by creating one.
  function problemsIn(box) {
    const text = box.value, rules = rulesFor(box);
    const out = [];
    const taken = [];
    let contexts = 0;

    tokensIn(text).forEach(function (t) {
      taken.push(t);
      const pool = knownNames(box, t.sigil);
      if (t.sigil === "@") {
        if (pool.indexOf(t.name) < 0) { out.push(Object.assign({ kind: "context" }, t)); return; }
        if (t.name === "waitingFor") return; // its own field, not the context
        if (!rules.contexts) { out.push(Object.assign({ kind: "no-context" }, t)); return; }
        if (++contexts > rules.contexts) out.push(Object.assign({ kind: "second-context" }, t));
        return;
      }
      if (rules.tags === false) { out.push(Object.assign({ kind: "not-here" }, t)); return; }
      const field = BOX_RULES.action.fields.concat("today").indexOf(t.name) >= 0;
      if (field && rules.fields.indexOf(t.name) < 0) {
        out.push(Object.assign({ kind: "not-here" }, t));
        return;
      }
      if (pool.indexOf(t.name) < 0) out.push(Object.assign({ kind: "tag" }, t));
    });

    // dates and prose are read from what the tokens left behind, so that a
    // name is never also a word
    let rest = text.split("");
    taken.forEach(function (t) { for (let i = t.start; i < t.end; i++) rest[i] = " "; });
    const left = rest.join("");

    DATE_RE.lastIndex = 0;
    let m;
    const dated = [];
    while ((m = DATE_RE.exec(left)) !== null) {
      const start = m.index + m[1].length;
      const t = { start: start, end: m.index + m[0].length, name: m[2], param: m[3], sigil: "" };
      t.text = text.slice(t.start, t.end);
      dated.push(t);
      const spec = rules.dates[m[2]];
      if (!spec) { out.push(Object.assign({ kind: "not-here" }, t)); continue; }
      const bad = dateProblem(spec, m[3]);
      if (bad) out.push(Object.assign({ kind: bad, words: spec.words }, t));
    }

    if (!rules.prose) {
      dated.forEach(function (t) { for (let i = t.start; i < t.end; i++) rest[i] = " "; });
      const words = rest.join("");
      const wordRe = /\S+/g;
      let w;
      while ((w = wordRe.exec(words)) !== null) {
        out.push({ kind: "prose", start: w.index, end: w.index + w[0].length, text: w[0], name: w[0], sigil: "" });
      }
    }
    return out.sort(function (a, b) { return a.start - b.start; });
  }

  // What is wrong with a date token's value, "" when nothing is. `ahead` is
  // the snooze rule: a word that lands on today is not a snooze, so it is
  // named as that rather than reported as an unreadable date (design.md,
  // "Time fields"). An ISO date in the past is left alone here for the same
  // reason the server leaves it alone — it is a claim that went stale, and the
  // weekly review is what catches it.
  function dateProblem(spec, val) {
    if (spec.date && DATE_OK.test(val)) return "";
    if (spec.ahead && (val === "today" || NDAYS_ZERO.test(val))) return "not-ahead";
    if (spec.words.indexOf(val) >= 0) return "";
    if (spec.days && NDAYS_OK.test(val)) return "";
    return spec.date ? "bad-date" : "bad-window";
  }

  // The mirror carries the marks: the same text in the same place, behind the
  // input, with its own text invisible so that only the wavy lines show.
  function paintBox(box) {
    const wrap = boxWrap(box);
    if (!wrap) return;
    const mirror = wrap.querySelector(".fmirror");
    const text = box.value;
    mirror.textContent = "";
    let at = 0;
    problemsIn(box).forEach(function (p) {
      if (p.start < at) return;
      mirror.appendChild(document.createTextNode(text.slice(at, p.start)));
      const mark = document.createElement("span");
      mark.className = "bad";
      mark.textContent = p.text;
      mirror.appendChild(mark);
      at = p.end;
    });
    mirror.appendChild(document.createTextNode(text.slice(at)));
    mirror.scrollLeft = box.scrollLeft;
    gateApply(box);
  }

  function paintAll() { tokenBoxes().forEach(paintBox); }

  // Apply is dead until the line differs from the one that is applied. The
  // input's own default value is that line, straight from the server, so
  // there is nothing to remember here. A meta box has no button of its own —
  // its form's Save is gated the same way, by gate()
  function gateApply(box) {
    const bar = filterBar();
    if (!bar || !bar.contains(box)) return;
    const apply = bar.querySelector(".apply");
    if (apply) apply.disabled = box.value === box.defaultValue;
  }

  function typingToken(box) {
    if (!box || document.activeElement !== box) return null;
    const before = box.value.slice(0, box.selectionStart);
    const m = TYPING_RE.exec(before) || TYPING_DATE_RE.exec(before);
    if (!m) return null;
    return { sigil: m[2], prefix: m[3], start: before.length - m[2].length - m[3].length };
  }

  // What a half-typed token may still become: the remembered names after an @
  // or a #, and the date words after `due:` or `snooze:`. A word is exactly as
  // hard to remember as a name is, and the panel it is written down in is
  // behind ? — the box already knows the list, so it may as well say it.
  //
  // The names are sorted because that list is a lookup; the dates are left in
  // the order they are written down in, because theirs is an order everybody
  // already knows and alphabetical would open with Friday.
  function poolFor(box, t) {
    if (t.sigil === "@" || t.sigil === "#") return knownNames(box, t.sigil).slice().sort();
    const spec = rulesFor(box).dates[t.sigil.slice(0, -1)];
    if (!spec) return [];
    const words = spec.words.slice();
    // a number is a count of days, and the unit is the only part of it left to
    // say. It goes first because it is what was already being typed
    const digits = /^\d+$/.test(t.prefix);
    const zero = digits && Number(t.prefix) === 0;
    if (spec.days && digits && !(spec.ahead && zero)) words.unshift(t.prefix + "days");
    return words;
  }

  function suggestionsFor(box, t) {
    const pool = poolFor(box, t);
    const p = t.prefix.toLowerCase();
    const starts = pool.filter(function (n) { return n.toLowerCase().indexOf(p) === 0; });
    const holds = pool.filter(function (n) { return n.toLowerCase().indexOf(p) > 0; });
    // nine rather than eight, because the date list is nine long and cutting
    // Sunday off the end of it would cost more than one more row does
    return starts.concat(holds).slice(0, 9);
  }

  function showSuggest(box) {
    const list = suggestList(box);
    if (!list) return;
    const t = typingToken(box);
    const names = t ? suggestionsFor(box, t) : [];
    if (!names.length) { hideSuggest(); return; }
    list.textContent = "";
    names.forEach(function (n, i) {
      const li = document.createElement("li");
      li.textContent = t.sigil + n;
      li.dataset.name = n;
      if (i === 0) li.className = "on";
      list.appendChild(li);
    });
    list.hidden = false;
    renderKeybar();
  }

  function hideSuggest() {
    const list = openSuggest();
    if (!list) return;
    list.hidden = true;
    renderKeybar();
  }

  function moveSuggest(delta) {
    const list = openSuggest();
    if (!list) return;
    const all = Array.from(list.children);
    const at = all.findIndex(function (li) { return li.classList.contains("on"); });
    const next = Math.max(0, Math.min(all.length - 1, (at < 0 ? 0 : at) + delta));
    all.forEach(function (li) { li.classList.remove("on"); });
    all[next].classList.add("on");
    all[next].scrollIntoView({ block: "nearest" });
  }

  // taking a name writes it where the half-typed one was, and leaves a space:
  // a line is a list of names and the next one is usually coming
  function takeSuggest(box, name) {
    const t = typingToken(box);
    if (!t || !box) return;
    const head = box.value.slice(0, t.start) + t.sigil + name + " ";
    box.value = head + box.value.slice(box.selectionStart);
    box.setSelectionRange(head.length, head.length);
    hideSuggest();
    paintBox(box);
  }

  function openFilter() {
    const bar = filterBar();
    if (!bar) return;
    bar.hidden = false;
    try { sessionStorage.setItem(FILTER_OPEN, viewKey()); } catch (err) { /* fine */ }
    const box = filterBox();
    box.focus();
    box.setSelectionRange(box.value.length, box.value.length);
    paintBox(box);
    renderKeybar();
  }

  // Closing takes the filters with it, because a box you cannot see must not
  // be narrowing the list behind it. Nothing to clear is no round trip.
  function closeFilter() {
    const bar = filterBar();
    if (!bar) return;
    try { sessionStorage.removeItem(FILTER_OPEN); } catch (err) { /* fine */ }
    hideSuggest();
    if (filterBox().defaultValue === "") {
      bar.hidden = true;
      renderKeybar();
      return;
    }
    window.location.href = bar.getAttribute("action") + "?f=1";
  }

  function restoreFilter() {
    const bar = filterBar();
    if (!bar) return;
    let open = null;
    try { open = sessionStorage.getItem(FILTER_OPEN); } catch (err) { /* fine */ }
    if (open && open === viewKey()) bar.hidden = false;
  }

  function applyFilter() {
    const bar = filterBar();
    if (!bar) return;
    hideSuggest();
    const bad = problemsIn(filterBox());
    if (bad.length) { askAbout(filterBox(), bad[0], applyFilter); return; }
    bar.submit();
  }

  // How near two names are, for "did you mean". Plain edit distance, because
  // what it is up against is a typo — a letter dropped, doubled or swapped.
  function editDistance(a, b) {
    const prev = [];
    for (let j = 0; j <= b.length; j++) prev[j] = j;
    for (let i = 1; i <= a.length; i++) {
      let last = prev[0];
      prev[0] = i;
      for (let j = 1; j <= b.length; j++) {
        const keep = prev[j];
        prev[j] = Math.min(prev[j] + 1, prev[j - 1] + 1, last + (a[i - 1] === b[j - 1] ? 0 : 1));
        last = keep;
      }
    }
    return prev[b.length];
  }

  // At most three, and only ones near enough to be worth offering: a list of
  // every name would be the remembered list, which is not what was asked
  function nearest(name, pool) {
    const limit = Math.max(2, Math.floor(name.length / 2));
    return pool.map(function (n) { return [n, editDistance(name.toLowerCase(), n.toLowerCase())]; })
      .filter(function (p) { return p[1] <= limit; })
      .sort(function (x, y) { return x[1] - y[1]; })
      .slice(0, 3)
      .map(function (p) { return p[0]; });
  }

  function unknownDialog() { return document.getElementById("unknown-dialog"); }

  // what the question interrupted, to be picked up again once it is answered
  let askContinue = null;

  // Rewrites the offending token — to another name, or to nothing — and picks
  // up where it left off: the next problem, or the thing that was interrupted.
  function resolveToken(box, problem, text) {
    const v = box.value;
    let out = v.slice(0, problem.start) + text + v.slice(problem.end);
    out = out.replace(/[ \t]{2,}/g, " ").replace(/^\s+/, "");
    box.value = out;
    paintBox(box);
    const form = box.closest("form");
    if (form) gate(form);
    const dlg = unknownDialog();
    if (dlg && dlg.open) dlg.close();
    const left = problemsIn(box);
    if (left.length) { askAbout(box, left[0], askContinue); return; }
    // whatever the question interrupted happens now that it is answered:
    // pressing Apply or Save once should not have to be done twice
    const go = askContinue;
    askContinue = null;
    box.focus();
    renderKeybar();
    if (go) go();
  }

  // Learning a name asks the server and stays where it is: the line may be
  // sitting in a form full of unsaved edits, and going to Settings and back
  // would throw them away. The write is still the server's, and the page's
  // copy of the list is updated from what it accepted.
  function createName(box, problem) {
    const kind = problem.sigil === "@" ? "contexts" : "tags";
    const body = new URLSearchParams({ name: problem.name });
    if (problem.param) body.set("param", problem.param);
    fetch("/settings/" + kind + "/add", {
      method: "POST",
      headers: { "Accept": "application/json", "Content-Type": "application/x-www-form-urlencoded" },
      body: body.toString(),
    }).then(function (res) {
      if (!res.ok) return res.text().then(function (t) { throw new Error(t.trim()); });
      const pane = document.querySelector(".pane");
      const key = kind === "contexts" ? "contexts" : "tags";
      pane.dataset[key] = (pane.dataset[key] || "") + problem.name + " ";
      resolveToken(box, problem, problem.text);
    }).catch(function (err) {
      const title = unknownDialog().querySelector("#unknown-title");
      title.textContent = String(err.message || err);
    });
  }

  // A box may say a problem in its own words where the general ones would be
  // wrong or unhelpful: what is missing on a project is not what is missing on
  // a view that simply does not filter by it.
  const BOX_SAYS = {
    project: {
      "no-context": function (p) { return "a project has no context; " + p.text + " belongs on an action under it"; },
    },
    "filter-tags": {
      "no-context": function (p) { return "this view filters by tag and by name; " + p.text + " has nothing to narrow here"; },
      "not-here": function (p) { return "this view filters by tag and by name; " + p.text + " has nothing to narrow here"; },
    },
    "filter-due": {
      "no-context": function (p) { return "this view filters by when something is due, by tag and by name; " + p.text + " has nothing to narrow here"; },
      "not-here": function (p) { return "this view filters by when something is due, by tag and by name; " + p.text + " has nothing to narrow here"; },
    },
    // said for two views now, so it says what is true of both: a someday item
    // is a raw capture and a schedule is text and a rule, and neither carries
    // a name of any kind
    "filter-name": {
      "no-context": function (p) { return "nothing here carries a context or a tag; this view filters by text only"; },
      "not-here": function (p) { return "nothing here carries a context or a tag; this view filters by text only"; },
      "tag": function (p) { return "nothing here carries a context or a tag; this view filters by text only"; },
    },
  };

  function problemText(box, problem) {
    const mine = BOX_SAYS[box.dataset.tokenbox];
    const say = (mine && mine[problem.kind]) || PROBLEM_TEXT[problem.kind];
    return say(problem);
  }

  const PROBLEM_TEXT = {
    "context": function (p) { return p.text + " is not a context the app knows"; },
    "tag": function (p) { return p.text + " is not a tag the app knows"; },
    "second-context": function (p) { return "an action has one context, and " + p.text + " is the second one asked for"; },
    "no-context": function (p) { return p.text + " is not something this line can say"; },
    "not-here": function (p) { return p.text + " is not something this line can say"; },
    "bad-date": function (p) {
      const k = p.name;
      return p.text + " is not a date — write it as " + k + ":2026-09-20, " + k + ":tomorrow, " + k + ":friday or " + k + ":3days";
    },
    "not-ahead": function (p) { return p.text + " names today, which is not a snooze — leave the snooze off instead"; },
    "bad-window": function (p) { return p.text + " is not one of " + p.words.join(", "); },
    "prose": function (p) { return "\u201c" + p.text + "\u201d is not notation — a name, or prose that belongs in the description"; },
  };

  function askAbout(box, problem, then) {
    const dlg = unknownDialog();
    if (!dlg) return;
    askContinue = then || null;
    dlg.querySelector("#unknown-title").textContent = problemText(box, problem);
    const choices = dlg.querySelector(".choices");
    choices.textContent = "";

    const known = problem.kind === "context" || problem.kind === "tag";
    const near = known ? nearest(problem.name, knownNames(box, problem.sigil)) : [];
    near.forEach(function (name, i) {
      const b = document.createElement("button");
      b.type = "button";
      b.dataset.key = String(i + 1);
      b.dataset.keyLabel = problem.sigil + name;
      b.setAttribute("data-kb-row", "");
      b.textContent = problem.sigil + name;
      const why = document.createElement("span");
      why.className = "why";
      why.textContent = "use this one";
      b.appendChild(why);
      b.addEventListener("click", function () {
        resolveToken(box, problem, problem.sigil + name + (problem.param ? "(" + problem.param + ")" : ""));
      });
      choices.appendChild(b);
    });

    const drop = document.createElement("button");
    drop.type = "button";
    drop.dataset.key = "r";
    drop.dataset.keyLabel = "remove it";
    drop.setAttribute("data-kb-row", "");
    drop.textContent = "Take " + problem.text + " out of the line";
    drop.addEventListener("click", function () { resolveToken(box, problem, ""); });
    choices.appendChild(drop);

    // Creating is only ever offered for a name that is simply not there yet,
    // and it goes through the same endpoint the Settings screen uses — one
    // place learns a name (see implementation.md, "The remembered lists").
    const make = dlg.querySelector("[data-create-name]");
    make.hidden = !known;
    if (!known) { delete make.dataset.key; make.removeAttribute("data-kb-row"); }
    else {
      make.textContent = "Create " + problem.text;
      make.dataset.key = "n";
      make.dataset.keyLabel = "create it";
      make.setAttribute("data-kb-row", "");
      make.onclick = function () { createName(box, problem); };
    }
    dlg.showModal();
    select(rows()[0]);
    renderKeybar();
  }

  // While a dialog is open its rows are the only rows, for the same reason its
  // keys are the only keys (see keyLive): j and k move through what is in
  // front of you, and the list behind a dialog is not that. The list keeps its
  // own selection while the dialog is up, because nothing here touches it.
  function rowScope() {
    return topDialog() || document;
  }

  function rows() {
    return Array.from(rowScope().querySelectorAll("[data-kb-row]"));
  }

  function selected() {
    return rowScope().querySelector("[data-kb-row].kb-selected");
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

  // Acting on a row must not cost the selection. Every row key posts a form
  // and the answer is a whole new page — boosted or not, the list is rebuilt
  // and the class marking the selection goes with the old one — so the row is
  // handed to the next page and claimed once, on arrival. Without this, `t`
  // and `c` each ended by dropping the cursor, and working down a list from
  // the keyboard meant pressing j back to where you already were.
  const HANDOVER = "kb-row-handover";

  // Which screen this is, read off the page rather than off the address bar:
  // on a boosted post the new page is in the DOM before htmx has finished with
  // the URL, and the answer has to be about the page that is actually on
  // screen. The pane carries it, and not the nav, because the nav is a panel
  // and can be off (implementation.md, "Panels").
  function viewKey() {
    const pane = document.querySelector(".pane[data-view]");
    return pane ? pane.dataset.view || location.pathname : location.pathname;
  }

  function handSelectionOn(row) {
    if (!row) return;
    try {
      sessionStorage.setItem(HANDOVER, JSON.stringify({
        view: viewKey(),
        href: row.dataset.href || "",
        i: rows().indexOf(row),
      }));
    } catch (err) { /* no session storage: the selection is lost, nothing else is */ }
  }

  function claimSelection() {
    let raw = null;
    try { raw = sessionStorage.getItem(HANDOVER); } catch (err) { return; }
    if (!raw || selected()) return;
    // a page with no rows cannot claim it, and must not swallow it either:
    // doing is a screen you go to from a row and come straight back to it, and
    // the row is expected to be where it was (design.md, "Doing one action")
    if (!rows().length) return;
    try { sessionStorage.removeItem(HANDOVER); } catch (err) { /* nothing to undo */ }
    let want;
    try { want = JSON.parse(raw); } catch (err) { return; }
    // only on the screen it was handed from. Completing an action can answer
    // with the project page instead of the list, and a row position means
    // nothing there — a selection restored onto a different screen would be
    // the app choosing an item nobody pointed at
    if (!want || want.view !== viewKey()) return;
    const all = rows();
    let row = want.href && all.find(function (r) { return r.dataset.href === want.href; });
    // the row can be gone, which is what completing one does. Then the
    // selection belongs where it stood: the item that took its place is under
    // the cursor and the list can be worked straight down without touching j
    if (!row && typeof want.i === "number" && want.i >= 0) row = all[Math.min(want.i, all.length - 1)];
    if (row) select(row);
  }

  function submitIn(row, cls) {
    const form = row.querySelector("form." + cls);
    if (!form) return;
    handSelectionOn(row);
    form.submit();
  }

  // the same for the mouse: the pick dot and the checkbox are buttons inside
  // the row, and pressing one of them is the same operation the key performs.
  // Only for the row that is already selected, though — clicking the dot on
  // some other row is not a way of pointing at it, and handing it the cursor
  // would be the app selecting something nobody selected. A native
  // form.submit() fires no submit event, which is why submitIn hands the row
  // on itself rather than relying on this
  document.addEventListener("submit", function (e) {
    // the filter line is asked about however it is submitted — the button is
    // the mouse's Enter, and both have to stop at a name the app cannot use
    const boxes = e.target.querySelectorAll ? e.target.querySelectorAll("[data-tokenbox]") : [];
    for (let i = 0; i < boxes.length; i++) {
      const bad = problemsIn(boxes[i]);
      // stopped in the capture phase, or htmx's own submit handler would send
      // the line anyway: preventDefault stops the browser, not another listener
      if (bad.length) {
        e.preventDefault();
        e.stopPropagation();
        const form = e.target;
        askAbout(boxes[i], bad[0], function () {
          if (form.requestSubmit) form.requestSubmit(); else form.submit();
        });
        return;
      }
    }
    if (e.target === filterBar()) return;
    const row = e.target.closest && e.target.closest("[data-kb-row]");
    if (row && row === selected()) handSelectionOn(row);
  }, true);

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
    // ctrl-v is the panels, from anywhere: the chooser if it is shut, and zen
    // if it is already up — the second press is the answer wanted most often,
    // and the dialog is a list of four keys rather than a place to be. Ctrl
    // and not cmd, because cmd-v is paste in every box on this machine and a
    // key of the app's must not take that away.
    if (e.ctrlKey && !e.metaKey && !e.altKey && e.key.toLowerCase() === "v") {
      const d = panelsDialog();
      if (!d) return;
      e.preventDefault();
      if (!d.open) { d.showModal(); renderKeybar(); return; }
      const zen = d.querySelector('[data-key="z"]');
      if (zen) press(zen);
      return;
    }

    // ctrl-f is the filter line, on the views that have one: it is the search
    // key every other program uses, and what this app has to search is its own
    // list rather than the page. Pressed again it closes the box and takes the
    // filters with it — a filter you cannot see is one you cannot undo.
    if (e.ctrlKey && !e.metaKey && !e.altKey && e.key.toLowerCase() === "f") {
      const bar = filterBar();
      if (!bar) return; // a view with no box leaves ctrl-f to the browser
      e.preventDefault();
      if (bar.hidden) openFilter(); else closeFilter();
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
    const unknown = unknownDialog();
    if (unknown && unknown.open && topDialog() === unknown) {
      if (e.key === "Escape") { e.preventDefault(); unknown.close(); renderKeybar(); return; }
      // the answers are a list, and a list is moved through with j and k. The
      // letters on them stay: a key that goes straight to an answer is worth
      // having on a question asked this often, and neither costs the other
      if (e.key === "j" || e.key === "k") { e.preventDefault(); move(e.key === "j" ? 1 : -1); return; }
      if (e.key === "Enter") {
        const row = selected();
        if (row) { e.preventDefault(); row.click(); }
        return;
      }
      const pick = branchFor(e);
      if (pick) { e.preventDefault(); press(pick); return; }
      if (!e.ctrlKey && !e.metaKey && !e.altKey) e.preventDefault();
      return;
    }
    const panels = panelsDialog();
    if (panels && panels.open) {
      // the four keys are the dialog's own declared controls, so pressing one
      // is pressing it; everything else is swallowed while it is up
      if (e.key === "Escape") { e.preventDefault(); panels.close(); renderKeybar(); return; }
      const choice = branchFor(e);
      if (choice) { e.preventDefault(); press(choice); return; }
      if (!e.ctrlKey && !e.metaKey && !e.altKey) e.preventDefault();
      return;
    }
    if (e.target.matches && e.target.matches("[data-tokenbox]")) {
      const box = e.target;
      const list = suggestList(box);
      const open = list && !list.hidden;
      if (e.key === "ArrowDown" || e.key === "ArrowUp") {
        e.preventDefault();
        if (!open) showSuggest(box); else moveSuggest(e.key === "ArrowDown" ? 1 : -1);
        return;
      }
      if (open && (e.key === "Enter" || e.key === "Tab")) {
        const on = list.querySelector("li.on");
        if (on) { e.preventDefault(); takeSuggest(box, on.dataset.name); return; }
      }
      // enter applies the filter line; on a meta line it submits the form it
      // is in, which is the browser's own answer and is checked on the way
      // out like any other submit
      if (e.key === "Enter" && box === filterBox()) { e.preventDefault(); applyFilter(); return; }
      // esc unwinds one step at a time, the way the project picker does: the
      // list first, then the box. It never closes the filter box — that is
      // ctrl-f, and it would take the filters with it
      if (e.key === "Escape" && open) { e.preventDefault(); hideSuggest(); return; }
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
        if (canDo(row)) {
          e.preventDefault();
          // leaving doing puts you back on this row, because you were on it
          handSelectionOn(row);
          window.location.href = doingHref(row);
        }
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
    if (e.target.closest("[data-timer]")) { e.preventDefault(); toggleTimer(); return; }
    const pick = e.target.closest(".fsuggest li");
    if (pick) {
      e.preventDefault();
      const box = pick.closest(".fbox").querySelector("[data-tokenbox]");
      box.focus();
      takeSuggest(box, pick.dataset.name);
      return;
    }
    if (!e.target.closest(".fbox")) hideSuggest();
    // clearing a whole filter at once: unchecking them one at a time is one
    // page load each, and the form's own change handler does not fire for a
    // box unchecked from here, so the submit is explicit
    const clear = e.target.closest("[data-clear-tags]");
    if (clear) {
      e.preventDefault();
      const form = clear.closest("form");
      if (!form) return;
      form.querySelectorAll("input[name=tag]:checked").forEach(function (box) { box.checked = false; });
      if (form.requestSubmit) form.requestSubmit(); else form.submit();
      return;
    }
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
        // the meta line is asked about here too: this dialog is confirmed by a
        // button and not by a submit, so it has to make the check itself
        const bad = problemsIn(meta);
        if (bad.length) { askAbout(meta, bad[0], function () { done(true); }); return; }
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
    // the old page's timer is counting for a screen that is no longer here
    startTimer();
    restoreFilter();
    paintAll();
    renderKeybar(); setupPickers(); gateAll();
    claimSelection();
  });

  // A refused post must never be silent. htmx does not swap a 4xx, so a
  // handler that answers with a plain 400 leaves the screen exactly as it was
  // — the press looks like it did nothing at all, which is how an invalid
  // schedule rule read as a broken Create button. A screen that refuses on
  // purpose renders itself back with the reason (see the schedule forms);
  // this is the net under everything that has not been given that treatment,
  // and it says the server's own words rather than inventing any.
  document.addEventListener("htmx:responseError", function (e) {
    const pane = document.querySelector(".pane");
    if (!pane) return;
    const xhr = e.detail && e.detail.xhr;
    const said = xhr && xhr.responseText ? xhr.responseText.trim() : "";
    const p = document.createElement("p");
    p.className = "error-banner";
    p.dataset.transient = "";
    p.textContent = said.split("\n")[0] || "That was refused.";
    const old = pane.querySelector(".error-banner[data-transient]");
    if (old) old.remove();
    pane.insertBefore(p, pane.firstChild);
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

  startTimer();
  restoreFilter();
  paintAll();
  renderKeybar();
  claimSelection();

  // filter forms apply themselves on any change
  document.addEventListener("change", function (e) {
    const form = e.target.closest("form[data-autosubmit]");
    if (form) form.submit();
  });
})();
