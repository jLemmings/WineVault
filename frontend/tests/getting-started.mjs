import { chromium, expect } from '@playwright/test';
import assert from 'node:assert/strict';
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage();
const errors = [];
page.on('pageerror', (e) => errors.push(e.message));
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
const bottles = [];
let preferences = { viewMode: 'floor-plan', typeRacks: {} };
const testRacks = [];
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
        racks: testRacks,
        revision: 1,
        preferences,
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
  if (path === '/api/cellar/preferences') {
    preferences = request.postDataJSON();
    return route.fulfill({ json: {} });
  }
  if (path === '/api/racks') {
    const body = request.postDataJSON();
    assert.equal(body.rows, 3);
    assert.equal(body.columns, 4);
    testRacks.push({ ...rack, ...body, capacity: 12 });
    return route.fulfill({ json: testRacks[0] });
  }
  if (path === '/api/bottles/batch') {
    const body = request.postDataJSON();
    bottles.push(
      ...body.slots.map((slot, i) => ({
        ...body.bottle,
        id: String(bottles.length + i + 1),
        slot,
        revision: 1,
      })),
    );
    return route.fulfill({ json: [] });
  }
  if (path.endsWith('/information'))
    return route.fulfill({
      json: { status: 'disabled', data: null, message: 'Provider unavailable' },
    });
  throw Error(`Unexpected API request: ${request.method()} ${path}`);
});
try {
  await page.goto(process.env.APP_URL || 'http://localhost:3000');
  const guide = page.getByRole('region', { name: 'Getting started' });
  await expect(guide).toBeVisible();
  await expect(guide.getByRole('button', { name: 'Add wine', exact: true })).toBeDisabled();
  await guide.getByRole('button', { name: 'Start with rack-only view' }).click();
  await expect(guide.getByRole('button', { name: 'Rack-only view selected' })).toBeDisabled();
  await guide.getByRole('button', { name: 'Add shelf', exact: true }).click();
  await page.getByRole('button', { name: 'Compact · 12 bottles' }).click();
  await page.getByRole('button', { name: /Save shelf|Create shelf/ }).click();
  await page.getByRole('dialog').waitFor({ state: 'hidden' });
  await guide.getByRole('button', { name: 'Add wine', exact: true }).click();
  await page.getByLabel('Wine name', { exact: true }).fill('First wine');
  await page.getByLabel('Region', { exact: true }).fill('France');
  await page.getByRole('button', { name: 'Add to cellar', exact: true }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await expect(guide).toBeVisible();
  await page.reload();
  await expect(guide).toBeVisible();
  await guide.getByRole('button', { name: 'Find my first bottle' }).click();
  await expect(page.locator('.bottle-slot.located')).toHaveCount(1);
  await page.locator('.bottle-slot.located').click();
  await page.getByRole('button', { name: 'Add more of this wine' }).click();
  await page.getByLabel('Number of bottles', { exact: true }).fill('2');
  await page.getByRole('button', { name: 'Add to cellar', exact: true }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  assert.equal(bottles.length, 3);
  assert.deepEqual(
    bottles.map((b) => b.slot),
    [0, 1, 2],
  );
  assert.ok(bottles.every((b) => b.name === 'First wine'));
  await guide.getByRole('button', { name: 'Finish setup' }).click();
  await expect(guide).toHaveCount(0);
  await page.reload();
  await expect(guide).toHaveCount(0);
  await page.getByRole('button', { name: 'Preferences', exact: true }).click();
  await page.getByRole('button', { name: 'Getting started guide' }).click();
  await expect(guide).toBeVisible();
  await page.setViewportSize({ width: 390, height: 844 });
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false);
  await guide.getByRole('button', { name: 'Skip for now' }).click();
  assert.deepEqual(errors, []);
  console.log(
    'PASS empty-cellar guide, rack-only preference, sample shelf, first wine, continued setup after reload, locator, and completion. API mocked.',
  );
} finally {
  await browser.close();
}
