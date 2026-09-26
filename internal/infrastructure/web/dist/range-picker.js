// Drives the day-range picker shared by the Stats and Attendance pages: a toggle
// button that opens a panel with presets, a Range / Single days switch, a live
// readout, a month grid and the Clear / Apply actions. The form keeps a noscript
// from/to fallback, so this only enhances the page. The server owns persistence:
// Apply submits the GET form, and the server stores the days for the session.
(function () {
  var FALLBACK_WEEKDAYS = ["Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"];
  var FALLBACK_MONTHS = [
    "Jan", "Feb", "Mar", "Apr", "May", "Jun",
    "Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
  ];
  var FALLBACK_DATE_FORMAT = "{DD} {Mon}";

  function t(key, fallback, vars) {
    var fn = window.LETTER && window.LETTER.t;
    return fn ? fn(key, fallback, vars) : fallback;
  }

  function pad(n) {
    return String(n).padStart(2, "0");
  }
  function key(y, m, d) {
    return y + "-" + pad(m + 1) + "-" + pad(d);
  }
  function keyOf(date) {
    return key(date.getFullYear(), date.getMonth(), date.getDate());
  }
  function parseKey(s) {
    var p = s.split("-");
    return new Date(Number(p[0]), Number(p[1]) - 1, Number(p[2]));
  }
  // Monday-based weekday index (0 = Monday … 6 = Sunday).
  function mondayIndex(date) {
    return (date.getDay() + 6) % 7;
  }
  function shiftDay(k, days) {
    var d = parseKey(k);
    d.setDate(d.getDate() + days);
    return keyOf(d);
  }

  // jsonList reads a JSON array data attribute. want, when given, is the exact
  // length the caller needs, so a short or malformed list falls back instead of
  // rendering half a calendar.
  function jsonList(raw, want) {
    try {
      var arr = JSON.parse(raw || "[]");
      if (!Array.isArray(arr)) return [];
      if (want && arr.length !== want) return [];
      return arr;
    } catch (e) {
      return [];
    }
  }

  function init(root) {
    var scope = root && root.querySelectorAll ? root : document;
    var forms = scope.querySelectorAll("form[data-range-picker]:not([data-cal])");
    if (!forms.length) return;

    forms.forEach(function (form) {
      form.dataset.cal = "1";
      var host = form.querySelector("[data-range-calendar]");
      if (!host) return;

      var MONTHS = jsonList(form.dataset.months, 12);
      if (!MONTHS.length) MONTHS = FALLBACK_MONTHS;
      var WEEKDAYS = jsonList(form.dataset.weekdays, 7);
      if (!WEEKDAYS.length) WEEKDAYS = FALLBACK_WEEKDAYS;
      var DATE_FORMAT = form.dataset.dateFormat || FALLBACK_DATE_FORMAT;

      var dataDays = {};
      jsonList(form.dataset.days).forEach(function (d) {
        dataDays[d] = true;
      });

      var initialKeys = jsonList(form.dataset.selected);
      var selected = {};
      function restoreInitialSelection() {
        selected = {};
        initialKeys.forEach(function (d) {
          selected[d] = true;
        });
      }
      restoreInitialSelection();

      // A cookie-restored selection leaves the address bar bare, so a copied link
      // showed its receiver another range. Write the resolved days to the URL, but
      // only a pinned selection: pinning the implicit default would freeze the
      // rolling window on reload.
      var loc = new URL(window.location.href);
      var pinned = ["days", "from", "to"].some(function (p) {
        return (loc.searchParams.get(p) || "").trim() !== "";
      });
      var samePage = new URL(form.action, window.location.href).pathname === window.location.pathname;
      if (!pinned && samePage && initialKeys.length && form.hasAttribute("data-explicit")) {
        loc.searchParams.set("days", initialKeys.join(","));
        history.replaceState(null, "", loc);
      }

      // The noscript from/to inputs would submit beside our hidden "days" field.
      form.querySelectorAll("noscript").forEach(function (n) {
        n.remove();
      });

      var hidden = document.createElement("input");
      hidden.type = "hidden";
      hidden.name = "days";
      hidden.setAttribute("data-range-days", "");
      form.appendChild(hidden);

      var ui = form.querySelector("[data-range-ui]");
      var toggle = form.querySelector("[data-range-toggle]");
      var label = form.querySelector("[data-range-label]");
      var pop = form.querySelector("[data-range-popover]");
      var backdrop = form.querySelector("[data-range-backdrop]");
      var live = form.querySelector("[data-range-live]");
      // The server's label carries the year, which the panel does not, so an
      // abandoned edit restores it rather than keeping the pending text.
      var initialLabel = label ? label.textContent : "";
      if (ui) ui.hidden = false;

      // Default the visible month to the latest selected day, else today.
      var anchorDay = initialKeys.length ? parseKey(initialKeys[initialKeys.length - 1]) : new Date();
      var view = { y: anchorDay.getFullYear(), m: anchorDay.getMonth() };

      var mode = "range"; // "range" or "days"
      // The armed range start (awaiting its last day) and the day under the
      // cursor, for the live span preview.
      var rangeAnchor = null;
      var rangeHover = null;
      var drag = { active: false, anchor: null, hover: null };
      // focusKey is the grid's roving tabindex target.
      var focusKey = null;
      // The panel is built once. cells maps a day key to its live cell, and
      // builtMonth names the month those cells belong to.
      var readoutEl = null;
      var readoutMain = null;
      var readoutHint = null;
      var titleEl = null;
      var gridEl = null;
      var modeButtons = {};
      var cells = {};
      var builtMonth = null;
      // Only a mouse gets the hover preview and the drag sweep. A touch tap also
      // fires mouseover, which would leave a phantom span on the grid.
      var pointerFine = true;

      function lastSelectedKey() {
        var keys = selectedKeys();
        return keys.length ? keys[keys.length - 1] : null;
      }

      function openPop() {
        if (!pop) return;
        pop.hidden = false;
        if (backdrop) backdrop.hidden = false;
        if (toggle) toggle.setAttribute("aria-expanded", "true");
        // Open idle, never half-way through a range someone left behind.
        rangeAnchor = null;
        rangeHover = null;
        focusKey = lastSelectedKey() || keyOf(new Date());
        showMonthOf(focusKey);
        sync();
        focusDay(focusKey);
      }
      // closePop leaves no trace of an abandoned edit: the applied selection and
      // the server's own toggle label come back.
      function closePop() {
        if (!pop) return;
        pop.hidden = true;
        if (backdrop) backdrop.hidden = true;
        if (toggle) toggle.setAttribute("aria-expanded", "false");
        rangeAnchor = null;
        rangeHover = null;
        restoreInitialSelection();
        if (label) label.textContent = initialLabel;
      }
      // A swap can replace this form, and a document listener would then live on
      // over a detached node. Each one drops itself on the first event after that.
      function onDocument(type, fn, capture) {
        function handler(e) {
          if (!form.isConnected) {
            document.removeEventListener(type, handler, capture);
            return;
          }
          fn(e);
        }
        document.addEventListener(type, handler, capture);
      }
      if (toggle && pop) {
        toggle.addEventListener("click", function (e) {
          e.stopPropagation();
          if (pop.hidden) openPop();
          else closePop();
        });
        if (backdrop) {
          backdrop.addEventListener("click", function () {
            closePop();
          });
        }
        form.querySelectorAll("[data-range-close]").forEach(function (b) {
          b.addEventListener("click", function () {
            closePop();
            toggle.focus();
          });
        });
        // Capture phase so the target is read BEFORE any in-panel click handler
        // runs. A month change replaces the day cells, and in the bubble phase
        // form.contains(e.target) would then be wrongly false and close the panel.
        onDocument(
          "click",
          function (e) {
            if (!pop.hidden && !form.contains(e.target)) closePop();
          },
          true,
        );
        onDocument("keydown", function (e) {
          if (e.key === "Escape" && !pop.hidden) {
            closePop();
            toggle.focus();
          }
        });
      }

      function selectedKeys() {
        return Object.keys(selected).sort();
      }
      function focusDay(k) {
        if (cells[k]) cells[k].focus();
      }

      // spanKeys returns every day between keys a and b inclusive (order-agnostic).
      function spanKeys(a, b) {
        var lo = parseKey(a);
        var hi = parseKey(b);
        if (lo > hi) {
          var swap = lo;
          lo = hi;
          hi = swap;
        }
        var keys = [];
        for (var d = new Date(lo); d <= hi; d.setDate(d.getDate() + 1)) {
          keys.push(keyOf(d));
        }
        return keys;
      }

      // The committed selection, plus a live preview of the drag span or of the
      // armed range.
      function paintedKeys() {
        var painted = {};
        Object.keys(selected).forEach(function (k) {
          painted[k] = true;
        });
        if (drag.active && drag.anchor && drag.hover) {
          spanKeys(drag.anchor, drag.hover).forEach(function (k) {
            painted[k] = true;
          });
        } else if (rangeAnchor && rangeHover) {
          spanKeys(rangeAnchor, rangeHover).forEach(function (k) {
            painted[k] = true;
          });
        }
        return painted;
      }

      // fmtDay renders one day in the locale's own day/month order, the pattern the
      // server passes in data-date-format.
      function fmtDay(k) {
        var d = parseKey(k);
        return DATE_FORMAT.replace(/\{(\w+)\}/g, function (whole, token) {
          if (token === "D") return String(d.getDate());
          if (token === "DD") return pad(d.getDate());
          if (token === "Mon") return MONTHS[d.getMonth()];
          if (token === "YYYY") return String(d.getFullYear());
          return whole;
        });
      }

      // The count stays out of the text because this side has no plural support,
      // so the label must never inflect.
      function summary() {
        var keys = selectedKeys();
        if (!keys.length) return { text: "", count: 0 };
        if (keys.length === 1) return { text: fmtDay(keys[0]), count: 0 };
        var first = keys[0];
        var last = keys[keys.length - 1];
        if (spanKeys(first, last).length === keys.length) {
          return {
            text: t("range.picker.readout_span", "{from} – {to}", { from: fmtDay(first), to: fmtDay(last) }),
            count: 0,
          };
        }
        return { text: t("range.picker.selected_days", "Selected days"), count: keys.length };
      }

      function button(text, cls) {
        var b = document.createElement("button");
        b.type = "button";
        b.className = cls;
        b.textContent = text;
        return b;
      }

      function countBadge(n) {
        var badge = document.createElement("span");
        badge.className = "cal-readout-count";
        badge.textContent = String(n);
        return badge;
      }

      var PRESETS = [
        ["last_7", "range.picker.preset_last_7", "Last 7 days"],
        ["last_30", "range.picker.preset_last_30", "Last 30 days"],
        ["this_week", "range.picker.preset_this_week", "This week"],
        ["this_month", "range.picker.preset_this_month", "This month"],
      ];

      function renderPresets() {
        var box = document.createElement("div");
        box.className = "cal-presets";
        PRESETS.forEach(function (p) {
          var b = button(t(p[1], p[2]), "cal-preset");
          b.dataset.calPreset = p[0];
          b.addEventListener("click", function () {
            applyPreset(p[0]);
            sync();
          });
          box.appendChild(b);
        });
        return box;
      }

      var MODES = [
        ["range", "range.picker.mode_range", "Range"],
        ["days", "range.picker.mode_days", "Single days"],
      ];

      // A mode switch keeps the selection: the days are the same set. Range mode
      // then replaces it on the first tap, single-days mode adds to it.
      function renderModes() {
        var box = document.createElement("div");
        box.className = "cal-modes";
        box.setAttribute("role", "group");
        MODES.forEach(function (m) {
          var b = button(t(m[1], m[2]), "cal-mode");
          b.dataset.calMode = m[0];
          modeButtons[m[0]] = b;
          b.addEventListener("click", function () {
            mode = m[0];
            rangeAnchor = null;
            rangeHover = null;
            sync();
          });
          box.appendChild(b);
        });
        return box;
      }

      function paintModes() {
        Object.keys(modeButtons).forEach(function (name) {
          modeButtons[name].setAttribute("aria-pressed", mode === name ? "true" : "false");
        });
      }

      // readoutParts is the panel's one description of the current step. The
      // readout and the live region both render it.
      function readoutParts() {
        if (rangeAnchor) {
          return {
            armed: true,
            count: 0,
            text: t("range.picker.readout_start", "Start: {from}", { from: fmtDay(rangeAnchor) }),
            hint: t("range.picker.hint_armed", "Now pick the last day."),
          };
        }
        var s = summary();
        return {
          armed: false,
          count: s.count,
          text: s.text,
          hint:
            mode === "days"
              ? t("range.picker.hint_days", "Tap a day to add or remove it.")
              : t("range.picker.hint_range", "Tap the first day, then the last day."),
        };
      }

      function renderReadout() {
        readoutEl = document.createElement("div");
        readoutEl.className = "cal-readout";
        readoutEl.setAttribute("data-range-readout", "");
        readoutMain = document.createElement("div");
        readoutMain.className = "cal-readout-main";
        readoutHint = document.createElement("div");
        readoutHint.className = "cal-readout-hint";
        readoutEl.appendChild(readoutMain);
        readoutEl.appendChild(readoutHint);
        return readoutEl;
      }

      function paintReadout(parts) {
        if (!readoutEl) return;
        readoutEl.classList.toggle("is-armed", parts.armed);
        if (parts.armed) readoutEl.setAttribute("data-range-armed", "");
        else readoutEl.removeAttribute("data-range-armed");
        readoutMain.textContent = "";
        if (parts.count) readoutMain.appendChild(countBadge(parts.count));
        if (parts.text) readoutMain.appendChild(document.createTextNode(parts.text));
        readoutHint.textContent = parts.hint;
      }

      // announce updates the live region in place. A region inserted together with
      // its text is not announced, so it must outlive the panel render.
      function announce(parts) {
        if (!live) return;
        var text = [parts.count ? String(parts.count) : "", parts.text, parts.hint].filter(Boolean).join(" ");
        if (live.textContent !== text) live.textContent = text;
      }

      // paintToggle keeps the toggle honest while the panel is open, an empty
      // selection included, so Clear reads as a change rather than a no-op.
      function paintToggle() {
        if (!label || !pop || pop.hidden) return;
        var s = summary();
        label.textContent = "";
        if (!s.text) {
          label.appendChild(document.createTextNode(t("range.picker.no_days", "No days")));
          return;
        }
        if (s.count) {
          label.appendChild(countBadge(s.count));
          label.appendChild(document.createTextNode(" "));
        }
        label.appendChild(document.createTextNode(s.text));
      }

      function navButton(glyph, delta, msgKey, fallback) {
        var b = button(glyph, "cal-nav");
        b.dataset.calStep = String(delta);
        b.setAttribute("aria-label", t(msgKey, fallback));
        b.addEventListener("click", function () {
          step(delta);
        });
        return b;
      }

      function renderHead() {
        var head = document.createElement("div");
        head.className = "cal-head";
        titleEl = document.createElement("span");
        titleEl.className = "cal-title";
        head.appendChild(navButton("‹", -1, "range.picker.prev_month", "Previous month"));
        head.appendChild(titleEl);
        head.appendChild(navButton("›", 1, "range.picker.next_month", "Next month"));
        return head;
      }

      function monthTitle() {
        return MONTHS[view.m] + " " + view.y;
      }

      // dayCell holds only what the month fixes. Everything the state moves is
      // painted onto the live cell afterwards.
      function dayCell(k, day, todayKey) {
        var cell = button(String(day), "cal-day");
        cell.dataset.date = k;
        cell.setAttribute("role", "gridcell");
        cell.setAttribute("aria-label", fmtDay(k));
        if (k === todayKey) cell.classList.add("is-today");
        if (dataDays[k]) {
          cell.classList.add("has-data");
          var dot = document.createElement("span");
          dot.className = "cal-dot";
          cell.appendChild(dot);
        }
        return cell;
      }

      function paintCell(cell, k, painted) {
        var on = !!painted[k];
        var isStart = on && !painted[shiftDay(k, -1)];
        var isEnd = on && !painted[shiftDay(k, 1)];
        cell.setAttribute("aria-selected", on ? "true" : "false");
        cell.classList.toggle("is-start", isStart);
        cell.classList.toggle("is-end", isEnd);
        cell.classList.toggle("is-in-range", on && !isStart && !isEnd);
        cell.classList.toggle("is-armed", k === rangeAnchor);
        cell.tabIndex = k === focusKey ? 0 : -1;
      }

      function paintGrid() {
        var painted = paintedKeys();
        Object.keys(cells).forEach(function (k) {
          paintCell(cells[k], k, painted);
        });
      }

      function gridRow() {
        var row = document.createElement("div");
        row.className = "cal-row";
        row.setAttribute("role", "row");
        return row;
      }

      function buildGrid() {
        cells = {};
        builtMonth = view.y + "-" + view.m;
        if (titleEl) titleEl.textContent = monthTitle();

        var grid = document.createElement("div");
        grid.className = "cal-grid";
        grid.setAttribute("role", "grid");
        grid.setAttribute("aria-label", monthTitle());

        var headRow = gridRow();
        WEEKDAYS.forEach(function (w) {
          var c = document.createElement("span");
          c.className = "cal-wd";
          c.setAttribute("role", "columnheader");
          c.textContent = w;
          headRow.appendChild(c);
        });
        grid.appendChild(headRow);

        var todayKey = keyOf(new Date());
        var daysInMonth = new Date(view.y, view.m + 1, 0).getDate();
        var lead = mondayIndex(new Date(view.y, view.m, 1));
        var focus = focusKey ? parseKey(focusKey) : null;
        if (!focus || focus.getMonth() !== view.m || focus.getFullYear() !== view.y) {
          focusKey = key(view.y, view.m, 1);
        }

        var row = null;
        for (var slot = 0; slot < lead + daysInMonth; slot++) {
          if (slot % 7 === 0) {
            row = gridRow();
            grid.appendChild(row);
          }
          if (slot < lead) {
            var blank = document.createElement("span");
            blank.className = "cal-blank";
            blank.setAttribute("role", "gridcell");
            row.appendChild(blank);
            continue;
          }
          var day = slot - lead + 1;
          var k = key(view.y, view.m, day);
          cells[k] = dayCell(k, day, todayKey);
          row.appendChild(cells[k]);
        }
        return grid;
      }

      function renderActions() {
        var actions = document.createElement("div");
        actions.className = "cal-actions";
        var clear = button(t("range.picker.clear", "Clear"), "cal-clear");
        clear.addEventListener("click", function () {
          // Apply then submits an empty day list, which the server reads as "back
          // to the page default".
          selected = {};
          rangeAnchor = null;
          rangeHover = null;
          sync();
        });
        var apply = button(t("range.picker.apply", "Apply"), "cal-apply");
        apply.addEventListener("click", submit);
        actions.appendChild(clear);
        actions.appendChild(apply);
        return actions;
      }

      function buildPanel() {
        host.innerHTML = "";
        var panel = document.createElement("div");
        panel.className = "cal-panel";
        panel.appendChild(renderPresets());
        panel.appendChild(renderModes());
        panel.appendChild(renderReadout());
        panel.appendChild(renderHead());
        gridEl = buildGrid();
        panel.appendChild(gridEl);
        panel.appendChild(renderActions());
        host.appendChild(panel);
        paint();
      }

      function paint() {
        var parts = readoutParts();
        paintReadout(parts);
        paintModes();
        paintGrid();
        announce(parts);
        paintToggle();
      }

      // sync builds a new month grid only when the rendered month changes. Every
      // other state change repaints the cells that are already there, so the node
      // under the pointer survives a hover and a click can land on it.
      function sync() {
        if (builtMonth !== view.y + "-" + view.m) {
          var next = buildGrid();
          gridEl.replaceWith(next);
          gridEl = next;
        }
        paint();
      }

      function step(delta) {
        var m = view.m + delta;
        view.y += Math.floor(m / 12);
        view.m = ((m % 12) + 12) % 12;
        focusKey = null;
        // The cursor left the day the preview pointed at, so the hover goes with
        // the month. The armed start stays: that range is still half-open.
        rangeHover = null;
        sync();
      }

      function showMonthOf(k) {
        var d = parseKey(k);
        view = { y: d.getFullYear(), m: d.getMonth() };
      }

      function selectKeys(keys) {
        selected = {};
        keys.forEach(function (k) {
          selected[k] = true;
        });
      }

      // applyPreset counts in the reader's own calendar days, the ones the grid
      // draws.
      function applyPreset(name) {
        var today = new Date();
        var from = new Date(today);
        var to = new Date(today);
        if (name === "last_7") {
          from.setDate(today.getDate() - 6);
        } else if (name === "last_30") {
          from.setDate(today.getDate() - 29);
        } else if (name === "this_week") {
          from.setDate(today.getDate() - mondayIndex(today));
          to = new Date(from);
          to.setDate(from.getDate() + 6);
        } else {
          from = new Date(today.getFullYear(), today.getMonth(), 1);
          to = new Date(today.getFullYear(), today.getMonth() + 1, 0);
        }
        selectKeys(spanKeys(keyOf(from), keyOf(to)));
        rangeAnchor = null;
        rangeHover = null;
        focusKey = keyOf(to);
        showMonthOf(focusKey);
      }

      function submit() {
        hidden.value = selectedKeys().join(",");
        // A GET submit rebuilds the query from form controls only, which would
        // drop the other params the URL carries (the stats filter tags). Carry
        // them over as hidden inputs; the range's own params stay form-owned.
        var params = new URL(window.location.href).searchParams;
        params.forEach(function (v, k) {
          if (k === "days" || k === "from" || k === "to") return;
          var input = document.createElement("input");
          input.type = "hidden";
          input.name = k;
          input.value = v;
          form.appendChild(input);
        });
        form.submit(); // the server stores the selection for the browser session
      }

      function toggleDay(k) {
        if (selected[k]) delete selected[k];
        else selected[k] = true;
      }

      // A drag accelerates the gesture of the current mode: a span in range mode,
      // one more handful of days in single-days mode.
      function commitDrag() {
        if (!drag.anchor || !drag.hover) return;
        if (mode === "range") selectKeys(spanKeys(drag.anchor, drag.hover));
        else selectKeys(Object.keys(paintedKeys()));
      }

      // pickDay is the one place a tap, a click and Enter all land, so every
      // pointer and the keyboard share one interaction model.
      function pickDay(k, toggleOne) {
        focusKey = k;
        if (toggleOne || mode === "days") {
          toggleDay(k);
          return;
        }
        if (!rangeAnchor) {
          rangeAnchor = k;
          rangeHover = null;
          selectKeys([k]);
          return;
        }
        if (rangeAnchor === k) {
          // Tapping the first day again keeps a one-day range.
          rangeAnchor = null;
          rangeHover = null;
          selectKeys([k]);
          return;
        }
        selectKeys(spanKeys(rangeAnchor, k));
        rangeAnchor = null;
        rangeHover = null;
      }

      // Two interaction paths share the calendar and must not double-handle one
      // tap: drag (mousedown→mouseover→mouseup) and click. A genuine multi-cell
      // drag commits in mouseup and sets didDrag so the trailing click is
      // ignored; a press that never moved leaves didDrag false and falls through
      // to the click handler.
      var didDrag = false;

      host.addEventListener("pointerdown", function (e) {
        pointerFine = e.pointerType === "mouse";
      });

      host.addEventListener("mousedown", function (e) {
        if (!pointerFine) return;
        var cell = e.target.closest ? e.target.closest(".cal-day") : null;
        if (!cell) return;
        e.preventDefault(); // don't start a text selection while sweeping
        didDrag = false; // fresh gesture; clear any flag a prior drag left unconsumed
        drag.active = true;
        drag.anchor = cell.dataset.date;
        drag.hover = cell.dataset.date;
      });
      // A hover changes the preview only, so it repaints the cells and never
      // rebuilds them.
      host.addEventListener("mouseover", function (e) {
        if (!pointerFine) return;
        if (drag.active) {
          var cell = e.target.closest ? e.target.closest(".cal-day") : null;
          if (cell && cell.dataset.date !== drag.hover) {
            drag.hover = cell.dataset.date;
            paint();
          }
          return;
        }
        // While a range start is armed, preview the span out to the hovered day.
        if (rangeAnchor) {
          var c = e.target.closest ? e.target.closest(".cal-day") : null;
          if (c && c.dataset.date !== rangeHover) {
            rangeHover = c.dataset.date;
            paint();
          }
        }
      });
      onDocument("mouseup", function () {
        if (!drag.active) return;
        // Only treat a real sweep (anchor ≠ hover) as a drag; a press that never
        // moved is left for the click handler so it joins the tap flow.
        var swept = drag.anchor !== drag.hover;
        if (swept) {
          commitDrag();
          rangeAnchor = null;
          rangeHover = null;
        }
        didDrag = swept;
        drag.active = false;
        drag.anchor = null;
        drag.hover = null;
        if (swept) paint();
      });

      host.addEventListener("click", function (e) {
        // Swallow the click that trails a real drag. Check before the cell lookup
        // and always clear the flag.
        if (didDrag) {
          didDrag = false;
          return;
        }
        var cell = e.target.closest ? e.target.closest(".cal-day") : null;
        if (!cell) return;
        pickDay(cell.dataset.date, e.shiftKey);
        sync();
        focusDay(focusKey);
      });

      function nextFocusKey(name, from) {
        var d = parseKey(from);
        switch (name) {
          case "ArrowLeft":
            return shiftDay(from, -1);
          case "ArrowRight":
            return shiftDay(from, 1);
          case "ArrowUp":
            return shiftDay(from, -7);
          case "ArrowDown":
            return shiftDay(from, 7);
          case "Home":
            return shiftDay(from, -mondayIndex(d));
          case "End":
            return shiftDay(from, 6 - mondayIndex(d));
          case "PageUp":
          case "PageDown":
            var delta = name === "PageUp" ? -1 : 1;
            var last = new Date(d.getFullYear(), d.getMonth() + delta + 1, 0).getDate();
            // A raw key would read "2026-00-15" in January. The date normalises the
            // month into the year beside it, so the grid keeps its focused cell.
            return keyOf(new Date(d.getFullYear(), d.getMonth() + delta, Math.min(d.getDate(), last)));
          default:
            return null;
        }
      }

      // Enter and Space reach pickDay through the cell's own click event.
      host.addEventListener("keydown", function (e) {
        var cell = e.target.closest ? e.target.closest(".cal-day") : null;
        if (!cell) return;
        var next = nextFocusKey(e.key, cell.dataset.date);
        if (!next) return;
        e.preventDefault();
        focusKey = next;
        showMonthOf(next);
        sync();
        focusDay(next);
      });

      buildPanel();
    });
  }

  document.addEventListener("DOMContentLoaded", function () {
    init(document);
  });
  document.addEventListener("htmx:afterSwap", function (e) {
    init(e.target);
  });
})();
