/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      // Go-RED design tokens (see docs/FRONTEND_NODE_RED_REDESIGN.md).
      // gr-blue/gr-skyblue/gr-fuchsia are tint/shade scales derived from the
      // three verified Go brand colors (Gopher Blue #00ADD8, Light Blue
      // #5DC9E2, Fuchsia #CE3262 — golang/go#29695, golang/go#25136). Each
      // scale keeps the brand hue constant and varies lightness/saturation;
      // the verified base hex lands exactly on the marked step.
      colors: {
        'gr-blue': {
          50: 'hsl(192, 100%, 96%)',
          100: 'hsl(192, 95%, 91%)',
          200: 'hsl(192, 95%, 83%)',
          300: 'hsl(192, 95%, 70%)',
          400: 'hsl(192, 95%, 55%)',
          500: '#00add8', // verified Gopher Blue
          600: 'hsl(192, 100%, 36%)',
          700: 'hsl(192, 100%, 29%)',
          800: 'hsl(192, 100%, 23%)',
          900: 'hsl(192, 100%, 17%)',
        },
        'gr-skyblue': {
          50: 'hsl(191, 70%, 97%)',
          100: 'hsl(191, 70%, 93%)',
          200: 'hsl(191, 70%, 85%)',
          300: 'hsl(191, 70%, 74%)',
          400: '#5dc9e2', // verified Light Blue
          500: 'hsl(191, 75%, 54%)',
          600: 'hsl(191, 80%, 45%)',
          700: 'hsl(191, 85%, 37%)',
          800: 'hsl(191, 90%, 29%)',
          900: 'hsl(191, 95%, 21%)',
        },
        'gr-fuchsia': {
          50: 'hsl(342, 65%, 97%)',
          100: 'hsl(342, 65%, 93%)',
          200: 'hsl(342, 63%, 85%)',
          300: 'hsl(342, 62%, 71%)',
          400: 'hsl(342, 61%, 60%)',
          500: '#ce3262', // verified Fuchsia
          600: 'hsl(342, 63%, 43%)',
          700: 'hsl(342, 65%, 35%)',
          800: 'hsl(342, 68%, 27%)',
          900: 'hsl(342, 70%, 20%)',
        },
        primary: {
          50: '#f0fdf4',
          100: '#dcfce7',
          200: '#bbf7d0',
          300: '#86efac',
          400: '#4ade80',
          500: '#22c55e',
          600: '#16a34a',
          700: '#15803d',
          800: '#166534',
          900: '#14532d',
        },
        secondary: {
          50: '#f8fafc',
          100: '#f1f5f9',
          200: '#e2e8f0',
          300: '#cbd5e1',
          400: '#94a3b8',
          500: '#64748b',
          600: '#475569',
          700: '#334155',
          800: '#1e293b',
          900: '#0f172a',
        },
        dark: {
          50: '#f7fafc',
          100: '#edf2f7',
          200: '#e2e8f0',
          300: '#cbd5e1',
          400: '#94a3b8',
          500: '#64748b',
          600: '#475569',
          700: '#334155',
          800: '#1e293b',
          900: '#111827',
          1000: '#0f172a',
        },
      },
      // Compact, Node-RED-like UI font stack (system sans, not a webfont).
      fontFamily: {
        sans: [
          '-apple-system',
          'BlinkMacSystemFont',
          '"Segoe UI"',
          'Roboto',
          'Helvetica',
          'Arial',
          'sans-serif',
        ],
      },
      // Node geometry tokens, approximated from commonly documented
      // Node-RED editor defaults (grid size, node height, port size).
      // Not yet cross-checked against a live NR instance/theme CSS — see
      // "Offene Fragen" in docs/FRONTEND_NODE_RED_REDESIGN.md.
      borderRadius: {
        'gr-node': '4px',
      },
      spacing: {
        'gr-grid': '20px',
        'gr-node-h': '30px',
        'gr-node-min-w': '100px',
        'gr-port': '10px',
      },
    },
  },
  plugins: [],
}