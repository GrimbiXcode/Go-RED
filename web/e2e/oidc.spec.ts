import { readFile } from 'node:fs/promises';
import { test, expect, type Page } from '@playwright/test';
import { OIDC_ISSUER, OIDC_SERVER_URL, startOidcProvider, type FakeProvider } from './oidcProvider';

/**
 * OIDC sign-in against the real server (the second instance from
 * playwright.config.ts) and a fake provider: the editor's automatic
 * redirect, the callback, the session cookie on REST and WebSocket, the
 * same-origin rule for writes, sign-out, the allowlist and the redirect
 * target check.
 */

let provider: FakeProvider;

test.beforeAll(async () => {
  provider = await startOidcProvider();
});

test.afterAll(async () => {
  await provider?.close();
});

/** Waits for the provider's page and signs in as one of its test users. */
async function signInAtProvider(page: Page, user: 'ada' | 'eve') {
  await page.waitForURL(`${OIDC_ISSUER}/authorize?**`);
  await page.getByTestId(`idp-user-${user}`).click();
}

async function openMenu(page: Page) {
  await page.getByRole('button', { name: 'Main menu' }).click();
}

test.describe('OIDC sign-in', () => {
  test('the editor signs in through the provider, uses the session, and signs out', async ({ page, context, request }) => {
    expect((await request.get(`${OIDC_SERVER_URL}/api/flows`)).status()).toBe(401);
    const info = await (await request.get(`${OIDC_SERVER_URL}/api/auth/info`)).json();
    expect(info).toEqual({ methods: ['oidc'], loginUrl: '/auth/login' });

    // Signed out, the editor goes to the provider on its own and comes back
    // to the page it was opened on.
    await page.goto(`${OIDC_SERVER_URL}/?from=e2e`);
    await signInAtProvider(page, 'ada');
    await page.waitForURL(`${OIDC_SERVER_URL}/?from=e2e`);
    await expect(page.getByRole('dialog', { name: 'Sign in' })).toHaveCount(0);
    // The WebSocket handshake carries the cookie too.
    await expect(page.getByTestId('app-header').getByTestId('ws-status')).toHaveAttribute('data-state', 'connected');
    await openMenu(page);
    await expect(page.getByTestId('signed-in-user')).toHaveText('Ada Lovelace');

    const cookies = await context.cookies();
    const session = cookies.find((cookie) => cookie.name === 'gored_session');
    expect(session).toMatchObject({ httpOnly: true, sameSite: 'Lax', path: '/' });
    expect(cookies.find((cookie) => cookie.name === 'gored_login')).toBeUndefined();

    // The cookie alone does not authorize a write: without a same-origin
    // Origin (which browsers always send, other sites cannot) it is a 401.
    const name = `OIDC flow ${Date.now()}`;
    expect((await page.request.post(`${OIDC_SERVER_URL}/api/flows`, { data: { name } })).status()).toBe(401);
    const created = await page.request.post(`${OIDC_SERVER_URL}/api/flows`, { data: { name }, headers: { Origin: OIDC_SERVER_URL } });
    expect(created.status()).toBe(201);
    await page.reload();
    await expect(page.getByRole('tab', { name })).toBeVisible();

    // Signing out clears the cookie and shows the dialog instead of going
    // straight back to the provider.
    const infoLoaded = page.waitForResponse((response) => response.url().endsWith('/api/auth/info'));
    await openMenu(page);
    await page.getByRole('menuitem', { name: 'Sign out' }).click();
    await infoLoaded;
    await expect(page.getByTestId('signed-out')).toBeVisible();
    expect(page.url().startsWith(OIDC_SERVER_URL)).toBe(true);
    expect((await context.cookies()).find((cookie) => cookie.name === 'gored_session')).toBeUndefined();
    expect((await page.request.get(`${OIDC_SERVER_URL}/api/flows`)).status()).toBe(401);

    // The dialog's button signs in again.
    const issued = provider.issued();
    await page.getByTestId('sso-button').click();
    await signInAtProvider(page, 'ada');
    await page.waitForURL(`${OIDC_SERVER_URL}/**`);
    await expect(page.getByRole('tab', { name })).toBeVisible();
    expect(provider.issued()).toBe(issued + 1);
  });

  test('edits made after the session expired survive signing in again', async ({ page, context }) => {
    await page.goto(`${OIDC_SERVER_URL}/`);
    await signInAtProvider(page, 'ada');
    await page.waitForURL(`${OIDC_SERVER_URL}/`);

    const flowId = `expiry-${Date.now()}`;
    const write = { headers: { Origin: OIDC_SERVER_URL } };
    expect((await page.request.post(`${OIDC_SERVER_URL}/api/flows`, { ...write, data: { id: flowId, name: 'Expiry' } })).status()).toBe(201);
    const nodes = { n1: { id: 'n1', type: 'debug', name: 'Before', position: { x: 120, y: 120 }, config: {}, disabled: false } };
    expect((await page.request.put(`${OIDC_SERVER_URL}/api/flows/${flowId}`, { ...write, data: { nodes, connections: [] } })).ok()).toBeTruthy();
    await page.goto(`${OIDC_SERVER_URL}/flow/${flowId}`);
    await expect(page.locator('.react-flow__node[data-id="n1"]')).toContainText('Before');

    // An edit autosaves while the session works, so the stash's base version
    // comes from a save response, not from the page load.
    await page.locator('.react-flow__node[data-id="n1"]').dblclick();
    const tray = page.getByTestId('node-config');
    await tray.getByPlaceholder('Optional display name').fill('Saved');
    await tray.getByRole('button', { name: 'Done' }).click();
    await expect(page.getByTestId('save-state')).toHaveAttribute('data-state', 'idle');
    await expect
      .poll(async () => (await (await page.request.get(`${OIDC_SERVER_URL}/api/flows/${flowId}`)).json()).nodes.n1.name)
      .toBe('Saved');

    // The session ends (the browser drops the cookie at Max-Age; the server
    // answers an expired one the same way), then the user keeps editing.
    await context.clearCookies();
    await page.locator('.react-flow__node[data-id="n1"]').dblclick();
    await tray.getByPlaceholder('Optional display name').fill('After expiry');
    await tray.getByRole('button', { name: 'Done' }).click();
    await expect(page.getByTestId('sso-button')).toBeVisible();
    expect(page.url()).toBe(`${OIDC_SERVER_URL}/flow/${flowId}`);

    // Signing in again keeps the edit: it reaches the server and the canvas,
    // and leaving for the provider asks no "leave site?" question.
    const dialogs: string[] = [];
    page.on('dialog', (dialog) => {
      dialogs.push(dialog.type());
      void dialog.accept();
    });
    await page.getByTestId('sso-button').click();
    await signInAtProvider(page, 'ada');
    await page.waitForURL(`${OIDC_SERVER_URL}/flow/${flowId}`);
    await expect(page.getByText('Your unsaved changes from before signing in were restored')).toBeVisible();
    await expect(page.getByTestId('stale-draft')).toHaveCount(0);
    await expect(page.locator('.react-flow__node[data-id="n1"]')).toContainText('After expiry');
    expect(dialogs).toEqual([]);
    await expect
      .poll(async () => (await (await page.request.get(`${OIDC_SERVER_URL}/api/flows/${flowId}`)).json()).nodes.n1.name, { timeout: 10_000 })
      .toBe('After expiry');
  });

  test('edits made on a version someone has saved over since are not applied without asking', async ({ page, context, browser }) => {
    await page.goto(`${OIDC_SERVER_URL}/`);
    await signInAtProvider(page, 'ada');
    await page.waitForURL(`${OIDC_SERVER_URL}/`);
    const flowId = `stale-${Date.now()}`;
    const write = { headers: { Origin: OIDC_SERVER_URL } };
    const node = (name: string) => ({ n1: { id: 'n1', type: 'debug', name, position: { x: 120, y: 120 }, config: {}, disabled: false } });
    expect((await page.request.post(`${OIDC_SERVER_URL}/api/flows`, { ...write, data: { id: flowId, name: 'Stale' } })).status()).toBe(201);
    expect((await page.request.put(`${OIDC_SERVER_URL}/api/flows/${flowId}`, { ...write, data: { nodes: node('Before'), connections: [] } })).ok()).toBeTruthy();
    await page.goto(`${OIDC_SERVER_URL}/flow/${flowId}`);
    const n1 = page.locator('.react-flow__node[data-id="n1"]');
    await expect(n1).toContainText('Before');

    // The session ends, the user edits, and leaves for the sign-in...
    await context.clearCookies();
    await n1.dblclick();
    const tray = page.getByTestId('node-config');
    await tray.getByPlaceholder('Optional display name').fill('Mine');
    await tray.getByRole('button', { name: 'Done' }).click();
    await page.getByTestId('sso-button').click();
    await page.waitForURL(`${OIDC_ISSUER}/authorize?**`);

    // ...while someone else saves the flow.
    const other = await browser.newContext();
    const otherPage = await other.newPage();
    await otherPage.goto(`${OIDC_SERVER_URL}/`);
    await signInAtProvider(otherPage, 'ada');
    await otherPage.waitForURL(`${OIDC_SERVER_URL}/`);
    expect((await otherPage.request.put(`${OIDC_SERVER_URL}/api/flows/${flowId}`, { ...write, data: { nodes: node('Theirs'), connections: [] } })).ok()).toBeTruthy();
    await other.close();

    // Back: the server's version is shown and the user is asked.
    await page.getByTestId('idp-user-ada').click();
    await page.waitForURL(`${OIDC_SERVER_URL}/flow/${flowId}`);
    const dialog = page.getByTestId('stale-draft');
    await expect(dialog).toBeVisible();
    await expect(n1).toContainText('Theirs');
    const serverName = async () => (await (await page.request.get(`${OIDC_SERVER_URL}/api/flows/${flowId}`)).json()).nodes.n1.name;
    expect(await serverName()).toBe('Theirs');

    const download = page.waitForEvent('download');
    await dialog.getByRole('button', { name: 'Download my changes' }).click();
    const file = await download;
    expect(file.suggestedFilename()).toBe(`flow-${flowId}.unsaved.json`);
    expect(JSON.parse(await readFile((await file.path())!, 'utf8')).nodes.n1.name).toBe('Mine');

    // Keeping them is one undoable step over the other version.
    await dialog.getByRole('button', { name: 'Keep my changes' }).click();
    await expect(dialog).toBeHidden();
    await expect(n1).toContainText('Mine');
    await expect.poll(serverName, { timeout: 10_000 }).toBe('Mine');
    await page.getByRole('button', { name: 'Undo' }).click();
    await expect(n1).toContainText('Theirs');
    await expect.poll(serverName, { timeout: 10_000 }).toBe('Theirs');
  });

  test('an account that is not on the allowlist is refused, without a redirect loop', async ({ page, context }) => {
    await page.goto(`${OIDC_SERVER_URL}/`);
    const callback = page.waitForResponse((response) => response.url().startsWith(`${OIDC_SERVER_URL}/auth/callback`));
    await signInAtProvider(page, 'eve');
    expect((await callback).status()).toBe(403);
    await expect(page.getByText('Your account is not allowed to use this Go-RED server.')).toBeVisible();
    expect((await context.cookies()).find((cookie) => cookie.name === 'gored_session')).toBeUndefined();

    // Back in the editor right after an automatic redirect, the dialog is
    // shown rather than sending the browser round again.
    const infoLoaded = page.waitForResponse((response) => response.url().endsWith('/api/auth/info'));
    await page.getByRole('link', { name: 'Back to the editor' }).click();
    await infoLoaded;
    await expect(page.getByTestId('sso-button')).toBeVisible();
    expect(page.url()).toBe(`${OIDC_SERVER_URL}/`);
  });

  test('the page after sign-in is always on this server', async ({ page }) => {
    await page.goto(`${OIDC_SERVER_URL}/auth/login?next=${encodeURIComponent('//evil.example/steal')}`);
    await signInAtProvider(page, 'ada');
    await page.waitForURL(`${OIDC_SERVER_URL}/`);
    await openMenu(page);
    await expect(page.getByTestId('signed-in-user')).toHaveText('Ada Lovelace');
  });
});
