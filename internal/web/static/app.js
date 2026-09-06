// The keyboard layer. Everything it triggers is a plain link or form that
// exists on the page — the server stays the single source of truth.
(function () {
  "use strict";

  let gPending = false;

  const jumps = {
    i: "/inbox", t: "/today", n: "/next", p: "/projects", k: "/tasks",
    w: "/waiting", c: "/calendar", s: "/someday", h: "/scheduler",
    r: "/review", a: "/archive", u: "/audit", e: "/settings",
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
    const nav = document.querySelector("nav");
    if (nav) nav.classList.toggle("gpending", on);
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
    return keys;
  }

  // Every entry is derived from what is actually on the page and what is
  // actually selected, so the bar can only ever offer a key that will do
  // something. A mode fills the view group and empties the global one:
  // while a dialog or an overlay is up, none of the global keys are live.
  function keybarGroups() {
    const dlg = captureDialog();
    if (dlg && dlg.open) return { view: [["\u21b5", "add"], ["esc", "cancel"]], global: [] };
    const help = document.getElementById("help");
    if (help && !help.hidden) return { view: [["esc", "close help"]], global: [] };
    if (gPending) return { view: [["\u2026", "press a marked key"], ["esc", "cancel"]], global: [] };

    const view = [];
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
    return Array.from(document.querySelectorAll("[data-key]")).map(function (el) {
      return [el.dataset.key, el.dataset.keyLabel || ""];
    });
  }

  function branchFor(key) {
    if (key.length !== 1) return null;
    return document.querySelector('[data-key="' + CSS.escape(key) + '"]');
  }

  // Pressing the key does exactly what clicking the control does: submit the
  // form, or follow the link. Nothing here knows what a branch means.
  function press(el) {
    if (el.tagName === "FORM") {
      if (el.requestSubmit) el.requestSubmit(); else el.submit();
      return;
    }
    const href = el.getAttribute("href");
    if (href) window.location.href = href;
  }

  function pushRowKeys(into, row) {
    if (!row) return;
    // opening an inbox item is processing it, so the two keys share a label
    if (row.hasAttribute("data-process")) into.push(["\u21b5 p", "process"]);
    else if (row.dataset.href) into.push(["\u21b5", "open"]);
    else if (row.querySelector("input[type=radio]")) into.push(["\u21b5", "pick"]);
    if (row.querySelector("form.kb-complete")) into.push(["c", "done"]);
    if (row.querySelector("form.kb-pick")) into.push(["t", "today"]);
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
    const branch = branchFor(e.key);
    if (branch) { e.preventDefault(); press(branch); return; }

    const row = selected();
    switch (e.key) {
      case "g": e.preventDefault(); setPending(true); break;
      case "j": e.preventDefault(); move(1); break;
      case "k": e.preventDefault(); move(-1); break;
      case "Enter":
      case "o": {
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
    const row = e.target.closest && e.target.closest("[data-kb-row]");
    if (row && e.target.type === "radio" && !row.classList.contains("kb-selected")) select(row);
  });

  // a click or a lost window abandons a half-typed "g" sequence
  document.addEventListener("click", function (e) {
    setPending(false);
    if (e.target.closest("[data-capture-open]")) { e.preventDefault(); openCapture(); return; }
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

  // hx-boost swaps the body, taking the rendered bar with it
  document.addEventListener("htmx:afterSwap", renderKeybar);

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
