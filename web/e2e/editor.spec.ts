import { test, expect, type APIRequestContext, type Page } from '@playwright/test';

/**
 * The editor's core loop against the real server: open a flow, edit it,
 * watch it autosave, deploy it, see live debug output, stop it, reload.
 */

const FLOW_ID = 'e2e-demo';

type SeedNode = { id: string; type: string; name?: string; position: { x: number; y: number }; config: Record<string, unknown>; disabled: boolean };
type SeedConnection = { id: string; sourceNode: string; sourcePort: string; targetNode: string; targetPort: string };

const demoNodes: Record<string, SeedNode> = {
  n1: { id: 'n1', type: 'inject', name: 'Every second', position: { x: 80, y: 120 }, config: { payload: { payload: 'tick' }, interval: 1000 }, disabled: false },
  n2: { id: 'n2', type: 'function', name: 'Transform', position: { x: 320, y: 120 }, config: { code: 'msg.count = (msg.count || 0) + 1; return msg;', useMsg: true }, disabled: false },
  n3: { id: 'n3', type: 'debug', name: 'Out', position: { x: 560, y: 120 }, config: {}, disabled: false },
};
const demoConnections: SeedConnection[] = [
  { id: 'c1', sourceNode: 'n1', sourcePort: 'output', targetNode: 'n2', targetPort: 'input' },
  { id: 'c2', sourceNode: 'n2', sourcePort: 'output', targetNode: 'n3', targetPort: 'input' },
];

async function seedFlow(request: APIRequestContext, nodes: Record<string, SeedNode>, connections: SeedConnection[]) {
  await request.delete(`/api/flows/${FLOW_ID}`).catch(() => undefined);
  const created = await request.post('/api/flows', { data: { id: FLOW_ID, name: 'E2E Demo', description: 'Inject → Function → Debug' } });
  expect(created.ok()).toBeTruthy();
  const updated = await request.put(`/api/flows/${FLOW_ID}`, { data: { nodes, connections } });
  expect(updated.ok()).toBeTruthy();
}

async function flowFromServer(request: APIRequestContext) {
  const response = await request.get(`/api/flows/${FLOW_ID}`);
  expect(response.ok()).toBeTruthy();
  return response.json();
}

async function waitForSaved(page: Page) {
  await expect(page.getByTestId('save-state')).toHaveAttribute('data-state', 'idle', { timeout: 10_000 });
}

/** Clicks an empty spot of the canvas so no node is selected. */
async function deselectAll(page: Page) {
  await page.locator('.react-flow__pane').click({ position: { x: 15, y: 15 } });
  await expect(page.getByTestId('flow-details')).toBeVisible();
}

async function dragNode(page: Page, nodeId: string, dx: number, dy: number) {
  const node = page.locator(`.react-flow__node[data-id="${nodeId}"]`);
  const box = await node.boundingBox();
  if (!box) throw new Error(`node ${nodeId} not visible`);
  const startX = box.x + box.width / 2;
  const startY = box.y + box.height / 2;
  await page.mouse.move(startX, startY);
  await page.mouse.down();
  await page.mouse.move(startX + dx / 2, startY + dy / 2, { steps: 6 });
  await page.mouse.move(startX + dx, startY + dy, { steps: 6 });
  await page.mouse.up();
}

function openDebugTab(page: Page) {
  return page.getByRole('button', { name: 'Debug', exact: true }).click();
}

