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
