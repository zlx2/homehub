import { useEffect, useState, type FormEvent, type ReactNode } from 'react';
import { startAuthentication, startRegistration } from '@simplewebauthn/browser';
import { Check, ChevronRight, Clipboard, FileText, Gauge, HardDriveUpload, KeyRound, LogOut, Menu, Plus, Server, Share2, Trash2, Upload, X } from 'lucide-react';
import { drop, iam, type DropItem, type PasskeyCredential, type SessionState } from './api';
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
      await iam.finishPasskeyLogin(credential); await reload();
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
  const nav = state.administrator ? [{ path: '/', title: '概览', icon: Gauge }, { path: '/drop', title: 'Drop', icon: HardDriveUpload }, { path: '/shares', title: '分享', icon: Share2 }, { path: '/security', title: '安全', icon: KeyRound }] : [{ path: '/drop', title: 'Drop', icon: HardDriveUpload }];
  return <div className="portal-shell">
    <aside className={`sidebar ${open ? 'open' : ''}`}><button className="mobile-close" onClick={() => setOpen(false)}><X/></button><button className="brand plain" onClick={() => navigate(state.administrator ? '/' : '/drop')}><span className="brand-mark">H</span><span>HomeHub</span></button><nav>{nav.map((item) => <button key={item.path} className={`nav-item ${path === item.path ? 'active' : ''}`} onClick={() => { navigate(item.path); setOpen(false); }}><item.icon size={19}/>{item.title}</button>)}</nav><div className="sidebar-footer"><span className="connection-dot"/><div><strong>{state.principal?.display_name}</strong><span>{state.principal?.kind === 'guest' ? '访客会话' : 'HomeHub 管理员'}</span></div><button className="logout" onClick={logout} aria-label="退出"><LogOut size={17}/></button></div></aside>
    <main className="main-content"><header className="topbar"><button className="menu-button" onClick={() => setOpen(true)}><Menu/></button><div className="mobile-brand"><span className="brand-mark">H</span><strong>HomeHub</strong></div><span className="top-status"><i/>已连接</span></header>{children}</main>
    <nav className="mobile-navigation">{nav.map((item) => <button key={item.path} className={path === item.path ? 'active' : ''} onClick={() => navigate(item.path)}><item.icon size={20}/><span>{item.title}</span></button>)}</nav>
  </div>;
}

function Overview({ name }: { name: string }) {
  const [data, setData] = useState<any>();
  useEffect(() => { fetch('/api/control/v1/overview').then((r) => r.ok ? r.json() : Promise.reject()).then(setData).catch(() => setData({ summary: { total_services: 0, healthy_services: 0 }, services: [] })); }, []);
  return <div className="content-wrap"><section className="hero"><div><span className="eyebrow">HOMEHUB V2</span><h1>你好，{name}</h1><p>一个入口，连接你的服务、设备与 Agent。</p></div></section><div className="stats"><article><span>服务</span><strong>{data?.summary?.total_services ?? '—'}</strong></article><article><span>健康</span><strong className="good">{data?.summary?.healthy_services ?? '—'}</strong></article><article><span>架构</span><strong>V2</strong></article></div><section className="section"><div className="section-heading"><div><span className="eyebrow">SERVICES</span><h2>服务状态</h2></div></div><div className="service-list">{(data?.services ?? []).map((service: any) => <article key={service.id}><span className={`service-icon ${service.status.state}`}><Server size={19}/></span><div><strong>{service.name}</strong><small>{service.description}</small></div><span className={`health ${service.status.state}`}>{service.status.state === 'healthy' ? '正常' : '不可用'}</span></article>)}</div></section></div>;
}

function Drop() {
  const [items, setItems] = useState<DropItem[]>([]); const [busy, setBusy] = useState(false); const [error, setError] = useState('');
  const load = () => drop.list().then((x) => setItems(x.items ?? [])).catch((e) => setError(message(e)));
  useEffect(() => { load(); }, []);
  async function upload(event: FormEvent<HTMLFormElement>) { event.preventDefault(); setBusy(true); setError(''); const form = event.currentTarget; try { await drop.create(new FormData(form)); form.reset(); await load(); } catch (e) { setError(message(e)); } finally { setBusy(false); } }
  return <div className="content-wrap narrow"><section className="page-title"><div><span className="eyebrow">ORIGINAL BYTES</span><h1>Drop</h1><p>临时传递文本和原始文件，不压缩、不转码。</p></div></section><form className="drop-composer" onSubmit={upload}><textarea name="text" placeholder="写点什么，或者只选择文件…"/><div className="composer-actions"><label className="file-picker"><Plus size={18}/>选择文件<input type="file" name="files" multiple /></label><select name="ttl_days" defaultValue="1"><option value="1">保留 1 天</option><option value="3">保留 3 天</option><option value="7">保留 7 天</option></select><button className="primary-button compact" disabled={busy}><Upload size={17}/>{busy ? '上传中' : '发送'}</button></div></form>{error && <p className="form-error">{error}</p>}<section className="drop-list">{items.length === 0 && <div className="empty"><HardDriveUpload/><strong>这里还是空的</strong><span>从任意设备发送第一条内容。</span></div>}{items.map((item) => <article className="drop-item" key={item.id}><div className="item-meta"><span>{new Date(item.created_at).toLocaleString()}</span><span>· {new Date(item.expires_at).toLocaleDateString()} 过期</span></div>{item.text && <p className="drop-text">{item.text}</p>}<div className="attachments">{item.attachments.map((file) => <a key={file.id} href={`/drop/v1/attachments/${file.id}`} target="_blank" rel="noreferrer">{file.media_type.startsWith('image/') ? <img src={`/drop/v1/attachments/${file.id}`} alt={file.original_name}/> : <span className="file-icon"><FileText/></span>}<span><strong>{file.original_name}</strong><small>{formatBytes(file.size)}</small></span><ChevronRight/></a>)}</div><button className="delete-item" onClick={() => drop.remove(item.id).then(load)}><Trash2 size={16}/>删除</button></article>)}</section></div>;
}