test.describe('flow editor', () => {
  test.beforeEach(async ({ request }) => {
    await seedFlow(request, demoNodes, demoConnections);
  });

  test('a flow URL survives a reload and the SPA fallback serves the app', async ({ page, request }) => {
    const response = await request.get(`/flow/${FLOW_ID}`);
    expect(response.status()).toBe(200);
    expect(await response.text()).toContain('<div id="root">');

    await page.goto(`/flow/${FLOW_ID}`);
    await expect(page.getByRole('tab', { name: 'E2E Demo' })).toHaveAttribute('aria-selected', 'true');
    await expect(page.locator('.react-flow__node')).toHaveCount(3);
  });

  test('edits autosave, deploy runs the flow, stop and redeploy work, reload keeps everything', async ({ page, request }) => {
    const consoleNoise: string[] = [];
    page.on('console', (message) => {
      if (message.type() !== 'debug') consoleNoise.push(`${message.type()}: ${message.text()}`);
    });
    page.on('pageerror', (error) => consoleNoise.push(`pageerror: ${error.message}`));
    let sockets = 0;
    page.on('websocket', () => sockets++);

    await page.goto('/');
    await expect(page).toHaveURL(new RegExp(`/flow/`));
    await page.getByRole('tab', { name: 'E2E Demo' }).click();
    await expect(page).toHaveURL(new RegExp(`/flow/${FLOW_ID}$`));
    await expect(page.locator('.react-flow__node')).toHaveCount(3);

    // Move a node: the draft is autosaved without any explicit action.
    await dragNode(page, 'n2', 150, 80);
    await waitForSaved(page);
    const afterMove = await flowFromServer(request);
    expect(Math.round(afterMove.nodes.n2.position.x)).not.toBe(320);

    // Deploy: the flow runs, debug output streams into the sidebar by itself.
    await deselectAll(page);
    const deploy = page.getByTestId('deploy-button');
    await expect(deploy).toBeEnabled();
    await deploy.click();
    await expect(page.getByTestId('flow-status')).toHaveText('running');
    expect((await flowFromServer(request)).status).toBe('running');
    await expect(deploy).toBeDisabled();

    await openDebugTab(page);
    await expect(page.getByTestId('debug-live')).toHaveAttribute('data-live', 'true');
    const firstEntry = page.getByTestId('debug-message').first();
    await expect(firstEntry).toBeVisible({ timeout: 10_000 });
    await expect(firstEntry).toContainText('Out');
    await expect(firstEntry).toContainText('tick');

    // Editing a running flow lights the Deploy button up again.
    await page.getByRole('button', { name: 'Info' }).click();
    await dragNode(page, 'n3', -40, 120);
    await waitForSaved(page);
    await expect(deploy).toBeEnabled();
    await expect(deploy).toHaveText(/●/);
    const movedX = Math.round((await flowFromServer(request)).nodes.n3.position.x);

    // Undo the move with the keyboard, redo it again.
    await deselectAll(page);
    await page.keyboard.press('Control+z');
    await waitForSaved(page);
    expect(Math.round((await flowFromServer(request)).nodes.n3.position.x)).toBe(560);
    await page.keyboard.press('Control+y');
    await waitForSaved(page);
    expect(Math.round((await flowFromServer(request)).nodes.n3.position.x)).toBe(movedX);

    // Stop keeps the flow, redeploy runs it again.
    await page.getByRole('button', { name: 'Stop' }).click();
    await expect(page.getByTestId('flow-status')).toHaveText('draft');
    await deploy.click();
    await expect(page.getByTestId('flow-status')).toHaveText('running');

    // Reload: position, status, the debug feed resumes and one socket per page.
    await page.reload();
    await expect(page.locator('.react-flow__node')).toHaveCount(3);
    await deselectAll(page);
    await expect(page.getByTestId('flow-status')).toHaveText('running');
    expect(Math.round((await flowFromServer(request)).nodes.n2.position.x)).not.toBe(320);
    await openDebugTab(page);
    await expect(page.getByTestId('debug-live')).toHaveAttribute('data-live', 'true');
    await expect(page.getByTestId('debug-message').first()).toBeVisible({ timeout: 10_000 });

    expect(sockets).toBe(2);
    expect(consoleNoise).toEqual([]);
  });

  test('node errors and node status reach the editor live and survive a reload', async ({ page, request }) => {
    await seedFlow(
      request,
      {
        n1: demoNodes.n1,
        fn: { id: 'fn', type: 'function', name: 'Explodes', position: { x: 320, y: 120 }, config: { code: "throw new Error('kaboom');", useMsg: true }, disabled: false },
        h1: { id: 'h1', type: 'http in', name: 'Hook', position: { x: 80, y: 300 }, config: { method: 'POST', path: '/e2e-hook' }, disabled: false },
      },
      [{ id: 'c1', sourceNode: 'n1', sourcePort: 'output', targetNode: 'fn', targetPort: 'input' }]
    );

    await page.goto(`/flow/${FLOW_ID}`);
    await page.getByTestId('deploy-button').click();
    await expect(page.getByTestId('flow-status')).toHaveText('running');

    // The http in node reports that it is listening; the canvas shows it.
    const status = page.locator('.react-flow__node[data-id="h1"] [data-testid="node-status"]');
    await expect(status).toHaveAttribute('data-fill', 'green');
    await expect(status).toContainText('listening');

    // A throwing function becomes an error entry in the debug sidebar.
    await openDebugTab(page);
    const errorEntry = page.locator('[data-testid="debug-message"][data-level="error"]').first();
    await expect(errorEntry).toBeVisible({ timeout: 10_000 });
    await expect(errorEntry).toContainText('Explodes');
    await expect(errorEntry).toContainText('kaboom');

    // The Info tab shows the counters of the selected node.
    await page.getByRole('button', { name: 'Info' }).click();
    await page.locator('.react-flow__node[data-id="fn"]').click();
    await expect(page.getByTestId('node-messages')).toContainText(/[1-9]/);

    // Stopping the flow clears the node status.
    await page.getByRole('button', { name: 'Stop' }).click();
    await expect(status).toHaveCount(0);

    // The debug history is part of the snapshot a fresh page receives.
    await page.reload();
    await openDebugTab(page);
    await expect(page.getByTestId('debug-live')).toHaveAttribute('data-live', 'true');
    await expect(page.locator('[data-testid="debug-message"][data-level="error"]').first()).toBeVisible();
  });

  test('switch rules become output ports and wires to removed outputs are pruned', async ({ page, request }) => {
    await seedFlow(
      request,
      {
        n1: demoNodes.n1,
        sw: {
          id: 'sw',
          type: 'switch',
          name: 'Route',
          position: { x: 320, y: 120 },
          config: { property: { type: 'msg', path: 'payload' }, rules: [{ operator: 'eq', value: { type: 'str', value: 'tick' } }, { operator: 'else' }], checkAll: true },
          disabled: false,
        },
        d1: { id: 'd1', type: 'debug', name: 'Matches', position: { x: 600, y: 60 }, config: {}, disabled: false },
        d2: { id: 'd2', type: 'debug', name: 'Rest', position: { x: 600, y: 200 }, config: {}, disabled: false },
      },
      [
        { id: 'c1', sourceNode: 'n1', sourcePort: 'output', targetNode: 'sw', targetPort: 'input' },
        { id: 'c2', sourceNode: 'sw', sourcePort: '0', targetNode: 'd1', targetPort: 'input' },
        { id: 'c3', sourceNode: 'sw', sourcePort: '1', targetNode: 'd2', targetPort: 'input' },
      ]
    );
    await page.goto(`/flow/${FLOW_ID}`);
    const switchNode = page.locator('.react-flow__node[data-id="sw"]');
    await expect(switchNode.locator('.react-flow__handle.source')).toHaveCount(2);
    await expect(switchNode.getByTestId('port-label')).toHaveText(['== tick', 'otherwise']);
    await expect(page.locator('.react-flow__edge')).toHaveCount(3);

    // Removing the second rule removes its port and the wire on it.
    await switchNode.dblclick();
    const tray = page.getByTestId('node-config');
    await expect(tray.getByTestId('list-item-rules-1')).toBeVisible();
    await tray.getByTestId('list-item-rules-1').getByRole('button', { name: 'Remove' }).click();
    await tray.getByTestId('config-done').click();
    await expect(page.getByText('1 connection to a removed output was deleted')).toBeVisible();
    await expect(switchNode.locator('.react-flow__handle.source')).toHaveCount(1);
    await expect(page.locator('.react-flow__edge')).toHaveCount(2);
    await waitForSaved(page);
    const saved = await flowFromServer(request);
    expect(saved.connections.map((c: { id: string }) => c.id).sort()).toEqual(['c1', 'c2']);
    expect(saved.nodes.sw.config.rules).toHaveLength(1);

    // One undo step brings the rule and its wire back.
    await deselectAll(page);
    await page.keyboard.press('Control+z');
    await expect(switchNode.locator('.react-flow__handle.source')).toHaveCount(2);
    await expect(page.locator('.react-flow__edge')).toHaveCount(3);
  });

  test('required fields block Done and the function node is edited in a code editor', async ({ page, request }) => {
    await seedFlow(
      request,
      {
        h1: { id: 'h1', type: 'http in', name: 'Hook', position: { x: 80, y: 120 }, config: { method: 'POST', path: '' }, disabled: false },
        fn: { id: 'fn', type: 'function', name: 'Transform', position: { x: 320, y: 120 }, config: { code: 'return input;' }, disabled: false },
      },
      [{ id: 'c1', sourceNode: 'h1', sourcePort: 'output', targetNode: 'fn', targetPort: 'input' }]
    );
    await page.goto(`/flow/${FLOW_ID}`);

    await page.locator('.react-flow__node[data-id="h1"]').dblclick();
    const tray = page.getByTestId('node-config');
    await expect(tray.getByTestId('config-done')).toBeDisabled();
    await expect(tray.getByTestId('field-error')).toHaveText('Required');
    await tray.getByPlaceholder('/hook').fill('/e2e-hook');
    await expect(tray.getByTestId('config-done')).toBeEnabled();
    await tray.getByTestId('config-done').click();
    await waitForSaved(page);
    expect((await flowFromServer(request)).nodes.h1.config.path).toBe('/e2e-hook');

    await page.locator('.react-flow__node[data-id="fn"]').dblclick();
    const editor = page.getByTestId('node-config').locator('.cm-content');
    await expect(editor).toContainText('return input;');
    await editor.click();
    await page.keyboard.press('Control+a');
    await page.keyboard.type('msg.payload = 42; return msg;');
    await page.getByTestId('config-tab-appearance').click();
    await page.getByLabel('Enabled').uncheck();
    await page.getByTestId('config-done').click();
    await waitForSaved(page);
    const fn = (await flowFromServer(request)).nodes.fn;
    expect(fn.config.code).toBe('msg.payload = 42; return msg;');
    expect(fn.disabled).toBe(true);
    await page.locator('.react-flow__node[data-id="fn"]').click();
    await expect(page.getByTestId('node-disabled')).toBeVisible();
  });

  test('node configuration is edited in the tray and persisted', async ({ page, request }) => {
    await page.goto(`/flow/${FLOW_ID}`);
    await page.locator('.react-flow__node[data-id="n2"]').dblclick();
    const tray = page.getByTestId('node-config');
    await expect(tray).toBeVisible();
    await tray.getByPlaceholder('Optional display name').fill('Renamed');
    await tray.getByRole('button', { name: 'Done' }).click();
    await waitForSaved(page);
    expect((await flowFromServer(request)).nodes.n2.name).toBe('Renamed');
    await expect(page.locator('.react-flow__node[data-id="n2"]')).toContainText('Renamed');
  });

  test('a deploy error is shown, not swallowed', async ({ page, request }) => {
    await request.put(`/api/flows/${FLOW_ID}`, {
      data: { nodes: { bad: { id: 'bad', type: 'mqtt in', position: { x: 0, y: 0 }, config: {}, disabled: false } }, connections: [] },
    });
    await page.goto(`/flow/${FLOW_ID}`);
    await page.getByTestId('deploy-button').click();
    await expect(page.getByRole('alert')).toContainText('broker is required');
    await expect(page.getByTestId('flow-status')).toHaveText('error');
  });

  test('the language can be switched and is remembered', async ({ page }) => {
    await page.goto(`/flow/${FLOW_ID}`);
    await page.getByRole('button', { name: 'Main menu' }).click();
    await page.getByRole('menuitem', { name: 'Deutsch' }).click();
    await expect(page.getByRole('button', { name: 'Hauptmenü' })).toBeVisible();
    await page.reload();
    await expect(page.getByRole('button', { name: 'Hauptmenü' })).toBeVisible();
    await page.getByRole('button', { name: 'Hauptmenü' }).click();
    await page.getByRole('menuitem', { name: 'English' }).click();
    await expect(page.getByRole('button', { name: 'Main menu' })).toBeVisible();
  });
});
