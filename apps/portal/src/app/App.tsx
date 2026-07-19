import { useEffect, useState, type FormEvent, type ReactNode } from 'react';
import { startAuthentication, startRegistration } from '@simplewebauthn/browser';
import { Check, ChevronRight, Clipboard, FileText, Gauge, HardDriveUpload, KeyRound, LogOut, Menu, Monitor, Plus, Share2, Shield, Trash2, Upload, X } from 'lucide-react';
import { drop, iam, type APIKeyInfo, type DropItem, type PasskeyCredential, type SessionInfo, type SessionState, type ShareInfo } from './api';
import { DashboardHome } from './DashboardHome';
import './security.css';

function message(error: unknown) {
  const code = error instanceof Error ? error.message : 'unknown';
  return ({ invalid_credentials: '用户名、密码或验证码不正确', invalid_bootstrap: '初始化令牌无效', invalid_totp: '动态验证码不正确', rate_limited: '尝试次数过多，请稍后再试', invalid_share: '分享链接无效或已过期' } as Record<string, string>)[code] ?? '操作失败，请稍后重试';
}

function Auth({ state, reload }: { state: SessionState; reload: () => Promise<void> }) {
  const [stage, setStage] = useState<'credentials' | 'totp'>('credentials');
  const [setup, setSetup] = useState<{ setup_id: string; manual_secret: string; provisioning_uri: string }>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setBusy(true); setError('');
    const values = Object.fromEntries(new FormData(event.currentTarget));
    try {
      if (state.setup_required && stage === 'credentials') {
        if (values.password !== values.confirm_password) throw new Error('password_mismatch');
        const result = await iam.beginSetup({ bootstrap_token: values.bootstrap_token, username: values.username, display_name: values.display_name, password: values.password });
        setSetup(result); setStage('totp');
      } else if (state.setup_required && setup) {
        await iam.confirmSetup({ setup_id: setup.setup_id, totp_code: values.totp_code }); await reload();
      } else {
        await iam.login({ username: values.username, password: values.password, totp_code: values.totp_code }); await reload();
      }
    } catch (cause) {
      setError(cause instanceof Error && cause.message === 'password_mismatch' ? '两次输入的密码不一致' : message(cause));
    } finally { setBusy(false); }
  }

  async function loginWithPasskey() {
    setBusy(true); setError('');
    try {
      const options = await iam.beginPasskeyLogin();
      const credential = await startAuthentication({ optionsJSON: options.publicKey ?? options });
      await iam.finishPasskeyLogin(credential, options.ceremony_token); await reload();
    } catch (cause) { setError(message(cause)); } finally { setBusy(false); }
  }

  return <main className="auth-page"><section className="auth-card">
    <div className="brand auth-brand"><span className="brand-mark">H</span><span>HomeHub</span></div>
    <div className="auth-copy"><span className="eyebrow">PERSONAL CONTROL PLANE</span><h1>{state.setup_required ? '建立你的 HomeHub' : '欢迎回来'}</h1><p>{stage === 'totp' ? '把密钥保存到 Bitwarden 的动态密码字段，然后输入当前验证码。' : state.setup_required ? '只需完成一次。以后使用密码与 Bitwarden 动态验证码登录。' : '使用 HomeHub 账户继续。'}</p></div>
    <form onSubmit={submit} className="form-stack">
      {state.setup_required && stage === 'credentials' && <>
        <label>Bitwarden 初始化令牌<input name="bootstrap_token" type="password" autoComplete="off" required /></label>
        <div className="form-row"><label>用户名<input name="username" autoComplete="username" minLength={3} required /></label><label>显示名称<input name="display_name" defaultValue="Luna" required /></label></div>
        <label>密码<input name="password" type="password" autoComplete="new-password" minLength={12} required /></label>
        <label>确认密码<input name="confirm_password" type="password" autoComplete="new-password" minLength={12} required /></label>
      </>}
      {state.setup_required && stage === 'totp' && setup && <>
        <div className="secret-box"><span>动态密码密钥</span><code>{setup.manual_secret}</code><button type="button" onClick={() => navigator.clipboard.writeText(setup.manual_secret)}><Clipboard size={16}/>复制</button></div>
        <a className="secondary-button" href={setup.provisioning_uri}>尝试在密码管理器中打开</a>
        <label>6 位动态验证码<input name="totp_code" inputMode="numeric" autoComplete="one-time-code" pattern="[0-9]{6}" maxLength={6} required autoFocus /></label>
      </>}
      {!state.setup_required && <><label>用户名<input name="username" autoComplete="username" required autoFocus /></label><label>密码<input name="password" type="password" autoComplete="current-password" required /></label><label>Bitwarden 动态验证码<input name="totp_code" inputMode="numeric" autoComplete="one-time-code" pattern="[0-9]{6}" maxLength={6} required /></label></>}
      {error && <p className="form-error">{error}</p>}
      <button className="primary-button" disabled={busy}>{busy ? '处理中…' : state.setup_required && stage === 'credentials' ? '继续配置动态密码' : state.setup_required ? '完成初始化' : '登录'}</button>
      {!state.setup_required && <><div className="auth-divider"><span>或</span></div><button type="button" className="secondary-button passkey-button" onClick={loginWithPasskey} disabled={busy}><KeyRound size={18}/>使用通行密钥</button></>}
    </form>
  </section></main>;
}

