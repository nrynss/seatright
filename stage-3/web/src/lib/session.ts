/** A signed-in diner. The token is the one the service issued. Nothing here invents one. */
export interface Session {
  token: string;
  displayName: string;
  userId: string;
}

const KEY = 'tablekeeper-session';

function readable(value: unknown): Session | null {
  if (!value || typeof value !== 'object') return null;
  const record = value as Partial<Session>;
  if (typeof record.token !== 'string' || record.token === '') return null;
  if (typeof record.userId !== 'string' || record.userId === '') return null;
  if (typeof record.displayName !== 'string') return null;
  return { token: record.token, displayName: record.displayName, userId: record.userId };
}

export function loadSession(storage: Storage = localStorage): Session | null {
  try {
    const raw = storage.getItem(KEY);
    if (!raw) return null;
    return readable(JSON.parse(raw) as unknown);
  } catch {
    return null;
  }
}

export function saveSession(session: Session, storage: Storage = localStorage): void {
  storage.setItem(KEY, JSON.stringify(session));
}

export function clearSession(storage: Storage = localStorage): void {
  storage.removeItem(KEY);
}
