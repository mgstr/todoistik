// The keyboard layer. Everything it triggers is a plain link or form that
// exists on the page — the server stays the single source of truth.
(function () {
  "use strict";

  let gPending = false;

  const jumps = {
    i: "/inbox", t: "/today", n: "/next", p: "/projects", k: "/tasks",
    w: "/waiting", c: "/calendar", s: "/someday", h: "/scheduler",
    r: "/review", a: "/archive", u: "/audit", d: "/dashboard", e: "/settings",
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

  // ctrl-m is g one level in: g goes to a view, ctrl-m goes to a control on
  // the screen already open. Ctrl, because the whole point is reaching another
  // box without leaving the one the hands are in — a bare letter could not,
  // since it would be typed into the box instead. It was ctrl-j until ctrl-j
  // and ctrl-k became the list's own keys (see moveKey), and ctrl-n until the
  // letters made the commonest jump ctrl-n m — the meta line is always m, so
  // the prefix may as well be the same key (implementation.md).
  let jPending = false;
  let jumpMap = {};

  const JUMPABLE = "input, textarea, select, button, .button, ul.list";

  // What a jump does when it arrives, which is whatever that thing is for: a
  // box is written in, a button is pressed, and a list is moved through.
  function jumpKind(el) {
    if (el.matches("ul.list")) return "list";
    if (el.tagName === "BUTTON" || el.classList.contains("button")) return "press";
    return "focus";
  }

  // Everything on this screen a jump can usefully arrive at. The rail is not
  // here — that is g's — and neither are a list row's own controls: the row
  // keys already reach them, and marking them would put eighteen hints on a
  // nine-item list. The list itself is one destination instead.
  function jumpTargets() {
    const scope = topDialog() || document;
    // a dialog that declares its own keys is a menu of them, not a form: its
    // letters already mean something, and a jump would be a second answer
    if (scope !== document && scope.querySelector("[data-key]")) return [];
    const out = [];
    scope.querySelectorAll(JUMPABLE).forEach(function (el) {
      if (el.closest("nav")) return;
      if (jumpKind(el) === "list") {
        // an empty list has no row to land on, so it is not a place to go
        if (!el.querySelector("[data-kb-row]")) return;
      } else {
        if (el.closest("[data-kb-row]")) return;
        // a control that cannot be pressed is not a destination — the same
        // rule the key bar follows. tabindex="-1" is how a box that is shown
        // rather than filled in says so (the project on an action's page)
        if (el.disabled || el.tabIndex < 0 || el.type === "hidden") return;
      }
      if (!el.getClientRects().length) return;
      out.push(el);
    });
    return out;
  }

  // What the screen calls this thing: the heading over a list, the name beside
  // a box, a button's own words. Empty where the screen names it nothing —
  // which is answered with a number rather than a letter, since a letter that
  // stands for nothing is not the guess the letters are for.
  function jumpName(el) {
    const kind = jumpKind(el);
    if (kind === "list") {
      for (let p = el.previousElementSibling; p; p = p.previousElementSibling) {
        if (/^H[1-3]$/.test(p.tagName)) return p.textContent.trim();
      }
      return "";
    }
    const label = el.closest("label");
    if (label) {
      // the name beside the box, then whatever else the label says: a tag chip
      // puts its word after its checkbox rather than in front of it
      const lb = label.querySelector(".lb") || label.childNodes[0];
      const word = lb ? (lb.textContent || "") : "";
      if (word.trim()) return word.trim();
      if ((label.textContent || "").trim()) return label.textContent.trim();
    }
    // a button's own words, but never a select's: its text is every option it
    // holds. Never a box's value either — a box named by what is typed into it
    // would change letter as it was typed into
    if (kind === "press" && (el.textContent || "").trim()) return el.textContent.trim();
    return (el.getAttribute("aria-label") || "").trim();
  }

  // The letter is the first of the control's own name, which is what makes it
  // guessable — and where two names start alike the first one on the screen
  // takes it and the second falls to its next free letter. Every letter shown
  // therefore goes somewhere, which is the same promise the key bar makes: a
  // key is never advertised without working.
  function assignJumpKeys(els) {
    const used = {}, out = [];
    let digit = 0;
    // Boxes and buttons choose first, lists after them. A list is named by the
    // heading over it, and on a project's page "Actions" and "Add an action"
    // both want `a` — the button is pressed far more often than the list is
    // stepped into, and the list has j/k reaching it anyway.
    const isList = function (el) { return jumpKind(el) === "list"; };
    const ordered = els.filter(function (el) { return !isList(el); }).concat(els.filter(isList));
    // A control may name its own letter, and a declared one is claimed before
    // any computed letter can take it. Without this the six buttons that have
    // no key of their own move under the hints: Delete was `l` inside a
    // project and `e` standing alone, because Detach left the screen and
    // every letter after it shifted up (keys.md, "Buttons that get no
    // letter").
    ordered.forEach(function (el) {
      const want = el.dataset.jump;
      if (want && !used[want]) { used[want] = true; out.push([want, el]); }
    });
    ordered.forEach(function (el) {
      if (el.dataset.jump && used[el.dataset.jump] &&
          out.some(function (p) { return p[1] === el; })) return;
      const name = jumpName(el).toLowerCase();
      let key = null;
      for (let i = 0; i < name.length; i++) {
        const c = name.charAt(i);
        if (c >= "a" && c <= "z" && !used[c]) { key = c; break; }
      }
      // nothing on the screen names it, or every letter of the name is spoken
      // for: it is numbered instead, in the order it is read in
      if (!key) {
        while (digit < 10 && used[String(digit)]) digit++;
        if (digit > 9) return;
        key = String(digit);
      }
      used[key] = true;
      out.push([key, el]);
    });
    return out;
  }

  // The hint is placed from the control's own rectangle rather than hung
  // inside it: a text box has nowhere to put a child, and half of what these
  // mark are buttons.
  function showJumpHints() {
    assignJumpKeys(jumpTargets()).forEach(function (pair) {
      const key = pair[0], el = pair[1], r = el.getBoundingClientRect();
      jumpMap[key] = el;
      const hint = document.createElement("span");
      hint.className = "jhint";
      hint.textContent = key;
      hint.setAttribute("aria-hidden", "true");
      hint.style.left = r.left + "px";
      // centred on a one-line control, on the first line of a taller one: a
      // note box that has grown is still written from the top down, and a
      // letter floating at its middle reads as marking the line it is beside
      hint.style.top = (r.top + Math.min(r.height / 2, 16)) + "px";
      document.body.appendChild(hint);
    });
    return Object.keys(jumpMap).length;
  }

  function setJumping(on) {
    document.querySelectorAll(".jhint").forEach(function (h) { h.remove(); });
    jumpMap = {};
    // a screen with nothing to jump to leaves the key to the browser rather
    // than lighting up an empty page
    jPending = on ? showJumpHints() > 0 : false;
    renderKeybar();
    return jPending;
  }

  // Arriving. A button is pressed rather than focused — a jump to a control
  // that then has to be pressed a second time is two keys for what the key bar
  // does in one, and every one of these is a control the screen was going to
  // act on anyway. A list is arrived at by selecting its first row, which is
  // what puts the row keys in reach. A box is focused at the end of what is
  // already in it: the first thing typed after a jump is meant to follow the
  // text, not to land in front of it — the same rule caretToEnd applies to a
  // field that opens focused.
  function jumpTo(el) {
    const kind = jumpKind(el);
    if (kind === "press") { press(el); return; }
    if (kind === "list") {
      // whatever had the focus gives it up first — arriving at a list with the
      // caret still in the filter box means j and k are typed into the box
      // rather than moving the selection, which is the jump not having
      // happened at all
      if (document.activeElement && document.activeElement.blur) document.activeElement.blur();
      const row = el.querySelector("[data-kb-row]");
      if (row) select(row);
      renderKeybar();
      return;
    }
    el.focus();
    if (typeof el.selectionStart === "number") {
      el.setSelectionRange(el.value.length, el.value.length);
    }
    renderKeybar();
  }

  // The capture dialog. A native <dialog> so the centring, the backdrop, the
  // focus trap and Escape-to-cancel all come from the browser. Enter submits
  // to /capture, which drops a text already sitting in the inbox without
  // complaint — the item is there, which is what matters.
  function captureDialog() {
    return document.getElementById("capture-dialog");
  }

  function captureBox(d) { return d.querySelector("[name=text]"); }

  function openCapture() {
    const d = captureDialog();
    if (!d || d.open) return;
    const box = captureBox(d);
    if (box) box.value = "";
    d.showModal();
    // the box is a textarea that is one line tall until something puts a
    // second line in it, so an emptied one has to be measured again — a box
    // inside a closed dialog measures 0 (see growAll)
    growAll(d);
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
    const box = captureBox(d);
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
    const keys = [["g g", "add to inbox"], ["g", "go to"]];
    // offered only where there is something to jump to, so a list view with
    // no form on it does not advertise a key that would light up nothing
    // `^m` arms a second key rather than pressing a control, so it is one of
    // the entries the bar lists without offering — a prefix is not a button
    if (jumpTargets().length) keys.push(["^m", "jump"]);
    if (document.getElementById("help")) keys.push(["?", "help", function () { toggleHelp(); }]);
    // the bookmarked filters, offered by the same rule as everything else
    // here: the digits keep a filter only where there is one to keep, and go
    // to one only where one is kept. The nine are nine keys and not a control,
    // so that entry says how to steer and presses nothing; `^0` opens one
    // thing and is therefore a button like the rest of them
    if (filterBar()) {
      const line = liveFilter();
      if (line) keys.push(["^1\u20269", "bookmark this filter"]);
      else if (anyBookmark()) keys.push(["^1\u20269", "go to a bookmark"]);
      keys.push(["^0", "bookmarks", function () { openBookmarks(); }]);
    }
    // a key marked data-global belongs to the app rather than to this view, so
    // it is read here and lands in the right half of the bar. Last, so a flag
    // whose label changes sits in the corner and does not shift the keys
    // beside it when it does
    declaredKeys("[data-key][data-global]").forEach(function (k) { keys.push(k); });
    return keys;
  }

  // Whether the caret is in something that swallows a bare letter. While a
  // box is being typed in, a bare letter is a character and not a command, so
  // every key that is one is dead — and the bar's whole promise is that what
  // it lists works right now.
  function typingNow() {
    const el = document.activeElement;
    return !!(el && el.closest && typing({ target: el }));
  }

  // The entries that survive that: a chord, esc, and the "needs …" line,
  // which is a reason rather than a key. Nothing is greyed out or annotated —
  // a key that cannot be pressed is not shown, which is the same rule the bar
  // follows everywhere else, and what the bar does say is then exactly what
  // one press will do.
  function chordOnly(pairs) {
    return pairs.filter(function (p) {
      return p[0].indexOf("^") >= 0 || p[0] === "esc" || p[0] === "\u2026";
    });
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
    const ld = linksDialog();
    if (ld && ld.open && topDialog() === ld) {
      // the links are the answers and each carries its own letter, so the bar
      // is that list — read off the dialog, like every other declared key
      return { view: declaredKeys("[data-key]").concat(
        [["j k", "move"], ["\u21b5", "open"], ["esc", "cancel"]]), global: [] };
    }
    const unknown = unknownDialog();
    if (unknown && unknown.open && topDialog() === unknown) {
      const keys = declaredKeys("[data-key]");
      if (rows().length > 1) keys.push(["j k", "move"]);
      keys.push(["\u21b5", "take"], ["esc", "cancel"]);
      return { view: keys, global: [] };
    }
    const bmk = bookmarksDialog();
    if (bmk && bmk.open) {
      // the same two answers the bar gives outside, said as the keys that are
      // live in here: the digits are bare while the dialog owns the keyboard
      const keys = [];
      const line = liveFilter();
      if (line) keys.push(["1\u20269", "bookmark this filter"]);
      else if (anyBookmark()) keys.push(["1\u20269", "go to a bookmark"]);
      const row = selected();
      if (rows().length > 1) keys.push(["j k", "move"]);
      if (row && (line || row.dataset.line)) keys.push(["\u21b5", line ? "bookmark here" : "go"]);
      if (row && row.dataset.line) keys.push(["\u232b", "clear"]);
      keys.push(["esc", "close"]);
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
    if (openSuggest()) {
      return { view: [["\u2193\u2191", "move"], ["\u21b5", "take"], ["esc", "back"]], global: [] };
    }
    if (filterBox() && document.activeElement === filterBox()) {
      // Enter is only a key here while there is something to ask about: the
      // list follows the rest of the line by itself
      const keys = [];
      const bad = problemsIn(filterBox());
      if (bad.length) keys.push(["\u21b5", "ask about " + bad[0].text]);
      if (createForm()) keys.push(["^\u21b5", "create"]);
      if (rows().length) keys.push(["^j ^k", "to the list"]);
      keys.push(["^f", "no filter"], ["esc", "leave the box"]);
      return { view: keys, global: globalKeys() };
    }
    if (jPending) {
      return { view: [["\u2026", "press a marked key"], ["esc", "cancel"]], global: [] };
    }
    if (gPending) {
      // z is the one jump with nothing on screen to mark: the processing
      // screen has no nav entry of its own (see implementation.md,
      // "Navigation"), so the bar is where it can be offered — and only while
      // there is an inbox to work down, or it would be a key that does nothing
      const view = [["\u2026", "press a marked key"]];
      if (document.querySelector(".pane[data-inbox-full]")) view.push(["z", "inbox zero"]);
      // the nine have nothing on screen to pin a tag to either \u2014 the dialog
      // they are drawn in is shut \u2014 so the bar carries them, as a range and
      // only where there is one to go to, the shape `^1\u20269` already has
      if (anyBookmark()) view.push(["1\u20269", "a bookmark"]);
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
    if (zero) view.push(["z", "inbox zero", function () { goInboxZero(); }]);
    if (!zero) pushRowKeys(view, row);
    if (!row) pushScreenKeys(view);
    if (rows().length) view.push(["j k", "move"]);
    if (zero) pushRowKeys(view, row);
    if (createForm()) view.push(["^↵", "create", function () { press(createForm()); }]);
    branchKeys().forEach(function (k) { view.push(k); });
    // offered only where the item under the cursor actually holds one, the way
    // every other entry here is derived from the page rather than written down
    if (itemLinks(linkScope()).length) view.push(["^o", "open link", function () { followLink(); }]);
    // last of the view's own answers, and beside the way out — see pushDeleteKey
    pushDeleteKey(view, row);
    const cancel = document.querySelector("[data-cancel]");
    if (cancel) {
      view.push([renderKey("b"),
        cancel.dataset.cancelLabel || "back",
        function () { leave(); }]);
    }
    const bar = filterBar();
    if (bar) {
      view.push(["^f", bar.hidden ? "filter" : "no filter",
        function () { if (filterBar().hidden) openFilter(); else closeFilter(); }]);
    }
    if (bar && !bar.hidden) view.push(["esc", "to the filter"]);
    // With the caret in a box the bar narrows to what a chord can still
    // reach, and says the one key that gets the letters back. This is the
    // mode made visible: in `command` almost everything goes and returns on
    // esc, in `modifier` almost nothing does — which is the difference the
    // two are being tried for (keys.md, "The bar while you are typing").
    if (typingNow()) {
      return {
        view: chordOnly(view).concat([["esc", "leave the box"]]),
        global: chordOnly(globalKeys()),
      };
    }
    return { view: view, global: globalKeys() };
  }

  // A screen can give its own controls keys, by declaring them on the control:
  // data-key is the key, data-key-label what the bar calls it. Both the bar and
  // the handler read the page, so — like the row keys above — a key can only
  // exist here if the thing it presses exists, and it can never be advertised
  // without working. Document order is the bar's order, which lets the template
  // decide how the answers read rather than this file.
  // A list that draws its own keys is summarised in the bar as a range rather
  // than listed key by key: the match list on the processing screen numbers
  // its own rows, which is where those numbers are read, and nine entries
  // saying "copy" would bury the six answers under them. The entry presses
  // nothing — it says how to steer, exactly as the bookmarks' `^1…9` does —
  // and a run of them ends the moment an ordinary key comes between, so the
  // bar can never claim a range that is not one.
  function branchKeys() {
    const out = [];
    let run = null;
    Array.from(document.querySelectorAll("[data-key]:not([data-global])")).filter(keyUsable).forEach(function (el) {
      if (!el.hasAttribute("data-key-quiet")) {
        run = null;
        out.push(keyEntry(el));
        return;
      }
      const k = renderKey(el.dataset.key, el);
      if (run) {
        run[0] = run.first + "…" + k;
        return;
      }
      run = [k, el.dataset.keyLabel || ""];
      run.first = k;
      out.push(run);
    });
    return out;
  }

  // The press is the control itself, through the same press() the key goes
  // through — so the letter and the pointer cannot come to mean different
  // things, which is the reason press() clicks a button rather than submitting
  // the form around it.
  function declaredKeys(sel) {
    return Array.from(document.querySelectorAll(sel)).filter(keyUsable).map(keyEntry);
  }

  // One control as the bar's four fields. Split out of declaredKeys because
  // the bar also walks the page itself, to collapse a numbered list into a
  // range (see branchKeys), and the two must build an entry the same way.
  function keyEntry(el) {
    return [renderKey(el.dataset.key, el), el.dataset.keyLabel || "",
      function () { press(el); }, tone(el)];
  }

  // What the control already says about itself. A style that paints tones
  // reads this rather than a list of which entries are loud, so the bar keeps
  // saying what the button said and the two cannot come to disagree — the same
  // reason the label is read off the control instead of written down here.
  function tone(el) {
    if (el.classList.contains("primary")) return "primary";
    if (el.classList.contains("danger")) return "danger";
    return "";
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

  // The palette the server just said, carried the last step by hand. It is on
  // the document element, because a scheme is a fact about the whole page and
  // `color-scheme` on anything smaller leaves the window's own ground behind
  // it unpainted — and hx-boost swaps the body, so nothing above it arrives
  // with the new page. The pane is where every other answer of the server's is
  // read, so it is where this one is read too: one source of truth, and it is
  // the server's (implementation.md, "Theme").
  function wearTheme() {
    const pane = document.querySelector(".pane");
    if (!pane || !pane.dataset.theme) return;
    document.documentElement.setAttribute("data-theme", pane.dataset.theme);
  }

  // ---- What the settings file said -------------------------------------
  //
  // Read off the pane, like every other answer the file gives
  // (implementation.md, "Settings file"). Both of these are on by default, so
  // a page with no pane at all — the login screen — answers the same as a file
  // that says nothing, and neither of them has a key bar to be wrong about
  // anyway.
  function paneSays(attr) {
    const pane = document.querySelector(".pane");
    return !pane || pane.hasAttribute(attr);
  }

  function anyLayout() { return paneSays("data-keys-any-layout"); }

  // ---- keys.mode ---------------------------------------------------------
  //
  // A control declares its letter and never its modifier (keys.md, "The three
  // modes"); this is the one place that decides how that letter is pressed.
  // A declaration that already carries `^` is a global — `^e`, `^o` — and is
  // not the screen's to begin with, so no mode touches it.
  //
  // What hybrid puts ctrl on is the *control*, not the letter, and the control
  // is the one that knows: data-key-typing marks a key that has to fire with
  // the caret in a box. Keyed on the letter it was wrong the moment two
  // buttons shared one — `a` is Add inside a project form and the Action
  // branch on a screen with nothing to type into, and only the first of those
  // is worth a modifier.
  function keysMode() {
    const pane = document.querySelector(".pane");
    const m = pane && pane.getAttribute("data-keys-mode");
    return m || "hybrid";
  }

  // What a declared key is actually pressed as, here, now. Everything that
  // reads a data-key goes through this — the handler and the bar both — so
  // the two can never disagree about what a screen is offering.
  function renderKey(decl, el) {
    if (!decl || decl.charAt(0) === "^") return decl;
    // A key that is not a letter is itself in all three modes (keys.md, "The
    // map"): there is nothing for a mode to change about it, and modifier
    // mode turning the match list's `3` into `^3` would have spent a digit
    // the bookmarks already answer to.
    if (!/^[a-z]$/.test(decl)) return decl;
    const mode = keysMode();
    if (mode === "command") return decl;
    if (mode === "modifier") return "^" + decl;
    return el && el.hasAttribute("data-key-typing") ? "^" + decl : decl;
  }
  function layoutMarker() { return paneSays("data-keys-layout-marker"); }

  // ---- Which layout the keyboard is in --------------------------------
  //
  // The keys work whatever the layout says, because keyOf below reads the
  // place rather than the letter. What that cannot answer is which layout it
  // is, and the answer is still worth having — not for the keys, but for the
  // boxes: what gets typed into a capture or a description is whatever the
  // layout types, and noticing after the sentence is a line to delete.
  //
  // Cyrillic and nothing else, because it is the one other layout this
  // keyboard is ever in (design.md, "Design principles"): a second script
  // would need a second name and there is no second keyboard to name.
  //
  // One source, and it is the keys. Chrome's keyboard map
  // (navigator.keyboard.getLayoutMap()) looked like the better one — it can
  // answer with nothing pressed — but on this machine it answers *wrong*: it
  // reports the ASCII layout underneath while macOS is in Russian. Polling it
  // put the marker in a fight with the keys it was supposed to agree with, and
  // the marker lost once a second — it lit on each keystroke and went out
  // between them, so typing a sentence in Russian made it blink all the way
  // through. A wrong answer arriving on a timer is worse than no answer, and
  // it is gone.
  //
  // So: the character that arrives, against the key it arrived from — the
  // comparison keyOf already makes, exact in every browser — and then it
  // *stays*. That is the difference between an indicator and a flicker. The
  // one in the menu bar holds its state until the state changes, and so does
  // this. The cost is that it learns on the first key after a switch rather
  // than at the switch itself, because nothing inside a page can see the
  // switch happen.

  const CYRILLIC = /[\u0400-\u04ff]/;
  const LAYOUT = "kb-layout";

  // Not-yet-known and Latin are the same silence. The question is "am I in the
  // other one", and only "yes" has anything to say — a marker that is up all
  // the time is furniture rather than a signal.
  let cyrillic = false;
  try { cyrillic = sessionStorage.getItem(LAYOUT) === "1"; } catch (err) { /* fine */ }

  // Remembered for the tab, because a g-jump is a real page load and the
  // marker must not blink off across one — blinking is the whole thing being
  // fixed here. sessionStorage rather than the server for the same reason the
  // selection uses it (see implementation.md, "Keyboard"): it decides nothing,
  // and losing it costs one keystroke of not knowing.
  function setLayout(on) {
    if (on === cyrillic) return;
    cyrillic = on;
    try {
      if (on) sessionStorage.setItem(LAYOUT, "1"); else sessionStorage.removeItem(LAYOUT);
    } catch (err) { /* fine */ }
    renderKeybar();
  }

  // ---- Which key was pressed ------------------------------------------
  //
  // A key is a place on the keyboard, not a letter the layout happens to
  // print on it. Reading e.key alone made every key in this file mean nothing
  // once the layout was Cyrillic: d arrives as в, and there is no data-key
  // for в, so the whole layer went silent — the app looked broken rather
  // than in another language. e.code names the physical key, unchanged by the
  // layout, and ЙЦУКЕН puts its letters on the same keys as QWERTY, so the
  // key under d is the key under в and pressing either is pressing d.
  //
  // e.key first and e.code only as the fallback, because a Latin layout that
  // moves its letters — Dvorak, Colemak — means what it prints, and asking
  // the position there would fight the layout instead of following it. The
  // fallback catches exactly the case where the layout has no Latin answer.
  //
  // Slash is here for the same reason: ЙЦУКЕН types . and , on that key,
  // so / and ? — the filter and the help — were not reachable at all.
  //
  // And last, the letter itself, read back to the place ЙЦУКЕН puts it. That
  // is the same answer e.code gives, so it never fires when e.code has one —
  // it is there for when it does not. An event can arrive with e.code empty,
  // and the key that lands in that gap is the key that silently does nothing
  // once and works on the second press, which is worse to use than one that
  // never works.
  const YCUKEN = "йцукенгшщзфывапролдячсмить";
  const QWERTY = "qwertyuiopasdfghjklzxcvbnm";

  function keyOf(e) {
    if (/^[a-z]$/i.test(e.key)) return e.key.toLowerCase();
    // off, the key is the letter the layout printed and nothing else, which
    // is what this file did before any of the below existed
    if (!anyLayout()) return e.key.length === 1 ? e.key.toLowerCase() : e.key;
    const letter = /^Key([A-Z])$/.exec(e.code);
    if (letter) return letter[1].toLowerCase();
    if (e.code === "Slash") return e.shiftKey ? "?" : "/";
    // and the same for `#`, for the same reason: it is a place on the keyboard
    // and the place prints something else in Cyrillic — shift-3 is `№` there,
    // so a key read off the character would work in Estonian and fail in
    // Russian, which is the one thing any_layout exists to prevent. Bare 3
    // still answers as 3, because this rule asks for the shift.
    if (e.code === "Digit3" && e.shiftKey) return "#";
    if (e.key && e.key.length === 1) {
      const at = YCUKEN.indexOf(e.key.toLowerCase());
      if (at >= 0) return QWERTY[at];
      // the same key / and ? live on, which ЙЦУКЕН prints . and , on
      if (e.key === ".") return "/";
      if (e.key === ",") return "?";
    }
    return e.key;
  }

  // A declared key may ask for ctrl, written "^a" — the same notation the bar
  // already uses for ctrl-enter. Ctrl and not cmd: cmd-a is select-all in every
  // text box on this machine, and a screen key must not take that away.
  function branchFor(e) {
    if (e.altKey || e.metaKey) return null;
    const k = keyOf(e);
    if (k.length !== 1) return null;
    const want = (e.ctrlKey ? "^" : "") + k;
    // Every declaration is rendered through the mode and compared, rather
    // than the pressed key being turned into a selector: the letter on the
    // control is the same in all three modes and only the chord changes, so
    // there is nothing to build a selector out of.
    const all = document.querySelectorAll("[data-key]");
    for (let i = 0; i < all.length; i++) {
      if (renderKey(all[i].dataset.key, all[i]) === want && keyUsable(all[i])) return all[i];
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
    // a link that opens in a place of its own is opened in one, key or mouse.
    // window.location here would be the app replacing itself with the
    // reference, which is the one thing design.md, "Following a link" forbids
    if (href && el.target === "_blank") { el.click(); return; }
    // every way out of a screen goes through goTo, so that the question about
    // unsaved work has one place to be asked from
    if (href) goTo(href, false);
  }

  // ---- Following a link (^o) -------------------------------------------
  //
  // design.md, "Following a link" gives ^o the link belonging to whatever the
  // cursor is on. The server has already said what each item holds, in
  // data-links on the row or on the wrapper of a one-item screen, so nothing
  // here reads item text: what counts as a link is decided once, in Go
  // (links.go), and this side only follows what it was handed.

  function linksDialog() { return document.getElementById("links-dialog"); }

  // The item ^o is about: the row under the cursor, and otherwise the one item
  // this screen is for. A list with nothing selected has no answer — which item
  // would it be? — so the key does nothing there and is not offered.
  function linkScope() {
    return selected() || document.querySelector("[data-links]:not([data-kb-row])");
  }

  // How far the key sees, read off the pane like every other answer the
  // settings file gives (implementation.md, "Settings file").
  function linksReach() {
    const pane = document.querySelector(".pane");
    return (pane && pane.dataset.linksReach) || "any";
  }

  // What ^o may follow on that item. Under "any" it is every link the item's
  // text holds, which the server wrote down; under "shown" it is the external
  // links this screen actually drew inside the same element. getAttribute and
  // not .href, because the browser normalises .href and the address followed
  // has to be the address written.
  function itemLinks(scope) {
    if (!scope) return [];
    if (linksReach() !== "shown") {
      return (scope.dataset.links || "").split(/\s+/).filter(Boolean);
    }
    const out = [], seen = {};
    scope.querySelectorAll("a.ext").forEach(function (a) {
      const u = a.getAttribute("href");
      if (u && !seen[u]) { seen[u] = true; out.push(u); }
    });
    return out;
  }

  // The letters the chooser hands out, in the order the links read. j and k are
  // not among them: they move through this list as they do through every other
  // one. Past the end of it a row has no letter and is still reached with j/k
  // and Enter, which is the promise the key bar makes — never a key that does
  // nothing, rather than a key for everything.
  const LINK_KEYS = "abcdefghilmnopqrstuvwxyz";

  function linkHost(u) {
    try { return new URL(u).host; } catch (err) { return u; }
  }

  // One answer: a real external link wearing exactly what every other one on a
  // screen wears, so that pressing its letter and clicking it are the same
  // event arriving at the same element.
  function linkChoice(u, i) {
    const a = document.createElement("a");
    a.className = "ext";
    a.href = u;
    a.target = "_blank";
    a.rel = "noopener noreferrer";
    a.setAttribute("data-kb-row", "");
    if (i < LINK_KEYS.length) {
      a.dataset.key = LINK_KEYS.charAt(i);
      // the bar names where the link goes rather than the whole address: a
      // column of full URLs along the bottom would be unreadable, and where it
      // goes is what is being chosen between
      a.dataset.keyLabel = linkHost(u);
      const mark = document.createElement("span");
      mark.className = "lkey";
      mark.textContent = LINK_KEYS.charAt(i);
      a.appendChild(mark);
    }
    a.appendChild(document.createTextNode(u));
    // the tab opens in its own place, so this page is still here afterwards and
    // the question that has now been answered has to come down
    a.addEventListener("click", function () {
      const d = linksDialog();
      if (d && d.open) { d.close(); renderKeybar(); }
    });
    return a;
  }

  // ^o. One link goes straight to its tab, which is nearly always what an item
  // holds; several put the chooser up, because a key that opened four tabs is
  // not one you would press to find out what an item is carrying. Either way
  // what opens it is an anchor being clicked — there is no second way to open a
  // link in this app, and so no second set of popup rules to get wrong.
  function followLink() {
    const dlg = linksDialog();
    const list = itemLinks(linkScope());
    if (!dlg || !list.length) return false;
    const choices = dlg.querySelector(".choices");
    choices.textContent = "";
    list.forEach(function (u, i) { choices.appendChild(linkChoice(u, i)); });
    if (list.length === 1) { choices.firstChild.click(); return true; }
    dlg.showModal();
    select(rows()[0]);
    renderKeybar();
    return true;
  }

  // The same three keys where the screen carries the form rather than a row:
  // an action's own page, a project's, the completion request, the doing
  // screen. Offered only with nothing under the cursor, because a row is what
  // they act on whenever there is one — which is the rule the handler follows
  // too, so the bar cannot offer a key that would act somewhere else.
  function pushScreenKeys(into) {
    if (screenForm("kb-complete")) into.push([renderKey("d"), "done", function () { actOn("kb-complete"); }]);
    if (screenForm("kb-pick")) into.push([renderKey("t"), "today", function () { actOn("kb-pick"); }]);
  }

  // Delete is added at the end of the view group rather than beside the keys
  // it used to follow, so that it lands next to the way out. It sat last in
  // the button row this bar replaced, for a reason that outlives the row: it
  // is the one control where being wrong is expensive (keys.md, "One letter,
  // one button"). Left where it was, `s save` and the screen's own answers
  // would now come after it and put it in the middle of the keys pressed all
  // day — and an entry a pointer can reach is worse to have there than a
  // letter was.
  function pushDeleteKey(into, row) {
    // a draft's own remove is offered with the rest of its row keys above: it
    // removes a line that was never written down, which is not this delete
    if (row && row.hasAttribute("data-draft")) return;
    const form = row ? deleteForm(row) : screenForm("kb-delete");
    if (form) into.push(["⌫", "delete", deleteHere, "danger"]);
  }

  function pushRowKeys(into, row) {
    if (!row) return;
    // an action written before its project exists is not on the list yet, so
    // none of the list's keys mean anything to it. What it has instead is
    // where it sits, and the keys for that are offered only where they would
    // do something: no "up" on the first row, no "down" on the last.
    if (row.hasAttribute("data-draft")) {
      into.push(["\u21b5", "edit", function () { openDraft(row); }]);
      // Moving the row itself is the movement keys with shift held: `u` and
      // `d` were spent on Undone and Done, and a draft is a row like any
      // other, so the delete key removes it the way the delete key removes anything.
      // Only past another draft: the saved rows above it are the project's
      // order and this form does not write them (see moveDraft).
      const up = row.previousElementSibling, down = row.nextElementSibling;
      if (up && up.hasAttribute("data-draft")) into.push(["K", "up", function () { moveDraft(row, -1); }]);
      if (down && down.hasAttribute("data-draft")) into.push(["J", "down", function () { moveDraft(row, 1); }]);
      into.push(["⌫", "remove", deleteHere, "danger"]);
      return;
    }
    // the one row on the project picker that is not a project: it opens the
    // dialog the new one is written in rather than going anywhere
    if (row.hasAttribute("data-newproject")) {
      into.push(["\u21b5", "new project", function () { newProjectDialog(); }]);
      return;
    }
    // opening an inbox item is processing it, so the key says so
    if (row.hasAttribute("data-process")) into.push(["\u21b5", "process", function () { openRow(row); }]);
    else if (row.dataset.href) into.push(["\u21b5", "open", function () { openRow(row); }]);
    else if (row.querySelector("input[type=radio]")) {
      into.push(["\u21b5", "pick", function () {
        const radio = row.querySelector("input[type=radio]");
        if (radio) { radio.checked = true; renderKeybar(); }
      }]);
    }
    if (row.querySelector("form.kb-complete")) into.push([renderKey("d"), "done", function () { actOn("kb-complete"); }]);
    if (canDo(row)) into.push(["w", "doing", function () {
      handSelectionOn(row);
      window.location.href = doingHref(row);
    }]);
    if (row.querySelector("form.kb-pick")) into.push([renderKey("t"), "today", function () { actOn("kb-pick"); }]);
    // The mark goes both ways, so the entry says the answer this press lands
    // on rather than the name of the control — the same rule the Settings
    // screen's `h theme dark` follows (keys.md, "What is built").
    if (row.querySelector("form.kb-review")) {
      into.push([renderKey("r"), reviewed(row) ? "unreviewed" : "reviewed",
        function () { actOn("kb-review"); }]);
    }
  }

  // Whether the row under the cursor is marked as walked. Read off the row
  // rather than kept in a variable here: the page arrives with the marks the
  // server drew, and this file only ever flips them.
  function reviewed(row) {
    return row.classList.contains("reviewed");
  }

  // Opening the row the cursor is on, which is what `↵` does and therefore
  // what the bar's entry for it must do — one path, so that the key and the
  // pointer cannot drift apart.
  function openRow(row) {
    if (row && row.dataset.href) goTo(row.dataset.href, false);
  }

  // Inbox Zero is the list's own link, followed. One path for the key and the
  // bar's entry, for the same reason openRow is one.
  function goInboxZero() {
    const list = document.querySelector("[data-inbox-zero]");
    if (list) window.location.href = list.dataset.inboxZero;
  }

  function toggleHelp() {
    const help = document.getElementById("help");
    if (help) help.hidden = !help.hidden;
    renderKeybar();
  }

  // ---- row and screen commands -------------------------------------------
  //
  // `d`, `t` and the delete key press the selected row's form, or — with no
  // row under the cursor — the screen's own. One mechanism, so an action's
  // page and a row of the list it was opened from answer the same key with
  // the same form (keys.md, "The map"). A row under the cursor is never
  // stepped over: if it carries no such form the key does nothing, rather
  // than reaching past it to act on the screen behind.
  // A row's forms are the row's, and the screen's keys must not reach past the
  // cursor into one — except where the row *is* what the screen is about, and
  // it says so with data-kb-subject. The project page's next action is the one
  // of those: it is drawn as its own row, because j/k, `enter` and `w` all
  // still mean what they mean on a row, and it is also half of what the screen
  // is for, so `d` and `t` answer for it with nothing selected. Walking onto a
  // heading to tick the one action the page opened to show you is a press that
  // asks the cursor for permission (keys.md, "What is built").
  //
  // Document order decides between candidates, which is what puts the subject
  // ahead of the screen's own buttons further down the page.
  function screenForm(cls) {
    const all = document.querySelectorAll("form." + cls);
    for (let i = 0; i < all.length; i++) {
      const row = all[i].closest("[data-kb-row]");
      if (row && !row.hasAttribute("data-kb-subject")) continue;
      if (!keyLive(all[i])) continue;
      const btn = all[i].querySelector("button");
      if (!btn || !btn.disabled) return all[i];
    }
    return null;
  }

  function actOn(cls) {
    // Completing is one of the three that shows itself leaving, so it is one
    // of the three that is deaf while it does. Picking is not: `t` changes a
    // tag on an item that stays exactly where it is, and pressing it twice is
    // a thing you meant (see "A moment that shows itself" below).
    if (cls === "kb-complete" && deaf()) return true;
    const row = selected();
    if (row) {
      if (!row.querySelector("form." + cls)) return false;
      // The mark is the one row control that does not leave the screen, so
      // it does not submit one: it is flipped where it stands (see
      // flipReview).
      if (cls === "kb-review") return flipReview(row);
      if (cls === "kb-complete") return withMotion("done", function () { submitIn(row, cls); });
      submitIn(row, cls);
      return true;
    }
    const form = screenForm(cls);
    if (!form) return false;
    const go = function () {
      if (form.requestSubmit) form.requestSubmit(); else form.submit();
    };
    if (cls === "kb-complete") return withMotion("done", go);
    go();
    return true;
  }

  // The mark at the head of a review row, flipped where it stands. Nothing
  // leaves the screen, so nothing is reloaded: the step is a list being walked
  // down, and a page that re-sorted itself at every press would move the next
  // row out from under the hands (implementation.md, "The weekly review
  // screens"). The server still decides which way it went — the row is drawn
  // from the answer rather than from what was sent, so a press that never
  // arrived leaves nothing on the screen claiming it did.
  function flipReview(row) {
    const form = row && row.querySelector("form.kb-review");
    if (!form) return false;
    fetch(form.getAttribute("action"), {
      method: "POST",
      headers: { "Accept": "application/json" },
      credentials: "same-origin",
    }).then(function (res) {
      if (!res.ok) throw new Error(String(res.status));
      return res.json();
    }).then(function (state) {
      showReview(row, state.reviewed);
    }).catch(function () { /* nothing was stamped, and nothing on screen says it was */ });
    return true;
  }

  // What a walked row looks like, in one place: the dot, what it says to a
  // reader who cannot see the dot, and the heading's count — which is the
  // step's own answer to how much of it is left.
  function showReview(row, on) {
    row.classList.toggle("reviewed", on);
    const btn = row.querySelector("form.kb-review button");
    if (btn) {
      btn.textContent = on ? "●" : "○";
      btn.setAttribute("aria-pressed", on ? "true" : "false");
    }
    const count = document.querySelector(".pagehead .count");
    if (count && /^\d+$/.test(count.textContent.trim())) {
      count.textContent = String(Math.max(0, Number(count.textContent) + (on ? -1 : 1)));
    }
    renderKeybar();
  }

  // Whether this event is the press of a declared letter, whatever the mode
  // renders that letter as.
  function pressedIs(decl, e) {
    return renderKey(decl) === (e.ctrlKey ? "^" : "") + keyOf(e);
  }

  // ---- Unsaved work -------------------------------------------------------
  //
  // A screen that is being written on says so in the title bar, and will not
  // be left without the question being asked. Both halves read the same one
  // fact: whether what is on the screen differs from what the server sent.
  //
  // It is a comparison of values and never a record of typing. A word typed
  // and deleted again leaves the form exactly as it was, and a screen that
  // called that unsaved would be colouring its title bar and stopping a press
  // over nothing (design.md, "A screen with unsaved work on it says so, and
  // asks before it is left."). Each field's own defaultValue
  // is what it is compared against, so nothing has to be remembered in here —
  // the same trick the filter box's Apply and the Save gate already use.
  //
  // Only the forms that ask are watched: `data-dirty-save` on a screen that
  // edits something, `data-dirty-new` on one that creates. A create screen is
  // dirty from the moment it opens and answers without being compared —
  // there is nothing behind it to be the same as, so everything on it is work
  // that would be lost.

  // Whether the guard is standing down because we are the ones leaving.
  let leaving = false;

  function guardedForms() {
    return Array.from(document.querySelectorAll("form[data-dirty-save], form[data-dirty-new]"));
  }

  // The plan's rows, as the one value they are. Two things the boxes' own
  // comparison cannot answer for. The order is part of what the form says, so
  // a row moved or removed is a change no single field differs over; and a
  // hidden input's value *is* its default — the two are one attribute in the
  // browser — so a row the dialog wrote looks untouched to the field-by-field
  // test however much was typed into it. So the list as the server drew it is
  // remembered here, and what is on the screen is compared against that.
  function draftShape() {
    return Array.from(document.querySelectorAll("[data-draft-list] [data-draft]"))
      .map(function (row) {
        const v = draftValues(row);
        return v.title + "␟" + v.meta + "␟" + v.description;
      });
  }
  let draftsAsDrawn = [];
  function rememberDrafts() { draftsAsDrawn = draftShape(); }

  function draftsChanged() {
    return draftShape().join("␞") !== draftsAsDrawn.join("␞");
  }

  function formDirty(f) {
    if (f.hasAttribute("data-dirty-new")) return true;
    return changed(f) || draftsChanged();
  }

  function dirtyForms() {
    return guardedForms().filter(function (f) { return keyLive(f) && formDirty(f); });
  }

  // The title bar wears it. It is the one strip on the screen that is about
  // the screen rather than about anything on it, which is what a page with
  // unwritten work on it needs said — and it is already in the eye's path on
  // the way to the keys. Nothing is coloured on a create screen's first paint
  // any differently than later: it is unsaved the whole time it is up.
  //
  // With the title bar off it falls to the pane, and it has to: processing
  // opens in zen mode by default, so the screen a project is written on is
  // exactly the one with no chrome to wear this. A panel is a thing shown and
  // never a thing hidden state goes away with — the same rule that keeps the
  // controls on a barless screen (implementation.md, "Panels").
  function renderDirty() {
    const on = dirtyForms().length > 0;
    const bar = document.getElementById("titlebar");
    const pane = document.querySelector(".pane");
    if (bar) bar.classList.toggle("dirty", on);
    if (pane) pane.classList.toggle("dirty", on && !bar);
  }

  // What would be lost, marked in the boxes it is in, while the question is
  // on the screen. Not while it is merely being typed: the marks are the
  // dialog's evidence and they go when it does.
  function markUnsaved(forms) {
    forms.forEach(function (f) {
      if (!f.hasAttribute("data-dirty-save")) return;
      fieldsIn(f).forEach(function (el) {
        let diff;
        if (el.type === "checkbox" || el.type === "radio") diff = el.checked !== el.defaultChecked;
        else if (el.tagName === "SELECT") {
          diff = Array.from(el.options).some(function (o) { return o.selected !== o.defaultSelected; });
        } else diff = el.value !== el.defaultValue;
        if (diff) el.classList.add("unsaved");
      });
    });
    // a row is marked when it is not one the server drew — the same
    // comparison, against the list as it was sent
    const drawn = draftsAsDrawn.slice();
    document.querySelectorAll("[data-draft-list] [data-draft]").forEach(function (row) {
      const v = draftValues(row);
      const i = drawn.indexOf(v.title + "␟" + v.meta + "␟" + v.description);
      if (i < 0) row.classList.add("unsaved");
      else drawn.splice(i, 1);
    });
  }

  function clearUnsaved() {
    document.querySelectorAll(".unsaved").forEach(function (el) { el.classList.remove("unsaved"); });
  }

  function leaveDialog() { return document.getElementById("leave-dialog"); }

  // Leaving a screen with unsaved work on it asks, and the question has the
  // two answers there are: keep it or lose it. This is the app's one
  // interruption, and it is not a confirmation — nothing here asks whether
  // you are sure, it offers the press you would otherwise have had to
  // remember to make first (keys.md, "Leaving a screen").
  function askBeforeLeaving(to, form) {
    const dlg = leaveDialog();
    if (!dlg) { leaving = true; window.location.href = to; return; }
    markUnsaved([form]);
    const save = dlg.querySelector("[data-leave-save]");
    const btn = makeButton(form);
    // Save says what the screen's own button says — Save, Create, Promote —
    // because it is that button, pressed from here. It is offered only when
    // that button could be pressed: a create screen with a required box still
    // empty has nothing it is allowed to keep, and then the only way on is to
    // lose it or to close the question and fill the box in.
    save.disabled = !btn || btn.disabled;
    save.textContent = btn ? btn.textContent.trim() : "Save";
    save.onclick = function () {
      // where the press was going goes with it, on a form that takes an
      // answer to that question — so saving on the way to Today lands on
      // Today rather than wherever the form would have gone by itself
      const back = form.elements && firstNamed(form, "back");
      if (back && to) back.value = to;
      leaving = true;
      dlg.close();
      submitScope(form);
    };
    dlg.querySelector("[data-leave-discard]").onclick = function () {
      leaving = true;
      dlg.close();
      withMotion("back", function () { window.location.href = to; });
    };
    dlg.onclose = function () { clearUnsaved(); renderKeybar(); };
    // The dialog owns both keys outright, the way the draft one does: esc
    // closes the question and leaves the screen where it is — which is the
    // third answer, and the one that needs no button, because staying is
    // what happens when you decline to answer. Enter presses the one the
    // page was going to lose.
    dlg.onkeydown = function (e) {
      if (e.key === "Escape") { e.stopPropagation(); return; }
      if (e.key !== "Enter") return;
      e.preventDefault(); e.stopPropagation();
      if (!save.disabled) save.onclick();
    };
    dlg.showModal();
    renderKeybar();
  }

  // Every way out of a screen goes through here, so that there is one place
  // the question is asked from and no route out that forgets to ask.
  function goTo(to, motion) {
    if (!to) return false;
    const dirty = dirtyForms();
    if (dirty.length && !leaving) { askBeforeLeaving(to, dirty[0]); return true; }
    leaving = true;
    if (!motion) { window.location.href = to; return true; }
    return withMotion("back", function () { window.location.href = to; });
  }

  function leave() {
    if (deaf()) return true;
    const cancel = document.querySelector("[data-cancel]");
    if (!cancel) return false;
    return goTo(cancel.dataset.cancel, true);
  }

  // The mouse's way out, and every link on the page is one: the rail, the
  // trail, Back, a row's title. Caught in the capture phase, because hx-boost
  // is listening for the same click and would have the next page on its way
  // before the question could be asked. A link that opens somewhere of its
  // own is left alone — it does not leave this screen (design.md, "Following
  // a link").
  document.addEventListener("click", function (e) {
    if (e.defaultPrevented || e.button !== 0) return;
    if (e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
    const a = e.target.closest && e.target.closest("a[href]");
    if (!a || (a.target && a.target !== "_self")) return;
    const href = a.getAttribute("href");
    if (!href || href.charAt(0) === "#") return;
    if (leaving || !dirtyForms().length) return;
    e.preventDefault();
    e.stopPropagation();
    // the path, not the href: it is handed to the form as where to go after
    // saving, and the server keeps a destination only while it is one of its
    // own (see localPath)
    goTo(a.pathname + a.search, false);
  }, true);

  // The one way out the app does not draw: a reload, the tab closing, an
  // address typed over this one. There is no dialog to offer there — the
  // browser asks its own question, in its own words — so all this does is
  // tell it there is something to ask about. Pressing a button on the page is
  // not that: a submit is the work being written down, and `leaving` is
  // already set by every route in here that goes somewhere on purpose.
  document.addEventListener("submit", function () { leaving = true; }, true);
  window.addEventListener("beforeunload", function (e) {
    if (leaving || !dirtyForms().length) return;
    e.preventDefault();
    e.returnValue = "";
  });

  // ---- A moment that shows itself ----------------------------------------
  //
  // `d`, `⌫` and `b` each leave a screen and bring another one of the same
  // shape back. Nothing about the answer said the press had landed, so the
  // press got made again — and on the two that destroy something, the second
  // one landed on an item nobody had read (design.md, "A moment that shows
  // itself"). The settings file says what each of the three looks like; this
  // is where it is worn.
  //
  // Two halves, and which one a value uses is the whole of the difference
  // between them. A *leaving* effect plays on the item that was acted on,
  // before the request goes: the item is still on the screen while it runs,
  // and the key is deaf for exactly as long. An *arriving* effect plays on the
  // page that comes back — it costs nothing, because that page is already
  // there, and it has to be handed the moment it is about, the way the cursor
  // already is (see HANDOVER).
  //
  // The deaf window is the fix and the motion is only the explanation for it:
  // an arriving effect cannot swallow the second press, because by the time it
  // plays that press has already been sent, so it is given a window of its own
  // on arrival rather than inheriting one.
  const ARRIVAL = "kb-anim-arrival";
  const LEAVING = { fade: true, strike: true, collapse: true, sweep: true };
  let deafUntil = 0;

  function deaf() { return Date.now() < deafUntil; }

  // What the settings file said about one of the three moments. `anim.ms = 0`
  // is how the file turns the whole of it off in one line, so it answers here
  // as "none" whatever the three effects say — one place to ask, rather than
  // every caller remembering that the number can veto the word.
  function animConf(kind) {
    const pane = document.querySelector(".pane");
    if (!pane) return { fx: "none", ms: 0 };
    const ms = parseInt(pane.getAttribute("data-anim-ms"), 10);
    if (!(ms > 0)) return { fx: "none", ms: 0 };
    return { fx: pane.getAttribute("data-anim-" + kind) || "none", ms: ms };
  }

  // What the effect is about. A row under the cursor is the item that was
  // acted on; with no row it is whatever block the screen is *about*, which
  // every screen in the app already says without being asked — the capture
  // being decided, the item being looked at, the one action on the doing
  // screen. `main` is the answer for a screen that says none of those, and the
  // only answer for `b`: leaving is about the screen and not about an item on
  // it, and there is no item it could honestly point at.
  function animTarget(kind) {
    if (kind !== "back") {
      const row = selected();
      if (row) return row;
      const sub = document.querySelector("#doing p") ||
        document.querySelector(".process .subject") ||
        document.querySelector(".process") ||
        document.querySelector(".item");
      if (sub) return sub;
    }
    return document.querySelector("main");
  }

  // Run whatever this moment wears, then do the thing. The request is not sent
  // first: a browser goes on painting the old page until the answer commits,
  // so firing both at once would cut the effect off after the few milliseconds
  // a server on this machine takes — the one arrangement where the motion is
  // paid for and never seen.
  function withMotion(kind, go) {
    const c = animConf(kind);
    // `none` is today's behaviour exactly, deaf window included — which is to
    // say without one. A file that says no motion at all and still swallowed
    // presses would be a file that lies about what it turned off.
    if (c.fx === "none") { go(); return true; }
    deafUntil = Date.now() + c.ms;
    // and then deaf again until the answer replaces the page. A leaving
    // effect ends with the item at zero opacity and still in the DOM, so a
    // press landing in the few milliseconds the request is in flight would
    // find the same form and post it a second time. Two seconds is the net
    // under a request that never answers at all; anything that does answer
    // clears this long before then.
    const fire = function () { deafUntil = Date.now() + 2000; go(); };
    if (!LEAVING[c.fx]) { handArrivalOn(kind); fire(); return true; }
    const el = animTarget(kind);
    if (!el) { fire(); return true; }
    playLeaving(el, c.fx, c.ms, kind);
    setTimeout(fire, c.ms);
    return true;
  }

  // Nothing is moving any more: the answer arrived, or it never will. Both
  // paths come through here so that neither can leave the keys deaf or the
  // pane clipped for the rest of the session.
  function motionOver() {
    deafUntil = 0;
    const pane = document.querySelector(".pane");
    if (pane) pane.classList.remove("anim-running");
  }

  function playLeaving(el, fx, ms, kind) {
    el.style.setProperty("--anim-ms", ms + "ms");
    // for as long as something is moving sideways, and no longer: see
    // style.css, "A moment that shows itself"
    const pane = document.querySelector(".pane");
    if (pane) pane.classList.add("anim-running");
    if (fx === "collapse") { collapse(el, ms); return; }
    if (fx === "sweep") {
      // away from the reading direction for a thing thrown away, along it for
      // a thing finished and for a screen being left behind
      el.classList.add(kind === "delete" ? "anim-sweep-l" : "anim-sweep-r");
      return;
    }
    if (fx === "strike") {
      // The rule is drawn across the item's title, which is the thing a line
      // through a title means something about. A screen that has no line of
      // text to draw it through — an action's own page is a form, and its
      // title is a box — says the other half of what strike says and only
      // that, rather than putting a 2px rule across a form.
      const line = el.matches("[data-anim-line]") ? el :
        el.querySelector(".title, .capture-text, [data-anim-line]");
      if (!line) { el.classList.add("anim-fade"); return; }
      line.style.setProperty("--anim-ms", ms + "ms");
      line.classList.add("anim-line");
      el.classList.add("anim-strike");
      return;
    }
    el.classList.add("anim-fade");
  }

  // Collapse is the one effect that cannot be a class: an element folding shut
  // has to be animated from the height it happens to have, and only the
  // browser knows that. Everything it zeroes is something that would otherwise
  // hold the gap open after the box itself had closed.
  function collapse(el, ms) {
    const box = el.getBoundingClientRect();
    const cs = getComputedStyle(el);
    el.style.overflow = "hidden";
    el.animate([
      {
        height: box.height + "px", opacity: 1,
        paddingTop: cs.paddingTop, paddingBottom: cs.paddingBottom,
        marginTop: cs.marginTop, marginBottom: cs.marginBottom
      },
      {
        height: "0px", opacity: 0,
        paddingTop: "0px", paddingBottom: "0px",
        marginTop: "0px", marginBottom: "0px"
      }
    ], { duration: ms, easing: "cubic-bezier(.4, 0, .2, 1)", fill: "forwards" });
  }

  // An arriving effect is about a page that does not exist yet, so the moment
  // travels the way the cursor and the scroll offset already do. Stored and
  // claimed once, for the reason those are: a marker left behind would play
  // the effect again on a page reached some other way.
  function handArrivalOn(kind) {
    try { sessionStorage.setItem(ARRIVAL, kind); } catch (err) { /* no session storage: no effect, nothing else */ }
  }

  function claimArrival() {
    let kind = null;
    try { kind = sessionStorage.getItem(ARRIVAL); } catch (err) { return; }
    if (!kind) return;
    try { sessionStorage.removeItem(ARRIVAL); } catch (err) { /* nothing to undo */ }
    const c = animConf(kind);
    // the file can have been changed to a leaving effect, or to none at all,
    // between the press and the answer — the marker is only a note of which
    // key was pressed, never of what it was wearing at the time
    if (c.fx === "none" || LEAVING[c.fx]) return;
    deafUntil = Date.now() + c.ms;
    if (c.fx === "stamp") { stamp(kind, c.ms); return; }
    const el = animTarget(kind);
    if (!el) return;
    el.style.setProperty("--anim-ms", c.ms + "ms");
    if (c.fx === "flash") el.classList.add("anim-flash", "flash-" + kind);
    else el.classList.add("anim-rise");
  }

  // The mark is over the pane and belongs to nothing on it, so it is written
  // here rather than into every template: it is the one effect that is about
  // the key rather than about an item, and a screen has no way of knowing
  // which key it was reached by.
  function stamp(kind, ms) {
    const pane = document.querySelector(".pane");
    if (!pane) return;
    const old = document.getElementById("anim-stamp");
    if (old) old.remove();
    const mark = document.createElement("div");
    mark.id = "anim-stamp";
    mark.className = kind;
    mark.style.setProperty("--anim-ms", ms + "ms");
    mark.textContent = kind === "done" ? "✓" : kind === "delete" ? "✕" : "←";
    pane.appendChild(mark);
    setTimeout(function () { mark.remove(); }, ms + 50);
  }

  // Read before the ctrl guard in the handler, because in modifier mode these
  // are ctrl keys too: the letter on the control is the same in all three
  // modes and only the chord changes.
  function rowCommand(e) {
    const row = selected();
    if (e.key === "Backspace" || e.key === "Delete") return deleteHere();
    if (pressedIs("d", e)) return actOn("kb-complete");
    if (pressedIs("t", e)) return actOn("kb-pick");
    if (pressedIs("r", e)) return actOn("kb-review");
    if (pressedIs("b", e)) return leave();
    // doing is navigation, so it is bare in every mode
    if (!e.ctrlKey && keyOf(e) === "w" && canDo(row)) {
      handSelectionOn(row);
      window.location.href = doingHref(row);
      return true;
    }
    return false;
  }

  // Deleting whatever the delete key would delete: the selected row's form, or
  // — with no row under the cursor — the screen's own. One function, because
  // the key and the bar's entry both go through it and the two must not come
  // to differ about which form that is. `deleteForm` and not a plain
  // querySelector, for the reason it gives: a context's chip holds the chips
  // of its parameters, and their deletes are not its own.
  function deleteHere() {
    if (deaf()) return true;
    const row = selected();
    // a draft is removed from the page and nothing is sent, so there is no
    // answer arriving for an effect to cover the gap before
    if (row && row.hasAttribute("data-draft")) { removeDraft(row); return true; }
    const form = row ? deleteForm(row) : screenForm("kb-delete");
    if (!form) return false;
    if (row) handSelectionOn(row);
    return withMotion("delete", function () { form.submit(); });
  }

  // The row's own delete, and only one that can be pressed: a name still
  // carried keeps its control disabled, and the bar must not offer a key the
  // control would refuse. :scope, because a context's chip holds the chips of
  // its parameters, and their deletes are not its own.
  function deleteForm(row) {
    const form = row && row.querySelector(":scope > form.kb-delete");
    const btn = form && form.querySelector("button");
    return btn && !btn.disabled ? form : null;
  }

  // What the create button in this scope offers, said the way the screen says
  // it: the button's own words when it can be pressed, and what is still blank
  // when it cannot.
  function makeKeys(scope) {
    const btn = makeButton(scope);
    if (!btn) return [];
    if (!btn.disabled) {
      // The button's own key is the better name for this when it has one and
      // that key is a chord: `^s save` and `^\u21b5 save` are one answer said
      // twice. In `command` mode the declared key is a bare letter and dead
      // where this entry is read, so there ^enter is the only way out of the
      // box and stays in the bar.
      const own = btn.dataset.key;
      if (own && renderKey(own, btn).indexOf("^") >= 0) return [];
      return [["^\u21b5", btn.textContent.trim().toLowerCase(),
        function () { press(btn); }]];
    }
    const need = missing(scope);
    if (!need.length) return [];
    return [["\u2026", "needs " + need.join(" and ")]];
  }

  // An entry is `[key, label]`, and one that presses something carries the
  // press as a third member and a tone as a fourth.
  //
  // An entry that presses something is a real <button>; one that only says how
  // to steer — `j k`, a `g` prefix, the "needs …" line — stays a <span>. Now
  // that the buttons live here rather than on the form, that difference is the
  // bar's second promise: it advertises only keys that work (keys.md, "What a
  // key is"), and only the entries that press something are pressable. The
  // element differs rather than only the class, so no paint keys.bar_style
  // chooses can blur it and no style sheet has to be trusted to keep the rule.
  function keygroup(cls, items) {
    const box = document.createElement("div");
    box.className = cls;
    items.forEach(function (pair) {
      const act = pair[2];
      const item = document.createElement(act ? "button" : "span");
      item.className = "k" + (pair[3] ? " " + pair[3] : "");
      if (act) {
        item.type = "button";
        // the caret stays where it was: the bar is chrome, and a button that
        // took focus on the way down would empty the very bar being clicked —
        // with the caret in a box the bar narrows to the chords (keys.md,
        // "The bar while you are typing"), so clicking `^↵ create` would
        // rewrite the bar out from under the click. The click still fires;
        // only the focus move is cancelled.
        item.addEventListener("mousedown", function (e) { e.preventDefault(); });
        item.addEventListener("click", function (e) { e.preventDefault(); act(); });
      }
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
    // Last, past the keys and outside both groups: it is not a key, and the
    // bar's promise is that everything in a group is one that works.
    if (cyrillic && layoutMarker()) {
      const mark = document.createElement("span");
      mark.className = "kb-layout";
      mark.textContent = "русский";
      bar.appendChild(mark);
    }
  }

  // The fields a form owns, which is not the same as the fields inside it: a
  // box may sit outside the form element and say which form it belongs to with
  // its own `form` attribute. The project page's next action does exactly
  // that, so that the two marks on its heading can be the little forms they
  // are in every list (see implementation.md, "Writing a project").
  //
  // `form.elements` is the owned set and is what the browser will actually
  // submit, which is the set the gate and the unsaved marks have to agree
  // with. A dialog is not a form and still answers with what is inside it.
  function fieldsIn(scope) {
    const owned = scope.elements
      ? Array.from(scope.elements)
      : Array.from(scope.querySelectorAll("input, textarea, select"));
    return owned.filter(function (el) {
      return el.tagName === "INPUT" || el.tagName === "TEXTAREA" || el.tagName === "SELECT";
    });
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
    return fieldsIn(scope)
      .filter(function (el) { return el.hasAttribute("required") && el.value.trim() === ""; })
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
    const needs = fieldsIn(scope).some(function (el) { return el.hasAttribute("required"); });
    const dirty = scope.hasAttribute && scope.hasAttribute("data-dirty-save");
    if (!needs && !dirty) return;
    btn.hidden = false;
    btn.disabled = (needs && missing(scope).length > 0) || (dirty && !formDirty(scope));
  }

  // A form that says data-dirty-save has a button meaning "keep this", and
  // there is nothing to keep until something differs from what the server
  // sent — see formDirty, which answers that for a whole form, the plan's
  // rows included. Each field's own defaultValue is what a box is compared
  // against, so nothing about the boxes has to be remembered in here — the
  // same trick the filter box's Apply uses.
  function changed(scope) {
    return fieldsIn(scope).some(function (el) {
      if (el.type === "checkbox" || el.type === "radio") return el.checked !== el.defaultChecked;
      if (el.tagName === "SELECT") {
        return Array.from(el.options).some(function (o) { return o.selected !== o.defaultSelected; });
      }
      return el.value !== el.defaultValue;
    });
  }

  function gateAll() {
    document.querySelectorAll("form, dialog").forEach(gate);
    // the same pass: a control whose pressability is read off a box belongs
    // with the buttons whose pressability is read off a form
    syncMetaCopy();
    renderDirty();
    renderKeybar();
  }
  // Moving into or out of a box changes which keys are live, so the bar is
  // re-read on both. focusin/focusout because they bubble — the boxes come
  // and go with every page swap — and focusout a tick later, once the focus
  // has actually landed somewhere.
  // — except focus landing inside the bar itself, which is a Tab away now that
  // the bar holds the controls. Re-rendering there rebuilds the very button
  // that was just reached and throws the focus back to the document, so a Tab
  // into the bar would move nowhere.
  function inKeybar(el) { return !!(el && el.closest && el.closest("#keybar")); }
  document.addEventListener("focusin", function (e) {
    if (inKeybar(e.target)) return;
    renderKeybar();
  });
  document.addEventListener("focusout", function (e) {
    if (inKeybar(e.relatedTarget)) return;
    setTimeout(renderKeybar, 0);
  });

  document.addEventListener("input", function (e) {
    // a token box paints itself and offers what you may be typing — and then
    // falls through, because it is a field in a form like any other and the
    // form's own button has to know that something changed
    if (e.target.matches && e.target.matches("[data-tokenbox]")) {
      paintBox(e.target);
      showSuggest(e.target);
    }
    // the title's mark follows the typing, so that fixing the word takes the
    // yellow off in the same keystroke that earned it
    if (e.target.matches && e.target.matches("[data-verbcheck]")) markVerb(e.target);
    // the title bar follows the typing: a word put back is a screen that is
    // saved again, and nothing about the colour is a memory of having typed
    renderDirty();
    // a tag typed onto the project's line is one the mark can bring down, and
    // one deleted off it is one it cannot — so the mark is re-read with every
    // keystroke, the way the gate is
    syncMetaCopy();
    // the form this box belongs to, which is not always the one it sits
    // inside: `el.form` is the owner, `form` attribute included, and that is
    // the button this typing has to reach (see fieldsIn)
    const scope = e.target.form || (e.target.closest && e.target.closest("form, dialog"));
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
  // `snooze:(buy the frame)` and `snooze:#42` — the snooze that names an
  // action rather than a day. It is taken out before DATE_RE runs, which
  // would otherwise read `#42` as a date and underline it, and the brackets
  // hold a title, so it cannot be one word the way every other value is.
  //
  // Whether the action it names exists is the server's to answer: this box
  // knows the notation and not the project. So the shape is accepted here and
  // a name that matches nothing comes back as a refusal on save, which is the
  // same division every unknown name on this line already follows.
  const SNOOZE_ACTION_RE = /(^|\s)snooze:(?:\(([^)]*)\)|#(\d+))/g;
  // what is being typed right now, which is a token that may still be empty.
  // A date key is a sigil like any other here — it is what has been committed
  // to and the rest is still open — except that its sigil is `due:` or
  // `snooze:` rather than one character
  const TYPING_RE = /(^|\s)([@#])([\p{L}\p{N}_-]*)$/u;
  const TYPING_DATE_RE = /(^|\s)([a-z]+:)([\p{L}\p{N}-]*)$/u;
  // and the same thing once the bracket is open: `snooze:(buy the fr`. It
  // needs a regex of its own because what is inside the brackets is a title,
  // so it has spaces in it — every other half-typed value is one word.
  const TYPING_SNOOZE_ACTION_RE = /(^|\s)(snooze:)\(([^)]*)$/u;
  const DATE_OK = /^\d{4}-\d{2}-\d{2}$/;
  const NDAYS_OK = /^\d+(d|days?)$/;
  const NDAYS_ZERO = /^0+(d|days?)$/;
  // `2weeks`: the period you are in and the ones before it — CompletedRange
  // in query.go is what reads it, and this only knows the shape
  const PERIODS_OK = /^0*[1-9]\d*(week|month|year)s?$/;

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
  // `action` says this snooze also takes a sibling instead of a day. Only an
  // action's line does: a project has no siblings to wait on, and neither has
  // the filter line, which asks about items rather than writing one.
  const WHEN_SNOOZE = { date: true, days: true, ahead: true, words: ["tomorrow"].concat(DAY_NAMES) };
  const WHEN_SNOOZE_ACTION = { date: true, days: true, ahead: true, action: true, words: ["tomorrow"].concat(DAY_NAMES) };
  const WHEN_WINDOW = { date: false, days: false, words: DUE_WINDOWS };
  // what was finished is asked about looking back: a day, a day name, or a
  // period. Any case, since `Monday` is how the word is written in prose
  const WHEN_DONE = { date: true, days: false, periods: true, anycase: true,
    words: ["today", "yesterday", "week", "month", "year"].concat(DAY_NAMES) };
  const BOX_RULES = {
    filter: { contexts: 1, fields: ["short", "medium", "long", "focus", "today"], dates: {}, prose: true },
    // some views filter by tag and by name and by nothing else — design.md
    // gives each view the subset it offers, and a line that quietly ignored
    // the rest would be the app pretending to have narrowed something
    "filter-tags": { contexts: 0, fields: [], dates: {}, prose: true },
    "filter-due": { contexts: 0, fields: [], dates: { due: WHEN_WINDOW }, prose: true },
    "filter-completed": { contexts: 0, fields: [], dates: { completed: WHEN_DONE }, prose: true },
    "filter-name": { contexts: 0, fields: [], tags: false, dates: {}, prose: true },
    // the Settings line asks about names rather than items, so a name nobody
    // has heard of is not a mistake there: it is the one that may be created
    "filter-names": { names: true, contexts: 1, fields: ["short", "medium", "long", "focus", "today"], dates: {}, prose: true, waiting: true },
    action: { contexts: 1, fields: ["short", "medium", "long", "focus", "today"], dates: { due: WHEN_DUE, snooze: WHEN_SNOOZE_ACTION }, prose: false, waiting: true },
    project: { contexts: 0, fields: [], dates: { snooze: WHEN_SNOOZE }, prose: false },
    // an idea carries tags and nothing else — not even its own snooze, which
    // is the date box beside the line (design.md, "Someday/maybe item")
    someday: { contexts: 0, fields: [], dates: {}, prose: false },
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
    if (rules.names) return [];
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

    // the action form of the snooze first, and blanked out of `rest` where it
    // matched, so that neither DATE_RE nor the prose sweep sees the title
    // inside its brackets as words that do not belong on the line
    let waited = [];
    if (rules.dates.snooze && rules.dates.snooze.action) {
      SNOOZE_ACTION_RE.lastIndex = 0;
      let sm;
      while ((sm = SNOOZE_ACTION_RE.exec(left)) !== null) {
        const start = sm.index + sm[1].length;
        waited.push({ start: start, end: sm.index + sm[0].length });
      }
      waited.forEach(function (t) { for (let i = t.start; i < t.end; i++) rest[i] = " "; });
    }
    const leftDates = waited.length ? rest.join("") : left;

    DATE_RE.lastIndex = 0;
    let m;
    const dated = [];
    while ((m = DATE_RE.exec(leftDates)) !== null) {
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
    out.sort(function (a, b) { return a.start - b.start; });
    // An open `snooze:(` is a title being typed, and a title is words with
    // spaces in it — so until the bracket is closed the line honestly reads as
    // an unreadable date followed by loose prose, and would be marked three or
    // four times while you spell one action's name. Nothing else on the line
    // has this problem: every other value is a single word, so a half-typed
    // one is a single mark that lands where the caret already is.
    //
    // So the span from the open bracket to the caret is left alone. The marks
    // come back the moment the bracket is closed, which is also the moment the
    // line means anything.
    const typing = typingToken(box);
    if (typing && typing.bracket) {
      return out.filter(function (p) { return p.end <= typing.start; });
    }
    return out;
  }

  // What is wrong with a date token's value, "" when nothing is. `ahead` is
  // the snooze rule: a word that lands on today is not a snooze, so it is
  // named as that rather than reported as an unreadable date (design.md,
  // "Time fields"). An ISO date in the past is left alone here for the same
  // reason the server leaves it alone — it is a claim that went stale, and the
  // weekly review is what catches it.
  function dateProblem(spec, val) {
    if (spec.anycase) val = val.toLowerCase();
    if (spec.date && DATE_OK.test(val)) return "";
    if (spec.periods && PERIODS_OK.test(val)) return "";
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
    scheduleLive(box);
    syncCreate();
  }

  // ---- Create, on the Settings line --------------------------------------
  //
  // The line is the name: `#bike` typed and matching nothing is what creating
  // #bike looks like. The button is offered for exactly one #name or @name that
  // neither list holds, compared without case — `#Car` beside #car is the drift
  // the list is there to stop, so it is not something to offer. What the page
  // cannot tell is whether @shop(Lidl) has a context to go under; that is the
  // server's refusal to say.
  //
  // A bare word is the verb list, which is the one remembered list written
  // with no sigil because a verb is written with no sigil in a title. It is
  // the only way a verb is learned, so the button says which list it means:
  // `Create #bike` is unambiguous from the word itself and `Create call` is
  // not, and a button that does not say what it makes is worse than a longer
  // one. Compared in lower case for the reason the others are compared
  // without it — `Call` and `call` are the same verb (see normVerb).
  const CREATABLE = /^([@#])[\p{L}\p{N}_-]+(\([^()]+\))?$/u;
  const CREATABLE_VERB = /^[\p{L}][\p{L}-]*$/u;

  function createForm() {
    const form = document.querySelector("form[data-create]");
    return form && !form.hidden && keyLive(form) ? form : null;
  }

  function syncCreate() {
    const form = document.querySelector("form[data-create]");
    const box = filterBox();
    if (!form || !box) return;
    const line = box.value.trim();
    const holder = document.querySelector("[data-names]");
    const names = (holder ? holder.dataset.names : "").toLowerCase().split("\n");
    const m = CREATABLE.exec(line);
    const verb = !m && CREATABLE_VERB.test(line) &&
      vocabNames("verbs").indexOf(line.toLowerCase()) < 0;
    const ok = !filterBar().hidden && (verb ||
      (!!m && !(m[1] === "#" && m[2]) && names.indexOf(line.toLowerCase()) < 0));
    form.hidden = !ok;
    if (ok) {
      form.querySelector("[name=q]").value = line;
      form.querySelector("button").textContent = verb ? "Create verb " + line : "Create " + line;
    }
  }

  function paintAll() { tokenBoxes().forEach(paintBox); }

  // ---- The verb a title opens with ---------------------------------------
  //
  // design.md ("Inbox Zero") asks an action's title to start with a verb, and
  // this is the whole of the asking: the box is drawn yellow and the save goes
  // through untouched. Nothing here refuses anything, which is why it can live
  // in the browser alone — there is no second copy of this rule on the server,
  // because there is nothing for the server to decide.
  //
  // Two answers, because the two languages answer differently. A Cyrillic
  // first word is read by shape: a Russian action title is an infinitive —
  // "Позвонить маме", "Купить молоко" — and an infinitive ends -ть, -ти or
  // -чь, so the list never has to hold one. A Latin first word has no shape to
  // read at all, the English imperative being the bare stem, and only the
  // remembered list can tell.
  //
  // The list is the escape hatch for both. A Russian imperative — "Позвони
  // маме" — is not an infinitive, and is learned exactly the way an English
  // verb is: written on the Settings line and created there.

  // CYRILLIC is the layout marker's, up in "Which layout is typing". One test
  // for "is this Russian", and one place it can be wrong.
  // -ся and -сь ride behind the ending rather than replacing it, so a
  // reflexive infinitive — "Разобраться с договором" — is the same rule with a
  // tail on it.
  const RU_INFINITIVE = /(ть|ти|чь)(ся|сь)?$/;
  // The nouns that end the way an infinitive does. -ость is the productive
  // half and is a rule; the rest is a list, because there is no rule
  // separating them from a verb — -есть was one until "Прочесть отчёт" walked
  // into it, and a rule that rejects a verb is worse than a list that misses a
  // noun. Deliberately not exhaustive, for that reason: a noun slipping
  // through is a mark that does not appear, which is the cheap way to be
  // wrong, and the expensive way is marking a title that was written right.
  const RU_NOUN_END = /ость$/;
  const RU_NOUNS = [
    "мать", "дочь", "ночь", "речь", "часть", "сеть", "смерть", "честь",
    "весть", "повесть", "месть", "лесть", "кость", "гость", "шерсть",
    "мелочь", "печать", "кровать", "память", "власть", "страсть", "треть",
    "четверть", "дичь",
  ];

  // The word a title opens with, spelled the way the verb list spells one.
  // The server has its own copy of this (app.FirstWord) for counting; this
  // one is here because the mark follows the typing.
  function firstWord(title) {
    const w = (title || "").trim().split(/\s+/)[0] || "";
    return w.replace(/^[^\p{L}]+/u, "").replace(/[^\p{L}-]+$/u, "").toLowerCase();
  }

  function startsWithVerb(title) {
    // an empty box is not a title that got it wrong, it is one not written
    // yet — and the form's own Create is already disabled while it is empty
    if (!(title || "").trim()) return true;
    const w = firstWord(title);
    // written, and opening with something that is not a word at all: "2
    // letters" is as far from a verb as "milk" is, and the two are told apart
    // only by checking the box before the word
    if (!w) return false;
    if (vocabNames("verbs").indexOf(w) >= 0) return true;
    if (!CYRILLIC.test(w)) return false;
    if (RU_NOUN_END.test(w) || RU_NOUNS.indexOf(w) >= 0) return false;
    return RU_INFINITIVE.test(w);
  }

  // Two things wear data-verbcheck, and the second is a review row. Outside
  // the review a title that got it wrong is left alone: a list you are working
  // from is not the place to be argued with about wording, and a badge on
  // every such row in Next actions would be a complaint you learn to read past.
  // The review is where the titles are being read rather than acted on, so it
  // is where they are pointed out (design.md, "Weekly review").
  function verbText(el) {
    if (el.tagName === "INPUT") return el.value;
    const t = el.querySelector(".title");
    return t ? t.textContent : "";
  }

  function markVerb(el) {
    const ok = startsWithVerb(verbText(el));
    const said = el.querySelector(".noverb");
    // a row says it in a word; a box says it in its own border, having no room
    // beside it for one
    if (said) said.hidden = ok;
    else el.classList.toggle("notverb", !ok);
  }

  function markVerbs(scope) {
    (scope || document).querySelectorAll("[data-verbcheck]").forEach(markVerb);
  }

  // ---- The filter line follows the typing ------------------------------
  //
  // There is no Apply. What the list shows is the line as far as the app can
  // read it: every token it cannot use yet is left out of what is asked for,
  // and stays marked where it is written, so a half-typed `@ho` leaves the
  // list as it was and `@home` narrows it. The question about a name nobody
  // knows is not asked mid-word — that would be every keystroke of every new
  // name — but on Enter, or when the caret leaves the box.
  //
  // The server still does the filtering. The page asks for itself with the new
  // line, exactly the request Apply used to make, and puts what came back
  // around the bar rather than replacing the bar, which is where the caret is.

  const LIVE_DELAY = 150;
  let liveTimer = null;
  let liveSeq = 0;

  // The line with everything the app cannot read yet taken out: the problems
  // problemsIn finds, and the pieces that are not a token at all until the
  // next key — a bare sigil, a key with no value, a parameter still open.
  function readableLine(box) {
    const out = box.value.split("");
    problemsIn(box).forEach(function (p) {
      for (let i = p.start; i < p.end; i++) out[i] = " ";
    });
    return out.join("").split(/\s+/).filter(function (w) {
      return w && !/^([@#]|[a-z]+:|\(.*)$/.test(w) && !/^[@#][^()]*\([^)]*$/.test(w);
    }).join(" ");
  }

  function scheduleLive(box) {
    if (box !== filterBox()) return;
    clearTimeout(liveTimer);
    liveTimer = setTimeout(liveApply, LIVE_DELAY);
  }

  function liveApply() {
    clearTimeout(liveTimer);
    const bar = filterBar(), box = filterBox();
    const main = document.querySelector("main");
    if (!bar || !box || !main || bar.hidden) return;
    const line = readableLine(box);
    const sent = bar.dataset.sent !== undefined ? bar.dataset.sent : box.defaultValue;
    if (line === sent) return;
    bar.dataset.sent = line;
    const url = bar.getAttribute("action") + "?f=1&q=" + encodeURIComponent(line);
    const seq = ++liveSeq;
    fetch(url, { credentials: "same-origin" }).then(function (res) {
      if (!res.ok) throw new Error(String(res.status));
      return res.text();
    }).then(function (html) {
      // an answer to a line that has since been typed past is not the list
      if (seq !== liveSeq) return;
      const doc = new DOMParser().parseFromString(html, "text/html");
      const fresh = doc.querySelector("main");
      const freshBar = fresh && fresh.querySelector("[data-filterbar]");
      if (!freshBar || bar.parentElement !== main || freshBar.parentElement !== fresh) {
        window.location.href = url;
        return;
      }
      Array.from(main.children).forEach(function (c) { if (c !== bar) c.remove(); });
      let before = true;
      Array.from(fresh.children).forEach(function (c) {
        if (c === freshBar) { before = false; return; }
        if (before) main.insertBefore(c, bar); else main.appendChild(c);
      });
      bar.querySelector(".fcount").replaceWith(freshBar.querySelector(".fcount"));
      // closing the bar asks the server to clear only when something is
      // applied, and this is now what is applied
      box.defaultValue = line;
      // the address, and the refresh, are this line now: a reload or the
      // background poll asking for the old one would put it back
      history.replaceState(history.state, "", url);
      document.querySelectorAll("[data-poll]").forEach(function (el) {
        el.setAttribute("hx-get", url);
        if (window.htmx) window.htmx.process(el);
      });
      if (window.htmx) window.htmx.process(main);
      paintAll(); setupForms(); gateAll(); growAll(); markVerbs();
      renderKeybar();
    }).catch(function () { delete bar.dataset.sent; });
  }

  function typingToken(box) {
    if (!box || document.activeElement !== box) return null;
    const before = box.value.slice(0, box.selectionStart);
    // the bracketed form first: `snooze:(buy` also matches TYPING_DATE_RE's
    // shape at the `snooze:` and would be read as a half-typed date word
    const open = TYPING_SNOOZE_ACTION_RE.exec(before);
    if (open) {
      return {
        sigil: open[2], prefix: open[3], bracket: true,
        start: before.length - open[2].length - open[3].length - 1, // the "(" too
      };
    }
    const m = TYPING_RE.exec(before) || TYPING_DATE_RE.exec(before);
    if (!m) return null;
    return { sigil: m[2], prefix: m[3], start: before.length - m[2].length - m[3].length };
  }

  // The actions this box's `snooze:` may name, off the box itself — unlike the
  // remembered names, which are the same on every screen and ride on the pane.
  function siblings(box) {
    if (!box || !box.dataset.siblings) return [];
    try {
      const all = JSON.parse(box.dataset.siblings);
      return Array.isArray(all) ? all : [];
    } catch (e) {
      return [];
    }
  }

  // How a chosen sibling is written, and how its row reads. The readable form
  // normally; the id where two open actions share a title, because
  // `snooze:(…)` would then name both and the server refuses it as ambiguous
  // (design.md, "Writing an action").
  //
  // The row still says the title in that case, with the id after it. Two rows
  // reading `snooze:#1` and `snooze:#2` are a choice between two things you
  // cannot tell apart — which is the ambiguity moved rather than answered.
  function snoozeEntry(all, sib) {
    const twins = all.filter(function (o) {
      return o.t.toLowerCase() === sib.t.toLowerCase();
    }).length > 1;
    const readable = "snooze:(" + sib.t + ")";
    if (!twins) return { insert: readable, label: readable };
    return { insert: "snooze:#" + sib.id, label: readable + " #" + sib.id };
  }

  // What a half-typed token may still become: the remembered names after an @
  // or a #, and the date words after `due:` or `snooze:`. A word is exactly as
  // hard to remember as a name is, and the panel it is written down in is
  // behind ? — the box already knows the list, so it may as well say it.
  //
  // The names are sorted because that list is a lookup; the dates are left in
  // the order they are written down in, because theirs is an order everybody
  // already knows and alphabetical would open with Friday.
  // Every entry is {name, insert}: what is matched against what has been
  // typed, and the text that replaces the half-typed token when it is taken.
  // insert is only needed where the two differ, which is the sibling snooze —
  // it is matched on the action's title and written as `snooze:(that title)`.
  // name is what the typed letters are matched against, insert the text that
  // replaces the half-typed token, label what the row reads. The last two
  // differ only where a title is not enough to name one action — see
  // snoozeEntry.
  function entry(name, insert, label, sib) {
    return { name: name, insert: insert, label: label, sib: !!sib };
  }

  function poolFor(box, t) {
    if (t.sigil === "@" || t.sigil === "#") {
      return knownNames(box, t.sigil).slice().sort().map(function (n) { return entry(n); });
    }
    const spec = rulesFor(box).dates[t.sigil.slice(0, -1)];
    if (!spec) return [];
    // inside the brackets only an action can be meant, so the dates are not
    // offered there — `snooze:(mon` is not a half-typed Monday
    const words = t.bracket ? [] : spec.words.slice();
    // a number is a count of days, and the unit is the only part of it left to
    // say. It goes first because it is what was already being typed
    const digits = /^\d+$/.test(t.prefix);
    const zero = digits && Number(t.prefix) === 0;
    if (!t.bracket && spec.days && digits && !(spec.ahead && zero)) words.unshift(t.prefix + "days");
    if (!t.bracket && spec.periods && digits && !zero) words.unshift(t.prefix + "weeks", t.prefix + "months", t.prefix + "years");
    const out = words.map(function (n) { return entry(n); });
    // and the actions this one may wait on, after the dates rather than among
    // them: a date is what a snooze usually is, and the list is read from the
    // top. They are last because they are the longer answer, not the rarer one
    if (!spec.action) return out;
    const all = siblings(box);
    return out.concat(all.map(function (sib) {
      const e = snoozeEntry(all, sib);
      return entry(sib.t, e.insert, e.label, true);
    }));
  }

  // How many rows each half of a `snooze:` list may have. The dates keep nine,
  // for the reason they always had: the list is nine long and cutting Sunday
  // off the end costs more than one more row does. The actions are counted
  // separately rather than sharing that nine, because sharing it meant a bare
  // `snooze:` spent every row on dates and showed one action — which is the
  // list not being there at all on the one keystroke most likely to open it.
  //
  // Six is what fits under the box without the panel becoming a view of its
  // own; past that, typing a word of the title is the way through, and the box
  // scrolls in the meantime.
  const SUGGEST_DATES = 9;
  const SUGGEST_ACTIONS = 6;

  function suggestionsFor(box, t) {
    const pool = poolFor(box, t);
    const p = t.prefix.toLowerCase();
    // what starts with the typed letters first, then what merely contains
    // them — a title is several words and the one you remember may be the
    // second, so containing is worth offering rather than only leading
    const rank = function (list) {
      const starts = list.filter(function (e) { return e.name.toLowerCase().indexOf(p) === 0; });
      const holds = list.filter(function (e) { return e.name.toLowerCase().indexOf(p) > 0; });
      return starts.concat(holds);
    };
    const dates = rank(pool.filter(function (e) { return !e.sib; })).slice(0, SUGGEST_DATES);
    const sibs = rank(pool.filter(function (e) { return e.sib; })).slice(0, SUGGEST_ACTIONS);
    return dates.concat(sibs);
  }

  function showSuggest(box) {
    const list = suggestList(box);
    if (!list) return;
    const t = typingToken(box);
    const names = t ? suggestionsFor(box, t) : [];
    if (!names.length) { hideSuggest(); return; }
    list.textContent = "";
    names.forEach(function (e, i) {
      const li = document.createElement("li");
      // the row says the whole token it will write, so what an entry does is
      // read off the list rather than worked out from the letters typed so far
      li.textContent = e.label || t.sigil + e.name;
      li.dataset.name = e.name;
      if (e.insert) li.dataset.insert = e.insert;
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
  // insert, when given, is the whole token — a sibling snooze writes brackets
  // round a title, or an id, neither of which is sigil-plus-name.
  function takeSuggest(box, name, insert) {
    const t = typingToken(box);
    if (!t || !box) return;
    const head = box.value.slice(0, t.start) + (insert || t.sigil + name) + " ";
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
      syncCreate();
      renderKeybar();
      return;
    }
    // hidden first, so leaving the box on the way out is not taken for leaving
    // it to go on filtering, which would ask about a line being thrown away
    bar.hidden = true;
    window.location.href = bar.getAttribute("action") + "?f=1";
  }

  function restoreFilter() {
    const bar = filterBar();
    if (!bar) return;
    let open = null;
    try { open = sessionStorage.getItem(FILTER_OPEN); } catch (err) { /* fine */ }
    if (open && open === viewKey()) bar.hidden = false;
  }

  // ---- The bookmarked filters ------------------------------------------
  //
  // Nine bookmarks under the digits — a view and a line each — and ctrl-0 is
  // the nine of them on the screen (design.md, "Bookmarked filters"). One key
  // with two answers, and the filter line decides which: with a filter up, the
  // digit keeps it; with no filter up, the digit goes to what is kept. That is
  // one idea said from whichever end you are standing at — this digit and this
  // filter belong together — rather than two meanings on one key, and it is
  // what makes a bookmark cost one press in each direction with no mode to
  // remember. `g 1`…`g 9` is the same going, said the way every other going in
  // the app is said, and it works where the chord cannot: a screen with no
  // filter line has no `^N`, and a bookmark that carries its own view does not
  // need one.
  //
  // The nine live on the server, like the panels and the per-view filter sets.
  // Nothing is kept in here: the dialog the server rendered is where the lines
  // are read from, the way every other key reads the page rather than a copy
  // of it.

  function bookmarksDialog() { return document.getElementById("bookmarks-dialog"); }

  function bookmarkRow(n) {
    const dlg = bookmarksDialog();
    return dlg ? dlg.querySelector('[data-slot="' + n + '"]') : null;
  }

  function bookmarkLine(n) {
    const row = bookmarkRow(n);
    return row ? row.dataset.line : "";
  }

  function anyBookmark() {
    const dlg = bookmarksDialog();
    return !!(dlg && dlg.querySelector('[data-slot]:not([data-line=""])'));
  }

  // The filter that is on the screen, as a line — the one applied, never the
  // one half-typed. The box's defaultValue is the last line the list was
  // actually narrowed by (liveApply writes it there), so what a bookmark
  // keeps is always the filter being looked at.
  function liveFilter() {
    const bar = filterBar(), box = filterBox();
    if (!bar || !box || bar.hidden) return "";
    return box.defaultValue.trim();
  }

  // Going to one opens the view the bookmark holds with the line it holds,
  // which is exactly the request typing that line on that view would make — so
  // a bookmark leaves the view in the state a typed filter leaves it in:
  // narrowed, remembered for the view, and with the bar up saying so.
  //
  // A slot kept before a bookmark knew its view holds a line and no view; that
  // one is still applied here, where it always was, which is what it meant
  // when it was kept. With no view and no filter line on this screen either
  // there is nowhere for it to land, and the press does nothing.
  function goToBookmark(n) {
    const row = bookmarkRow(n);
    if (!row || !row.dataset.line) return false;
    const bar = filterBar();
    const to = row.dataset.view ? "/" + row.dataset.view : bar && bar.getAttribute("action");
    if (!to) return false;
    window.location.href = to + "?f=1&q=" + encodeURIComponent(row.dataset.line);
    return true;
  }

  // Keeping one must not move the screen: the caret is usually still in the
  // filter line when the digit is pressed, and a page that reloaded under it
  // would cost the line being typed. So it is the write and nothing else —
  // the same shape the token box uses to learn a name — and the row is filled
  // in from what the server stored rather than from what was sent, since the
  // server is what decides how a line reads once it is a filter set — and
  // which view the bookmark now opens, which the form carries and the key
  // layer never works out for itself.
  function writeBookmark(n, line) {
    const form = document.querySelector("[data-bookmark-save]");
    if (!form) return;
    const view = form.querySelector("[name=view]");
    fetch(form.getAttribute("action"), {
      method: "POST",
      headers: { "Accept": "application/json", "Content-Type": "application/x-www-form-urlencoded" },
      body: new URLSearchParams({ slot: String(n), view: view ? view.value : "", q: line }).toString(),
    }).then(function (res) {
      if (!res.ok) throw new Error(String(res.status));
      return res.json();
    }).then(function (kept) {
      showBookmark(kept.slot, kept.view, kept.name, kept.line);
    }).catch(function () { /* nothing was stored, and nothing on screen says it was */ });
  }

  // The row, saying what the slot now holds — the view it opens and the line
  // it opens it with, in that order and drawn the way the layout draws them.
  // An empty slot still reads as a slot, because the empty ones are the answer
  // to "where does the next one go" (see the dialog in the layout).
  function showBookmark(n, view, name, line) {
    const row = bookmarkRow(n);
    if (!row) return;
    row.dataset.line = line;
    row.dataset.view = view || "";
    const text = row.querySelector(".line");
    text.textContent = "";
    if (line) {
      if (name) {
        const where = document.createElement("span");
        where.className = "in";
        where.textContent = name;
        text.appendChild(where);
      }
      text.appendChild(document.createTextNode(line));
    } else {
      const none = document.createElement("span");
      none.className = "empty";
      none.textContent = "empty";
      text.appendChild(none);
    }
    let clear = row.querySelector("[data-bookmark-clear]");
    if (line && !clear) {
      clear = document.createElement("button");
      clear.type = "button";
      clear.className = "clear";
      clear.setAttribute("data-bookmark-clear", "");
      clear.tabIndex = -1;
      clear.title = "clear (\u232b)";
      clear.textContent = "\u232b";
      row.appendChild(clear);
    } else if (!line && clear) clear.remove();
    renderKeybar();
  }

  // One press of a digit, wherever it was pressed: ctrl-N on the view, and the
  // bare digit inside the dialog, which is the same question asked from the
  // list of answers. A view with no filter line has neither a filter to keep
  // nor anywhere to put one, so the key is not the app's there and is never
  // offered.
  function pressBookmark(n) {
    if (!filterBar()) return false;
    const line = liveFilter();
    if (line) { writeBookmark(n, line); return true; }
    if (!bookmarkLine(n)) return false;
    return goToBookmark(n);
  }

  function openBookmarks() {
    const dlg = bookmarksDialog();
    if (!dlg || dlg.open || !filterBar()) return false;
    dlg.showModal();
    select(rows()[0]);
    renderKeybar();
    return true;
  }

  // The cursor is given back on the way out: these rows stay in the page while
  // the dialog is shut, and a selection left on one would be a cursor sitting
  // on something nobody can see.
  function closeBookmarks() {
    const dlg = bookmarksDialog();
    if (!dlg || !dlg.open) return;
    select(null);
    dlg.close();
    renderKeybar();
  }

  // Enter, and leaving the box: the moments to ask about what the list has
  // been leaving out. With nothing to ask, what is typed is applied now
  // rather than after the pause.
  function applyFilter() {
    const bar = filterBar();
    if (!bar) return;
    hideSuggest();
    const bad = problemsIn(filterBox());
    if (bad.length) { askAbout(filterBox(), bad[0], applyFilter); return; }
    liveApply();
  }

  // Leaving is read a tick later, once the focus has landed: a dialog opening
  // over the box, the window losing focus and the bar being closed are not
  // leaving the line, and asking then would be a question about nothing.
  document.addEventListener("focusout", function (e) {
    const box = filterBox();
    if (!box || e.target !== box) return;
    setTimeout(function () {
      if (!document.hasFocus() || document.activeElement === box || topDialog()) return;
      if (filterBar().hidden) return;
      const bad = problemsIn(box);
      if (bad.length) askAbout(box, bad[0], applyFilter);
    }, 0);
  });

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
    "filter-completed": {
      "no-context": function (p) { return "this view filters by when something was finished, by tag and by name; " + p.text + " has nothing to narrow here"; },
      "not-here": function (p) { return "this view filters by when something was finished, by tag and by name; " + p.text + " has nothing to narrow here"; },
      "bad-date": function (p) { return p.text + " is not a day or a period — write it as completed:2026-09-13, completed:yesterday, completed:monday, completed:month or completed:3weeks"; },
    },
    // an idea carries the area it is about and nothing else — not a field an
    // action has, and not a date: the two refusals differ only in the wording
    // of what it is being told to leave off
    someday: {
      "no-context": function (p) { return "a someday/maybe item has no context; " + p.text + " belongs on the action it becomes"; },
      // a date token has no sigil, which is what tells the two apart
      "not-here": function (p) {
        if (p.sigil === "") return "a someday/maybe item has no dates; it waits on the list until you decide about it";
        return "a someday/maybe item has no " + p.text + "; that belongs on the action it becomes";
      },
    },
    // the Scheduler's line: a schedule is text and a rule, and carries no
    // name of any kind. Someday/Maybe used to say this too, until an idea
    // started carrying the area of responsibility it belongs to
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

  // A dialog that keeps its rows between openings — the bookmarks — leaves
  // them in the page while it is shut, and j/k on the list behind would
  // otherwise step through nine rows nobody can see. The same guard covers
  // the answers left behind in the unknown-name and link choosers.
  function rows() {
    return Array.from(rowScope().querySelectorAll("[data-kb-row]")).filter(function (r) {
      const d = r.closest("dialog");
      return !d || d.open;
    });
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
  //
  // A view's name is not enough on its own: a screen inside a view borrows it,
  // so the project page opened from Next calls itself `next` exactly as the
  // Next list does — and the two have different lists on them. Completing the
  // last action of a project from Next then handed the cursor to the project's
  // own action list, which put it on the action just completed and took the
  // screen's own Done off the bar with it. `data-step` is the app's existing
  // answer to "is this the view's list, or a screen inside it" — the trail's
  // second crumb, set by ui.go's step() — carried onto the pane.
  function viewKey() {
    const pane = document.querySelector(".pane[data-view]");
    if (!pane) return location.pathname;
    const view = pane.dataset.view || location.pathname;
    return pane.hasAttribute("data-step") ? view + "/step" : view;
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
    // where the list was left goes with the row: the answer is a new page and
    // the offset would go with the old one
    handPlaceOn();
  }

  // Acting on a row must not cost the place in the list either. `main` is the
  // scrollport, not the window (see style.css), so a page that arrives is a
  // brand-new `main` with a scrollTop of zero: on a list taller than the
  // window, picking an item halfway down answered by throwing the screen back
  // to the first row. The selection cannot carry this on its own — a click on
  // the pick dot or the checkbox is that control's click and does not take the
  // cursor (see implementation.md, "Item lines") — so the offset is a
  // handover of its own, stored and claimed by the same rules.
  const PLACE = "kb-place-handover";

  function handPlaceOn() {
    const main = document.querySelector("main");
    if (!main) return;
    try {
      sessionStorage.setItem(PLACE, JSON.stringify({ view: viewKey(), top: main.scrollTop }));
    } catch (err) { /* no session storage: the place is lost, nothing else is */ }
  }

  // The two handovers are taken back the same way, and only where they were
  // claimed or found to be nonsense — a stored line that will not parse is not
  // something to keep offering the next page.
  function dropPlace() {
    try { sessionStorage.removeItem(PLACE); } catch (err) { /* nothing to undo */ }
  }

  function dropSelection() {
    try { sessionStorage.removeItem(HANDOVER); } catch (err) { /* nothing to undo */ }
  }

  function claimPlace() {
    let raw = null;
    try { raw = sessionStorage.getItem(PLACE); } catch (err) { return; }
    if (!raw) return;
    // a page with no list cannot claim a place in one, and must not swallow it
    // either — the same rule the selection is claimed under, for the same
    // screen: doing is reached from a row and comes straight back to the list
    if (!rows().length) return;
    let want;
    try { want = JSON.parse(raw); } catch (err) { dropPlace(); return; }
    // and only on the screen it was handed from: completing an action can
    // answer with the project page, where an offset down the list it was
    // completed on means nothing. It is not cleared there either, for the
    // reason a screen with no list does not clear it — see claimSelection
    if (!want || want.view !== viewKey()) return;
    dropPlace();
    const main = document.querySelector("main");
    // a list that is now shorter clamps this itself, which is the answer
    // wanted: the end of what is left rather than an offset past it
    if (main) main.scrollTop = want.top;
  }

  function claimSelection() {
    let raw = null;
    try { raw = sessionStorage.getItem(HANDOVER); } catch (err) { return; }
    if (!raw || selected()) return;
    // a page with no rows cannot claim it, and must not swallow it either:
    // doing is a screen you go to from a row and come straight back to it, and
    // the row is expected to be where it was (design.md, "Doing one action")
    if (!rows().length) return;
    let want;
    try { want = JSON.parse(raw); } catch (err) { dropSelection(); return; }
    // Only the list it was handed from claims it, and only that list clears
    // it. Completing an action can answer with the project page, which has a
    // list of its own — the project's actions — and a row position from the
    // list you were working means nothing on it: restoring the cursor there
    // would be the app choosing an item nobody pointed at. Throwing it away
    // there would be as wrong, and for the reason a screen with no rows keeps
    // it: you are one press from being back on the list it belongs to, and it
    // is expected to still be where you left it (design.md, "Doing one
    // action")
    if (!want || want.view !== viewKey()) return;
    dropSelection();
    const all = rows();
    let row = want.href && all.find(function (r) { return r.dataset.href === want.href; });
    // the row can be gone, which is what completing one does. Then the
    // selection belongs where it stood: the item that took its place is under
    // the cursor and the list can be worked straight down without touching j
    if (!row && typeof want.i === "number" && want.i >= 0) row = all[Math.min(want.i, all.length - 1)];
    if (row) select(row);
  }

  // A row the server says to arrive on: the name Settings has just created,
  // so that the key after ctrl-enter is already about it. Only when nothing
  // else claimed the cursor, and only a row the page marked.
  function arrive() {
    if (selected()) return;
    const row = document.querySelector("[data-kb-arrive]");
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
    if (!row) return;
    // The mark is flipped where it stands whether the key or the pointer
    // pressed it — one path, so the two cannot come to do different things.
    // Stopped in the capture phase, like the filter line above: preventDefault
    // stops the browser, and hx-boost would otherwise send the form itself and
    // swap the whole page back under the walk.
    if (e.target.classList && e.target.classList.contains("kb-review")) {
      e.preventDefault();
      e.stopPropagation();
      flipReview(row);
      return;
    }
    // the place is kept whichever row was pressed, though: the cursor is a
    // thing you put somewhere and the scroll offset is not — a dot clicked
    // without one is still a list you are standing halfway down
    handPlaceOn();
    if (row === selected()) handSelectionOn(row);
  }, true);

  // ctrl-j and ctrl-k are j and k with the letters spoken for: the form vim
  // itself uses, and the one the project picker already took. In a list they
  // move, like j and k do. From the filter line they are the way out of the box
  // and into the list it narrows — j and k cannot be, being letters the line is
  // typed in — and the caret goes with them, or the next j would be typed into
  // the filter. In any other box they are left alone: nothing is under a meta
  // line to move to, and ctrl-k is the line's own kill-to-end on this machine.
  function moveKey(e, delta) {
    if (!rows().length) return false;
    const box = filterBox();
    if (box && e.target === box) {
      hideSuggest();
      box.blur();
      move(delta);
      return true;
    }
    if (typing(e)) return false;
    move(delta);
    return true;
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
    // What was typed against where it was typed from — and it holds until a
    // key says otherwise. Unmodified keys only: with ctrl or cmd held a
    // browser may report the Latin letter it would match an accelerator
    // against rather than the one the layout types, and believing that would
    // put the marker out on every ^v.
    if (/^Key/.test(e.code) && !e.ctrlKey && !e.metaKey && !e.altKey && layoutMarker()) {
      setLayout(CYRILLIC.test(e.key));
    }

    // While a jump is pending the next key is the jump and nothing else, so
    // this is read before every other key on the page.
    if (jPending) {
      // a modifier pressed on its own is not an answer, so it does not count
      // as one: holding shift to reach a key must not throw the jump away
      if (e.key === "Shift" || e.key === "Control" || e.key === "Alt" || e.key === "Meta") return;
      const hint = keyOf(e);
      const to = hint.length === 1 ? jumpMap[hint] : null;
      setJumping(false);
      // esc puts the hints away and leaves everything else alone — in a dialog
      // that means the dialog stays, since the browser would otherwise take
      // the same key as "close me"
      if (e.key === "Escape") { e.preventDefault(); return; }
      if (e.ctrlKey || e.metaKey || e.altKey) return;
      if (to) { e.preventDefault(); jumpTo(to); }
      return;
    }

    // And the same for the view jump, one level out: while `g` is pending the
    // next key is the jump and nothing else, so this is read before every
    // other key on the page too. It has to be. Four of the fourteen letters
    // are also buttons — `d` is Done and the Dashboard, and `t`, `r` and `w`
    // are spent twice the same way (keys.md, "The map") — and the prefix is
    // the whole of what makes that safe, which it can only be from in front of
    // the keys it is protecting. Read after them, as it was, `g d` completed
    // the row under the cursor and went nowhere, on every screen with a
    // selection: the row commands sit both above the ctrl guard and below it,
    // so there is no single line this could have been slipped in behind.
    if (gPending) {
      // a modifier pressed on its own is not an answer, so it does not count
      // as one: holding shift to reach a key must not throw the jump away
      if (e.key === "Shift" || e.key === "Control" || e.key === "Alt" || e.key === "Meta") return;
      setPending(false);
      // navigation is bare in every mode (keys.md, "The map"), so a chord is
      // not a jump. It is spent taking the overlay away and does nothing else
      // — the same answer ctrl-m gives a chord pressed into its hints
      if (e.metaKey || e.ctrlKey || e.altKey) return;
      const to = keyOf(e);
      if (to === "g") { e.preventDefault(); openCapture(); return; }
      // a bookmark is a going like any other now that it holds its own view
      // (design.md, "Bookmarked filters"), so it answers the key the app says
      // "go to" with. An empty slot is a destination that does not exist and
      // the press is spent on nothing — left to the browser rather than
      // swallowed, which is what an unbound letter after `g` already does
      if (/^[1-9]$/.test(to)) {
        if (goToBookmark(Number(to))) e.preventDefault();
        return;
      }
      const dest = jumps[to];
      if (dest) {
        e.preventDefault();
        goTo(dest, false);
      }
      return;
    }

    // ctrl-v is the panels, from anywhere: the chooser if it is shut, and zen
    // if it is already up — the second press is the answer wanted most often,
    // and the dialog is a list of four keys rather than a place to be. Ctrl
    // and not cmd, because cmd-v is paste in every box on this machine and a
    // key of the app's must not take that away.
    if (e.ctrlKey && !e.metaKey && !e.altKey && keyOf(e) === "v") {
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
    if (e.ctrlKey && !e.metaKey && !e.altKey && keyOf(e) === "f") {
      const bar = filterBar();
      if (!bar) return; // a view with no box leaves ctrl-f to the browser
      e.preventDefault();
      if (bar.hidden) openFilter(); else closeFilter();
      return;
    }

    // The bookmarked filters: ctrl-0 puts the nine on the screen, ctrl-1 to
    // ctrl-9 keep the filter that is up or go to the one kept (keys.md, "The
    // map"). Globals, so ctrl in every mode — and read here, beside the two
    // other keys that are about the filter rather than about the list.
    //
    // Not while a dialog is up: a dialog owns the keyboard, and the filter
    // these are about is on the page behind it. The bookmarks dialog's own
    // digits are handled where the rest of its keys are.
    if (e.ctrlKey && !e.metaKey && !e.altKey && /^[0-9]$/.test(keyOf(e)) && !topDialog()) {
      const digit = Number(keyOf(e));
      const did = digit === 0 ? openBookmarks() : pressBookmark(digit);
      // a digit with nothing to keep and nothing kept is not the app's key,
      // and is left to the browser rather than swallowed to do nothing
      if (did) { e.preventDefault(); return; }
    }

    // A screen key that asks for ctrl is live wherever the screen is, text
    // boxes included — reaching it without leaving the field is the whole
    // point of the modifier, and the reason a screen would choose one. Not
    // while a dialog is up: a dialog owns the keyboard, and the control the
    // key presses is on the page behind it.
    if (e.ctrlKey && !e.metaKey && !e.altKey && !document.querySelector("dialog[open]")) {
      const ctrlBranch = branchFor(e);
      if (ctrlBranch) { e.preventDefault(); press(ctrlBranch); return; }
      // and the row and screen commands with it, which in modifier mode are
      // ctrl keys like any other. Letters only: the delete key is never
      // modified, and reading it here would take backspace away from the box
      // being typed in.
      if (keyOf(e).length === 1 && rowCommand(e)) { e.preventDefault(); return; }
    }

    // ^o follows a link in the item under the cursor. Read here rather than
    // left to the declared-key layer above, because what it presses is not a
    // control on the page: it is the item the selection is on, which changes
    // with every j and k. After that layer, so a screen wanting ^o for
    // something of its own would keep it — and, like it, not while a dialog is
    // up, since the item is on the page behind one.
    if (e.ctrlKey && !e.metaKey && !e.altKey && keyOf(e) === "o" &&
        !document.querySelector("dialog[open]")) {
      // an item with no link leaves the key to the browser rather than
      // swallowing it to do nothing
      if (followLink()) e.preventDefault();
      return;
    }

    // ctrl-m marks every control on the screen with a letter and takes the
    // next key as the one to go to. It is read after the screen's own declared
    // keys, so a screen that wanted ^m for something of its own would keep it,
    // and it is not inside the block above because that one stands down for a
    // dialog — a form in a dialog is exactly where a jump is wanted.
    if (e.ctrlKey && !e.metaKey && !e.altKey && keyOf(e) === "m") {
      if (setJumping(true)) e.preventDefault();
      return;
    }

    if (e.ctrlKey && !e.metaKey && !e.altKey && (keyOf(e) === "j" || keyOf(e) === "k")) {
      if (moveKey(e, keyOf(e) === "j" ? 1 : -1)) e.preventDefault();
      return;
    }

    const dlg = captureDialog();
    if (dlg && dlg.open) {
      // The dialog owns the keyboard while it is up, and both of its keys are
      // handled here rather than left to the browser: a modal <dialog> closes
      // itself on Escape and a lone text field submits itself on Enter, but
      // both are UA behaviours with edge cases, and these two keys are the
      // whole interaction.
      if (e.key === "Escape") { e.preventDefault(); closeCapture(dlg); }
      // shift-enter is the newline, because an item's text may run to more
      // than one line (design.md, "Inbox item") and Enter is what adds. The
      // modifier is on the second line rather than on adding: one line is the
      // common case, and moving which key adds would cost more than the second
      // line is worth. Left to the browser, which inserts it and fires the
      // input event the box grows on
      if (e.key === "Enter" && e.shiftKey) return;
      if (e.key === "Enter") { e.preventDefault(); submitCapture(dlg); }
      return;
    }
    const linkdlg = linksDialog();
    if (linkdlg && linkdlg.open && topDialog() === linkdlg) {
      if (e.key === "Escape") { e.preventDefault(); linkdlg.close(); renderKeybar(); return; }
      // the answers are a list, and a list is moved through with j and k —
      // which is exactly why neither is one of the letters handed out
      const step = keyOf(e);
      if (step === "j" || step === "k") { e.preventDefault(); move(step === "j" ? 1 : -1); return; }
      if (e.key === "Enter") {
        const row = selected();
        if (row) { e.preventDefault(); row.click(); }
        return;
      }
      const one = branchFor(e);
      if (one) { e.preventDefault(); press(one); return; }
      if (!e.ctrlKey && !e.metaKey && !e.altKey) e.preventDefault();
      return;
    }
    const unknown = unknownDialog();
    if (unknown && unknown.open && topDialog() === unknown) {
      if (e.key === "Escape") { e.preventDefault(); unknown.close(); renderKeybar(); return; }
      // the answers are a list, and a list is moved through with j and k. The
      // letters on them stay: a key that goes straight to an answer is worth
      // having on a question asked this often, and neither costs the other
      const step = keyOf(e);
      if (step === "j" || step === "k") { e.preventDefault(); move(step === "j" ? 1 : -1); return; }
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
    const bmk = bookmarksDialog();
    if (bmk && bmk.open) {
      // A digit here means what it means outside: keep this filter, or go to
      // that one. The dialog is where the nine are read, so it must not be a
      // second set of rules about them.
      if (e.key === "Escape") { e.preventDefault(); closeBookmarks(); return; }
      const k = keyOf(e);
      if (k === "j" || k === "k") { e.preventDefault(); move(k === "j" ? 1 : -1); return; }
      // the delete key empties a slot, the way it removes a row anywhere else.
      // The row itself stays: a slot is a place, and the nine places are the
      // whole map — an empty one is where the next bookmark goes
      if (e.key === "Backspace" || e.key === "Delete") {
        const row = selected();
        if (row && row.dataset.line) { e.preventDefault(); writeBookmark(row.dataset.slot, ""); }
        return;
      }
      if (e.key === "Enter") {
        const row = selected();
        if (row) { e.preventDefault(); pressBookmark(Number(row.dataset.slot)); }
        return;
      }
      if (!e.ctrlKey && !e.metaKey && !e.altKey) {
        if (/^[1-9]$/.test(k)) { e.preventDefault(); pressBookmark(Number(k)); return; }
        e.preventDefault();
      }
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
        if (on) { e.preventDefault(); takeSuggest(box, on.dataset.name, on.dataset.insert); return; }
      }
      // enter applies the filter line; on a meta line it submits the form it
      // is in, which is the browser's own answer and is checked on the way
      // out like any other submit
      if (e.key === "Enter" && !e.ctrlKey && !e.metaKey && box === filterBox()) { e.preventDefault(); applyFilter(); return; }
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
        // the filter line's form is the filter, and applying it is plain
        // Enter's; what ctrl-enter commits there is the name the line spells
        if (e.target === filterBox()) { const make = createForm(); if (make) press(make); return; }
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
      const make = createForm();
      if (make) { e.preventDefault(); press(make); return; }
      const row = selected();
      const here = (row && row.closest("form")) ||
        (document.activeElement && document.activeElement.closest &&
          document.activeElement.closest("form"));
      if (here) { e.preventDefault(); submitScope(here); }
      return;
    }
    // The row and screen commands, read before the ctrl guard: in modifier
    // mode `d`, `t` and `b` arrive with ctrl held.
    if (!e.altKey && !e.metaKey && rowCommand(e)) { e.preventDefault(); return; }

    if (e.metaKey || e.ctrlKey || e.altKey) return;

    // a key the page declares beats the standing map: on a screen that has
    // its own answers, those are what the letters mean there
    const branch = branchFor(e);
    if (branch) { e.preventDefault(); press(branch); return; }

    const row = selected();
    switch (keyOf(e)) {
      case "g": e.preventDefault(); setPending(true); break;
      // shift moves the row itself rather than the cursor, which is what a
      // draft needs now that `u` and `d` are Undone and Done
      case "j":
        e.preventDefault();
        if (e.shiftKey && row && row.hasAttribute("data-draft")) moveDraft(row, 1);
        else move(1);
        break;
      case "k":
        e.preventDefault();
        if (e.shiftKey && row && row.hasAttribute("data-draft")) moveDraft(row, -1);
        else move(-1);
        break;
      case "Enter":
      case "o": {
        if (row && row.hasAttribute("data-draft")) { e.preventDefault(); openDraft(row); break; }
        if (row && row.hasAttribute("data-newproject")) { e.preventDefault(); newProjectDialog(); break; }
        const radio = row && row.querySelector("input[type=radio]");
        if (radio) { e.preventDefault(); radio.checked = true; renderKeybar(); break; }
        if (row && row.dataset.href) { e.preventDefault(); goTo(row.dataset.href, false); }
        break;
      }
      case "z": {
        // Inbox Zero is only p over and over: the same screen, fed the oldest
        // item each time instead of the selected one.
        if (document.querySelector("[data-inbox-zero]")) { e.preventDefault(); goInboxZero(); }
        break;
      }
      case "?": {
        e.preventDefault();
        toggleHelp();
        break;
      }
      case "Escape": {
        const help = document.getElementById("help");
        if (help && !help.hidden) { help.hidden = true; renderKeybar(); break; }
        // esc unwinds one step and never navigates: leaving a screen is `b`
        // (keys.md, "Leaving a screen"). It used to close things and then,
        // with nothing left to close, follow data-cancel — two meanings on
        // one key, which is tolerable until the keyboard is modal and leaving
        // a box is the commonest press in the app.
        select(null);
        // with the filter line up, leaving the list goes back to the line: the
        // way back from ctrl-j, and the same one step out that esc in the box
        // already is. With the bar down there is nowhere to go back to
        const bar = filterBar();
        if (bar && !bar.hidden) {
          e.preventDefault();
          const box = filterBox();
          box.focus();
          box.setSelectionRange(box.value.length, box.value.length);
          renderKeybar();
        }
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
    if (e.target.closest && e.target.closest("form")) renderKeybar();
    const row = e.target.closest && e.target.closest("[data-kb-row]");
    if (row && e.target.type === "radio" && !row.classList.contains("kb-selected")) select(row);
  });

  // a click or a lost window abandons a half-typed "g" sequence
  document.addEventListener("click", function (e) {
    setPending(false);
    setJumping(false);
    if (e.target.closest("[data-capture-open]")) { e.preventDefault(); openCapture(); return; }
    if (e.target.closest("[data-timer]")) { e.preventDefault(); toggleTimer(); return; }
    const pick = e.target.closest(".fsuggest li");
    if (pick) {
      e.preventDefault();
      const box = pick.closest(".fbox").querySelector("[data-tokenbox]");
      box.focus();
      takeSuggest(box, pick.dataset.name, pick.dataset.insert);
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
    // the bookmarks dialog: clicking a row is pressing its digit, and the
    // mark beside a full one empties it — the same two things its keys do
    const wipe = e.target.closest("[data-bookmark-clear]");
    if (wipe) {
      e.preventDefault();
      const full = wipe.closest("[data-slot]");
      if (full) writeBookmark(full.dataset.slot, "");
      return;
    }
    const slot = e.target.closest("#bookmarks-dialog [data-slot]");
    if (slot) { e.preventDefault(); pressBookmark(Number(slot.dataset.slot)); return; }
    if (e.target.closest("[data-draft-add]")) { e.preventDefault(); openDraft(null); return; }
    if (e.target.closest("[data-newproject]")) { e.preventDefault(); newProjectDialog(); return; }
    const row = rowFromEvent(e);
    if (row) select(row);
  });
  window.addEventListener("blur", function () { setPending(false); setJumping(false); });
  // the hints are placed in viewport coordinates, so a scroll would leave them
  // behind the controls they name
  window.addEventListener("scroll", function () { setJumping(false); }, true);

  // double click is the mouse's Enter: on the Inbox that is processing the
  // item, everywhere else it is opening it — the same data-href either way
  document.addEventListener("dblclick", function (e) {
    const row = rowFromEvent(e);
    if (row && row.dataset.href) { e.preventDefault(); goTo(row.dataset.href, false); }
  });

  // Actions written before their project exists (the project branch of
  // processing). They are held in the form as rows of hidden fields rather
  // than saved as they are written, because there is nothing yet to save them
  // to: design.md will not make a project without an action, so the project
  // and its actions are created in the one submit. A refused form therefore
  // has to carry them back, which is why they are form fields and not state
  // kept in here — see implementation.md, "Writing a project".
  function draftDialog() { return document.getElementById("draft-dialog"); }

  // The form a draft row belongs to. Not `row.closest("form")` any more: on a
  // project's own page the form element holds the project's fields and the
  // list sits below it, joined by name — the same arrangement the open next
  // action's boxes use. One form per screen that writes a plan, so asking the
  // document is asking the right thing.
  function draftForm() { return document.querySelector("form[data-drafts]"); }

  function draftValues(row) {
    return {
      title: row.querySelector("[name=atitle]").value,
      meta: row.querySelector("[name=ameta]").value,
      description: row.querySelector("[name=adescription]").value,
    };
  }

  // What a draft's meta line says, drawn as the badges an action row wears.
  // `actionrow` stays the one definition of what a row of this list looks
  // like: this paints the same classes with the same words, off a line that
  // has not been saved and so has no action behind it to ask. A token the
  // notation does not know is left out rather than shown — the box it was
  // typed in has already marked it, and a row is not where it gets fixed.
  function paintDraft(row) {
    const into = row.querySelector(".draftbadges");
    if (!into) return;
    const v = draftValues(row);
    row.querySelector(".title").textContent = v.title;
    into.textContent = "";
    const add = function (cls, text, title) {
      const b = document.createElement("span");
      b.className = cls ? "badge " + cls : "badge";
      b.textContent = text;
      if (title) b.title = title;
      into.appendChild(b);
    };
    tokensOf(v.meta.trim()).forEach(function (t) {
      const wait = /^@waitingFor\((.+)\)$/.exec(t);
      if (wait) { add("wait", "→ " + wait[1]); return; }
      if (t.charAt(0) === "@") { add("ctx", t); return; }
      if (t === "#short" || t === "#medium" || t === "#long") { add("", t.slice(1)); return; }
      if (t === "#focus") { add("focus", "★", "needs focus"); return; }
      // the list rows leave #today out of the badges too: the dot says it,
      // and here there is no dot to press yet
      if (t === "#today") return;
      if (t.charAt(0) === "#") { add("tag", t); return; }
      const due = /^due:(.+)$/.exec(t);
      if (due) { add("due", "due " + due[1]); return; }
      const zzz = /^snooze:\(?(.+?)\)?$/.exec(t);
      if (zzz) { add("", "zzz until " + zzz[1]); return; }
    });
    // the one thing about a draft the badges cannot say, because an action
    // row has nothing to say it with either: that there is a description
    if (v.description.trim()) add("", "note");
  }

  function writeDraft(row, v) {
    row.querySelector("[name=atitle]").value = v.title;
    row.querySelector("[name=ameta]").value = v.meta;
    row.querySelector("[name=adescription]").value = v.description;
    paintDraft(row);
  }

  // Adding or removing a row changes nothing the gate reads — the action a
  // project cannot be without is the open box above the list, and that is a
  // required field like any other (see implementation.md, "Writing a
  // project"). What is left is re-reading the form, because the row keys the
  // bar offers depend on what is in the list.
  function syncDrafts(form) {
    if (!form) return;
    document.querySelectorAll("[data-draft-list] [data-draft]").forEach(paintDraft);
    gate(form);
    renderDirty();
    renderKeybar();
  }

  // A draft moves among the drafts and no further. On a project's own page
  // the list holds the plan's saved rows as well, and those are the project's
  // order rather than this form's — a row that could be shuffled past them
  // would be claiming to reorder something this Save does not write.
  function moveDraft(row, delta) {
    const other = delta < 0 ? row.previousElementSibling : row.nextElementSibling;
    if (!other || !other.hasAttribute("data-draft")) return;
    if (delta < 0) row.parentNode.insertBefore(row, other);
    else row.parentNode.insertBefore(other, row);
    row.scrollIntoView({ block: "nearest" });
    renderDirty();
    renderKeybar();
  }

  function removeDraft(row) {
    const form = draftForm();
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
    const form = draftForm();
    const title = dlg.querySelector("[name=title]");
    const meta = dlg.querySelector("[name=meta]");
    const desc = dlg.querySelector("[name=description]");
    const ok = dlg.querySelector("[data-draft-ok]");
    // Nothing is seeded here. The capture's body goes to the project's first
    // action, which the screen now shows open in its own boxes with the body
    // already in it (design.md, "Inbox Zero") — this dialog only ever writes
    // an action after that one, and those open empty.
    const v = row ? draftValues(row) : { title: "", meta: "", description: "" };
    title.value = v.title; meta.value = v.meta; desc.value = v.description;
    dlg.querySelector("h2").textContent = row ? "Edit action" : "Add an action";
    ok.textContent = row ? "Save action" : "Create action";
    // the trail says which of the two this is, the way it does for every other
    // step: writing an action into a plan is a place, and editing one already
    // in it is a different place (implementation.md, "Panels")
    dlg.dataset.crumb = row ? "Edit action" : "Create action";

    gate(dlg);
    dlg.showModal();
    growAll(dlg);
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

  // The project's tags, onto its next action — `#` on the project's page, the
  // third mark on the Next action heading (design.md, "Editing items").
  //
  // It is done in here and nothing is posted, for the reason the review mark
  // is: the answer is a box on this page redrawn where it stands. Both lines
  // belong to the one form and neither is saved until Save, so this moves a
  // draft into a draft — and a press you did not want costs nothing but a
  // second press or leaving without saving.
  //
  // Which form is said by the button (`data-copy-meta` holds its id) rather
  // than looked up by shape, and the boxes are read off `form.elements`: the
  // action's line sits outside the form element and belongs to it by its own
  // `form` attribute (see fieldsIn).
  // The first field of a name, which is what a repeated name means here: the
  // plan's rows carry `ameta` too, and the open boxes are the first of that
  // list everywhere — in the browser's submission order and in the reader on
  // the server. `namedItem` hands back a list once there is more than one,
  // and a list's own `.value` is the radio-group answer, which is "" for
  // boxes like these — so asking it directly would quietly read nothing.
  function firstNamed(form, name) {
    const got = form.elements.namedItem(name);
    if (!got) return null;
    return got.tagName === undefined && got.length !== undefined ? got[0] : got;
  }

  function metaBoxes(btn) {
    const form = document.getElementById(btn.dataset.copyMeta);
    if (!form || !form.elements) return null;
    const from = firstNamed(form, "meta");
    const into = firstNamed(form, "ameta");
    return from && into ? { form: form, from: from, into: into } : null;
  }

  function tokensOf(line) { return line.split(/\s+/).filter(Boolean); }

  // Tags only. A project's line holds tags and a snooze and nothing else
  // (design.md, "Writing a project"), and a snooze must not come down: on an
  // action it means "not workable yet", so copying one would park the very
  // action the project is waiting on.
  //
  // What it reads is the box, not what was saved: both lines are being written
  // right now, and the tag you have just typed on the project is the one you
  // want on the action.
  function projectTags(b) {
    return tokensOf(b.from.value).filter(function (t) { return t.charAt(0) === "#"; });
  }

  // One key, both ways, the way `t` and the review mark are — and with nothing
  // stored to say which: whether the action already carries every one of the
  // project's tags is read off the two lines. Null is nothing to copy, which
  // is a control that cannot be pressed and therefore a key the bar must not
  // offer (see keyUsable).
  function metaCopyMode(b) {
    const tags = projectTags(b);
    if (!tags.length) return null;
    const have = tokensOf(b.into.value);
    return tags.every(function (t) { return have.includes(t); }) ? "drop" : "take";
  }

  function copyMeta(btn) {
    const b = metaBoxes(btn);
    const mode = b && metaCopyMode(b);
    if (!mode) return;
    const tags = projectTags(b);
    let have = tokensOf(b.into.value);
    if (mode === "drop") {
      have = have.filter(function (t) { return !tags.includes(t); });
    } else {
      tags.forEach(function (t) { if (!have.includes(t)) have.push(t); });
    }
    b.into.value = have.join(" ");
    // What an `input` event would have done, minus the one thing it would also
    // have done: the completion list must not open, because nothing here is
    // being typed. The line is written back in a fixed order when it is saved,
    // so appending is all this has to get right (design.md, "Writing an
    // action").
    paintBox(b.into);
    renderDirty();
    syncMetaCopy();
    gate(b.form);
    renderKeybar();
  }

  // The mark says which way the next press goes, and the bar's entry says it
  // too — the same rule the theme row's `h theme dark` follows: an entry names
  // the answer it lands on, not the control it is.
  function syncMetaCopy() {
    document.querySelectorAll("[data-copy-meta]").forEach(function (btn) {
      const b = metaBoxes(btn);
      const mode = b && metaCopyMode(b);
      btn.disabled = !mode;
      btn.dataset.keyLabel = mode === "drop" ? "drop project tags" : "project tags";
      btn.title = mode === "drop"
        ? "take the project's tags off this action (#)"
        : "put the project's tags on this action (#)";
    });
  }

  document.addEventListener("click", function (e) {
    const btn = e.target.closest && e.target.closest("[data-copy-meta]");
    if (!btn) return;
    e.preventDefault();
    copyMeta(btn);
  });

  // The new project is held, not created: until the action form is submitted
  // there is no action to be its first, and a project without one is a thing
  // design.md will not make. So this dialog writes nothing — it carries the
  // two fields on to the form that will write both, which is the next step of
  // the same path and is why Create leaves the screen (design.md, "Inbox
  // Zero").
  function newProjectDialog() {
    const dlg = document.getElementById("newproject-dialog");
    if (!dlg || !dlg.dataset.npNext) return;
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
    growAll(dlg);
    title.focus();
    renderKeybar();

    function done(confirmed) {
      if (!confirmed) {
        dlg.close();
        renderKeybar();
        return;
      }
      if (!ready()) { (title.value.trim() === "" ? title : dod).focus(); return; }
      const to = dlg.dataset.npNext +
        "&newproject=" + encodeURIComponent(title.value.trim()) +
        "&newdod=" + encodeURIComponent(dod.value.trim());
      dlg.close();
      goTo(to, false);
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

  // What a page that has just arrived needs doing to it, whether it came from
  // the server on a boost or from a background refresh.
  function setupForms() {
    // the list as the server drew it, which is what "unsaved" is measured
    // against — remembered before anything in here touches it
    rememberDrafts();
    leaving = false;
    document.querySelectorAll("[data-drafts]").forEach(syncDrafts);
    renderDirty();
  }
  setupForms();
  gateAll();
  growAll();
  markVerbs();

  // Whether a background refresh may replace the list under you. The poll in
  // the layout is filtered on this, and it is deliberately generous about
  // what counts as busy: a refresh that is skipped costs 30 seconds of a
  // number being stale, and one that is not costs the row you were on.
  //
  // On window, and not in this closure, because htmx compiles a trigger
  // filter into a bare Function and resolves an unqualified name against the
  // event first and window second. It answers a strict boolean for the same
  // reason: htmx fires the request only when the filter returns exactly true.
  window.tkIdle = function () {
    // a dialog is a question waiting for an answer
    if (document.querySelector("dialog[open]")) return false;
    // the cursor is a claim on a row, and only esc gives it back
    if (document.querySelector("[data-kb-row].kb-selected")) return false;
    const el = document.activeElement;
    if (!el) return true;
    if (el.isContentEditable) return false;
    // a focused button is not work in progress — it is where the last click
    // left the focus, and treating it as busy would switch the refresh off
    // for the rest of the page's life
    return !/^(INPUT|TEXTAREA|SELECT)$/.test(el.tagName);
  };

  // A poll that arrives at the login page — the session gone, the redirect
  // followed — carries no <main> at all, and a select that matches nothing
  // swaps in nothing. Blanking the screen is a far worse answer than leaving
  // it as it was, so a poll only swaps a response that is a page.
  //
  // A poll is known by what it swaps, and not by the element that asked:
  // htmx's own `triggerEvent` overwrites `detail.elt` with whatever the event
  // is dispatched on, and a swap event is dispatched on the target — so `elt`
  // here is `main` or `nav` and never the poll div, which is how this test
  // used to pass on every response and blank the screen anyway. The two
  // targets are exact, everything else on the page being hx-boost and swapping
  // the body: `main` is the list poll and `nav` is the rail's.
  document.addEventListener("htmx:beforeSwap", function (e) {
    const target = e.detail && e.detail.target;
    if (!target || (target.tagName !== "MAIN" && target.tagName !== "NAV")) return;
    const said = e.detail.serverResponse || "";
    if (said.indexOf("<main>") < 0) { e.detail.shouldSwap = false; return; }
    // The list refresh replaces `main`, which is the scrollport, so the same
    // list arriving again would arrive at the top of itself. This is the one
    // page change nobody asked for — a timer asked for it — so it must move
    // the screen least of all. The place is handed on the way a row action
    // hands it, since the swap is what destroys the element holding it.
    if (target.tagName === "MAIN") handPlaceOn();
  });

  // hx-boost swaps the body, taking the rendered bar with it
  document.addEventListener("htmx:afterSwap", function (e) {
    // The rail refresh replaces the rail and nothing else. None of what
    // follows is about the rail, and some of it must not be redone on a
    // timer: startTimer() restarts from zero, so a doing screen left open
    // would count the same half-minute over and over. Two things do have to
    // follow the new count, and they are exactly the two the count is read
    // through elsewhere.
    const target = e && e.detail && e.detail.target;
    if (target && target.tagName === "NAV") {
      // "the inbox needs emptying" is read off the pane and not off the rail,
      // because the rail is a panel and can be off — so the fresh count has
      // to be carried the last step by hand. The rail is where the server
      // just said it; the pane stays the one place anything asks.
      const pane = document.querySelector(".pane");
      const said = target.dataset.inbox;
      if (pane && said !== undefined) {
        if (said === "0") pane.removeAttribute("data-inbox-full");
        else pane.setAttribute("data-inbox-full", "");
      }
      // the bar offers "z inbox zero" on that flag, so it is re-read now
      renderKeybar();
      return;
    }
    // the old page's timer is counting for a screen that is no longer here
    startTimer();
    wearTheme();
    restoreFilter();
    paintAll();
    renderKeybar(); setupForms(); gateAll(); markVerbs();
    growAll();
    // the place before the cursor: a row claimed onto a screen that is already
    // scrolled where it was is in view, and `scrollIntoView({block:"nearest"})`
    // on a row in view moves nothing
    claimPlace();
    claimSelection();
    arrive();
    // last, and after the cursor: an arriving effect plays on the item under
    // the cursor where there is one, and the cursor is only just here
    motionOver();
    claimArrival();
  });

  // A refused post must never be silent. htmx does not swap a 4xx, so a
  // handler that answers with a plain 400 leaves the screen exactly as it was
  // — the press looks like it did nothing at all, which is how an invalid
  // schedule rule read as a broken Create button. A screen that refuses on
  // purpose renders itself back with the reason (see the schedule forms);
  // this is the net under everything that has not been given that treatment,
  // and it says the server's own words rather than inventing any.
  document.addEventListener("htmx:responseError", function (e) {
    // no page is arriving, so the moment handed to one must not sit there
    // waiting to be claimed by whatever screen is reached next — and the keys
    // have to come back, on the screen that is still here
    try { sessionStorage.removeItem(ARRIVAL); } catch (err) { /* nothing to undo */ }
    motionOver();
    // nothing arrived, so the screen that was being left is still here and
    // still holds whatever was typed into it — the guard has to come back up.
    // A swap puts it back on settle; a refusal that swaps nothing would leave
    // it down for the rest of the page's life
    leaving = false;
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

  // A box that says data-grow is exactly as tall as what is in it: one line
  // when it is empty, one more for every line written into it. The height is
  // read off the content rather than counted in newlines, so a line that wraps
  // grows the box the same way a line that was typed does — which is what the
  // eye means by "another line" whichever way it arrived.
  //
  // It is the same shape as the caret rule below: it decides nothing and
  // stores nothing, and the server neither knows nor cares how tall the box
  // was (see implementation.md, "Stack").
  function growBox(el) {
    // height:auto first, or scrollHeight can only ever report the height the
    // box already has — a box that has grown could never shrink back
    el.style.height = "auto";
    // scrollHeight is the content box; the borders are what is left over
    el.style.height = (el.scrollHeight + el.offsetHeight - el.clientHeight) + "px";
  }

  // A box inside a dialog measures 0 while the dialog is closed, so growing
  // is done again when one opens rather than only when the page loads.
  function growAll(scope) {
    (scope || document).querySelectorAll("textarea[data-grow]").forEach(growBox);
  }

  document.addEventListener("input", function (e) {
    if (e.target.matches && e.target.matches("textarea[data-grow]")) growBox(e.target);
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
  claimPlace();
  claimSelection();
  arrive();
  claimArrival();
})();