function Layout({ state, path, navigate, logout, children }: { state: SessionState; path: string; navigate: (path: string) => void; logout: () => void; children: ReactNode }) {
  const [open, setOpen] = useState(false);
  const nav = state.administrator ? [
    { path: '/', title: '概览', icon: Gauge },
    { path: '/drop', title: 'Drop', icon: HardDriveUpload },
    { path: '/shares', title: '分享', icon: Share2 },
    { path: '/security', title: '安全', icon: KeyRound },
  ] : [{ path: '/drop', title: 'Drop', icon: HardDriveUpload }];
  return <div className="portal-shell">
    <aside className={`sidebar ${open ? 'open' : ''}`}><button className="mobile-close" onClick={() => setOpen(false)}><X/></button><button className="brand plain" onClick={() => navigate(state.administrator ? '/' : '/drop')}><span className="brand-mark">H</span><span>HomeHub</span></button><nav>{nav.map((item) => <button key={item.path} className={`nav-item ${path === item.path ? 'active' : ''}`} onClick={() => { navigate(item.path); setOpen(false); }}><item.icon size={19}/>{item.title}</button>)}</nav><div className="sidebar-footer"><span className="connection-dot"/><div><strong>{state.principal?.display_name}</strong><span>HomeHub 管理员</span></div><button className="logout" onClick={logout} aria-label="退出"><LogOut size={17}/></button></div></aside>
    <main className="main-content"><header className="topbar"><button className="menu-button" onClick={() => setOpen(true)}><Menu/></button><div className="mobile-brand"><span className="brand-mark">H</span><strong>HomeHub</strong></div><span className="top-status"><i/>已连接</span></header>{children}</main>
    <nav className="mobile-navigation">{nav.map((item) => <button key={item.path} className={path === item.path ? 'active' : ''} onClick={() => navigate(item.path)}><item.icon size={20}/><span>{item.title}</span></button>)}</nav>
  </div>;
}

function Drop() {
  const [items, setItems] = useState<DropItem[]>([]); const [busy, setBusy] = useState(false); const [error, setError] = useState('');
  const load = () => drop.list().then((x) => setItems(x.items ?? [])).catch((e) => setError(message(e)));
  useEffect(() => { load(); }, []);
  async function upload(event: FormEvent<HTMLFormElement>) { event.preventDefault(); setBusy(true); setError(''); const form = event.currentTarget; try { await drop.create(new FormData(form)); form.reset(); await load(); } catch (e) { setError(message(e)); } finally { setBusy(false); } }
  return <div className="content-wrap narrow"><section className="page-title"><div><span className="eyebrow">ORIGINAL BYTES</span><h1>Drop</h1><p>临时传递文本和原始文件，不压缩、不转码。</p></div></section><form className="drop-composer" onSubmit={upload}><textarea name="text" placeholder="写点什么，或者只选择文件…"/><div className="composer-actions"><label className="file-picker"><Plus size={18}/>选择文件<input type="file" name="files" multiple /></label><select name="ttl_days" defaultValue="1"><option value="1">保留 1 天</option><option value="3">保留 3 天</option><option value="7">保留 7 天</option></select><button className="primary-button compact" disabled={busy}><Upload size={17}/>{busy ? '上传中' : '发送'}</button></div></form>{error && <p className="form-error">{error}</p>}<section className="drop-list">{items.length === 0 && <div className="empty"><HardDriveUpload/><strong>这里还是空的</strong><span>从任意设备发送第一条内容。</span></div>}{items.map((item) => <article className="drop-item" key={item.id}><div className="item-meta"><span>{new Date(item.created_at).toLocaleString()}</span><span>· {new Date(item.expires_at).toLocaleDateString()} 过期</span></div>{item.text && <p className="drop-text">{item.text}</p>}<div className="attachments">{item.attachments.map((file) => <a key={file.id} href={`/drop/v1/attachments/${file.id}`} target="_blank" rel="noreferrer">{file.media_type.startsWith('image/') ? <img src={`/drop/v1/attachments/${file.id}`} alt={file.original_name}/> : <span className="file-icon"><FileText/></span>}<span><strong>{file.original_name}</strong><small>{formatBytes(file.size)}</small></span><ChevronRight/></a>)}</div><button className="delete-item" onClick={() => drop.remove(item.id).then(load)}><Trash2 size={16}/>删除</button></article>)}</section></div>;
}

