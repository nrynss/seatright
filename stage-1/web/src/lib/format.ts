const WEEKDAYS = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'] as const;
const MONTHS = [
  'January',
  'February',
  'March',
  'April',
  'May',
  'June',
  'July',
  'August',
  'September',
  'October',
  'November',
  'December',
] as const;

const DATE = /^(\d{4})-(\d{2})-(\d{2})$/;
const TIME = /^([01]\d|2[0-3]):([0-5]\d)$/;

function civilDate(isoDate: string): Date {
  const match = DATE.exec(isoDate);
  if (!match) {
    throw new Error(`Invalid calendar date: ${isoDate}`);
  }
  const year = Number(match[1]);
  const month = Number(match[2]);
  const day = Number(match[3]);
  const date = new Date(Date.UTC(year, month - 1, day));
  if (
    date.getUTCFullYear() !== year ||
    date.getUTCMonth() !== month - 1 ||
    date.getUTCDate() !== day
  ) {
    throw new Error(`Invalid calendar date: ${isoDate}`);
  }
  return date;
}

/** Thursday 24 September 2026. The calendar date is read as a civil date, not as UTC midnight shifted into a local zone. */
export function formatLongDate(isoDate: string): string {
  const date = civilDate(isoDate);
  const weekday = WEEKDAYS[date.getUTCDay()];
  const month = MONTHS[date.getUTCMonth()];
  return `${weekday} ${date.getUTCDate()} ${month} ${date.getUTCFullYear()}`;
}

/** 18:00 becomes 6:00 PM. This is the wall clock, independent of the host zone. */
export function formatClock(hhmm: string): string {
  const match = TIME.exec(hhmm);
  if (!match) {
    throw new Error(`Invalid local time: ${hhmm}`);
  }
  const hour24 = Number(match[1]);
  const minute = match[2];
  const period = hour24 >= 12 ? 'PM' : 'AM';
  const hour12 = hour24 % 12 || 12;
  return `${hour12}:${minute} ${period}`;
}

function partValue(parts: Intl.DateTimeFormatPart[], type: Intl.DateTimeFormatPartTypes): string {
  return parts.find((part) => part.type === type)?.value ?? '';
}

/** Minutes east of UTC for the named zone at this instant. */
function offsetMinutes(instant: Date, timeZone: string): number {
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone,
    hourCycle: 'h23',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  }).formatToParts(instant);
  const year = Number(partValue(parts, 'year'));
  const month = Number(partValue(parts, 'month'));
  const day = Number(partValue(parts, 'day'));
  let hour = Number(partValue(parts, 'hour'));
  if (hour === 24) hour = 0;
  const minute = Number(partValue(parts, 'minute'));
  const second = Number(partValue(parts, 'second'));
  const asUtc = Date.UTC(year, month - 1, day, hour, minute, second);
  return Math.round((asUtc - instant.getTime()) / 60000);
}

/** An instant whose wall clock in `timeZone` is the given civil date and time. */
export function zonedInstant(isoDate: string, hhmm: string, timeZone: string): Date {
  civilDate(isoDate);
  if (!TIME.test(hhmm)) {
    throw new Error(`Invalid local time: ${hhmm}`);
  }
  const [year, month, day] = isoDate.split('-').map(Number);
  const [hour, minute] = hhmm.split(':').map(Number);
  const utcGuess = Date.UTC(year, month - 1, day, hour, minute);
  const first = offsetMinutes(new Date(utcGuess), timeZone);
  const second = offsetMinutes(new Date(utcGuess - first * 60000), timeZone);
  return new Date(utcGuess - second * 60000);
}

/** Long zone name, for example Central European Summer Time. */
export function timezoneLabel(isoDate: string, hhmm: string, timeZone: string): string {
  const instant = zonedInstant(isoDate, hhmm, timeZone);
  const parts = new Intl.DateTimeFormat('en-GB', {
    timeZone,
    timeZoneName: 'long',
  }).formatToParts(instant);
  const name = partValue(parts, 'timeZoneName');
  if (!name) {
    throw new Error(`No timezone name for ${timeZone}`);
  }
  return name;
}

export function bookingSummary(restaurant: string, label: string, isoDate: string, hhmm: string): string {
  return `${restaurant} · Table ${label} · ${formatLongDate(isoDate)} at ${formatClock(hhmm)} (${hhmm})`;
}
