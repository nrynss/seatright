import { api } from './client';

/** One call through the Keel adapter. Screens never reach the network any other way. */
export interface TransportRequest {
  method?: string;
  token?: string | null;
  body?: unknown;
  signal?: AbortSignal;
}

export interface Transport {
  (path: string, init?: TransportRequest): Promise<unknown>;
}

/** Same-origin JSON. A bearer header is added only when the caller passes a token. */
export const liveTransport: Transport = async (path, init = {}) => {
  const headers: Record<string, string> = { Accept: 'application/json' };
  if (init.body !== undefined) {
    headers['Content-Type'] = 'application/json; charset=utf-8';
  }
  if (init.token) {
    headers.Authorization = `Bearer ${init.token}`;
  }
  return api(path, {
    method: init.method ?? (init.body !== undefined ? 'POST' : 'GET'),
    headers,
    body: init.body !== undefined ? JSON.stringify(init.body) : undefined,
    signal: init.signal,
  });
};
