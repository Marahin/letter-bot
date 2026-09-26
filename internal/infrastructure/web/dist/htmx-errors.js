// htmx swaps nothing on a 4xx/5xx or a network failure, so without this a failed
// request would look like a click that did nothing. Handlers answer htmx errors
// with a short, localized text/plain body (Deps.RenderError), shown here as a toast
// in the #letter-toast live region the layout renders.
(function () {
  var HIDE_MS = 6000;
  var MAX_BODY = 300;
  var timer = 0;

  function t(key, fallback) {
    return window.LETTER && window.LETTER.t ? window.LETTER.t(key, fallback) : fallback;
  }

  function hide() {
    var el = document.getElementById("letter-toast");
    if (el) el.classList.remove("show");
  }

  function show(msg) {
    var el = document.getElementById("letter-toast");
    if (!el) return;
    // Emptied first, so a screen reader announces a repeat of the same message.
    el.textContent = "";
    window.setTimeout(function () {
      el.textContent = msg;
      el.classList.add("show");
    }, 30);
    window.clearTimeout(timer);
    timer = window.setTimeout(hide, HIDE_MS);
  }

  function plainBody(xhr) {
    if (!xhr || (xhr.getResponseHeader("Content-Type") || "").indexOf("text/plain") !== 0) return "";
    var body = (xhr.responseText || "").trim();
    return body.length <= MAX_BODY ? body : "";
  }

  document.addEventListener("htmx:responseError", function (e) {
    show(plainBody(e.detail && e.detail.xhr) || t("shell.toast.error", "Something went wrong. Try again."));
  });
  document.addEventListener("htmx:sendError", function () {
    show(t("shell.toast.offline", "Could not reach Letter. Check your connection and try again."));
  });
  document.addEventListener("click", function (e) {
    if (e.target && e.target.closest && e.target.closest("#letter-toast")) hide();
  });
})();
