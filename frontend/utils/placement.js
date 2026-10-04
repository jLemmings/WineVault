export const wineTypes = ['Red', 'White', 'Rosé', 'Champagne', 'Sparkling', 'Dessert'];

export function preferredWineTypes(preferences) {
  const preferred = preferences?.typeRacks || {};
  return [
    ...wineTypes.filter((type) => Object.hasOwn(preferred, type)),
    ...wineTypes.filter((type) => !Object.hasOwn(preferred, type)),
  ];
}

// Suggest free slots without changing inventory. Explicit user selections remain authoritative.
export function suggestPlacement({
  racks,
  bottles,
  preferences,
  type,
  quantity = 1,
  fallbackRack = '',
}) {
  if (!Number.isInteger(quantity) || quantity < 1)
    return { rack: fallbackRack, slots: [], message: '' };
  const free = (rack) =>
    Array.from({ length: rack.capacity }, (_, slot) => slot).filter(
      (slot) => !bottles.some((b) => b.rack === rack.id && b.slot === slot),
    );
  const preferredID = preferences?.typeRacks?.[type];
  const preferred = racks.find((r) => r.id === preferredID);
  const fallback = racks.find((r) => r.id === fallbackRack);
  const candidates = [preferred, fallback, ...racks].filter(
    (r, i, all) => r && all.findIndex((other) => other?.id === r.id) === i,
  );
  const chosen =
    candidates.find((r) => free(r).length >= quantity) ||
    candidates.find((r) => free(r).length) ||
    fallback ||
    racks[0];
  if (!chosen) return { rack: '', slots: [], message: 'Add a shelf to give this wine a home.' };
  const slots = free(chosen).slice(0, quantity);
  const message = !slots.length
    ? 'No free slots are available. Add a shelf or free a slot.'
    : preferred && chosen.id !== preferred.id
      ? `Your preferred ${type.toLowerCase()} section does not have enough free slots. Suggested ${chosen.name} instead.`
      : preferredID && !preferred
        ? 'Your preferred section is unavailable. Suggested another shelf.'
        : preferred
          ? `Suggested a free slot in your ${type.toLowerCase()} section: ${chosen.name}.`
          : `Suggested a free slot in ${chosen.name}.`;
  return { rack: chosen.id, slots, message };
}
