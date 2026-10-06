// flatpickr Polish calendar names. See l10n/ru.js for the shape and why these are
// plain scripts rather than the upstream UMD bundles.
(function () {
  if (typeof flatpickr === "undefined") return;
  flatpickr.l10ns.pl = {
    weekdays: {
      shorthand: ["Nd", "Pn", "Wt", "Śr", "Cz", "Pt", "So"],
      longhand: ["Niedziela", "Poniedziałek", "Wtorek", "Środa", "Czwartek", "Piątek", "Sobota"],
    },
    months: {
      shorthand: ["Sty", "Lut", "Mar", "Kwi", "Maj", "Cze", "Lip", "Sie", "Wrz", "Paź", "Lis", "Gru"],
      longhand: [
        "Styczeń",
        "Luty",
        "Marzec",
        "Kwiecień",
        "Maj",
        "Czerwiec",
        "Lipiec",
        "Sierpień",
        "Wrzesień",
        "Październik",
        "Listopad",
        "Grudzień",
      ],
    },
    firstDayOfWeek: 1,
    ordinal: function () {
      return ".";
    },
    rangeSeparator: " - ",
    weekAbbreviation: "tydz.",
    scrollTitle: "Przewiń, aby zwiększyć",
    toggleTitle: "Kliknij, aby przełączyć",
    amPM: ["AM", "PM"],
    yearAriaLabel: "Rok",
    monthAriaLabel: "Miesiąc",
    hourAriaLabel: "Godzina",
    minuteAriaLabel: "Minuta",
    time_24hr: true,
  };
})();
