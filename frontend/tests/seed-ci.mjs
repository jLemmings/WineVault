// Only bootstrap a fresh, disposable CI instance. Never reset an existing owner.
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { request } from '@playwright/test';

assert.equal(process.env.WINEVAULT_CI_BOOTSTRAP, '1', 'CI bootstrap must be explicitly enabled');
const username = process.env.WINEVAULT_TEST_USERNAME;
const password = process.env.WINEVAULT_TEST_PASSWORD;
assert.ok(
  username && password && process.env.WINEVAULT_API_LOG,
  'CI credentials and API log are required',
);
const api = await request.newContext({
  baseURL: process.env.WINEVAULT_CI_API_URL || 'http://127.0.0.1:8080',
  extraHTTPHeaders: { 'X-WineVault-Request': '1' },
});
async function json(response) {
  assert.ok(
    response.ok(),
    `CI fixture request failed (${response.status()}): ${await response.text()}`,
  );
  return response.json();
}
try {
  const status = await json(await api.get('/api/auth/status'));
  assert.equal(
    status.setupRequired,
    true,
    'Refusing to bootstrap an already configured installation',
  );
  const log = await readFile(process.env.WINEVAULT_API_LOG, 'utf8');
  const setupCode = log.match(/one-time setup code: ([a-f0-9]{64})/)?.[1];
  assert.ok(setupCode, 'Fresh owner setup code was not found in the CI API log');
  await json(await api.post('/api/auth/setup', { data: { setupCode, username, password } }));
  const cellar = await json(await api.get('/api/cellar'));
  assert.equal(cellar.racks.length, 0, 'CI fixture expects an empty cellar');
  await json(
    await api.post('/api/racks', {
      data: {
        revision: cellar.revision,
        name: 'CI shelf',
        short: 'CI shelf',
        wall: 'North',
        grapes: '',
        temp: 13,
        color: 'red',
        rows: 5,
        columns: 6,
      },
    }),
  );
  const ready = await json(await api.get('/api/cellar'));
  // Slot B1 ensures shrinking to one slot is rejected by the editor suite.
  await json(
    await api.post('/api/bottles', {
      data: {
        name: 'CI fixture wine',
        vintage: 2020,
        region: 'France',
        type: 'Red',
        rack: ready.racks[0].id,
        slot: 1,
      },
    }),
  );
  console.log('Created disposable CI owner, shelf, and bottle fixture.');
} finally {
  await api.dispose();
}
