import { useState, type ReactNode } from 'react';
import { NodeShell } from '../components/NodeShell';
import { NodeIcon } from '../components/CategoryIcon';
import { DebugEntry } from '../components/DebugPanel';
import { PropertyFields } from '../components/config/PropertyFields';
import { GoRedMark } from '../components/Header';
import { CATEGORY_FILLS, CATEGORY_ORDER, contrastRatio, getCategoryColor } from '../utils/nodeCategories';
import { useThemeStore, type ResolvedTheme } from '../lib/theme';
import type { DebugMessage, Property, Schema } from '../types/generated';

/**
 * Living style guide at /styleguide: the design tokens, every node
 * category, node states, ports, controls, form widgets and debug entries,
 * light and dark side by side. Everything on this page is the real
 * component, so it cannot drift from the editor.
 */

const TOKEN_GROUPS: { title: string; tokens: string[] }[] = [
  { title: 'Surfaces', tokens: ['bg-app', 'bg-panel', 'bg-surface', 'bg-sunken', 'bg-header', 'bg-canvas', 'grid-dot'] },
  { title: 'Text and lines', tokens: ['fg', 'fg-muted', 'fg-faint', 'line', 'line-strong'] },
  { title: 'Accent', tokens: ['accent', 'accent-strong', 'accent-soft', 'accent-text', 'selection'] },
  { title: 'Feedback', tokens: ['danger', 'danger-soft', 'danger-text', 'warn', 'warn-soft', 'warn-text', 'ok', 'ok-soft', 'ok-text'] },
  { title: 'Wires', tokens: ['wire', 'wire-hover'] },
];

const SAMPLE_SCHEMA: Schema = {
  properties: {
    url: prop({ label: 'URL', order: 1, widget: 'text', placeholder: 'https://example.org', description: 'Where to send the request' }),
    method: prop({ label: 'Method', order: 2, widget: 'select', enum: ['GET', 'POST'], default: 'GET' }),
    timeoutMs: prop({ type: 'number', label: 'Timeout', order: 3, widget: 'duration', unit: 'ms', default: 30000 }),
    property: prop({ type: 'object', label: 'Property', order: 4, widget: 'typedInput', typedInput: { types: ['msg', 'flow', 'global'], default: 'msg' }, default: { type: 'msg', path: 'payload' } }),
    retry: prop({ type: 'boolean', label: 'Retry on failure', order: 5, widget: 'boolean', default: true }),
    headers: prop({ type: 'object', label: 'Headers', order: 6, widget: 'keyValue', group: 'Advanced' }),
    code: prop({ label: 'Function', order: 7, widget: 'code', language: 'javascript', default: 'msg.payload = 42;\nreturn msg;', group: 'Advanced' }),
  },
  required: ['url'],
};

function prop(overrides: Partial<Property>): Property {
  return { type: 'string', description: '', default: undefined, enum: [], pattern: '', ...overrides };
}

const SAMPLE_DEBUG: DebugMessage[] = [
  { id: '1', flowId: 'f', nodeId: 'n3', nodeName: 'Out', nodeType: 'debug', level: 'debug', topic: 'sensors/temp', payload: 21.5, timestamp: new Date().toISOString() },
  { id: '2', flowId: 'f', nodeId: 'n3', nodeName: 'Out', nodeType: 'debug', level: 'debug', payload: { temperature: 21.5, unit: 'C', tags: ['indoor', 'living-room'], meta: { source: 'mqtt' } }, timestamp: new Date().toISOString() },
  { id: '3', flowId: 'f', nodeId: 'fn', nodeName: 'Transform', nodeType: 'function', level: 'error', payload: 'JavaScript execution error: ReferenceError: foo is not defined', timestamp: new Date().toISOString() },
];

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="mb-10">
      <h2 className="text-lg font-semibold text-fg mb-3">{title}</h2>
      {children}
    </section>
  );
}

function ThemeFrame({ theme, children }: { theme: ResolvedTheme; children: ReactNode }) {
  return (
    <div data-theme={theme} className="bg-app text-fg rounded-lg border border-line p-4 space-y-4">
      <div className="text-2xs uppercase tracking-wide text-faint">{theme}</div>
      {children}
    </div>
  );
}

function Swatch({ token }: { token: string }) {
  return (
    <div className="flex items-center gap-2 text-xs">
      <span className="w-6 h-6 rounded border border-line shrink-0" style={{ background: `var(--${token})` }} />
      <span className="font-mono text-2xs text-muted">--{token}</span>
    </div>
  );
}

function PortDot({ color }: { color: string }) {
  return <span className="inline-block w-2.5 h-2.5 rounded-full bg-panel" style={{ border: `2px solid ${color}` }} />;
}

function CategoryRow() {
  return (
    <div className="grid grid-cols-2 gap-x-6 gap-y-4">
      {CATEGORY_ORDER.map((category) => {
        const color = getCategoryColor(category);
        return (
          <div key={category} className="flex items-center gap-3">
            <PortDot color={color.fill} />
            <NodeShell category={category} label={category} icon={<NodeIcon category={category} className="w-4 h-4" />} status={{ fill: 'green', shape: 'dot', text: 'ready' }} />
            <PortDot color={color.fill} />
            <span className="font-mono text-2xs text-faint">
              {CATEGORY_FILLS[category]} · {contrastRatio('#ffffff', CATEGORY_FILLS[category]).toFixed(1)}:1
            </span>
          </div>
        );
      })}
    </div>
  );
}

