import { chromium, expect } from '@playwright/test';
import assert from 'node:assert/strict';
const browser = await chromium.launch({ headless: true }),
  page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
const errors = [];
page.on('pageerror', (e) => errors.push(e.message));
const base = process.env.APP_URL || 'http://localhost:3000';
const rack = (id, name, x) => ({
  id,
  name,
  short: name,
  wall: 'North',
  rows: 2,
  columns: 3,
  capacity: 6,
  temp: 12,
  x,
  y: 0.1,
  width: 1,
  depth: 0.5,
  rotation: 0,
  color: 'red',
});
const cellar = {
  id: 'test',
  name: 'Cellar',
  owner: 'Owner',
  room: 'Room',
  revision: 1,
  width: 5,
  depth: 4,
  racks: [rack('A', 'Reds', 0.1), rack('B', 'Whites', 1.5), rack('C', 'Champagne', 3)],
  bottles: [
    {
      id: '1',
      name: 'Occupied white',
      vintage: 2020,
      region: 'France',
      type: 'White',
      rack: 'B',
      slot: 0,
      revision: 1,
    },
  ],
  preferences: { viewMode: 'floor-plan', typeRacks: { White: 'B', Champagne: 'C' } },
  layout: {
    shape: 'rectangle',
    floor: 'stone',
    doorWall: 'south',
    doorOffset: 0,
    doorWidth: 0.8,
    tableEnabled: false,
  },
};
let additions = 0;
await page.route('**/api/**', async (route) => {
  const req = route.request(),
    path = new URL(req.url()).pathname;
  if (path === '/api/auth/status')
    return route.fulfill({
      json: { authenticated: true, setupRequired: false, username: 'Owner' },
    });
  if (path === '/api/cellar') return route.fulfill({ json: cellar });
  if (path === '/api/cellar/preferences') {
    const body = req.postDataJSON();
    assert.equal(body.revision, cellar.revision);
    cellar.preferences = { viewMode: body.viewMode, typeRacks: body.typeRacks };
    cellar.revision++;
    return route.fulfill({ json: { preferences: cellar.preferences, revision: cellar.revision } });
  }
  if (path === '/api/wine-scan/status') return route.fulfill({ json: { available: false } });
  if (path.startsWith('/api/barcodes/'))
    return route.fulfill({
      json: {
        status: 'recognized',
        name: 'Scanned champagne',
        region: 'Champagne, France',
        type: 'Champagne',
        barcode: '5901234123457',
        confidence: 'medium',
        source: 'Your cellar',
      },
    });
  if (path === '/api/bottles/batch') {
    const body = req.postDataJSON();
    assert.equal(body.bottle.type, 'White');
    assert.equal(body.bottle.rack, 'B');
    assert.deepEqual(body.slots, [1]);
    additions++;
    cellar.bottles.push({ ...body.bottle, id: 'added', slot: 1, revision: 1 });
    return route.fulfill({ json: [] });
  }
  throw Error(`Unexpected API request ${path}`);
});
try {
  await page.goto(base);
  await page.getByRole('button', { name: 'Preferences', exact: true }).click();
  let settings = page.getByRole('dialog', { name: 'Cellar settings' });
  await settings.getByLabel('Cellar view', { exact: true }).selectOption('racks-only');
  await expect(
    settings.getByRole('heading', { name: 'Preferred wine types & sections' }),
  ).toHaveCount(0);
  await settings.getByRole('button', { name: 'Save preferences', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Save preferences', exact: true })).toBeEnabled();
  await settings.getByRole('button', { name: 'Close dialog', exact: true }).click();
  await expect(page.locator('.racks-view')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Floor plan', exact: true })).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Edit room', exact: true })).toHaveCount(0);
  await page.reload();
  await expect(page.locator('.racks-view')).toBeVisible();
  await page.getByRole('button', { name: 'Add wine', exact: true }).click();
  let dialog = page.getByRole('dialog', { name: 'Add wine' });
  await expect(dialog.locator('select').first()).toHaveValue('White');
  await expect(dialog.locator('select').nth(1)).toHaveValue('B');
  await expect(dialog.getByRole('button', { name: 'Slot B1', exact: true })).toHaveAttribute(
    'aria-pressed',
    'true',
  );
  await dialog.locator('select').first().selectOption('Champagne');
  await expect(dialog.locator('select').nth(1)).toHaveValue('C');
  await expect(dialog.getByRole('button', { name: 'Slot A1', exact: true })).toHaveAttribute(
    'aria-pressed',
    'true',
  );
  // A manual shelf change overrides the suggestion.
  await dialog.locator('select').nth(1).selectOption('A');
  await expect(dialog.getByRole('button', { name: 'Slot A1', exact: true })).toHaveAttribute(
    'aria-pressed',
    'true',
  );
  await dialog.locator('select').first().selectOption('White');
  await dialog.getByLabel('Wine name', { exact: true }).fill('Test white');
  await dialog.getByLabel('Region', { exact: true }).fill('France');
  await dialog.getByRole('button', { name: 'Add to cellar', exact: true }).click();
  await expect(dialog).toHaveCount(0);
  assert.equal(additions, 1);
  // Clicking an empty slot preserves that exact location even when the default type prefers another shelf.
  await page
    .locator('.rack-card')
    .first()
    .getByRole('button', { name: 'Rack Reds slot C2 empty, add bottle' })
    .click();
  dialog = page.getByRole('dialog', { name: 'Add wine' });
  await expect(dialog.locator('select').nth(1)).toHaveValue('A');
  await expect(dialog.getByRole('button', { name: 'Slot C2', exact: true })).toHaveAttribute(
    'aria-pressed',
    'true',
  );
  await dialog.getByRole('button', { name: 'Close dialog', exact: true }).click();
  // Recognition uses the same placement rules.
  await page.getByRole('button', { name: 'Scan label', exact: true }).click();
  const scanner = page.getByRole('dialog', { name: 'Wine label scanner' });
  await scanner.getByRole('button', { name: 'Barcode', exact: true }).click();
  await scanner.getByLabel('Barcode number', { exact: true }).fill('5901234123457');
  await scanner.getByRole('button', { name: 'Look up barcode', exact: true }).click();
  await expect(scanner.locator('select').last()).toHaveValue('C');
  await expect(scanner.getByRole('button', { name: 'Slot A1', exact: true })).toHaveAttribute(
    'aria-pressed',
    'true',
  );
  await scanner.getByRole('button', { name: 'Close wine scanner', exact: true }).click();
  // A full preferred section is explained rather than selecting occupied slots.
  cellar.bottles.push(
    ...Array.from({ length: 6 }, (_, slot) => ({
      id: 'full' + slot,
      rack: 'C',
      slot,
      name: 'Stored champagne',
      region: 'France',
      type: 'Champagne',
      vintage: 2020,
    })),
  );
  await page.reload();
  await page.getByRole('button', { name: 'Add wine', exact: true }).click();
  dialog = page.getByRole('dialog', { name: 'Add wine' });
  await dialog.locator('select').first().selectOption('Champagne');
  await expect(dialog.getByText(/does not have enough free slots/)).toBeVisible();
  await expect(dialog.locator('select').nth(1)).not.toHaveValue('C');
  await dialog.getByRole('button', { name: 'Close dialog', exact: true }).click();
  await page.setViewportSize({ width: 390, height: 844 });
  await page.getByRole('button', { name: 'Cellar settings', exact: true }).click();
  settings = page.getByRole('dialog', { name: 'Cellar settings' });
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false);
  await settings.getByLabel('Cellar view', { exact: true }).selectOption('floor-plan');
  await settings.getByRole('button', { name: 'Save preferences', exact: true }).click();
  await expect(
    settings.getByRole('button', { name: 'Save preferences', exact: true }),
  ).toBeEnabled();
  await settings.getByRole('button', { name: 'Close dialog', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Floor plan', exact: true })).toBeVisible();
  assert.deepEqual(errors, []);
  console.log(
    'PASS persisted view preference, preserved section assignments, manual and scanned placement, explicit slot override, full-section fallback, mobile settings, floor-plan re-enable. API mocked.',
  );
} finally {
  await browser.close();
}
