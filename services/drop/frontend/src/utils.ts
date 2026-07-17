export const TTL_OPTIONS = [1, 3, 7] as const;

export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB"];
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  const value = bytes / 1024 ** index;
  return `${value >= 10 || index === 0 ? value.toFixed(0) : value.toFixed(1)} ${units[index]}`;
}

export function formatTime(value: string): string {
  return new Intl.DateTimeFormat("zh-CN", {
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).format(new Date(value));
}

export function dayKey(value: string | Date): string {
  const date = value instanceof Date ? value : new Date(value);
  return `${date.getFullYear()}-${date.getMonth()}-${date.getDate()}`;
}

export function dayLabel(value: string): string {
  const date = new Date(value);
  const today = new Date();
  const yesterday = new Date(today);
  yesterday.setDate(today.getDate() - 1);
  if (dayKey(date) === dayKey(today)) return "今天";
  if (dayKey(date) === dayKey(yesterday)) return "昨天";
  return new Intl.DateTimeFormat("zh-CN", { month: "long", day: "numeric" }).format(date);
}

export function expiryText(value: string): string {
  const milliseconds = new Date(value).getTime() - Date.now();
  if (milliseconds <= 0) return "即将过期";
  const hours = Math.ceil(milliseconds / 3_600_000);
  if (hours < 24) return `${hours} 小时后过期`;
  return `${Math.ceil(hours / 24)} 天后过期`;
}

export function ttlLabel(days: number): string {
  return days === 1 ? "24 小时" : `${days} 天`;
}

export function formatDuration(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds <= 1) return "不到 1 秒";
  if (seconds < 60) return `约 ${Math.ceil(seconds)} 秒`;
  const minutes = Math.ceil(seconds / 60);
  if (minutes < 60) return `约 ${minutes} 分钟`;
  const hours = Math.floor(minutes / 60);
  const remainder = minutes % 60;
  return remainder ? `约 ${hours} 小时 ${remainder} 分钟` : `约 ${hours} 小时`;
}

export function fileExtension(name: string): string {
  const part = name.includes(".") ? name.split(".").pop() : "FILE";
  return (part || "FILE").slice(0, 4).toUpperCase();
}

export type TextToken = { type: "text" | "link"; value: string };

export function linkify(value: string): TextToken[] {
  const pattern = /https?:\/\/[^\s]+/g;
  const tokens: TextToken[] = [];
  let index = 0;
  for (const match of value.matchAll(pattern)) {
    const offset = match.index ?? 0;
    if (offset > index) tokens.push({ type: "text", value: value.slice(index, offset) });
    tokens.push({ type: "link", value: match[0] });
    index = offset + match[0].length;
  }
  if (index < value.length) tokens.push({ type: "text", value: value.slice(index) });
  return tokens;
}

export function readableError(reason: unknown): string {
  return reason instanceof Error ? reason.message : "操作失败，请重试";
}

export function readPreviewHistory(): Set<string> {
  try {
    const value = JSON.parse(sessionStorage.getItem("drop.preview-history.v1") || "[]") as unknown;
    return new Set(Array.isArray(value) ? value.filter((item): item is string => typeof item === "string").slice(-200) : []);
  } catch {
    return new Set();
  }
}

export function rememberPreview(id: string): void {
  try {
    const ids = readPreviewHistory();
    ids.add(id);
    sessionStorage.setItem("drop.preview-history.v1", JSON.stringify(Array.from(ids).slice(-200)));
  } catch {
    // Session storage can be unavailable in private or restrictive browsing modes.
  }
}
