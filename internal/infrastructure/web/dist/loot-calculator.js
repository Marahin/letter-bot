// Loot Calculator: copy buttons and the session history. The history lives in
// this browser's localStorage only (the server never stores loot data).
// Delegated on document, so the result htmx swaps in needs no set-up.
(function () {
  var KEY = "letter:loot-history";
  var MAX = 20;
  var RESET_MS = 2000;

  function t(key, fallback, vars) {
    return window.LETTER && window.LETTER.t ? window.LETTER.t(key, fallback, vars) : fallback;
  }

  function read() {
    try {
      var list = JSON.parse(localStorage.getItem(KEY) || "[]");
      return Array.isArray(list) ? list : [];
    } catch (e) {
      return [];
    }
  }

  function write(list) {
    try {
      localStorage.setItem(KEY, JSON.stringify(list));
    } catch (e) {}
  }

  function signedAmount(n) {
    var digits = String(Math.abs(n)).replace(/\B(?=(\d{3})+(?!\d))/g, ".");
    return (n > 0 ? "+" : n < 0 ? "−" : "") + digits + " gp";
  }

  function label(entry) {
    if (entry.label) return entry.label;
    var d = new Date(entry.savedAt);
    return isNaN(d) ? "" : d.toLocaleString(document.documentElement.lang || undefined);
  }

  function render() {
    var section = document.querySelector("[data-loot-history]");
    if (!section) return;
    var list = section.querySelector("[data-loot-history-list]");
    var tpl = section.querySelector("template[data-loot-history-item]");
    var empty = section.querySelector("[data-loot-history-empty]");
    var clear = section.querySelector("[data-loot-history-clear]");
    var items = read();
    list.textContent = "";
    items.forEach(function (entry, i) {
      var node = tpl.content.firstElementChild.cloneNode(true);
      var names = (entry.names || []).join(", ");
      node.querySelector('[data-field="label"]').textContent = label(entry);
      node.querySelector('[data-field="names"]').textContent = names;
      node.querySelector('[data-field="names"]').title = names;
      var total = node.querySelector('[data-field="total"]');
      total.textContent = signedAmount(entry.total || 0);
      total.classList.add(entry.total > 0 ? "text-emerald-400" : entry.total < 0 ? "text-rose-400" : "text-zone-100");
      var load = node.querySelector("[data-loot-load]");
      load.setAttribute("data-index", String(i));
      load.setAttribute("aria-label", t("tools.loot.history.load_label", "Load the session of {names}", { names: names }));
      list.appendChild(node);
    });
    if (empty) empty.hidden = items.length > 0;
    if (clear) clear.hidden = items.length === 0;
  }

  function save(entry) {
    var items = read().filter(function (e) {
      return e.key !== entry.key;
    });
    entry.savedAt = new Date().toISOString();
    items.unshift(entry);
    write(items.slice(0, MAX));
    render();
  }

  // captureResult saves the result on the page once, then moves focus to it, or
  // to the field when the form came back with an error.
  function captureResult(moveFocus) {
    var main = document.getElementById("loot-main");
    if (!main || main.hasAttribute("data-loot-seen")) return;
    main.setAttribute("data-loot-seen", "");
    var island = document.getElementById("loot-entry");
    if (island && main.contains(island)) {
      try {
        save(JSON.parse(island.textContent));
      } catch (e) {}
    }
    if (!moveFocus) return;
    var target = main.querySelector("[data-loot-focus]") || main.querySelector('[aria-invalid="true"]');
    if (target) target.focus();
  }

  function copyText(text) {
    if (navigator.clipboard && window.isSecureContext) {
      return navigator.clipboard.writeText(text);
    }
    return new Promise(function (resolve, reject) {
      var area = document.createElement("textarea");
      area.value = text;
      area.setAttribute("readonly", "");
      area.style.position = "fixed";
      area.style.opacity = "0";
      document.body.appendChild(area);
      area.select();
      var ok = false;
      try {
        ok = document.execCommand("copy");
      } catch (e) {}
      document.body.removeChild(area);
      if (ok) resolve();
      else reject(new Error("copy refused"));
    });
  }

  function resetButton(btn) {
    var span = btn.querySelector("[data-copy-label]");
    if (span && btn.hasAttribute("data-label")) span.textContent = btn.getAttribute("data-label");
    btn.removeAttribute("data-copied");
    clearTimeout(btn.letterCopyTimer);
  }

  function announce(btn, ok) {
    var scope = btn.closest("section") || document;
    var status = scope.querySelector("[data-loot-status]");
    if (status) {
      status.textContent = "";
      if (!ok) {
        status.textContent = t("tools.loot.copy.failed", "Copy failed. Select the text and copy it by hand.");
      } else if (btn.hasAttribute("data-copy-code")) {
        status.appendChild(document.createTextNode(t("tools.loot.copy.done", "Copied:") + " "));
        var code = document.createElement("code");
        code.className = "text-zone-200";
        code.textContent = btn.getAttribute("data-copy");
        status.appendChild(code);
      } else {
        status.textContent = btn.getAttribute("data-copy-status") || t("tools.loot.copy.copied", "Copied");
      }
    }
    if (!ok) return;
    document.querySelectorAll("[data-copied]").forEach(resetButton);
    var span = btn.querySelector("[data-copy-label]");
    if (span) {
      if (!btn.hasAttribute("data-label")) btn.setAttribute("data-label", span.textContent);
      span.textContent = t("tools.loot.copy.copied", "Copied");
    }
    btn.setAttribute("data-copied", "");
    btn.letterCopyTimer = setTimeout(function () {
      resetButton(btn);
    }, RESET_MS);
  }

  function load(index) {
    var entry = read()[index];
    var section = document.querySelector("[data-loot-history]");
    if (!entry || !section) return;
    var action = section.getAttribute("data-loot-action");
    if (window.htmx) {
      window.htmx.ajax("POST", action, { target: "#loot-main", swap: "outerHTML", values: { session: entry.text } });
      return;
    }
    var form = document.createElement("form");
    form.method = "post";
    form.action = action;
    var field = document.createElement("input");
    field.type = "hidden";
    field.name = "session";
    field.value = entry.text;
    form.appendChild(field);
    document.body.appendChild(form);
    form.submit();
  }

  document.addEventListener("click", function (e) {
    if (!e.target.closest) return;
    var btn = e.target.closest("[data-copy]");
    if (btn) {
      copyText(btn.getAttribute("data-copy")).then(
        function () {
          announce(btn, true);
        },
        function () {
          announce(btn, false);
        },
      );
      return;
    }
    var loadBtn = e.target.closest("[data-loot-load]");
    if (loadBtn) {
      load(Number(loadBtn.getAttribute("data-index")));
      return;
    }
    if (e.target.closest("[data-loot-clear]")) {
      var input = document.querySelector("[data-loot-input]");
      if (input) {
        input.value = "";
        input.focus();
      }
      return;
    }
    if (e.target.closest("[data-loot-history-clear]")) {
      if (!window.confirm(t("tools.loot.history.clear_confirm", "Remove every saved session from this browser?"))) return;
      write([]);
      render();
    }
  });

  document.addEventListener("htmx:afterSettle", function () {
    captureResult(true);
  });

  function init() {
    render();
    captureResult(false);
  }
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", init);
  else init();
})();
