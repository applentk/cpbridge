/**
 * Formats an ISO 8601 UTC timestamp, Date object, or timestamp number into a string
 * suitable for `<input type="datetime-local">` in the user's local timezone: "YYYY-MM-DDTHH:mm".
 */
export function toDateTimeLocalValue(date: Date | string | number | null | undefined): string {
  if (!date) return "";
  const d = typeof date === "object" && date instanceof Date ? date : new Date(date);
  if (isNaN(d.getTime())) return "";

  const pad = (n: number) => n.toString().padStart(2, "0");
  const year = d.getFullYear();
  const month = pad(d.getMonth() + 1);
  const day = pad(d.getDate());
  const hours = pad(d.getHours());
  const minutes = pad(d.getMinutes());

  return `${year}-${month}-${day}T${hours}:${minutes}`;
}

/**
 * Converts a `<input type="datetime-local">` value ("YYYY-MM-DDTHH:mm")
 * into an ISO 8601 UTC string ("YYYY-MM-DDTHH:mm:ss.sssZ").
 */
export function fromDateTimeLocalValue(value: string | null | undefined): string {
  if (!value) return "";
  const d = new Date(value);
  if (isNaN(d.getTime())) return "";
  return d.toISOString();
}
