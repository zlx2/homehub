export function formatBytes(size: number) {
  if (size < 1024) return `${size} B`;
  if (size < 1024 ** 2) return `${(size / 1024).toFixed(1)} KB`;
  if (size < 1024 ** 3) return `${(size / 1024 ** 2).toFixed(1)} MB`;
  return `${(size / 1024 ** 3).toFixed(1)} GB`;
}

export function formatTime(value: string) { return new Date(value).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }); }
export function dayKey(value: string) { const date = new Date(value); return `${date.getFullYear()}-${date.getMonth()}-${date.getDate()}`; }
export function dayLabel(value: string) {
  const date = new Date(value); const today = new Date(); const yesterday = new Date(Date.now() - 86400000);
  if (dayKey(value) === dayKey(today.toISOString())) return '今天';
  if (dayKey(value) === dayKey(yesterday.toISOString())) return '昨天';
  return date.toLocaleDateString('zh-CN', { month: 'long', day: 'numeric' });
}
export function expiryText(value: string) {
  const hours = Math.max(0, (new Date(value).getTime() - Date.now()) / 3600000);
  if (hours < 1) return `${Math.max(1, Math.ceil(hours * 60))} 分钟后过期`;
  if (hours < 24) return `${Math.ceil(hours)} 小时后过期`;
  return `${Math.ceil(hours / 24)} 天后过期`;
}
export function fileExtension(name: string) { const value = name.split('.').pop() || 'FILE'; return value.slice(0, 5).toUpperCase(); }
