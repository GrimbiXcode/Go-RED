/**
 * Dev-only visual reference for the Go-RED design tokens introduced in
 * Phase 0 of docs/FRONTEND_NODE_RED_REDESIGN.md. Not part of the shipped
 * app UI — only mounted when running in dev mode (see App.tsx).
 */

const steps = [50, 100, 200, 300, 400, 500, 600, 700, 800, 900] as const;

// Tailwind's JIT scanner only picks up class names that appear as literal
// substrings in source files, so the swatch classes are spelled out here
// instead of built via template interpolation (`bg-${name}-${step}` would
// never be generated).
const scales = [
  {
    label: 'Gopher Blue (verified: 500)',
    verifiedStep: 500,
    classes: {
      50: 'bg-gr-blue-50',
      100: 'bg-gr-blue-100',
      200: 'bg-gr-blue-200',
      300: 'bg-gr-blue-300',
      400: 'bg-gr-blue-400',
      500: 'bg-gr-blue-500',
      600: 'bg-gr-blue-600',
      700: 'bg-gr-blue-700',
      800: 'bg-gr-blue-800',
      900: 'bg-gr-blue-900',
    },
  },
  {
    label: 'Light Blue (verified: 400)',
    verifiedStep: 400,
    classes: {
      50: 'bg-gr-skyblue-50',
      100: 'bg-gr-skyblue-100',
      200: 'bg-gr-skyblue-200',
      300: 'bg-gr-skyblue-300',
      400: 'bg-gr-skyblue-400',
      500: 'bg-gr-skyblue-500',
      600: 'bg-gr-skyblue-600',
      700: 'bg-gr-skyblue-700',
      800: 'bg-gr-skyblue-800',
      900: 'bg-gr-skyblue-900',
    },
  },
  {
    label: 'Fuchsia (verified: 500)',
    verifiedStep: 500,
    classes: {
      50: 'bg-gr-fuchsia-50',
      100: 'bg-gr-fuchsia-100',
      200: 'bg-gr-fuchsia-200',
      300: 'bg-gr-fuchsia-300',
      400: 'bg-gr-fuchsia-400',
      500: 'bg-gr-fuchsia-500',
      600: 'bg-gr-fuchsia-600',
      700: 'bg-gr-fuchsia-700',
      800: 'bg-gr-fuchsia-800',
      900: 'bg-gr-fuchsia-900',
    },
  },
] as const satisfies ReadonlyArray<{
  label: string;
  verifiedStep: number;
  classes: Record<(typeof steps)[number], string>;
}>;

function ColorScale({ label, verifiedStep, classes }: (typeof scales)[number]) {
  return (
    <div className="mb-6">
      <div className="text-sm font-semibold text-gray-700 mb-2">{label}</div>
      <div className="flex rounded overflow-hidden border border-gray-200">
        {steps.map((step) => (
          <div
            key={step}
            className={`${classes[step]} flex-1 h-16 flex items-end justify-center pb-1 text-[10px] font-mono`}
            style={{ color: step >= 500 ? 'white' : '#1e293b' }}
          >
            {step}
            {step === verifiedStep ? ' *' : ''}
          </div>
        ))}
      </div>
    </div>
  );
}

function StatusSample({
  colorClass,
  label,
}: {
  colorClass: string;
  label: string;
}) {
  return (
    <div className="flex items-center gap-2 text-sm">
      <span className={`w-2.5 h-2.5 rounded-full ${colorClass}`} />
      <span>{label}</span>
    </div>
  );
}

export function StyleGuide() {
  return (
    <div className="p-8 max-w-4xl mx-auto">
      <h1 className="text-xl font-bold mb-1">Go-RED Design Tokens</h1>
      <p className="text-sm text-gray-500 mb-8">
        Phase 0 style guide — see docs/FRONTEND_NODE_RED_REDESIGN.md. Values marked with * are
        verified against the official Go brand book (golang/go#29695, golang/go#25136).
      </p>

      <section className="mb-10">
        <h2 className="text-lg font-semibold mb-4">Farbskalen</h2>
        {scales.map((s) => (
          <ColorScale key={s.label} {...s} />
        ))}
      </section>

      <section className="mb-10">
        <h2 className="text-lg font-semibold mb-4">Statusfarben (semantisch)</h2>
        <div className="flex flex-col gap-2 bg-white p-4 rounded border border-gray-200 w-fit">
          <StatusSample colorClass="bg-gr-blue-500" label="running / deployed" />
          <StatusSample colorClass="bg-gr-skyblue-400" label="processing" />
          <StatusSample colorClass="bg-gr-fuchsia-500" label="error" />
          <StatusSample colorClass="bg-gray-300" label="idle / draft" />
        </div>
      </section>

      <section className="mb-10">
        <h2 className="text-lg font-semibold mb-4">Typografie</h2>
        <div className="bg-white p-4 rounded border border-gray-200 font-sans">
          <div className="text-[13px]">
            Body-Text 13px, Systemschrift-Stack (font-sans) — kompakter als Tailwind-Default, wie
            im dichten Node-RED-UI.
          </div>
        </div>
      </section>

      <section>
        <h2 className="text-lg font-semibold mb-4">Node-Geometrie (Referenz, ungeprüft)</h2>
        <div
          className="bg-gr-blue-500 text-white flex items-center px-2 rounded-gr-node"
          style={{
            height: 'var(--gr-node-height)',
            minWidth: 'var(--gr-node-min-width)',
            width: 'fit-content',
          }}
        >
          <span
            className="bg-white/70 rounded-full mr-2"
            style={{ width: 'var(--gr-port-size)', height: 'var(--gr-port-size)' }}
          />
          <span className="text-sm">node</span>
        </div>
        <p className="text-xs text-gray-400 mt-2">
          Höhe/Breite/Radius/Port-Größe aus dokumentierten NR-Defaults übernommen, noch nicht
          gegen eine laufende Node-RED-Instanz verifiziert (offener Punkt aus Phase 0).
        </p>
      </section>
    </div>
  );
}
