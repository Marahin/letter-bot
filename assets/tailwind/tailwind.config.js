/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    // The web shell lives in internal/infrastructure/web and each feature in its own
    // sibling package (internal/infrastructure/<feature>-http), so scan the whole infra
    // tree, otherwise classes used only in one feature's templ get purged.
    "./internal/infrastructure/**/*.templ",
    "./internal/infrastructure/**/*_templ.go",
  ],
  theme: {
    extend: {
      colors: {
        // TibiaLoot.com (SC:X Manager design), a warm-leaning near-black tactical surface system.
        // The "zone" scale is kept as the token name so existing markup stays
        // compact; the values are the design system's neutral ramp. Elevation
        // steps up only slightly (#0A0A0B → #111113 → #17171A) and reads through
        // borders first. Consume via bg-zone-*/text-zone-*/border-zone-*.
        zone: {
          50: "#F4F4F5", // primary text
          100: "#F4F4F5", // primary text
          200: "#D4D4D8",
          300: "#A8A8B0", // secondary text
          400: "#71717A", // tertiary text
          500: "#52525B", // disabled text
          600: "#3A3A42", // strong border
          700: "#2A2A30", // default border
          800: "#1F1F23", // hover / raised
          850: "#17171A", // input / raised surface
          900: "#111113", // card surface
          950: "#0A0A0B", // page background
        },
        // The one brand orange. DEFAULT fills buttons/active/focus; "bright" is
        // the lighter accent text on dark (and the fill hover tint).
        signal: {
          DEFAULT: "#F97316",
          bright: "#FB923C",
        },
      },
      boxShadow: {
        // Deep, soft shadows on the near-black canvas, no inset white highlight
        // (the design carries elevation through borders, not glass sheen).
        glass: "0 16px 48px rgba(0, 0, 0, 0.55)",
        "glass-sm": "0 6px 20px rgba(0, 0, 0, 0.45)",
        // Accent glow, reserved for the primary CTA on a view.
        accent: "0 6px 22px rgba(249, 115, 22, 0.28)",
      },
      fontFamily: {
        // Space Grotesk, page titles, server/squad names, section headers.
        display: ["'Space Grotesk'", "'Hanken Grotesk'", "system-ui", "sans-serif"],
        // Hanken Grotesk, all functional copy, labels, table cells.
        sans: ["'Hanken Grotesk'", "system-ui", "-apple-system", "Segoe UI", "sans-serif"],
        // JetBrains Mono, cron, stat figures, KDA, IDs, version strings.
        mono: ["'JetBrains Mono'", "ui-monospace", "SF Mono", "Menlo", "monospace"],
      },
    },
  },
  plugins: [],
};
