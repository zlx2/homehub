import { serviceURL } from "@/paths";
import type { APIErrorBody, DropItem, StatusReport } from "@/types";

async function responseError(response: Response): Promise<Error> {
  try {
    const body = await response.json() as APIErrorBody;
    return new Error(body.error?.message || `请求失败 (${response.status})`);
  } catch {
    return new Error(`请求失败 (${response.status})`);
  }
}

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const response = await fetch(url, {
    ...init,
    headers: { Accept: "application/json", ...init?.headers },
  });
  if (response.status === 401) {
    location.reload();
    throw new Error("登录状态已失效");
  }
  if (!response.ok) throw await responseError(response);
  return response.json() as Promise<T>;
}

export async function listItems(signal?: AbortSignal): Promise<DropItem[]> {
  const result = await request<{ items: DropItem[] }>(serviceURL("/api/v1/items?limit=100"), { signal });
  return result.items || [];
}

export function readFullText(url: string): Promise<Response> {
  return fetch(url);
}

export async function updateExpiry(id: string, ttlDays: number): Promise<void> {
  await request(serviceURL(`/api/v1/items/${encodeURIComponent(id)}/expiry`), {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ ttl_days: ttlDays }),
  });
}

export async function deleteItem(id: string): Promise<void> {
  const response = await fetch(serviceURL(`/api/v1/items/${encodeURIComponent(id)}`), { method: "DELETE" });
  if (response.status === 401) {
    location.reload();
    throw new Error("登录状态已失效");
  }
  if (!response.ok) throw await responseError(response);
}

export function loadStatus(): Promise<StatusReport> {
  return request<StatusReport>(serviceURL("/api/v1/status"));
}

// Kept only so the legacy, unmounted authorization view remains type-checkable
// during the transition. HomeHub never registers this endpoint.
export async function redeemAuthCode(code: string): Promise<void> {
  const response = await fetch(serviceURL("/api/v1/auth/redeem"), {
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "application/json" },
    body: JSON.stringify({ code }),
  });
  if (!response.ok) throw await responseError(response);
}