function Shares() {
  const [shares, setShares] = useState<ShareInfo[]>([]);
  const [showCreate, setShowCreate] = useState(false);
  const [created, setCreated] = useState('');
  const load = () => iam.shares().then((x) => setShares(x.shares ?? []));
  useEffect(() => { load(); }, []);

  async function doCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const values = Object.fromEntries(new FormData(event.currentTarget));
    const hours = Number(values.hours) || 24;
    const body: any = {
      share_type: values.share_type || 'service',
      service_id: values.service_id || 'drop',
      actions: (String(values.actions || 'drop.item.read,drop.item.list').split(',').map((s: string) => s.trim())),
      expires_at: new Date(Date.now() + hours * 3600_000).toISOString(),
    };
    if (body.share_type === 'resource') {
      body.resource_id = values.resource_id;
    }
    if (values.max_uses) body.max_uses = Number(values.max_uses);
    try {
      const result = await iam.createShare(body);
      const link = `https://zlx2.com/#share=${encodeURIComponent(result.token)}&path=${encodeURIComponent('/drop')}`;
      setCreated(link);
      await navigator.clipboard.writeText(link).catch(() => {});
      setShowCreate(false);
      await load();
    } catch (e) { alert(message(e)); }
  }

  return <div className="content-wrap narrow"><section className="page-title"><span className="eyebrow">DIRECT CAPABILITY</span><h1>分享</h1><p>创建 Drop 服务或单文件分享链接，可限制有效期和使用次数。</p></section>
    <button className="primary-button compact" onClick={() => setShowCreate(!showCreate)} style={{ marginBottom: 16 }}><Plus size={17}/>创建分享</button>
    {showCreate && <form className="form-stack share-form" onSubmit={doCreate} style={{ marginBottom: 24, padding: 16, border: '1px solid var(--border)', borderRadius: 8 }}>
      <div className="form-row">
        <label>类型<select name="share_type" defaultValue="service"><option value="service">服务分享</option><option value="resource">资源分享</option></select></label>
        <label>服务<select name="service_id" defaultValue="drop"><option value="drop">Drop</option></select></label>
      </div>
      <label>Actions<input name="actions" defaultValue="drop.item.read,drop.item.list" /></label>
      <label>资源 ID (资源分享时填写)<input name="resource_id" placeholder="Drop item ID" /></label>
      <div className="form-row">
        <label>有效期<select name="hours" defaultValue="24"><option value="1">1 小时</option><option value="24">1 天</option><option value="168">7 天</option><option value="720">30 天</option></select></label>
        <label>最大使用次数 (可选)<input name="max_uses" type="number" min="1" placeholder="无限制" /></label>
      </div>
      <button className="primary-button compact"><Share2 size={17}/>生成</button>
    </form>}
    {created && <div className="created-link"><Check/><input readOnly value={created}/><button onClick={() => navigator.clipboard.writeText(created)}><Clipboard/></button></div>}
    <div className="share-list">{shares.length === 0 && <div className="empty"><Share2/><strong>还没有分享</strong></div>}{shares.map((share) => <article key={share.id}><div><strong>{share.service_id} · {share.share_type}</strong><small>{share.revoked_at ? '已撤销' : `${new Date(share.expires_at).toLocaleString()} 过期`} · 已用 {share.use_count}{share.max_uses ? `/${share.max_uses}` : ''}次</small><small className="actions-tag">{share.actions.join(', ')}</small></div>{!share.revoked_at && <button onClick={() => iam.revokeShare(share.id).then(load)}>撤销</button>}</article>)}</div></div>;
}

