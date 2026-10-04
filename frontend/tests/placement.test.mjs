import test from 'node:test';
import assert from 'node:assert/strict';
import { suggestPlacement, preferredWineTypes } from '../utils/placement.js';
const racks = [
  { id: 'A', name: 'Reds', capacity: 3 },
  { id: 'B', name: 'Whites', capacity: 4 },
];
const preferences = { typeRacks: { White: 'B', Red: 'A', Champagne: 'B' } };
test('suggest matching section and skip occupied slots for a batch', () => {
  const p = suggestPlacement({
    racks,
    bottles: [
      { rack: 'B', slot: 0 },
      { rack: 'B', slot: 2 },
    ],
    preferences,
    type: 'White',
    quantity: 2,
    fallbackRack: 'A',
  });
  assert.equal(p.rack, 'B');
  assert.deepEqual(p.slots, [1, 3]);
});
test('full preferred section falls back with an explanation', () => {
  const p = suggestPlacement({
    racks,
    bottles: Array.from({ length: 4 }, (_, slot) => ({ rack: 'B', slot })),
    preferences,
    type: 'Champagne',
    fallbackRack: 'A',
  });
  assert.equal(p.rack, 'A');
  assert.deepEqual(p.slots, [0]);
  assert.match(p.message, /does not have enough free slots/);
});
test('unassigned types, removed sections, and empty cellars stay usable', () => {
  assert.equal(
    suggestPlacement({ racks, bottles: [], preferences, type: 'Dessert', fallbackRack: 'B' }).rack,
    'B',
  );
  assert.equal(
    suggestPlacement({
      racks,
      bottles: [],
      preferences: { typeRacks: { White: 'deleted' } },
      type: 'White',
    }).rack,
    'A',
  );
  assert.deepEqual(
    suggestPlacement({ racks: [], bottles: [], preferences, type: 'Red' }).slots,
    [],
  );
});
test('preferred types are first while all supported types remain selectable', () => {
  const ordered = preferredWineTypes({ typeRacks: { White: 'B', Champagne: 'A' } });
  assert.deepEqual(ordered.slice(0, 2), ['White', 'Champagne']);
  assert.equal(new Set(ordered).size, 6);
});
