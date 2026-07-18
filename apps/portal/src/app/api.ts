export type Principal = {
  id: string;
  subject: string;
  kind: 'human' | 'guest';
  username?: string;
  display_name: string;
  realm: string;
};

export type SessionState = {
  authenticated: boolean;
  setup_required: boolean;
  principal?: Principal;
  administrator?: boolean;
};

export type Attachment = {
  id: string;
  original_name: string;
  media_type: string;
  size: number;
  created_at: string;
};

export type DropItem = {
  id: string;
  text?: string;
  creator_subject: string;
  created_at: string;
  expires_at: string;
  total_size: number;
  attachments: Attachment[];
};

export type PasskeyCredential = {
  id: string;
  name: string;
  created_at: string;
  last_used_at?: string;
};

function cookie(name: string) {
  return document.cookie.split('; ').find((part) => part.startsWith(`${name}=`))?.slice(name.length + 1) ?? '';
}

export async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  if (init.body && !(init.body instanceof FormData)) headers.set('Content-Type', 'application/json');
  if (init.method && !['GET', 'HEAD'].includes(init.method.toUpperCase())) headers.set('X-CSRF-Token', decodeURIComponent(cookie('hh_csrf')));
  const response = await fetch(path, { ...init, headers, credentials: 'same-origin' });
  if (!response.ok) {
    const body = await response.json().catch(() => ({}));
    throw new Error(body.error || `request_${response.status}`);
  }
  if (response.status === 204) return undefined as T;
  return response.json() as Promise<T>;
}

export const iam = {
  session: () => request<SessionState>('/api/iam/v1/session'),
  beginSetup: (body: object) => request<{ setup_id: string; manual_secret: string; provisioning_uri: string }>('/api/iam/v1/setup/begin', { method: 'POST', body: JSON.stringify(body) }),
  confirmSetup: (body: object) => request<SessionState>('/api/iam/v1/setup/confirm', { method: 'POST', body: JSON.stringify(body) }),
  login: (body: object) => request<SessionState>('/api/iam/v1/login', { method: 'POST', body: JSON.stringify(body) }),
  beginPasskeyLogin: () => request<any>('/api/iam/v1/passkeys/login/begin', { method: 'POST' }),
  finishPasskeyLogin: (credential: object) => request<SessionState>('/api/iam/v1/passkeys/login/finish', { method: 'POST', body: JSON.stringify(credential) }),
  beginPasskeyRegistration: () => request<any>('/api/iam/v1/passkeys/registration/begin', { method: 'POST' }),
  finishPasskeyRegistration: (credential: object, name: string) => request<{ registered: boolean }>('/api/iam/v1/passkeys/registration/finish', {
    method: 'POST', body: JSON.stringify(credential), headers: { 'X-HomeHub-Passkey-Name': name },
  }),
  passkeys: () => request<{ passkeys: PasskeyCredential[] }>('/api/iam/v1/passkeys'),
  deletePasskey: (id: string) => request<void>(`/api/iam/v1/passkeys/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  logout: () => request<void>('/api/iam/v1/logout', { method: 'POST' }),
  redeem: (token: string) => request<SessionState>('/api/iam/v1/shares/redeem', { method: 'POST', body: JSON.stringify({ token }) }),
  shares: () => request<{ shares: Array<{ id: string; grants: Array<{ service_id: string; relation: string }>; expires_at: string; revoked_at?: string }> }>('/api/iam/v1/shares'),
  createShare: (hours: number) => request<{ id: string; token: string; expires_at: string }>('/api/iam/v1/shares', {
    method: 'POST', body: JSON.stringify({ grants: [{ service_id: 'drop', relation: 'viewer' }], expires_at: new Date(Date.now() + hours * 3600_000).toISOString() }),
  }),
  revokeShare: (id: string) => request<void>(`/api/iam/v1/shares/${encodeURIComponent(id)}`, { method: 'DELETE' }),
};

export const drop = {
  list: () => request<{ items: DropItem[] }>('/drop/v1/items'),
  create: (data: FormData) => request<DropItem>('/drop/v1/items', { method: 'POST', body: data }),
  remove: (id: string) => request<void>(`/drop/v1/items/${encodeURIComponent(id)}`, { method: 'DELETE' }),
};