function SessionsTab() {
  const [sessions, setSessions] = useState<SessionInfo[]>([]);
  const [currentID, setCurrentID] = useState('');
  const load = () => iam.sessions().then((x) => { setSessions(x.sessions ?? []); setCurrentID(x.current_session_id); });
  useEffect(() => { load(); }, []);
  return <div className="content-wrap narrow"><section className="page-title"><span className="eyebrow">DEVICE MANAGEMENT</span><h1>活跃会话</h1><p>这些设备当前已登录。你可以单独或一键撤销其他会话。</p></section>
    {sessions.length > 1 && <button className="secondary-button" style={{ marginBottom: 16 }} onClick={() => iam.revokeOtherSessions().then(load)}><Trash2 size={16}/>撤销其他所有会话</button>}
    <div className="session-list">{sessions.length === 0 && <div className="empty"><Monitor/><strong>无活跃会话</strong></div>}{sessions.map((s) => <article key={s.id} className={s.id === currentID ? 'current' : ''}><div><strong>{s.auth_methods?.join(', ') || 'Session'}</strong><small>{`${s.remote_ip || '未知 IP'} · 创建: ${new Date(s.created_at).toLocaleString()} · 最近: ${new Date(s.last_seen_at).toLocaleString()}`}</small></div><div className="session-actions">{s.id === currentID && <span className="badge">当前</span>}{s.revoked_at ? <span className="badge revoked">已撤销</span> : s.id !== currentID && <button onClick={() => iam.revokeSession(s.id).then(load)}>撤销</button>}</div></article>)}</div></div>;
}

function APIKeysTab() {
  const [keys, setKeys] = useState<APIKeyInfo[]>([]);
  const [showCreate, setShowCreate] = useState(false);
  const [newToken, setNewToken] = useState('');
  const load = () => iam.apiKeys().then((x) => setKeys(x.api_keys ?? []));
  useEffect(() => { load(); }, []);

  async function doCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const values = Object.fromEntries(new FormData(event.currentTarget));
    const body: any = {
      name: values.name,
      kind: values.kind || 'agent',
      scopes: (String(values.scopes || '*').split(',').map((s: string) => s.trim())),
    };
    if (values.expires_in_days) body.expires_in_days = Number(values.expires_in_days);
    try {
      const result = await iam.createAPIKey(body);
      setNewToken(result.token);
      setShowCreate(false);
      await load();
    } catch (e) { alert(message(e)); }
  }

  return <div className="content-wrap narrow"><section className="page-title"><span className="eyebrow">LONG-LIVED TOKENS</span><h1>API Keys</h1><p>用于脚本、iOS 快捷指令和 Hermes Agent 的长期访问令牌。</p></section>
    <button className="primary-button compact" onClick={() => setShowCreate(!showCreate)} style={{ marginBottom: 16 }}><Plus size={17}/>创建 Key</button>
    {showCreate && <form className="form-stack" onSubmit={doCreate} style={{ marginBottom: 24, padding: 16, border: '1px solid var(--border)', borderRadius: 8 }}>
      <label>名称<input name="name" placeholder="例如: Hermes Agent" required /></label>
      <div className="form-row">
        <label>类型<select name="kind" defaultValue="agent"><option value="agent">Agent</option><option value="device">Device</option><option value="service">Service</option></select></label>
        <label>有效期天数 (可选)<input name="expires_in_days" type="number" min="1" placeholder="永久" /></label>
      </div>
      <label>Scopes (逗号分隔)<input name="scopes" defaultValue="*" /></label>
      <button className="primary-button compact"><KeyRound size={17}/>创建</button>
    </form>}
    {newToken && <div className="created-link token-warning"><Check/><div><strong>保存此 Token！它只会显示一次。</strong></div><textarea readOnly value={newToken} rows={2}/><button onClick={() => { navigator.clipboard.writeText(newToken); setNewToken(''); }}><Clipboard/>已复制</button></div>}
    <div className="key-list">{keys.length === 0 && <div className="empty"><KeyRound/><strong>还没有 API Key</strong></div>}{keys.map((key) => <article key={key.id}><div><strong>{key.name}</strong><small>{key.kind} · {key.scopes?.join(', ')}</small><small>{key.revoked_at ? '已撤销' : key.expires_at ? `${new Date(key.expires_at).toLocaleDateString()} 到期` : '永久有效'} · 创建于 {new Date(key.created_at).toLocaleDateString()}{key.last_used_at ? ` · 上次使用: ${new Date(key.last_used_at).toLocaleString()}` : ' · 未使用'}</small></div>{!key.revoked_at && <button onClick={() => iam.revokeAPIKey(key.id).then(load)}>撤销</button>}</article>)}</div></div>;
}

