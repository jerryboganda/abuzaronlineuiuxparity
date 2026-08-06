function pad(value: number): string {
  return String(value).padStart(2, '0');
}

export function formatLegacySessionTimestamp(date: Date): string {
  return `${pad(date.getDate())}/${pad(date.getMonth() + 1)}/${String(date.getFullYear()).slice(-2)} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
}

export function formatLegacyTitle(username: string | null | undefined, date: Date): string {
  const operator = username?.trim().toUpperCase() || 'ADMIN';
  return `WASEELA   ABUZAR V3 01.01.2025 : ${operator} : ${formatLegacySessionTimestamp(date)}`;
}
