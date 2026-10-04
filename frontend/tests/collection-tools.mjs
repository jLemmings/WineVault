import { chromium, expect } from '@playwright/test';
import assert from 'node:assert/strict';
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage();
const errors = [];
page.on('pageerror', (e) => errors.push(e.message));
const year = new Date().getFullYear();
const rack = {
  id: 'A',
  name: 'Shelf',
  short: 'Shelf',
  wall: 'North',
  rows: 2,
  columns: 3,
  capacity: 6,
  temp: 12,
  x: 0,
  y: 0,
  width: 1,
  depth: 1,
  rotation: 0,
  color: 'red',
};
let bottles = [
  {
    id: '1',
    name: 'Estate Reserve',
    vintage: 2020,
    region: 'France',
    type: 'Red',
    rack: 'A',
    slot: 0,
    revision: 1,
    drinkStart: year - 1,
    drinkEnd: year + 1,
  },
  {
    id: '2',
    name: 'Estate Reserve',
    vintage: 2020,
    region: 'France',
    type: 'Red',
    rack: 'A',
    slot: 1,
    revision: 1,
    drinkStart: year - 1,
    drinkEnd: year + 1,
  },
  {
    id: '3',
    name: 'Future White',
    vintage: 2023,
    region: 'Italy',
    type: 'White',
    rack: 'A',
    slot: 2,
    revision: 1,
    drinkStart: year + 2,
    drinkEnd: year + 5,
  },
];
let enjoyed = [],
  history = [];
await page.route('**/api/**', async (route) => {
  const request = route.request(),
    path = new URL(request.url()).pathname;
  if (path === '/api/auth/status')
    return route.fulfill({
      json: { authenticated: true, setupRequired: false, username: 'Owner' },
    });
  if (path === '/api/cellar')
    return route.fulfill({
      json: {
        id: 'test',
        name: 'Cellar',
        owner: 'Owner',
        room: 'Room',
        width: 5,
        depth: 4,
        racks: [rack],
        bottles,
        layout: {
          shape: 'rectangle',
          floor: 'stone',
          doorWall: 'south',
          doorOffset: 0,
          doorWidth: 0.8,
          tableEnabled: false,
        },
      },
    });
  if (path.endsWith('/information'))
    return route.fulfill({
      json: { status: 'disabled', data: null, message: 'Provider unavailable' },
    });
  if (path === '/api/history') return route.fulfill({ json: { entries: history, nextCursor: '' } });
  if (path === '/api/enjoyed') return route.fulfill({ json: enjoyed });
  const match = path.match(/^\/api\/bottles\/(\d+)(?:\/(window|restore))?$/);
  if (match) {
    const id = match[1],
      body = request.postDataJSON(),
      b = bottles.find((b) => b.id === id);
    if (match[2] === 'window') {
      assert.equal(body.revision, b.revision);
      for (const other of bottles.filter(
        (other) => other.name === b.name && other.vintage === b.vintage,
      )) {
        other.drinkStart = body.start;
        other.drinkEnd = body.end;
        other.revision++;
      }
    } else if (match[2] === 'restore') {
      const old = enjoyed.find((b) => b.id === id);
      bottles.push({ ...old, rack: body.rack, slot: body.slot });
      enjoyed = enjoyed.filter((b) => b.id !== id);
    } else {
      assert.equal(body.revision, b.revision);
      Object.assign(b, body);
      b.revision++;
    }
    return route.fulfill({ json: { saved: true } });
  }
  if (path === '/api/bottles' && request.method() === 'DELETE') {
    const id = new URL(request.url()).searchParams.get('id');
    enjoyed.push({ ...bottles.find((b) => b.id === id) });
    bottles = bottles.filter((b) => b.id !== id);
    return route.fulfill({ status: 204 });
  }
  throw Error(`Unexpected API request: ${request.method()} ${path}`);
});
try {
  await page.goto(process.env.APP_URL || 'http://localhost:3000');
  await page.getByRole('button', { name: 'Wine collection', exact: false }).first().click();
  await page.locator('.wine-table>button').first().waitFor();
  await expect(page.locator('.wine-table>button')).toHaveCount(3);
  await page.getByLabel('Group matching wines').check();
  await expect(page.locator('.wine-table>button')).toHaveCount(2);
  await page.getByLabel('Region', { exact: true }).selectOption('Italy');
  await expect(page.locator('.wine-table>button')).toHaveCount(1);
  await page.getByRole('button', { name: /Drink next ·/ }).click();
  await expect(page.locator('.wine-table>button')).toHaveCount(1);
  await page.locator('.wine-table>button').first().click();
  await page.getByRole('button', { name: 'Edit bottle details', exact: true }).click();
  await page.getByLabel('Wine name', { exact: true }).fill('Corrected Estate');
  await page.getByRole('button', { name: 'Save corrections', exact: true }).click();
  await page.getByRole('heading', { name: 'Corrected Estate', exact: true }).waitFor();
  await page.getByLabel('Start year', { exact: true }).fill(String(year));
  await page.getByLabel('End year', { exact: true }).fill(String(year + 3));
  await page.getByRole('button', { name: 'Save window', exact: true }).click();
  await page.waitForFunction(
    () => document.querySelector('input[type=number][max="9999"]')?.value !== undefined,
  );
  await page.getByRole('button', { name: 'Mark as enjoyed', exact: true }).click();
  await page.getByRole('button', { name: 'Undo', exact: true }).click();
  await page.getByRole('heading', { name: 'Undo an enjoyed bottle' }).waitFor();
  await page.getByLabel('Empty slot', { exact: true }).selectOption('0');
  await page.getByRole('button', { name: 'Restore bottle', exact: true }).click();
  await page.getByText('No bottles available to restore.').waitFor();
  assert.equal(bottles.length, 3);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.getByRole('button', { name: 'Wine collection', exact: false }).first().click();
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false);
  await page.locator('.wine-table>button').first().click();
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false);
  const dialog = page.getByRole('dialog');
  const first = dialog.getByRole('button', { name: 'Close dialog', exact: true }),
    last = dialog.getByRole('button', { name: 'Mark as enjoyed', exact: true });
  await first.focus();
  await page.keyboard.press('Shift+Tab');
  await expect(last).toBeFocused();
  await page.keyboard.press('Tab');
  await expect(first).toBeFocused();
  await page.keyboard.press('Escape');
  await page.getByRole('dialog').waitFor({ state: 'hidden' });
  assert.equal(
    await page.evaluate(() => document.activeElement?.closest('.wine-table') !== null),
    true,
  );
  assert.deepEqual(errors, []);
  console.log(
    'PASS grouping, filters, drink next, editing, windows, enjoyment undo, mobile, Escape and focus restoration. API mocked.',
  );
} finally {
  await browser.close();
}
