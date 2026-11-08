import { describe, it, expect, afterEach } from 'vitest';
import { editorKeysBlocked, inDialog, isEditableTarget } from '../lib/keyboard';

function mount(html: string): HTMLElement {
  const host = document.createElement('div');
  host.innerHTML = html;
  document.body.appendChild(host);
  return host;
}

afterEach(() => {
  document.body.innerHTML = '';
});

describe('keyboard guards', () => {
  it('treats text fields as editable and buttons as not', () => {
    const host = mount('<input id="i"><textarea id="t"></textarea><select id="s"></select><button id="b"></button>');
    expect(isEditableTarget(host.querySelector('#i'))).toBe(true);
    expect(isEditableTarget(host.querySelector('#t'))).toBe(true);
    expect(isEditableTarget(host.querySelector('#s'))).toBe(true);
    expect(isEditableTarget(host.querySelector('#b'))).toBe(false);
    expect(isEditableTarget(null)).toBe(false);
  });

  it('blocks editor keys with focus inside a dialog, such as the edit tray', () => {
    const host = mount('<div role="dialog"><button id="close"></button></div><button id="outside"></button>');
    expect(editorKeysBlocked({ target: host.querySelector('#close') })).toBe(true);
    expect(editorKeysBlocked({ target: host.querySelector('#outside') })).toBe(false);
    expect(editorKeysBlocked({ target: document.body })).toBe(false);
  });

  it('blocks editor keys everywhere while a modal dialog is open', () => {
    mount('<div role="dialog" aria-modal="true"></div>');
    expect(inDialog({ target: document.body })).toBe(true);
    expect(editorKeysBlocked({ target: document.body })).toBe(true);
  });
});
