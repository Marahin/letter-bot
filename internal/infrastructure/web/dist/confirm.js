// One guard for every control carrying data-confirm: the browser asks the question
// on the attribute before the action runs. The text rides on the attribute rather
// than an inline onsubmit because a translation carrying an apostrophe would close
// the JS string literal the attribute value is spliced into.
//
// A form confirms on submit (which covers the keyboard), anything else on click. The
// click listener runs in the capture phase, so cancelling stops an htmx request
// before htmx's own listener sees the event, and it skips forms so a form never
// asks twice.
(function () {
  function ask(el) {
    return confirm(el.getAttribute("data-confirm"));
  }

  document.addEventListener(
    "click",
    function (event) {
      var el = event.target && event.target.closest ? event.target.closest("[data-confirm]") : null;
      if (!el || el.tagName === "FORM") return;
      if (!ask(el)) {
        event.preventDefault();
        event.stopPropagation();
      }
    },
    true,
  );

  document.addEventListener(
    "submit",
    function (event) {
      var form = event.target;
      if (!form || !form.hasAttribute || !form.hasAttribute("data-confirm")) return;
      if (!ask(form)) {
        event.preventDefault();
        event.stopPropagation();
      }
    },
    true,
  );
})();
