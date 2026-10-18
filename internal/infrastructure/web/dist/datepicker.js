// Initialises flatpickr on every .js-datetime input so date fields render and
// accept a European dd/mm/yyyy HH:MM value regardless of the browser's locale.
// The visible (alt) input shows the European format; the real, submitted input
// keeps the ISO "Y-m-dTH:i" value the server already parses. Re-runs after htmx
// swaps so dynamically inserted forms (the reservation editor) get pickers too.
//
// Inside a <dialog> the calendar is appended to the dialog itself: showModal()
// puts the dialog in the top layer and makes the rest of the page inert, so a
// body-appended calendar would paint behind it and ignore clicks. It is
// position:fixed (placeInViewport), so the dialog's scroll box neither clips it
// nor grows scrollbars around it.
(function () {
  var GAP = 4;
  var MARGIN = 8;

  // The visible dd/mm/yyyy format is locale-neutral, but the calendar's weekday
  // and month names are not: they come from a flatpickr l10n bundle. The shell
  // loads l10n/<code>.js for the active locale (English needs none), so applying
  // the entry that matches <html lang> is the whole wiring.
  function localize() {
    if (typeof flatpickr === "undefined" || !flatpickr.l10ns) return;
    var l10n = flatpickr.l10ns[document.documentElement.lang];
    if (l10n) flatpickr.localize(l10n);
  }

  function placeInViewport(fp) {
    var cal = fp.calendarContainer;
    var field = fp.altInput || fp.input;
    if (!cal || !field) return;
    var rect = field.getBoundingClientRect();
    var height = cal.offsetHeight;
    var width = cal.offsetWidth;
    var below = window.innerHeight - rect.bottom;
    var above = rect.top;
    var top = below < height + GAP && above > below ? rect.top - height - GAP : rect.bottom + GAP;
    var left = Math.max(MARGIN, Math.min(rect.left, window.innerWidth - width - MARGIN));
    cal.style.position = "fixed";
    cal.style.top = Math.max(MARGIN, top) + "px";
    cal.style.left = left + "px";
    cal.style.right = "auto";
  }

  function dialogOptions(dlg) {
    var reposition = null;
    function unlisten() {
      if (reposition) dlg.removeEventListener("scroll", reposition, true);
      reposition = null;
    }
    return {
      appendTo: dlg,
      position: placeInViewport,
      onOpen: function (selectedDates, dateStr, fp) {
        reposition = function () {
          fp._positionCalendar();
        };
        // Capture: scroll does not bubble, and the dialog's inner box is what scrolls.
        dlg.addEventListener("scroll", reposition, true);
      },
      onClose: unlisten,
      // destroy() does not fire onClose, and the listener would outlive fp.config.
      onDestroy: unlisten,
    };
  }

  function initPickers(root) {
    if (typeof flatpickr === "undefined") return;
    var scope = root && root.querySelectorAll ? root : document;
    scope.querySelectorAll("input.js-datetime:not([data-fp])").forEach(function (el) {
      el.dataset.fp = "1";
      var options = {
        enableTime: true,
        time_24hr: true,
        altInput: true,
        altFormat: "d/m/Y H:i",
        dateFormat: "Y-m-d\\TH:i",
        allowInput: true,
        // Default is 5, which gives the inline minute spinner step=5: a stored
        // time like 17:08 then fails native validation and silently blocks the
        // whole form's submit. Step 1 accepts any stored minute.
        minuteIncrement: 1,
      };
      var dlg = el.closest("dialog");
      if (dlg) Object.assign(options, dialogOptions(dlg));
      flatpickr(el, options);
    });
  }

  // Date-only fields (the reservation filter): the same European display, an ISO
  // Y-m-d value.
  function initDatePickers(root) {
    if (typeof flatpickr === "undefined") return;
    var scope = root && root.querySelectorAll ? root : document;
    scope.querySelectorAll("input.js-date:not([data-fp])").forEach(function (el) {
      el.dataset.fp = "1";
      flatpickr(el, {
        altInput: true,
        altFormat: "d/m/Y",
        dateFormat: "Y-m-d",
        allowInput: true,
      });
    });
  }

  document.addEventListener("DOMContentLoaded", function () {
    localize();
    initPickers(document);
    initDatePickers(document);
  });
  document.addEventListener("htmx:afterSwap", function (e) {
    initPickers(e.target);
    initDatePickers(e.target);
  });
  // A dialog calendar lives outside its field's form, so a swap that drops the
  // field would leave it behind: each reopened edit would add another.
  document.addEventListener("htmx:beforeCleanupElement", function (e) {
    var fp = e.target._flatpickr;
    if (fp) {
      fp.close();
      fp.destroy();
    }
  });
})();
