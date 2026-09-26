// Drives the native <dialog> modals (currently the "New session" composer).
// Opening with showModal() gives ESC-to-close and a ::backdrop for free; this
// module wires the rest by event delegation on document, so htmx-swapped content
// inside a dialog keeps working with no re-init:
//   * click [data-modal-open="<id>"] -> open that dialog
//   * click [data-modal-close]       -> close the enclosing dialog
//   * click the dialog's backdrop     -> close (the click lands on the <dialog>
//     element itself; clicks on the inner wrapper/children do not)
// A successful create fires an `HX-Trigger: session-created` response header;
// htmx dispatches that as a `session-created` event, which closes any open modal.
(function () {
  function openModal(id) {
    var dlg = document.getElementById(id);
    if (dlg && typeof dlg.showModal === "function" && !dlg.open) dlg.showModal();
  }

  function closeOpenModals() {
    document.querySelectorAll("dialog[data-modal][open]").forEach(function (dlg) {
      dlg.close();
    });
  }

  document.addEventListener("click", function (e) {
    var opener = e.target.closest("[data-modal-open]");
    if (opener) {
      openModal(opener.dataset.modalOpen);
      return;
    }
    if (e.target.closest("[data-modal-close]")) {
      var dlg = e.target.closest("dialog");
      if (dlg) dlg.close();
      return;
    }
    // A click whose target is the dialog element itself is a backdrop click,
    // clicks on the inner wrapper or its children target those instead.
    if (e.target.matches("dialog[data-modal]")) e.target.close();
  });

  // Close the modal once a create succeeds (HX-Trigger: session-created). The
  // event bubbles from the form (or the body, if the form was swapped out), so
  // listening on document catches it either way.
  document.addEventListener("session-created", closeOpenModals);
  // A saved reservation (HX-Trigger: reservation-saved) closes its dialog too.
  document.addEventListener("reservation-saved", closeOpenModals);
})();
