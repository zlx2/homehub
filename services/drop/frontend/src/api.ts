import type { DropItem, DropStatus } from './types';

const base = '/drop/v1';

function cookie(name: string) {
  return document.cookie.split('; ').find((part) => part.startsWith(`${name}=`))?.slice(name.length + 1) ?? '';
}

export function csrfToken() { return decodeURIComponent(cookie('hh_csrf')); }

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set('Accept', 'application/json');
  if (init.method && !['GET', 'HEAD'].includes(init.method.toUpperCase())) headers.set('X-CSRF-Token', csrfToken());
  const response = await fetch(`${base}${path}`, { ...init, headers, credentials: 'same-origin' });
  if (!response.ok) {
    const body = await response.json().catch(() => ({}));
    throw new Error(body.error || `请求失败 (${response.status})`);
  }
  if (response.status === 204) return undefined as T;
  return response.json() as Promise<T>;
}

export const dropAPI = {
  list: (signal?: AbortSignal) => request<{ items: DropItem[] }>('/items?limit=100', { signal }),
  remove: (id: string) => request<void>(`/items/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  updateExpiry: (id: string, ttlDays: number) => request<DropItem>(`/items/${encodeURIComponent(id)}/expiry`, { method: 'PATCH', body: JSON.stringify({ ttl_days: ttlDays }), headers: { 'Content-Type': 'application/json' } }),
  status: () => request<DropStatus>('/status'),
  eventsURL: `${base}/events`,
};

export function attachmentURL(attachment: { id: string }) { return `${base}/attachments/${encodeURIComponent(attachment.id)}`; }
