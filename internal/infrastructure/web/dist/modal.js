// Drives the native <dialog> modals (the new and edit reservation forms).
// Opening with showModal() gives ESC-to-close and a ::backdrop for free; this
// module wires the rest by event delegation on document, so htmx-swapped content
// inside a dialog keeps working with no re-init:
//   * click [data-modal-open="<id>"] -> open that dialog
//   * click [data-modal-close]       -> close the enclosing dialog
//   * click the dialog's backdrop     -> close (the click lands on the <dialog>
//     element itself; clicks on the inner wrapper/children do not)
// A saved reservation fires an `HX-Trigger: reservation-saved` response header;
// htmx dispatches that as an event, which closes any open modal.
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

  function openPickers(dlg) {
    var open = [];
    dlg.querySelectorAll("input").forEach(function (el) {
      if (el._flatpickr && el._flatpickr.isOpen) open.push(el._flatpickr);
    });
    return open;
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

  // Escape closes an open calendar first, not the whole dialog. flatpickr may
  // close the calendar on keydown before the dialog's cancel fires, so the
  // keydown (capture, ahead of flatpickr) records that a calendar was open.
  var escapedPicker = null;
  window.addEventListener(
    "keydown",
    function (e) {
      if (e.key !== "Escape") return;
      var dlg = document.querySelector("dialog[data-modal][open]");
      escapedPicker = dlg && openPickers(dlg).length ? dlg : null;
    },
    true,
  );
  document.addEventListener(
    "cancel",
    function (e) {
      var dlg = e.target;
      if (!dlg.matches || !dlg.matches("dialog[data-modal]")) return;
      var pickers = openPickers(dlg);
      if (escapedPicker === dlg || pickers.length) {
        e.preventDefault();
        pickers.forEach(function (fp) {
          fp.close();
        });
      }
      escapedPicker = null;
    },
    true,
  );
  document.addEventListener(
    "close",
    function (e) {
      if (e.target.matches && e.target.matches("dialog[data-modal]")) {
        openPickers(e.target).forEach(function (fp) {
          fp.close();
        });
      }
    },
    true,
  );

  document.addEventListener("reservation-saved", closeOpenModals);
})();
