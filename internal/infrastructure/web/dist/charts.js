// On-hover value tooltips for the server-rendered charts. Anything carrying a
// `data-chart-tip` attribute shows its text: line-point hit targets, bars and
// sparklines, all of which are HTML elements (the SVG holds only geometry, no
// glyphs, so nothing inside it stretches with the plot). This wires a
// single floating tooltip via event delegation on document, so it works for
// every chart on the page including htmx-swapped ones with no re-init.
(function () {
  var tip;

  function ensure() {
    if (tip) return tip;
    tip = document.createElement("div");
    tip.id = "chart-tooltip";
    // Inline styles (not Tailwind classes, this file isn't scanned by Tailwind)
    // matching the zone palette: raised surface, default border, mono figures.
    tip.style.cssText = [
      "position:fixed",
      "z-index:60",
      "pointer-events:none",
      "display:none",
      "padding:4px 8px",
      "border-radius:6px",
      "border:1px solid #2A2A30",
      "background:#17171A",
      "color:#F4F4F5",
      "font-family:'JetBrains Mono',ui-monospace,monospace",
      "font-size:12px",
      "line-height:1.2",
      "white-space:nowrap",
      "box-shadow:0 6px 20px rgba(0,0,0,0.45)",
    ].join(";");
    document.body.appendChild(tip);
    return tip;
  }

  function place(x, y) {
    var t = ensure();
    var pad = 14;
    var w = t.offsetWidth;
    var left = x + pad;
    // Flip to the left of the cursor near the right edge so it never clips.
    if (left + w > window.innerWidth - 8) left = x - pad - w;
    t.style.left = left + "px";
    t.style.top = y + pad + "px";
  }

  function show(text, x, y) {
    var t = ensure();
    t.textContent = text;
    t.style.display = "block";
    place(x, y);
  }

  function hide() {
    if (tip) tip.style.display = "none";
  }

  document.addEventListener("mouseover", function (e) {
    var el = e.target.closest("[data-chart-tip]");
    if (el) show(el.getAttribute("data-chart-tip"), e.clientX, e.clientY);
  });

  document.addEventListener("mousemove", function (e) {
    if (!tip || tip.style.display !== "block") return;
    if (e.target.closest("[data-chart-tip]")) place(e.clientX, e.clientY);
    else hide();
  });

  document.addEventListener("mouseout", function (e) {
    if (e.target.closest("[data-chart-tip]")) hide();
  });
})();
