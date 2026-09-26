// Initialises flatpickr on every .js-datetime input so date fields render and
// accept a European dd/mm/yyyy HH:MM value regardless of the browser's locale.
// The visible (alt) input shows the European format; the real, submitted input
// keeps the ISO "Y-m-dTH:i" value the server already parses. Re-runs after htmx
// swaps so dynamically inserted forms (the session editor) get pickers too.
//
// static:true renders the calendar inline (absolutely positioned within the
// field's wrapper) rather than appended to <body>. That keeps it inside the
// "New session" <dialog>, which showModal() puts in the browser's top layer, a
// body-appended calendar would otherwise paint *behind* the modal.
//
// Window persistence: fields opting in via data-fp-persist (the composer's
// starts_at / post_at) stash their typed value in sessionStorage, keyed by page
// path + field name, so a chosen date survives leaving and returning to the page
// (or a refresh) in the same browser window. The store only seeds an *empty*
// field, so a server re-render that already carries the user's value (a failed
// submit) is never clobbered. The keys clear on a successful create so the next
// "New session" opens blank.
(function () {
  // The visible dd/mm/yyyy format is locale-neutral, but the calendar's weekday
  // and month names are not: they come from a flatpickr l10n bundle. The shell
  // loads l10n/<code>.js for the active locale (English needs none), so applying
  // the entry that matches <html lang> is the whole wiring.
  function localize() {
    if (typeof flatpickr === "undefined" || !flatpickr.l10ns) return;
    var l10n = flatpickr.l10ns[document.documentElement.lang];
    if (l10n) flatpickr.localize(l10n);
  }

  // Inside the "New session" modal the calendar renders inline (static), so a
  // scrollable modal can clip it if the field sits low. On open, scroll the field
  // to the top of the modal so the calendar has room below. Outside a dialog
  // (the in-list session editors) the page scrolls normally, so leave it be.
  function scrollFieldIntoModalView(selectedDates, dateStr, instance) {
    var field = instance.altInput || instance.input;
    if (field && field.closest("dialog")) field.scrollIntoView({ block: "start" });
  }

  // storeKey scopes a persisted value to the current page and the field's name,
  // so different guilds (distinct paths) and starts_at vs post_at never collide.
  function storeKey(el) {
    return "letter-datetime:" + window.location.pathname + ":" + el.name;
  }
  function loadStored(key) {
    try {
      return (window.sessionStorage.getItem(key) || "").trim();
    } catch (e) {
      return "";
    }
  }
  function saveStored(key, value) {
    try {
      if (value) window.sessionStorage.setItem(key, value);
      else window.sessionStorage.removeItem(key);
    } catch (e) {
      /* sessionStorage may be unavailable (privacy mode); persistence is best-effort */
    }
  }

  function initPickers(root) {
    if (typeof flatpickr === "undefined") return;
    var scope = root && root.querySelectorAll ? root : document;
    scope.querySelectorAll("input.js-datetime:not([data-fp])").forEach(function (el) {
      el.dataset.fp = "1";
      var persist = el.dataset.fpPersist !== undefined;
      // Seed an empty opt-in field from the store before constructing, so flatpickr
      // picks the value up. A field already carrying a value (a real session, or a
      // failed-submit re-render) keeps it untouched.
      if (persist && !el.value) {
        var stored = loadStored(storeKey(el));
        if (stored) el.value = stored;
      }
      flatpickr(el, {
        enableTime: true,
        time_24hr: true,
        altInput: true,
        altFormat: "d/m/Y H:i",
        dateFormat: "Y-m-d\\TH:i",
        allowInput: true,
        static: true,
        // Default is 5, which gives the inline minute spinner step=5: a stored
        // time like 17:08 then fails native validation and silently blocks the
        // whole form's submit. Step 1 accepts any stored minute.
        minuteIncrement: 1,
        onOpen: scrollFieldIntoModalView,
        onChange: persist
          ? function (selectedDates, dateStr, instance) {
              saveStored(storeKey(instance.input), instance.input.value);
            }
          : undefined,
      });
    });
  }

  document.addEventListener("DOMContentLoaded", function () {
    localize();
    initPickers(document);
  });
  document.addEventListener("htmx:afterSwap", function (e) {
    initPickers(e.target);
  });
  // A successful create fires `session-created`; drop the composer's stored values
  // so the next "New session" opens blank rather than re-seeding the just-used date.
  document.addEventListener("session-created", function () {
    document.querySelectorAll("input.js-datetime[data-fp-persist]").forEach(function (el) {
      saveStored(storeKey(el), "");
    });
  });
})();
