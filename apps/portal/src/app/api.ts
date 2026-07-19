export type Principal = {
  id: string;
  username?: string;
  display_name: string;
  kind: string;
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

export type SessionInfo = {
  id: string;
  created_at: string;
  last_seen_at: string;
  remote_ip: string;
  auth_methods: string[];
  revoked_at?: string;
};

export type APIKeyInfo = {
  id: string;
  name: string;
  kind: string;
  scopes: string[];
  created_at: string;
  last_used_at?: string;
  last_used_ip?: string;
  expires_at?: string;
  revoked_at?: string;
};

export type ShareInfo = {
  id: string;
  share_type: string;
  service_id: string;
  resource_type?: string;
  resource_id?: string;
  actions: string[];
  expires_at: string;
  max_uses?: number;
  use_count: number;
  revoked_at?: string;
  created_at: string;
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
  finishPasskeyLogin: (credential: object, ceremonyToken: string) => request<SessionState>('/api/iam/v1/passkeys/login/finish', { method: 'POST', body: JSON.stringify(credential), headers: { 'X-HomeHub-Ceremony-Token': ceremonyToken } }),
  beginPasskeyRegistration: () => request<any>('/api/iam/v1/passkeys/registration/begin', { method: 'POST' }),
  finishPasskeyRegistration: (credential: object, name: string, ceremonyToken: string) => request<{ registered: boolean }>('/api/iam/v1/passkeys/registration/finish', {
    method: 'POST', body: JSON.stringify(credential), headers: { 'X-HomeHub-Passkey-Name': name, 'X-HomeHub-Ceremony-Token': ceremonyToken },
  }),
  passkeys: () => request<{ passkeys: PasskeyCredential[] }>('/api/iam/v1/passkeys'),
  deletePasskey: (id: string) => request<void>(`/api/iam/v1/passkeys/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  logout: () => request<void>('/api/iam/v1/logout', { method: 'POST' }),

  // Sessions
  sessions: () => request<{ sessions: SessionInfo[]; current_session_id: string }>('/api/iam/v1/sessions'),
  revokeSession: (id: string) => request<void>(`/api/iam/v1/sessions/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  revokeOtherSessions: () => request<{ revoked: number }>('/api/iam/v1/sessions', { method: 'DELETE' }),

  // API Keys
  apiKeys: () => request<{ api_keys: APIKeyInfo[] }>('/api/iam/v1/api-keys'),
  createAPIKey: (body: { name: string; kind: string; scopes: string[]; expires_in_days?: number }) => request<{ id: string; token: string; name: string; kind: string }>('/api/iam/v1/api-keys', { method: 'POST', body: JSON.stringify(body) }),
  revokeAPIKey: (id: string) => request<void>(`/api/iam/v1/api-keys/${encodeURIComponent(id)}`, { method: 'DELETE' }),

  // Shares
  shares: () => request<{ shares: ShareInfo[] }>('/api/iam/v1/shares'),
  createShare: (body: object) => request<{ id: string; token: string; share_type: string }>('/api/iam/v1/shares', { method: 'POST', body: JSON.stringify(body) }),
  revokeShare: (id: string) => request<void>(`/api/iam/v1/shares/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  redeem: (token: string) => request<SessionState>('/api/iam/v1/shares/redeem', { method: 'POST', body: JSON.stringify({ token }) }),
};

export const drop = {
  list: () => request<{ items: DropItem[] }>('/drop/v1/items'),
  create: (data: FormData) => request<DropItem>('/drop/v1/items', { method: 'POST', body: data }),
  remove: (id: string) => request<void>(`/drop/v1/items/${encodeURIComponent(id)}`, { method: 'DELETE' }),
};