function Security() {
  const [tab, setTab] = useState<'passkeys' | 'sessions' | 'api-keys'>('passkeys');
  return <div className="security-page">
    <div className="security-tabs">
      <button className={tab === 'passkeys' ? 'active' : ''} onClick={() => setTab('passkeys')}><KeyRound size={17}/>通行密钥</button>
      <button className={tab === 'sessions' ? 'active' : ''} onClick={() => setTab('sessions')}><Monitor size={17}/>会话</button>
      <button className={tab === 'api-keys' ? 'active' : ''} onClick={() => setTab('api-keys')}><Shield size={17}/>API Keys</button>
    </div>
    {tab === 'passkeys' && <PasskeysTab/>}
    {tab === 'sessions' && <SessionsTab/>}
    {tab === 'api-keys' && <APIKeysTab/>}
  </div>;
}

function PasskeysTab() {
  const [passkeys, setPasskeys] = useState<PasskeyCredential[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const load = () => iam.passkeys().then((result) => setPasskeys(result.passkeys ?? [])).catch((cause) => setError(message(cause)));
  useEffect(() => { load(); }, []);

  async function register() {
    setBusy(true); setError('');
    try {
      const options = await iam.beginPasskeyRegistration();
      const credential = await startRegistration({ optionsJSON: options.publicKey ?? options });
      await iam.finishPasskeyRegistration(credential, 'Bitwarden Passkey', options.ceremony_token);
      await load();
    } catch (cause) { setError(message(cause)); } finally { setBusy(false); }
  }

  return <div className="content-wrap narrow"><section className="page-title"><span className="eyebrow">ACCOUNT SECURITY</span><h1>通行密钥</h1><p>把通行密钥保存到 Bitwarden 后，手机和电脑都可以直接确认登录。密码与动态验证码继续作为恢复方式。</p></section><section className="security-card"><div><span className="security-icon"><KeyRound/></span><h2>添加通行密钥</h2><p>只保存公钥凭据，Bitwarden 中的私钥不会发送到 HomeHub。</p></div><button className="primary-button compact" onClick={register} disabled={busy}><Plus size={17}/>{busy ? '正在创建…' : '添加通行密钥'}</button></section>{error && <p className="form-error">{error}</p>}<div className="passkey-list">{passkeys.length === 0 && <div className="empty"><KeyRound/><strong>还没有通行密钥</strong><span>添加后即可免输密码登录。</span></div>}{passkeys.map((passkey) => <article key={passkey.id}><div><strong>{passkey.name}</strong><small>创建于 {new Date(passkey.created_at).toLocaleString()}{passkey.last_used_at ? ` · 最近使用 ${new Date(passkey.last_used_at).toLocaleString()}` : ''}</small></div><button onClick={() => iam.deletePasskey(passkey.id).then(load)}><Trash2 size={16}/>删除</button></article>)}</div></div>;
}

function formatBytes(size: number) { if (size < 1024) return `${size} B`; if (size < 1024 ** 2) return `${(size / 1024).toFixed(1)} KB`; return `${(size / 1024 ** 2).toFixed(1)} MB`; }

export function App() {
  const [state, setState] = useState<SessionState>(); const [path, setPath] = useState(location.pathname); const [error, setError] = useState('');
  const reload = async () => setState(await iam.session());
  useEffect(() => { const hash = new URLSearchParams(location.hash.slice(1)); const token = hash.get('share'); if (token) { iam.redeem(token).then(() => { history.replaceState({}, '', hash.get('path') || '/drop'); setPath(location.pathname); return reload(); }).catch((e) => { setError(message(e)); reload(); }); } else reload(); const pop = () => setPath(location.pathname); addEventListener('popstate', pop); return () => removeEventListener('popstate', pop); }, []);
  function navigate(next: string) { if (next === '/drop') { location.assign('/drop/'); return; } history.pushState({}, '', next); setPath(next); }
  if (!state) return <div className="loading"><span className="brand-mark">H</span><i/></div>;
  if (!state.authenticated) return <><Auth state={state} reload={reload}/>{error && <div className="toast">{error}</div>}</>;
  const actualPath = !state.administrator && path !== '/drop' ? '/drop' : path;
  if (state.administrator && actualPath === '/') return <DashboardHome name={state.principal?.display_name ?? 'Luna'} navigate={navigate}/>;
  return <Layout state={state} path={actualPath} navigate={navigate} logout={() => iam.logout().finally(reload)}>{actualPath === '/drop' ? <Drop/> : actualPath === '/shares' ? <Shares/> : <Security/>}</Layout>;
}