function Shares() {
  const [shares, setShares] = useState<any[]>([]); const [hours, setHours] = useState(24); const [created, setCreated] = useState('');
  const load = () => iam.shares().then((x) => setShares(x.shares)); useEffect(() => { load(); }, []);
  async function create() { const share = await iam.createShare(hours); const url = `https://zlx2.com/#share=${encodeURIComponent(share.token)}&path=${encodeURIComponent('/drop')}`; setCreated(url); await navigator.clipboard.writeText(url).catch(() => {}); await load(); }
  return <div className="content-wrap narrow"><section className="page-title"><span className="eyebrow">DIRECT CAPABILITY</span><h1>快捷分享</h1><p>链接打开即用，不注册、不绑定。访客只能查看与下载 Drop 内容。</p></section><div className="share-builder"><select value={hours} onChange={(e) => setHours(Number(e.target.value))}><option value={1}>1 小时</option><option value={24}>1 天</option><option value={168}>7 天</option></select><button className="primary-button compact" onClick={create}><Share2 size={17}/>生成 Drop 链接</button></div>{created && <div className="created-link"><Check/><input readOnly value={created}/><button onClick={() => navigator.clipboard.writeText(created)}><Clipboard/></button></div>}<div className="share-list">{shares.map((share) => <article key={share.id}><div><strong>Drop · 只读</strong><small>{share.revoked_at ? '已撤销' : `${new Date(share.expires_at).toLocaleString()} 过期`}</small></div>{!share.revoked_at && <button onClick={() => iam.revokeShare(share.id).then(load)}>撤销</button>}</article>)}</div></div>;
}

function Security() {
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
      await iam.finishPasskeyRegistration(credential, 'Bitwarden Passkey');
      await load();
    } catch (cause) { setError(message(cause)); } finally { setBusy(false); }
  }

  return <div className="content-wrap narrow"><section className="page-title"><span className="eyebrow">ACCOUNT SECURITY</span><h1>登录与通行密钥</h1><p>把通行密钥保存到 Bitwarden 后，手机和电脑都可以直接确认登录。密码与动态验证码继续作为恢复方式。</p></section><section className="security-card"><div><span className="security-icon"><KeyRound/></span><h2>通行密钥</h2><p>只保存公钥凭据，Bitwarden 中的私钥不会发送到 HomeHub。</p></div><button className="primary-button compact" onClick={register} disabled={busy}><Plus size={17}/>{busy ? '正在创建…' : '添加通行密钥'}</button></section>{error && <p className="form-error">{error}</p>}<div className="passkey-list">{passkeys.length === 0 && <div className="empty"><KeyRound/><strong>还没有通行密钥</strong><span>添加后即可免输密码登录。</span></div>}{passkeys.map((passkey) => <article key={passkey.id}><div><strong>{passkey.name}</strong><small>创建于 {new Date(passkey.created_at).toLocaleString()}{passkey.last_used_at ? ` · 最近使用 ${new Date(passkey.last_used_at).toLocaleString()}` : ''}</small></div><button onClick={() => iam.deletePasskey(passkey.id).then(load)}><Trash2 size={16}/>删除</button></article>)}</div></div>;
}

function formatBytes(size: number) { if (size < 1024) return `${size} B`; if (size < 1024 ** 2) return `${(size / 1024).toFixed(1)} KB`; return `${(size / 1024 ** 2).toFixed(1)} MB`; }

export function App() {
  const [state, setState] = useState<SessionState>(); const [path, setPath] = useState(location.pathname); const [error, setError] = useState('');
  const reload = async () => setState(await iam.session());
  useEffect(() => { const hash = new URLSearchParams(location.hash.slice(1)); const token = hash.get('share'); if (token) { iam.redeem(token).then(() => { history.replaceState({}, '', hash.get('path') || '/drop'); setPath(location.pathname); return reload(); }).catch((e) => { setError(message(e)); reload(); }); } else reload(); const pop = () => setPath(location.pathname); addEventListener('popstate', pop); return () => removeEventListener('popstate', pop); }, []);
  function navigate(next: string) { history.pushState({}, '', next); setPath(next); }
  if (!state) return <div className="loading"><span className="brand-mark">H</span><i/></div>;
  if (!state.authenticated) return <><Auth state={state} reload={reload}/>{error && <div className="toast">{error}</div>}</>;
  const actualPath = !state.administrator && path !== '/drop' ? '/drop' : path;
  return <Layout state={state} path={actualPath} navigate={navigate} logout={() => iam.logout().finally(reload)}>{actualPath === '/drop' ? <Drop/> : actualPath === '/shares' ? <Shares/> : actualPath === '/security' ? <Security/> : <Overview name={state.principal?.display_name ?? 'Luna'}/>}</Layout>;
}
