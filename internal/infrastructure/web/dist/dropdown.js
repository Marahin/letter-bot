// Close behaviour shared by every <details data-dropdown> menu (the clan switcher,
// the language picker, the top-bar menu). A native <details> stays open until
// its own summary is clicked again; readers expect a click outside, or Escape, to
// close it. This adds both, for any dropdown in any template, so the behaviour is
// consistent across the app. Opening one dropdown also closes the others.
(function () {
  function closeExcept(keep) {
    document.querySelectorAll("details[data-dropdown][open]").forEach(function (d) {
      if (d !== keep) d.open = false;
    });
  }

  document.addEventListener("click", function (event) {
    var inside = event.target && event.target.closest ? event.target.closest("details[data-dropdown]") : null;
    closeExcept(inside);
  });

  document.addEventListener("keydown", function (event) {
    if (event.key !== "Escape") return;
    var open = document.querySelector("details[data-dropdown][open]");
    if (!open) return;
    open.open = false;
    var summary = open.querySelector("summary");
    if (summary) summary.focus();
  });
})();
