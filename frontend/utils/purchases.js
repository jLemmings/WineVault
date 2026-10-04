export const currencies = ['CHF', 'EUR', 'USD', 'GBP', 'CAD', 'AUD'];
export function priceMinor(value) {
  if (value === '' || value == null) return null;
  const text = String(value).trim();
  if (!/^\d+(\.\d{1,2})?$/.test(text))
    throw new Error('Enter a non-negative price with at most two decimal places.');
  const [whole, fraction = ''] = text.split('.');
  const amount = Number(whole) * 100 + Number(fraction.padEnd(2, '0'));
  if (!Number.isSafeInteger(amount) || amount > 1000000000000)
    throw new Error('Price is too large.');
  return amount;
}
export const priceText = (amount) =>
  amount == null ? '' : `${Math.floor(amount / 100)}.${String(amount % 100).padStart(2, '0')}`;
export function purchasePayload(draft) {
  const amount = priceMinor(draft.price);
  return {
    priceMinor: amount,
    currency: amount == null ? '' : draft.currency,
    purchaseDate: draft.purchaseDate || '',
    seller: (draft.seller || '').trim(),
  };
}
export function money(amount, currency) {
  return new Intl.NumberFormat(undefined, { style: 'currency', currency }).format(amount / 100);
}
export function purchaseSummary(entries) {
  const totals = new Map(),
    months = new Map();
  for (const entry of entries) {
    if (entry.priceMinor == null) continue;
    const total = totals.get(entry.currency) || {
      currency: entry.currency,
      total: 0,
      cellar: 0,
      bottles: 0,
      undated: 0,
    };
    total.total += entry.priceMinor;
    total.bottles++;
    if (entry.inCellar) total.cellar += entry.priceMinor;
    if (!entry.purchaseDate) total.undated += entry.priceMinor;
    totals.set(entry.currency, total);
    if (entry.purchaseDate) {
      const month = entry.purchaseDate.slice(0, 7),
        key = `${month}:${entry.currency}`;
      const group = months.get(key) || { month, currency: entry.currency, amount: 0, bottles: 0 };
      group.amount += entry.priceMinor;
      group.bottles++;
      months.set(key, group);
    }
  }
  return {
    totals: [...totals.values()].sort((a, b) => a.currency.localeCompare(b.currency)),
    months: [...months.values()].sort(
      (a, b) => b.month.localeCompare(a.month) || a.currency.localeCompare(b.currency),
    ),
  };
}
