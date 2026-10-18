// Reads the catalog strings a page hands its scripts and exposes window.LETTER.t.
//
// Every <script type="application/json" data-letter-strings> island in the document
// is merged, later ones winning, so the shell and a feature page can each ship
// their own and an htmx-swapped fragment can bring more. Islands are re-read on
// every lookup (there are one or two, and t() runs on user interactions), which is
// what makes a swapped-in island work without a re-init hook.
//
// Placeholders are {name}, not the %s verbs the server's printer uses: these
// strings never go through it.
(function () {
  var letter = (window.LETTER = window.LETTER || {});

  function merged() {
    var out = {};
    document.querySelectorAll("script[data-letter-strings]").forEach(function (el) {
      if (!el.letterStrings) {
        try {
          el.letterStrings = JSON.parse(el.textContent || "{}");
        } catch (e) {
          el.letterStrings = {};
        }
      }
      Object.keys(el.letterStrings).forEach(function (key) {
        out[key] = el.letterStrings[key];
      });
    });
    return out;
  }

  // t returns the translated string for a dotted catalog key. fallback is the
  // English source, so a page that forgot the key still renders words rather than
  // an id. vars fills {name} placeholders.
  letter.t = function (key, fallback, vars) {
    var value = merged()[key];
    if (typeof value !== "string") value = typeof fallback === "string" ? fallback : key;
    if (!vars) return value;
    return value.replace(/\{(\w+)\}/g, function (match, name) {
      return Object.prototype.hasOwnProperty.call(vars, name) ? String(vars[name]) : match;
    });
  };
})();
