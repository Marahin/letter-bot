// Alpine component for the shared searchable picker: a text input that filters
// an options panel, with a hidden input carrying the picked value for the form.
// The page hands it its element ids and the stored value through x-data, and the
// options themselves through a JSON island named "<id>-options".
// Alpine re-hydrates pickers swapped in over the wire via its mutation observer.
//
// Registration runs on alpine:init when this file loads first, and straight away
// when Alpine is already running: a page may pull this file in on demand, after
// alpine:init has fired, and Alpine reads the component name when it hydrates the
// element rather than when it starts.
(function () {
  function factory(config) {
    return {
      id: (config && config.id) || "",
      options: [],
      // headings[i] is the heading option i sits under. An option that carries
      // no group of its own joins the one above it, so the page sends a heading
      // once per run instead of once per option.
      headings: [],
      value: "",
      open: false,
      query: "",
      hi: 0,
      // Like a native select: a stored value no option carries falls back to
      // the first option, so a stale value is never submitted back.
      init: function () {
        var island = document.getElementById(this.id + "-options");
        try {
          this.options = island ? JSON.parse(island.textContent) : [];
        } catch (e) {
          this.options = [];
        }
        var heading = "";
        this.headings = this.options.map(function (o) {
          heading = o.group || heading;
          return heading;
        });
        var selected = (config && config.selected) || "";
        var hit = this.options.filter(function (o) { return o.value === selected; })[0];
        this.value = hit ? hit.value : this.options.length ? this.options[0].value : "";
      },
      // filtered narrows the options by the typed query, matching the label. Each
      // hit carries the heading it sits under, so a filtered panel keeps its
      // headings even when the option that opened a run is filtered out.
      get filtered() {
        var q = this.query.trim().toLowerCase();
        var headings = this.headings;
        var out = [];
        this.options.forEach(function (o, i) {
          if (q && o.label.toLowerCase().indexOf(q) < 0) return;
          out.push({ value: o.value, label: o.label, group: headings[i] || "" });
        });
        return out;
      },
      // groups cuts filtered into runs of options sharing a heading, so the panel
      // prints one heading per run. Each item keeps its index in filtered, which
      // is what the keyboard highlight counts in.
      get groups() {
        var out = [];
        var run = null;
        this.filtered.forEach(function (o, i) {
          if (!run || run.name !== o.group) {
            run = { name: o.group, key: o.group + "\u0000" + i, items: [] };
            out.push(run);
          }
          run.items.push({ o: o, i: i });
        });
        return out;
      },
      // selectedLabel is the closed control's readout: the picked option's label.
      selectedLabel: function () {
        var v = this.value;
        var hit = this.options.filter(function (o) { return o.value === v; })[0];
        return hit ? hit.label : "";
      },
      // openPanel opens on the full list, with no query and the stored pick
      // highlighted, so Enter without arrowing re-commits the current value
      // instead of flipping to the first option.
      openPanel: function () {
        var v = this.value;
        var idx = this.options.map(function (o) { return o.value; }).indexOf(v);
        this.open = true;
        this.query = "";
        this.hi = idx >= 0 ? idx : 0;
      },
      pick: function (v) { this.value = v; this.open = false; },
      // optionId names each option for aria-activedescendant, off the panel id.
      optionId: function (i) { return this.id + "-opt-" + i; },
      // confirmPick is the Enter handler: it commits the highlighted (or first)
      // match. No match keeps the current value.
      confirmPick: function () {
        var o = this.filtered[this.hi] || this.filtered[0];
        if (o) { this.pick(o.value); }
      },
      // nav moves the keyboard highlight; on a closed panel it opens it instead,
      // the combobox convention for the arrow keys.
      nav: function (delta) {
        if (!this.open) { this.openPanel(); return; }
        var n = this.filtered.length;
        if (!n) { this.hi = 0; return; }
        this.hi = Math.max(0, Math.min(n - 1, this.hi + delta));
      },
    };
  }

  function register() {
    Alpine.data("letterCombobox", factory);
  }

  if (window.Alpine) {
    register();
  } else {
    document.addEventListener("alpine:init", register);
  }
})();
