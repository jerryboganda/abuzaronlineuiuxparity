/**
 * Calendar dates in the legacy application are local business dates, not UTC
 * instants.  Keep the date displayed in the browser, report filters, and
 * persisted transaction timestamps on the same calendar day in every
 * timezone.
 */
export function localDateString(date = new Date()): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

/**
 * Encode a local business date as a stable UTC timestamp. Noon UTC avoids a
 * date rollover when the API normalizes the value or the client is running in
 * a timezone west/east of the server.
 */
export function localDateAtNoonUtc(dateString: string): string {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(dateString);
  if (!match) return new Date().toISOString();
  return new Date(`${match[1]}-${match[2]}-${match[3]}T12:00:00Z`).toISOString();
}
