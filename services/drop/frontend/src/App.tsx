import { useCallback, useEffect, useMemo, useRef, useState, type ChangeEvent, type DragEvent, type FormEvent, type KeyboardEvent } from 'react';
import { attachmentURL, csrfToken, dropAPI } from './api';
import { Icon } from './Icon';
import type { Attachment, DropItem, DropStatus } from './types';
import { dayKey, dayLabel, expiryText, fileExtension, formatBytes, formatTime } from './utils';

const TTL_OPTIONS = [1, 3, 7];

function linkify(value: string) {
  return value.split(/(https?:\/\/[^\s]+)/g).map((part, index) => /^https?:\/\//.test(part) ? <a key={index} href={part} target="_blank" rel="noreferrer" onClick={(event) => event.stopPropagation()}>{part}</a> : part);
}

function Lightbox({ images, initial, close }: { images: Attachment[]; initial: number; close: () => void }) {
  const [index, setIndex] = useState(initial);
  const image = images[index];
  useEffect(() => {
    const key = (event: globalThis.KeyboardEvent) => {
      if (event.key === 'Escape') close();
      if (event.key === 'ArrowLeft') setIndex((value) => (value - 1 + images.length) % images.length);
      if (event.key === 'ArrowRight') setIndex((value) => (value + 1) % images.length);
    };
    addEventListener('keydown', key); return () => removeEventListener('keydown', key);
  }, [close, images.length]);
  return <div className="image-lightbox" role="dialog" aria-modal="true">
    <header className="lightbox-header"><div className="lightbox-title"><strong>{image.original_name}</strong><span>{formatBytes(image.size)}</span></div><a className="lightbox-action lightbox-action--download" href={`${attachmentURL(image)}?download=1`} download={image.original_name}><Icon name="download"/></a><button className="lightbox-action" onClick={close} aria-label="关闭"><Icon name="close"/></button></header>
    <div className="lightbox-stage"><img src={attachmentURL(image)} alt={image.original_name}/>{images.length > 1 && <><button className="lightbox-nav lightbox-nav--previous" onClick={() => setIndex((index - 1 + images.length) % images.length)}>‹</button><button className="lightbox-nav lightbox-nav--next" onClick={() => setIndex((index + 1) % images.length)}>›</button></>}</div>
    <div className="lightbox-dots">{images.map((item, position) => <i className={position === index ? 'active' : ''} key={item.id}/>)}</div>
  </div>;
}

function AttachmentCard({ attachment, open }: { attachment: Attachment; open: () => void }) {
  const image = attachment.media_type.startsWith('image/') && attachment.media_type !== 'image/svg+xml';
  const video = attachment.media_type.startsWith('video/');
  const [loaded, setLoaded] = useState(false);
  const [play, setPlay] = useState(false);
  return <div className={`attachment ${image ? 'attachment--image' : ''} ${video ? 'attachment--video' : ''}`}>
    {image ? <div className="attachment-visual"><button className={`image-link ${loaded ? 'is-loaded' : ''}`} type="button" onClick={open}><img src={attachmentURL(attachment)} alt={attachment.original_name} loading="lazy" onLoad={() => setLoaded(true)}/>{!loaded && <span className="image-loading"><i/>正在加载预览</span>}</button></div>
      : video ? <div className="attachment-visual attachment-video">{play ? <video src={attachmentURL(attachment)} controls playsInline preload="metadata"/> : <button className="preview-trigger video-trigger" type="button" onClick={() => setPlay(true)}><span className="video-play">▶</span><span>点按在页面内播放</span></button>}</div>
      : <div className="file-glyph"><span>{fileExtension(attachment.original_name)}</span></div>}
    <div className="attachment-details"><span className="attachment-name" title={attachment.original_name}>{attachment.original_name}</span><span className="attachment-size">{formatBytes(attachment.size)}</span></div>
    <a className="attachment-download" href={`${attachmentURL(attachment)}?download=1`} download={attachment.original_name} aria-label={`下载 ${attachment.original_name}`}><Icon name="download"/></a>
  </div>;
}

function MessageCard({ item, copy, remove, expiry }: { item: DropItem; copy: () => void; remove: () => void; expiry: (days: number) => void }) {
  const [menu, setMenu] = useState(false);
  const [expiryMenu, setExpiryMenu] = useState(false);
  const [lightbox, setLightbox] = useState<number>();
  const images = item.attachments.filter((file) => file.media_type.startsWith('image/') && file.media_type !== 'image/svg+xml');
  return <article className="drop-card">
    <div className="drop-card-content">
      {item.text && <p className="drop-text drop-text--copyable" role="button" tabIndex={0} title="点击复制全文" onClick={copy}>{linkify(item.text)}</p>}
      {item.attachments.length > 0 && <div className={`attachment-grid ${item.attachments.length === 1 ? 'attachment-grid--single' : ''}`}>{item.attachments.map((file) => <AttachmentCard key={file.id} attachment={file} open={() => setLightbox(images.findIndex((image) => image.id === file.id))}/>)}</div>}
      <div className="drop-meta"><time dateTime={item.created_at}>{formatTime(item.created_at)}</time><span>{expiryText(item.expires_at)}</span>{item.total_size > 0 && <span>{formatBytes(item.total_size)}</span>}</div>
    </div>
    <div className="card-actions"><button className="quiet-icon-button" type="button" aria-label="更多操作" onClick={() => { setMenu(!menu); setExpiryMenu(false); }}><Icon name="more"/></button>
      {menu && <div className="card-menu card-menu--down" role="menu">{expiryMenu ? <><button className="menu-back" type="button" onClick={() => setExpiryMenu(false)}><Icon name="back"/><span>调整有效期</span></button><div className="menu-separator"/>{TTL_OPTIONS.map((days) => <button key={days} type="button" onClick={() => { expiry(days); setMenu(false); }}><Icon name="clock"/><span>保留 {days} 天</span></button>)}</> : <>{item.text && <button type="button" onClick={() => { copy(); setMenu(false); }}><Icon name="copy"/><span>复制全文</span></button>}<button className="menu-expiry-action" type="button" onClick={() => setExpiryMenu(true)}><Icon name="clock"/><span><strong>有效期</strong><small>{expiryText(item.expires_at)}</small></span><span className="menu-chevron">›</span></button><div className="menu-separator"/><button className="danger-action" type="button" onClick={() => { remove(); setMenu(false); }}><Icon name="trash"/><span>彻底删除</span></button></>}</div>}
    </div>
    {lightbox !== undefined && lightbox >= 0 && <Lightbox images={images} initial={lightbox} close={() => setLightbox(undefined)}/>} 
  </article>;
}

function Settings({ ttl, setTTL, showToast }: { ttl: number; setTTL: (days: number) => void; showToast: (value: string) => void }) {
  const [open, setOpen] = useState(false); const [status, setStatus] = useState<DropStatus>(); const [loading, setLoading] = useState(false);
  async function toggle() {
    const next = !open; setOpen(next);
    if (next && !status) { setLoading(true); try { setStatus(await dropAPI.status()); } catch { showToast('暂时无法读取状态'); } finally { setLoading(false); } }
  }
  return <div className="settings-popover"><button className="composer-icon-button" type="button" aria-label="消息设置" onClick={toggle}><Icon name="settings"/></button>{open && <section className="settings-panel"><div className="settings-heading"><div><p className="menu-caption">设置</p><h2>保存期限</h2></div><button className="panel-close" onClick={() => setOpen(false)}><Icon name="close"/></button></div><div className="ttl-list">{TTL_OPTIONS.map((days) => <button className={ttl === days ? 'selected' : ''} key={days} onClick={() => { setTTL(days); setOpen(false); showToast(`新消息保留 ${days} 天`); }}><span>保留 {days} 天</span>{ttl === days && <Icon name="check"/>}</button>)}</div><div className={`status-block ${loading ? 'loading' : ''}`}>{status ? <><div className="status-row"><span>存储空间</span><strong>{formatBytes(status.storage.used_bytes)} / {formatBytes(status.storage.quota_bytes)}</strong></div><div className="storage-track"><i style={{ width: `${Math.min(100, status.storage.used_bytes / status.storage.quota_bytes * 100)}%` }}/></div><div className="status-row"><span>内容</span><strong>{status.storage.item_count} 条 · {status.storage.attachment_count} 个文件</strong></div><p>登录和分享权限由 HomeHub 统一管理</p></> : <span>{loading ? '正在读取状态…' : '暂时无法读取状态'}</span>}</div></section>}</div>;
}

function Composer({ sent, showToast, connection }: { sent: () => void; showToast: (value: string) => void; connection: 'connected'|'connecting'|'disconnected'|'offline' }) {
  const [text, setText] = useState(''); const [files, setFiles] = useState<File[]>([]); const [ttl, setTTL] = useState(1); const [busy, setBusy] = useState(false); const [progress, setProgress] = useState(0); const [error, setError] = useState(''); const [dragging, setDragging] = useState(false);
  const input = useRef<HTMLInputElement>(null); const textarea = useRef<HTMLTextAreaElement>(null); const request = useRef<XMLHttpRequest | undefined>(undefined);
  const total = files.reduce((sum, file) => sum + file.size, 0);
  function addFiles(next: FileList | File[]) { setFiles((current) => [...current, ...Array.from(next).filter((file) => !current.some((item) => item.name === file.name && item.size === file.size)).slice(0, 10 - current.length)]); }
  function upload(event?: FormEvent) {
    event?.preventDefault(); if (busy || (!text.trim() && files.length === 0)) return;
    const form = new FormData(); if (text) form.append('text', text); form.append('ttl_days', String(ttl)); files.forEach((file) => form.append('files', file, file.name));
    const xhr = new XMLHttpRequest(); request.current = xhr; setBusy(true); setError(''); setProgress(1); xhr.open('POST', '/drop/v1/items'); xhr.responseType = 'json'; xhr.setRequestHeader('Idempotency-Key', crypto.randomUUID()); xhr.setRequestHeader('X-CSRF-Token', csrfToken());
    xhr.upload.onprogress = (value) => value.lengthComputable && setProgress(Math.round(value.loaded / value.total * 100));
    xhr.onload = () => { setBusy(false); request.current = undefined; if (xhr.status >= 200 && xhr.status < 300) { setText(''); setFiles([]); setProgress(0); showToast('已发送'); sent(); } else setError(xhr.response?.error || `发送失败 (${xhr.status})`); };
    xhr.onerror = () => { setBusy(false); request.current = undefined; setError('网络连接中断，内容仍保留在发送区'); };
    xhr.onabort = () => { setBusy(false); request.current = undefined; setError('已取消上传，内容仍保留在发送区'); };
    xhr.send(form);
  }
  function handleKey(event: KeyboardEvent<HTMLTextAreaElement>) { if (event.key === 'Enter' && !event.shiftKey && !event.nativeEvent.isComposing) { event.preventDefault(); upload(); } }
  function drop(event: DragEvent) { event.preventDefault(); setDragging(false); if (event.dataTransfer.files.length) addFiles(event.dataTransfer.files); }
  return <div className="composer-dock"><form className={`composer-box ${dragging ? 'is-dragging' : ''}`} onSubmit={upload} onDragOver={(event) => { event.preventDefault(); setDragging(true); }} onDragLeave={() => setDragging(false)} onDrop={drop}>
    {dragging && <div className="page-drop-overlay"><div className="page-drop-prompt"><Icon name="plus"/><strong>松开以添加文件</strong><span>可放到页面任意位置</span></div></div>}
    <textarea ref={textarea} value={text} rows={1} placeholder="粘贴文字、网址或截图…" disabled={busy} onChange={(event) => { setText(event.target.value); event.target.style.height = 'auto'; event.target.style.height = `${Math.min(event.target.scrollHeight, 180)}px`; }} onKeyDown={handleKey} onPaste={(event) => event.clipboardData.files.length && addFiles(event.clipboardData.files)}/>
    {files.length > 0 && <div className="selected-files">{files.map((file, index) => <div className="selected-file" key={`${file.name}-${file.lastModified}`}><span className="selected-file-type">{fileExtension(file.name)}</span><span className="selected-file-copy"><strong>{file.name}</strong><small>{formatBytes(file.size)}</small></span><button type="button" onClick={() => setFiles((items) => items.filter((_, position) => position !== index))}><Icon name="close"/></button></div>)}</div>}
    {busy && <div className="upload-progress"><div className="upload-progress-copy"><strong>{progress >= 100 ? '服务器正在保存' : `正在上传 · ${progress}%`}</strong><span>请稍候</span></div><div className="upload-track"><i style={{ width: `${progress}%` }}/></div></div>}
    <div className="composer-footer"><div className="composer-tools"><input ref={input} type="file" multiple hidden onChange={(event: ChangeEvent<HTMLInputElement>) => event.target.files && addFiles(event.target.files)}/><button className="composer-icon-button" type="button" disabled={busy} onClick={() => input.current?.click()}><Icon name="plus"/></button><Settings ttl={ttl} setTTL={setTTL} showToast={showToast}/>{files.length > 0 && <span className="file-total">{files.length} 个文件 · {formatBytes(total)}</span>}</div><div className="composer-actions"><span className={`connection-light connection-light--${connection}`} title={connection === 'connected' ? '实时连接正常' : '正在连接'}/><button className={`send-button ${busy ? 'send-button--stop' : ''}`} type="button" disabled={!busy && !text.length && files.length === 0} onClick={() => busy ? request.current?.abort() : upload()}><Icon name={busy ? 'stop' : 'send'}/></button></div></div>
    {error && <p className="composer-error">{error}</p>}
  </form></div>;
}

export function App() {
  const [items, setItems] = useState<DropItem[]>([]); const [loading, setLoading] = useState(true); const [refreshing, setRefreshing] = useState(false); const [toast, setToast] = useState(''); const [pending, setPending] = useState<DropItem>(); const [connection, setConnection] = useState<'connected'|'connecting'|'disconnected'|'offline'>('connecting');
  const toastTimer = useRef<number | undefined>(undefined);
  const showToast = useCallback((value: string) => { setToast(value); clearTimeout(toastTimer.current); toastTimer.current = window.setTimeout(() => setToast(''), 2400); }, []);
  const load = useCallback(async (feedback = false) => { if (feedback) setRefreshing(true); try { const result = await dropAPI.list(); setItems(result.items ?? []); if (feedback) showToast('已刷新'); } catch (error) { showToast(error instanceof Error ? error.message : '读取失败'); } finally { setLoading(false); setRefreshing(false); } }, [showToast]);
  useEffect(() => { void load(); const events = new EventSource(dropAPI.eventsURL); events.onopen = () => setConnection('connected'); events.addEventListener('items_changed', () => void load()); events.onerror = () => setConnection(navigator.onLine ? 'disconnected' : 'offline'); return () => events.close(); }, [load]);
  useEffect(() => { if (!loading) requestAnimationFrame(() => scrollTo({ top: document.documentElement.scrollHeight })); }, [loading]);
  const groups = useMemo(() => { const output: Array<{ key: string; label: string; items: DropItem[] }> = []; [...items].sort((a,b) => +new Date(a.created_at) - +new Date(b.created_at)).forEach((item) => { const key = dayKey(item.created_at); let group = output.at(-1); if (!group || group.key !== key) { group = { key, label: dayLabel(item.created_at), items: [] }; output.push(group); } group.items.push(item); }); return output; }, [items]);
  async function changeExpiry(item: DropItem, days: number) { try { const updated = await dropAPI.updateExpiry(item.id, days); setItems((values) => values.map((value) => value.id === item.id ? updated : value)); showToast(`有效期已改为 ${days} 天`); } catch { showToast('调整有效期失败'); } }
  async function remove() { if (!pending) return; try { await dropAPI.remove(pending.id); setItems((values) => values.filter((item) => item.id !== pending.id)); setPending(undefined); showToast('已彻底删除'); } catch { showToast('删除失败'); } }
  return <main className="workspace"><button className={`floating-refresh ${refreshing ? 'spinning' : ''}`} style={{ left: 20, top: 18 }} onClick={() => load(true)} aria-label="刷新"><Icon name="refresh"/></button><section className="timeline">{loading ? <><div className="day-divider"><span>正在读取</span></div>{[1,2,3].map((value) => <div className="skeleton-card" key={value}><i/><span/><small/></div>)}</> : groups.length ? <div className="timeline-feed">{groups.map((group) => <section className="day-group" key={group.key}><div className="day-divider"><span>{group.label}</span></div><div className="day-items">{group.items.map((item) => <MessageCard key={item.id} item={item} copy={() => navigator.clipboard.writeText(item.text ?? '').then(() => showToast('已复制全文'))} remove={() => setPending(item)} expiry={(days) => changeExpiry(item, days)}/>)}</div></section>)}</div> : <section className="empty-state"><div className="empty-mark"><span/></div><h1>这里还很安静</h1><p>粘贴一段文字、截图，或者添加文件。</p></section>}</section><Composer sent={() => load()} showToast={showToast} connection={connection}/>
    {pending && <div className="dialog-backdrop" role="dialog" aria-modal="true"><section className="confirm-dialog"><div className="confirm-icon"><Icon name="trash"/></div><h2>删除这条消息？</h2><p>消息和全部附件会立即永久删除，此操作无法恢复。</p><div className="confirm-actions"><button className="secondary-button" onClick={() => setPending(undefined)}>取消</button><button className="danger-button" onClick={remove}>彻底删除</button></div></section></div>}
    {toast && <div className="toast" role="status">{toast}</div>}
  </main>;
}
