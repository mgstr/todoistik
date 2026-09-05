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
    if (on) showHints(); else clearHints();
    renderKeybar();
  }

  // The key bar. Every entry is derived from what is actually on the page and
  // what is actually selected, so the bar can only ever offer a key that will
  // do something — the ? panel is the full map, this is the reachable subset.
  function keybarItems() {
    const dlg = captureDialog();
    if (dlg && dlg.open) return [["\u21b5", "add"], ["esc", "cancel"]];
    const help = document.getElementById("help");
    if (help && !help.hidden) return [["esc", "close help"]];
    if (gPending) return [["\u2026", "press a marked key"], ["esc", "cancel"]];

    const items = [];
    if (selected()) items.push(["\u21b5", "open"], ["c", "done"], ["t", "today"]);
    if (rows().length) items.push(["j k", "move"]);
    if (document.querySelector(".namebox")) items.push(["/", "filter"]);
    items.push(["q", "add to inbox"], ["g", "go to"], ["?", "keys"]);
    return items;
  }

  function renderKeybar() {
    const bar = document.getElementById("keybar");
    if (!bar) return;
    bar.textContent = "";
    keybarItems().forEach(function (pair) {
      const item = document.createElement("span");
      const key = document.createElement("b");
      key.textContent = pair[0];
      item.appendChild(key);
      item.append(pair[1]);
      bar.appendChild(item);
    });
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

  function typing(e) {
    const t = e.target;
    return t && (t.tagName === "INPUT" || t.tagName === "TEXTAREA" || t.tagName === "SELECT" || t.isContentEditable);
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
      if (e.key === "Escape") e.target.blur();
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

    const row = selected();
    switch (e.key) {
      case "g": e.preventDefault(); setPending(true); break;
      case "j": e.preventDefault(); move(1); break;
      case "k": e.preventDefault(); move(-1); break;
      case "Enter":
      case "o":
        if (row && row.dataset.href) { e.preventDefault(); window.location.href = row.dataset.href; }
        break;
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
        if (help && !help.hidden) { help.hidden = true; renderKeybar(); }
        else select(null);
        break;
      }
    }
  });

  // a click or a lost window abandons a half-typed "g" sequence
  document.addEventListener("click", function (e) {
    setPending(false);
    if (e.target.closest("[data-capture-open]")) { e.preventDefault(); openCapture(); }
  });
  window.addEventListener("blur", function () { setPending(false); });

  // hx-boost swaps the body, taking the rendered bar with it
  document.addEventListener("htmx:afterSwap", renderKeybar);

  renderKeybar();

  // filter forms apply themselves on any change
  document.addEventListener("change", function (e) {
    const form = e.target.closest("form[data-autosubmit]");
    if (form) form.submit();
  });
})();
