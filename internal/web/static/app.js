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
    if (typing(e)) {
      if (e.key === "Escape") e.target.blur();
      return;
    }
    if (e.metaKey || e.ctrlKey || e.altKey) return;

    if (gPending) {
      gPending = false;
      const dest = jumps[e.key];
      if (dest) {
        e.preventDefault();
        window.location.href = dest;
      }
      return;
    }

    const row = selected();
    switch (e.key) {
      case "g": gPending = true; break;
      case "j": e.preventDefault(); move(1); break;
      case "k": e.preventDefault(); move(-1); break;
      case "Enter":
      case "o":
        if (row && row.dataset.href) { e.preventDefault(); window.location.href = row.dataset.href; }
        break;
      case "c": if (row) { e.preventDefault(); submitIn(row, "kb-complete"); } break;
      case "t": if (row) { e.preventDefault(); submitIn(row, "kb-pick"); } break;
      case "q": {
        e.preventDefault();
        const cap = document.getElementById("quick-capture");
        if (cap) cap.focus();
        break;
      }
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
        break;
      }
      case "Escape": {
        const help = document.getElementById("help");
        if (help && !help.hidden) help.hidden = true;
        else select(null);
        break;
      }
    }
  });

  // filter forms apply themselves on any change
  document.addEventListener("change", function (e) {
    const form = e.target.closest("form[data-autosubmit]");
    if (form) form.submit();
  });
})();