function NodeStates() {
  const icon = <NodeIcon icon="timer" category="function" className="w-4 h-4" />;
  return (
    <div className="flex flex-wrap gap-6 items-start">
      <NodeShell category="function" label="Default" icon={icon} />
      <NodeShell category="function" label="Selected" icon={icon} selected />
      <NodeShell category="function" label="Disabled" icon={icon} disabled />
      <NodeShell category="function" label="Error status" icon={icon} status={{ fill: 'red', shape: 'ring', text: 'disconnected' }} />
      <NodeShell category="function" label="Connecting" icon={icon} status={{ fill: 'yellow', shape: 'dot', text: 'connecting' }} />
      <NodeShell category="flow-control" label="Note" icon={<NodeIcon icon="message-square" category="flow-control" className="w-4 h-4" />} color="#FEF3C7" />
    </div>
  );
}

function Controls() {
  return (
    <div className="flex flex-wrap gap-2 items-center">
      <button className="px-3 h-8 rounded-md bg-accent text-accent-fg text-xs font-semibold hover:bg-accent-strong">Primary</button>
      <button className="px-3 h-8 rounded-md border border-line-strong text-xs font-medium text-fg hover:bg-surface">Secondary</button>
      <button className="px-3 h-8 rounded-md bg-danger text-danger-fg text-xs font-medium hover:bg-danger-strong">Danger</button>
      <button className="px-3 h-8 rounded-md bg-accent text-accent-fg text-xs font-semibold opacity-40 cursor-not-allowed" disabled>
        Disabled
      </button>
      <span className="px-2 py-0.5 rounded-full bg-accent-soft text-accent-text text-2xs">chip</span>
      <span className="px-2 py-0.5 rounded-full bg-danger-soft text-danger-text text-2xs">error</span>
      <span className="px-2 py-0.5 rounded-full bg-warn-soft text-warn-text text-2xs">warning</span>
      <span className="px-2 py-0.5 rounded-full bg-ok-soft text-ok-text text-2xs">ok</span>
    </div>
  );
}

function Widgets() {
  const [config, setConfig] = useState<Record<string, unknown>>({ url: '', method: 'GET', timeoutMs: 30000, property: { type: 'msg', path: 'payload' }, retry: true, headers: { Accept: 'application/json' }, code: 'msg.payload = 42;\nreturn msg;' });
  return (
    <div className="max-w-sm">
      <PropertyFields
        schema={SAMPLE_SCHEMA}
        config={config}
        onChange={(key, value) => setConfig((prev) => ({ ...prev, [key]: value }))}
        errors={{ url: 'Required' }}
        context={{ flow: null, nodeTypes: [] }}
        setInvalid={() => undefined}
      />
    </div>
  );
}

function Typography() {
  return (
    <div className="space-y-1 text-fg">
      <div className="text-2xl font-semibold">24 px · headings</div>
      <div className="text-xl font-semibold">20 px · page titles</div>
      <div className="text-lg font-semibold">16 px · section titles</div>
      <div className="text-base">14 px · dialogs</div>
      <div className="text-sm">13 px · body, node labels (Inter)</div>
      <div className="text-xs">12 px · form fields, palette</div>
      <div className="text-2xs">11 px · status lines, hints</div>
      <div className="font-mono text-xs">JetBrains Mono · code, ids, payloads</div>
    </div>
  );
}

function GuideBody() {
  return (
    <>
      <Section title="Categories">
        <CategoryRow />
      </Section>
      <Section title="Node states">
        <NodeStates />
      </Section>
      <Section title="Controls">
        <Controls />
      </Section>
      <Section title="Widgets">
        <Widgets />
      </Section>
      <Section title="Debug entries">
        <div className="space-y-1.5 max-w-md">
          {SAMPLE_DEBUG.map((message) => (
            <DebugEntry key={message.id} message={message} label={message.nodeName || message.nodeId} />
          ))}
        </div>
      </Section>
      <Section title="Typography">
        <Typography />
      </Section>
    </>
  );
}

export function StyleGuide() {
  const resolved = useThemeStore((state) => state.resolved);
  const setPreference = useThemeStore((state) => state.setPreference);
  const other: ResolvedTheme = resolved === 'dark' ? 'light' : 'dark';

  return (
    <div className="min-h-screen bg-app text-fg overflow-y-auto">
      <div className="p-8 max-w-6xl mx-auto">
        <div className="flex items-center gap-3 mb-1">
          <GoRedMark className="w-8 h-8" />
          <h1 className="text-xl font-bold">Go-RED style guide</h1>
          <button className="ml-auto px-3 h-8 rounded-md border border-line-strong text-xs font-medium hover:bg-surface" onClick={() => setPreference(other)}>
            Switch page to {other}
          </button>
        </div>
        <p className="text-sm text-muted mb-8">
          Tokens v2 (docs/NEXT_LEVEL_PLAN.md, Phase 4). Every text on background pair below is checked for 4.5:1 by src/test/tokens.test.ts; the
          components are the editor&apos;s own.
        </p>

        <Section title="Tokens">
          <div className="grid grid-cols-2 gap-4">
            {(['light', 'dark'] as ResolvedTheme[]).map((theme) => (
              <ThemeFrame key={theme} theme={theme}>
                {TOKEN_GROUPS.map((group) => (
                  <div key={group.title}>
                    <div className="text-xs font-semibold text-fg mb-1">{group.title}</div>
                    <div className="grid grid-cols-2 gap-1">
                      {group.tokens.map((token) => (
                        <Swatch key={token} token={token} />
                      ))}
                    </div>
                  </div>
                ))}
              </ThemeFrame>
            ))}
          </div>
        </Section>

        <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
          {(['light', 'dark'] as ResolvedTheme[]).map((theme) => (
            <ThemeFrame key={theme} theme={theme}>
              <GuideBody />
            </ThemeFrame>
          ))}
        </div>
      </div>
    </div>
  );
}
