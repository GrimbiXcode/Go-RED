import { test, expect, type APIRequestContext, type Page } from '@playwright/test';

/**
 * Two editor tabs on one flow. Saves are ordered by the server's revision:
 * an idle tab follows the other's saves, and a tab whose save was overtaken
 * pulls the newer version and reapplies only its own changes. Nobody is
 * asked anything, and nothing either tab did is lost.
 */

const FLOW_ID = 'e2e-concurrent';

const node = (id: string, name: string, y: number) => ({ id, type: 'debug', name, position: { x: 120, y }, config: {}, disabled: false });

async function seed(request: APIRequestContext) {
  await request.delete(`/api/flows/${FLOW_ID}`).catch(() => undefined);
  expect((await request.post('/api/flows', { data: { id: FLOW_ID, name: 'Concurrent' } })).ok()).toBeTruthy();
  const nodes = { n1: node('n1', 'One', 100), n2: node('n2', 'Two', 260) };
  expect((await request.put(`/api/flows/${FLOW_ID}`, { data: { nodes, connections: [] } })).ok()).toBeTruthy();
}

async function serverNodes(request: APIRequestContext) {
  return (await (await request.get(`/api/flows/${FLOW_ID}`)).json()).nodes as Record<string, { name: string }>;
}

async function open(page: Page) {
  await page.goto(`/flow/${FLOW_ID}`);
  await expect(page.locator('.react-flow__node[data-id="n1"]')).toContainText('One');
  await expect(page.getByTestId('app-header').getByTestId('ws-status')).toHaveAttribute('data-state', 'connected');
}

async function rename(page: Page, id: string, name: string) {
  await page.locator(`.react-flow__node[data-id="${id}"]`).dblclick();
  const tray = page.getByTestId('node-config');
  await tray.getByPlaceholder('Optional display name').fill(name);
  await tray.getByRole('button', { name: 'Done' }).click();
}

async function waitForSaved(page: Page) {
  await expect(page.getByTestId('save-state')).toHaveAttribute('data-state', 'idle', { timeout: 10_000 });
}

/** Holds the page's next save of the flow until release() is called. */
async function holdNextSave(page: Page) {
  let release!: () => void;
  const released = new Promise<void>((resolve) => (release = resolve));
  let held: () => void;
  const reached = new Promise<void>((resolve) => (held = resolve));
  let holding = true;
  await page.route(`**/api/flows/${FLOW_ID}`, async (route) => {
    if (holding && route.request().method() === 'PUT') {
      holding = false;
      held();
      await released;
    }
    await route.continue();
  });
  return { reached, release: () => release() };
}

test.describe('two tabs on one flow', () => {
  test.beforeEach(async ({ request }) => {
    await seed(request);
  });

  test('an idle tab follows the other tab without saving anything itself', async ({ context }) => {
    const a = await context.newPage();
    const b = await context.newPage();
    await open(a);
    await open(b);
    const bSaves: string[] = [];
    b.on('request', (r) => {
      if (r.method() === 'PUT') bSaves.push(r.url());
    });

    await rename(a, 'n1', 'From A');
    await waitForSaved(a);
    await expect(b.locator('.react-flow__node[data-id="n1"]')).toContainText('From A');
    await waitForSaved(b);
    expect(bSaves).toEqual([]);
    await expect(b.getByRole('dialog')).toHaveCount(0);
  });

  test('edits to different nodes in both tabs both survive', async ({ context, request }) => {
    const a = await context.newPage();
    const b = await context.newPage();
    await open(a);
    await open(b);

    // B's save is overtaken by A's.
    const hold = await holdNextSave(b);
    await rename(b, 'n2', 'From B');
    await hold.reached;
    await rename(a, 'n1', 'From A');
    await waitForSaved(a);
    hold.release();
    await waitForSaved(b);

    await expect.poll(async () => Object.values(await serverNodes(request)).map((n) => n.name).sort(), { timeout: 10_000 }).toEqual(['From A', 'From B']);
    await expect(b.locator('.react-flow__node[data-id="n1"]')).toContainText('From A');
    await expect(a.locator('.react-flow__node[data-id="n2"]')).toContainText('From B');
    await expect(a.getByRole('dialog')).toHaveCount(0);
    await expect(b.getByRole('dialog')).toHaveCount(0);
  });

  test('the same field edited in both tabs takes the later save, everywhere', async ({ context, request }) => {
    const a = await context.newPage();
    const b = await context.newPage();
    await open(a);
    await open(b);

    const hold = await holdNextSave(b);
    await rename(b, 'n1', 'From B');
    await hold.reached;
    await rename(a, 'n1', 'From A');
    await waitForSaved(a);
    hold.release();
    await waitForSaved(b);

    await expect.poll(async () => (await serverNodes(request)).n1.name, { timeout: 10_000 }).toBe('From B');
    await expect(a.locator('.react-flow__node[data-id="n1"]')).toContainText('From B');
    expect((await serverNodes(request)).n2.name).toBe('Two');
  });
});
