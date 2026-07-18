export type Attachment = {
  id: string;
  original_name: string;
  media_type: string;
  size: number;
  content_url: string;
};

export type DropItem = {
  id: string;
  text?: string;
  created_at: string;
  expires_at: string;
  total_size: number;
  attachments: Attachment[];
};

export type DropStatus = {
  status: string;
  storage: { used_bytes: number; quota_bytes: number; item_count: number; attachment_count: number };
};
