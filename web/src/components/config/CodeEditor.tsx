import { useEffect, useRef } from 'react';
import { EditorState, type Extension } from '@codemirror/state';
import { EditorView, keymap, lineNumbers, highlightActiveLine, drawSelection, placeholder as placeholderExt } from '@codemirror/view';
import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands';
import { bracketMatching, defaultHighlightStyle, indentOnInput, syntaxHighlighting } from '@codemirror/language';
import { autocompletion, closeBrackets, closeBracketsKeymap, completionKeymap, type CompletionContext, type CompletionResult } from '@codemirror/autocomplete';
import { javascript } from '@codemirror/lang-javascript';
import { json } from '@codemirror/lang-json';

export type CodeLanguage = 'javascript' | 'json' | 'mustache' | 'text';

export interface CodeEditorProps {
  value: string;
  onChange: (value: string) => void;
  language?: CodeLanguage;
  placeholder?: string;
  /** DOM id of the editor's content element, so a label can point at it. */
  id?: string;
  minHeight?: string;
  'aria-label'?: string;
}

/** Message properties the Function node sees; offered after "msg." */
const MESSAGE_FIELDS = ['payload', 'topic', 'headers', 'statusCode', 'req', 'res', 'error', 'parts', 'count', 'filename', 'status'];

function messageCompletions(context: CompletionContext): CompletionResult | null {
  const word = context.matchBefore(/(msg|flow|global)\.\w*/);
  if (!word && !context.explicit) return null;
  if (word && word.text.startsWith('msg.')) {
    return {
      from: word.from + 4,
      options: MESSAGE_FIELDS.map((label) => ({ label, type: 'property' })),
      validFor: /^\w*$/,
    };
  }
  if (word) {
    const from = word.from + word.text.indexOf('.') + 1;
    return {
      from,
      options: [
        { label: 'get', type: 'method', apply: 'get("")', detail: '(key)' },
        { label: 'set', type: 'method', apply: 'set("", )', detail: '(key, value)' },
      ],
      validFor: /^\w*$/,
    };
  }
  const start = context.matchBefore(/\w*/);
  return {
    from: start ? start.from : context.pos,
    options: [
      { label: 'msg', type: 'variable' },
      { label: 'flow', type: 'variable' },
      { label: 'global', type: 'variable' },
      { label: 'return msg;', type: 'keyword' },
    ],
    validFor: /^\w*$/,
  };
}

function languageExtensions(language: CodeLanguage): Extension[] {
  switch (language) {
    case 'javascript':
      return [javascript(), autocompletion({ override: [messageCompletions] })];
    case 'json':
      return [json()];
    default:
      return [];
  }
}

const theme = EditorView.theme({
  '&': { fontSize: '12px', border: '1px solid var(--line)', borderRadius: '6px', backgroundColor: 'var(--bg-panel)', color: 'var(--fg)' },
  '&.cm-focused': { outline: '2px solid var(--accent)', outlineOffset: '-1px' },
  '.cm-content': { fontFamily: '"JetBrains Mono", ui-monospace, SFMono-Regular, Menlo, monospace', padding: '4px 0', caretColor: 'var(--fg)' },
  '.cm-cursor': { borderLeftColor: 'var(--fg)' },
  '.cm-gutters': { backgroundColor: 'var(--bg-surface)', color: 'var(--fg-faint)', border: 'none' },
  '.cm-activeLine': { backgroundColor: 'var(--bg-surface)' },
  '.cm-activeLineGutter': { backgroundColor: 'var(--bg-sunken)' },
  '.cm-selectionBackground, &.cm-focused .cm-selectionBackground': { backgroundColor: 'var(--selection) !important' },
  '.cm-tooltip': { backgroundColor: 'var(--bg-panel)', border: '1px solid var(--line)', color: 'var(--fg)' },
  '.cm-tooltip-autocomplete ul li[aria-selected]': { backgroundColor: 'var(--accent-soft)', color: 'var(--fg)' },
  '.cm-scroller': { overflow: 'auto' },
});

/**
 * CodeMirror 6 wrapper used by the code and JSON widgets: JavaScript with
 * completions for msg./flow./global., JSON with highlighting, plain text
 * otherwise. The editor is created once; external value changes are
 * applied only while the editor is not focused, so typing never fights
 * with a re-render.
 */
export function CodeEditor({ value, onChange, language = 'text', placeholder, id, minHeight = '8rem', 'aria-label': ariaLabel }: CodeEditorProps) {
  const host = useRef<HTMLDivElement>(null);
  const view = useRef<EditorView | null>(null);
  const latestOnChange = useRef(onChange);
  latestOnChange.current = onChange;

  useEffect(() => {
    if (!host.current) return;
    const extensions: Extension[] = [
      lineNumbers(),
      history(),
      drawSelection(),
      indentOnInput(),
      bracketMatching(),
      closeBrackets(),
      highlightActiveLine(),
      syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
      keymap.of([...closeBracketsKeymap, ...defaultKeymap, ...historyKeymap, ...completionKeymap, indentWithTab]),
      ...languageExtensions(language),
      theme,
      EditorView.theme({ '.cm-content, .cm-gutter': { minHeight } }),
      EditorView.updateListener.of((update) => {
        if (update.docChanged) latestOnChange.current(update.state.doc.toString());
      }),
      EditorView.contentAttributes.of({ 'aria-label': ariaLabel || 'code', ...(id ? { id } : {}) }),
    ];
    if (placeholder) extensions.push(placeholderExt(placeholder));

    const editor = new EditorView({
      state: EditorState.create({ doc: value, extensions }),
      parent: host.current,
    });
    view.current = editor;
    return () => {
      editor.destroy();
      view.current = null;
    };
    // The editor is created once per language; value changes are synced below.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [language]);

  useEffect(() => {
    const editor = view.current;
    if (!editor || editor.hasFocus) return;
    const current = editor.state.doc.toString();
    if (current !== value) {
      editor.dispatch({ changes: { from: 0, to: current.length, insert: value } });
    }
  }, [value]);

  return <div ref={host} data-testid="code-editor" data-language={language} />;
}
