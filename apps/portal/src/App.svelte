<script lang="ts">
  import { onMount } from 'svelte';
  import AdminAccess from './AdminAccess.svelte';
  import InviteEnrollment from './InviteEnrollment.svelte';

  type Health = { status: 'healthy' | 'unhealthy' | 'unknown'; checked_at?: string; latency_ms?: number };
  type Service = { id: string; name: string; description: string; icon: string; route?: string; visibility: 'owner' | 'shared' | 'internal'; share_enabled: boolean; health: Health };
  type SystemInfo = { name: string; version: string; commit: string; environment: string; auth_enabled: boolean; time: string };
  type Principal = { id: string; username: string; display_name: string; scopes: string[] };
  type SetupResult = { setup_id: string; manual_secret: string; provisioning_uri: string; qr_data_url: string; expires_at: string };

  let services: Service[] = [];
  let system: SystemInfo | null = null;
  let principal: Principal | null = null;
  let setupRequired = false;
  let authLoading = true;
  let loading = false;
  let error = '';
  let authError = '';
  let lastUpdated: Date | null = null;
  let bootstrapToken = '';
  let username = '';
  let password = '';
  let confirmPassword = '';
  let totpCode = '';
  let setup: SetupResult | null = null;
  let submitting = false;
  let inviteToken = '';

  async function api(path: string, init?: RequestInit) {
    const response = await fetch(path, { cache: 'no-store', ...init });
    const body = response.status === 204 ? null : await response.json().catch(() => null);
    if (!response.ok) {
      const message = body?.message || body?.error || `请求失败（${response.status}）`;
      throw new Error(message);
    }
    return body;
  }

  async function loadSession() {
    authLoading = true;
    try {
      const state = await api('/api/v1/auth/session');
      principal = state.authenticated ? state.principal : null;
      setupRequired = state.setup_required;
      if (principal) await refresh();
    } catch (cause) {
      authError = cause instanceof Error ? cause.message : '无法读取登录状态';
    } finally {
      authLoading = false;
    }
  }

  async function refresh() {
    if (!principal) return;
    loading = services.length === 0;
    try {
      const [systemData, serviceData] = await Promise.all([api('/api/v1/system'), api('/api/v1/services')]);
      system = systemData;
      services = serviceData.services;
      lastUpdated = new Date();
      error = '';
    } catch (cause) {
      error = cause instanceof Error ? cause.message : '无法连接 HomeHub Control';
      if (error === 'authentication_required') await loadSession();
    } finally {
      loading = false;
    }
  }

  async function beginSetup() {
    authError = '';
    if (password !== confirmPassword) { authError = '两次输入的密码不一致'; return; }
    submitting = true;
    try {
      setup = await api('/api/v1/setup/begin', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ bootstrap_token: bootstrapToken, username, password })
      });
      password = ''; confirmPassword = ''; bootstrapToken = '';
    } catch (cause) {
      authError = authMessage(cause);
    } finally { submitting = false; }
  }

  async function confirmSetup() {
    if (!setup) return;
    authError = ''; submitting = true;
    try {
      const result = await api('/api/v1/setup/confirm', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ setup_id: setup.setup_id, totp_code: totpCode })
      });
      principal = result.principal; setupRequired = false; setup = null; totpCode = '';
      await refresh();
    } catch (cause) { authError = authMessage(cause); }
    finally { submitting = false; }
  }

  async function login() {
    authError = ''; submitting = true;
    try {
      const result = await api('/api/v1/auth/login', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password, totp_code: totpCode })
      });
      principal = result.principal; password = ''; totpCode = '';
      await refresh();
    } catch (cause) { authError = authMessage(cause); }
    finally { submitting = false; }
  }

  async function logout() {
    try {
      await api('/api/v1/auth/logout', { method: 'POST', headers: { 'X-CSRF-Token': cookie('homehub_csrf') || cookie('__Host-homehub_csrf') } });
    } finally {
      principal = null; services = []; system = null; await loadSession();
    }
  }

  async function completeInvitation(enrolled: Principal) {
    principal = enrolled;
    inviteToken = '';
    setupRequired = false;
    await refresh();
  }

  function cookie(name: string) {
    const prefix = `${encodeURIComponent(name)}=`;
    return document.cookie.split('; ').find((value) => value.startsWith(prefix))?.slice(prefix.length) || '';
  }
  function authMessage(cause: unknown) {
    const value = cause instanceof Error ? cause.message : '请求失败';
    const labels: Record<string, string> = {
      invalid_bootstrap_token: '初始化令牌无效或已过期', invalid_totp: '动态验证码不正确',
      invalid_credentials: '用户名、密码或动态验证码不正确', rate_limited: '尝试次数过多，请 15 分钟后再试',
      setup_unavailable: '初始化已完成或本次初始化已过期', invalid_origin: '请求来源不受信任'
    };
    return labels[value] || value;
  }
  function healthLabel(status: Health['status']) { return status === 'healthy' ? '运行正常' : status === 'unhealthy' ? '服务异常' : '等待探测'; }
  function visibilityLabel(value: Service['visibility']) { return value === 'owner' ? '仅自己' : value === 'shared' ? '可分享' : '内部组件'; }
  function checkedAt(value?: string) { return value ? new Intl.DateTimeFormat('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' }).format(new Date(value)) : '尚未检查'; }

  onMount(() => {
    const match = location.hash.match(/^#invite=([A-Za-z0-9_-]{32,})$/);
    inviteToken = match?.[1] || '';
    loadSession();
    const timer = window.setInterval(() => { if (principal) refresh(); }, 5000);
    return () => window.clearInterval(timer);
  });
</script>

<svelte:head><meta name="description" content="HomeHub personal service control plane" /></svelte:head>

{#if authLoading}
  <main class="auth-shell"><div class="auth-card loading-card"><div class="brand-mark">H</div><p>正在建立安全会话…</p></div></main>
{:else if !principal}
  {#if inviteToken}
    <InviteEnrollment token={inviteToken} onComplete={completeInvitation} />
  {:else}<main class="auth-shell">
    <section class="auth-card">
      <div class="auth-brand"><div class="brand-mark">H</div><div><p class="eyebrow">PERSONAL SERVICE FABRIC</p><h1>HomeHub</h1></div></div>
      {#if setupRequired}
        {#if setup}
          <div class="auth-copy"><p class="kicker">TWO-FACTOR SETUP</p><h2>绑定动态验证码</h2><p>用 Bitwarden 扫描二维码，或手动输入密钥。验证成功后初始化才会完成。</p></div>
          <div class="totp-setup"><img src={setup.qr_data_url} alt="HomeHub TOTP 二维码" /><code>{setup.manual_secret}</code></div>
          <form on:submit|preventDefault={confirmSetup}>
            <label>6 位动态验证码<input bind:value={totpCode} inputmode="numeric" autocomplete="one-time-code" minlength="6" maxlength="6" required /></label>
            {#if authError}<p class="auth-error">{authError}</p>{/if}
            <button class="primary" disabled={submitting}>{submitting ? '正在验证…' : '完成初始化并登录'}</button>
          </form>
        {:else}
          <div class="auth-copy"><p class="kicker">OWNER BOOTSTRAP</p><h2>初始化所有者</h2><p>初始化令牌只在服务器本地生成，有效期 24 小时且仅可使用一次。</p></div>
          <form on:submit|preventDefault={beginSetup}>
            <label>初始化令牌<input bind:value={bootstrapToken} type="password" autocomplete="off" required /></label>
            <label>用户名<input bind:value={username} autocomplete="username" pattern={'[a-zA-Z0-9_.-]{3,64}'} required /></label>
            <label>密码<input bind:value={password} type="password" autocomplete="new-password" minlength="12" required /></label>
            <label>确认密码<input bind:value={confirmPassword} type="password" autocomplete="new-password" minlength="12" required /></label>
            {#if authError}<p class="auth-error">{authError}</p>{/if}
            <button class="primary" disabled={submitting}>{submitting ? '正在创建…' : '继续绑定动态验证码'}</button>
          </form>
        {/if}
      {:else}
        <div class="auth-copy"><p class="kicker">SECURE SIGN IN</p><h2>欢迎回来</h2><p>使用你的账号密码和 Bitwarden 中的动态验证码登录。</p></div>
        <form on:submit|preventDefault={login}>
          <label>用户名<input bind:value={username} autocomplete="username" required /></label>
          <label>密码<input bind:value={password} type="password" autocomplete="current-password" required /></label>
          <label>6 位动态验证码<input bind:value={totpCode} inputmode="numeric" autocomplete="one-time-code" minlength="6" maxlength="6" required /></label>
          {#if authError}<p class="auth-error">{authError}</p>{/if}
          <button class="primary" disabled={submitting}>{submitting ? '正在登录…' : '登录 HomeHub'}</button>
        </form>
      {/if}
    </section>
  </main>{/if}
{:else}
  <main class="shell">
    <header class="topbar">
      <div class="brand"><div class="brand-mark">H</div><div><p class="eyebrow">PERSONAL SERVICE FABRIC</p><h1>HomeHub</h1></div></div>
      <div class="header-meta">{#if system}<span class="environment">{system.environment}</span><span>v{system.version}</span>{/if}<span>{principal.username}</span><button class="refresh" on:click={refresh}>刷新</button><button class="refresh" on:click={logout}>退出</button></div>
    </header>
    <section class="hero"><div><p class="kicker">CONTROL PLANE</p><h2>你的服务，保持清醒地运转。</h2><p class="hero-copy">统一入口管理服务状态与访问边界。目录会根据当前账号的有效授权自动过滤。</p></div>
      <div class="summary"><div><strong>{services.filter((s) => s.health.status === 'healthy').length}</strong><span>健康服务</span></div><div><strong>{services.length}</strong><span>已登记</span></div><div><strong>{services.filter((s) => s.share_enabled).length}</strong><span>允许分享</span></div></div>
    </section>
    <aside class="notice secure"><span class="notice-dot"></span>{principal.scopes.includes('admin') ? '管理员' : '朋友'}会话已启用：密码 + TOTP，闲置 12 小时后过期。</aside>
    <section class="section-heading"><div><p class="kicker">SERVICE DIRECTORY</p><h2>服务目录</h2></div><p>{lastUpdated ? `更新于 ${lastUpdated.toLocaleTimeString('zh-CN')}` : '正在读取状态'}</p></section>
    {#if error}<div class="error-panel"><strong>状态读取失败</strong><span>{error}</span></div>{/if}
    {#if loading}<div class="service-grid"><div class="service-card skeleton"></div><div class="service-card skeleton"></div></div>
    {:else if services.length}<div class="service-grid">{#each services as service}<article class="service-card" class:unhealthy={service.health.status === 'unhealthy'}>
      <div class="card-top"><div class="service-icon">{service.icon || '◇'}</div><div class="health" data-status={service.health.status}><span></span>{healthLabel(service.health.status)}</div></div>
      <div class="service-main"><p class="service-id">{service.id}</p><h3>{service.name}</h3><p>{service.description}</p></div>
      <dl class="service-meta"><div><dt>访问范围</dt><dd>{visibilityLabel(service.visibility)}</dd></div><div><dt>探测延迟</dt><dd>{service.health.latency_ms ?? 0} ms</dd></div><div><dt>最后检查</dt><dd>{checkedAt(service.health.checked_at)}</dd></div></dl>
      <footer class="card-footer"><span class:enabled={service.share_enabled}>{service.share_enabled ? '允许分享' : '禁止分享'}</span>{#if service.route}<a href={service.route}>打开服务 ↗</a>{:else}<span class="no-route">内部组件</span>{/if}</footer>
    </article>{/each}</div>{:else}<div class="empty-state"><strong>暂时没有可访问的服务</strong><p>账号已经生效，但管理员还没有为你分配业务服务。</p></div>{/if}
    {#if principal.scopes.includes('admin')}<AdminAccess {services} />{/if}
  </main>
{/if}
