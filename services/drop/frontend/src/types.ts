export type Role = "guest" | "owner" | "hermes";

export interface Attachment {
  id: string;
  original_name: string;
  mime_type: string;
  size: number;
  previewable: boolean;
  download_url: string;
  preview_url?: string;
}

export interface DropItem {
  id: string;
  text_preview?: string;
  text_truncated: boolean;
  text_size: number;
  has_text: boolean;
  source: Role;
  created_at: string;
  expires_at: string;
  total_size: number;
  attachments: Attachment[];
  full_text_url?: string;
}

export interface AuthCode {
  code: string;
  expires_at: string;
  session_ttl_seconds?: number;
  redeem_url?: string;
  qr_data_url?: string;
}

export interface TrustedSession {
  id: number;
  device_name: string;
  created_at: string;
  last_seen_at: string;
  expires_at: string;
  last_ip?: string;
  current: boolean;
}

export interface StatusReport {
  status: string;
  storage: {
    used_bytes: number;
    quota_bytes: number;
    item_count: number;
    attachment_count: number;
  };
  traffic: {
    last_24_hours: { public_bytes: number; tailscale_bytes: number; hermes_bytes: number; total_bytes: number };
    last_30_days: { public_bytes: number; tailscale_bytes: number; hermes_bytes: number; total_bytes: number };
  };
  traffic_note: string;
  sse_clients: number;
}

export interface APIErrorBody {
  error?: { message?: string };
}
