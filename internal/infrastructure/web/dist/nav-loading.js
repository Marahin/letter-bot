// A top progress bar for every full-page navigation, so no view shows dead air while the
// next page loads. Navigation is a full document load here, so the bar lives in the page
// the user leaves: paint holding keeps it on screen until the destination paints.
//
// The bar shows only after a short delay. An instant navigation tears this document down
// before the timer fires, so a fast page never flashes the bar. The bar animates with
// transform, which runs on the compositor and keeps moving while the main thread is busy
// (the build calculator parses a large dataset on load).
(function () {
  var DELAY_MS = 140;
  var bar = null;
  var timer = 0;

  function ensureBar() {
    if (bar) return bar;
    bar = document.createElement("div");
    bar.className = "letter-navbar";
    bar.setAttribute("aria-hidden", "true");
    document.body.appendChild(bar);
    return bar;
  }

  function show() {
    var b = ensureBar();
    b.classList.remove("run");
    void b.offsetWidth; // restart the animation from zero
    b.classList.add("run");
  }

  function hide() {
    if (bar) bar.classList.remove("run");
  }

  function start() {
    if (timer) return;
    timer = window.setTimeout(function () {
      timer = 0;
      show();
    }, DELAY_MS);
  }

  function cancel() {
    if (timer) {
      window.clearTimeout(timer);
      timer = 0;
    }
    hide();
  }

  function plainLeftClick(e) {
    return e.button === 0 && !e.metaKey && !e.ctrlKey && !e.shiftKey && !e.altKey;
  }

  function boosts(el) {
    return el.hasAttribute("hx-get") || el.hasAttribute("hx-post") || el.hasAttribute("hx-boost");
  }

  document.addEventListener("click", function (e) {
    if (e.defaultPrevented || !plainLeftClick(e)) return;
    var a = e.target && e.target.closest ? e.target.closest("a[href]") : null;
    if (!a || a.hasAttribute("download") || boosts(a)) return;
    if (a.target && a.target !== "_self") return;
    var url;
    try {
      url = new URL(a.getAttribute("href"), location.href);
    } catch (_) {
      return;
    }
    if (url.origin !== location.origin) return;
    // Same path and query is the same document: a hash jump or a self link, not a load.
    if (url.pathname === location.pathname && url.search === location.search) return;
    start();
  });

  document.addEventListener("submit", function (e) {
    if (e.defaultPrevented) return;
    var f = e.target;
    if (!f || boosts(f)) return;
    if ((f.getAttribute("method") || "").toLowerCase() === "dialog") return;
    if (f.target && f.target !== "_self") return;
    start();
  });

  // A back/forward restore from the cache re-shows this page with the bar mid-run.
  window.addEventListener("pageshow", cancel);
})();
