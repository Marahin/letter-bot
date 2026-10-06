// Author picker of the reservation dialog (managers only). Typing over the author
// makes it free text, so the hidden Discord id is cleared; picking a suggested
// past author fills both. Delegated on document, so forms swapped in by htmx
// need no set-up.
(function () {
  document.addEventListener("input", function (e) {
    var input = e.target.closest && e.target.closest("[data-author-input]");
    if (!input || !input.form) return;
    var id = input.form.querySelector("[data-author-id]");
    if (id) id.value = "";
  });

  document.addEventListener("click", function (e) {
    var pick = e.target.closest && e.target.closest("[data-author-pick]");
    if (!pick) return;
    var form = pick.closest("form");
    if (!form) return;
    var input = form.querySelector("[data-author-input]");
    var id = form.querySelector("[data-author-id]");
    if (input) {
      input.value = pick.dataset.author || "";
      input.focus();
    }
    if (id) id.value = pick.dataset.id || "";
    var list = pick.closest("ul");
    if (list && list.parentNode) list.parentNode.textContent = "";
  });
})();
