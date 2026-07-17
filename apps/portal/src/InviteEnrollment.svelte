<script lang="ts">
  type Principal = { id: string; username: string; display_name: string; scopes: string[] };
  type SetupResult = { setup_id: string; manual_secret: string; provisioning_uri: string; qr_data_url: string; expires_at: string };

  export let token: string;
  export let onComplete: (principal: Principal) => Promise<void> | void;

  let username = '';
  let password = '';
  let confirmPassword = '';
  let totpCode = '';
  let setup: SetupResult | null = null;
  let error = '';
  let submitting = false;

  async function request(path: string, body: unknown) {
    const response = await fetch(path, {
      method: 'POST', cache: 'no-store', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    });
    const result = await response.json().catch(() => null);
    if (!response.ok) throw new Error(result?.message || result?.error || `请求失败（${response.status}）`);
    return result;
  }

  async function begin() {
    error = '';
    if (password !== confirmPassword) { error = '两次输入的密码不一致'; return; }
    submitting = true;
    try {
      setup = await request('/api/v1/invitations/begin', { token, username, password });
      password = ''; confirmPassword = '';
      history.replaceState(null, '', `${location.pathname}${location.search}`);
    } catch (cause) {
      error = message(cause);
    } finally { submitting = false; }
  }

  async function confirm() {
    if (!setup) return;
    error = ''; submitting = true;
    try {
      const result = await request('/api/v1/invitations/confirm', { setup_id: setup.setup_id, totp_code: totpCode });
      await onComplete(result.principal);
    } catch (cause) {
      error = message(cause);
    } finally { submitting = false; }
  }

  function message(cause: unknown) {
    const value = cause instanceof Error ? cause.message : '请求失败';
    const labels: Record<string, string> = {
      invalid_invitation: '邀请无效、已撤销或已经过期',
      invitation_already_claimed: '该邀请正在注册中；若刚才中断，请等待 15 分钟后重试',
      username_unavailable: '这个用户名已经被使用',
      invalid_totp: '动态验证码不正确',
      invitation_unavailable: '本次注册已过期或不可用',
      invalid_origin: '请求来源不受信任'
    };
    return labels[value] || value;
  }
</script>

<main class="auth-shell">
  <section class="auth-card">
    <div class="auth-brand"><div class="brand-mark">H</div><div><p class="eyebrow">PRIVATE INVITATION</p><h1>HomeHub</h1></div></div>
    {#if setup}
      <div class="auth-copy"><p class="kicker">TWO-FACTOR SETUP</p><h2>绑定动态验证码</h2><p>用 Bitwarden 扫描二维码。动态验证码验证成功后，朋友账号和服务权限才会正式创建。</p></div>
      <div class="totp-setup"><img src={setup.qr_data_url} alt="HomeHub TOTP 二维码" /><code>{setup.manual_secret}</code></div>
      <form on:submit|preventDefault={confirm}>
        <label>6 位动态验证码<input bind:value={totpCode} inputmode="numeric" autocomplete="one-time-code" pattern="[0-9]{6}" minlength="6" maxlength="6" required /></label>
        {#if error}<p class="auth-error">{error}</p>{/if}
        <button class="primary" disabled={submitting}>{submitting ? '正在验证…' : '完成注册并登录'}</button>
      </form>
    {:else}
      <div class="auth-copy"><p class="kicker">FRIEND ENROLLMENT</p><h2>接受私人邀请</h2><p>创建你自己的登录信息。密码至少 12 位，下一步还需要绑定 Bitwarden 动态验证码。</p></div>
      <form on:submit|preventDefault={begin}>
        <label>用户名<input bind:value={username} autocomplete="username" pattern={'[a-zA-Z0-9_.-]{3,64}'} required /></label>
        <label>密码<input bind:value={password} type="password" autocomplete="new-password" minlength="12" maxlength="256" required /></label>
        <label>确认密码<input bind:value={confirmPassword} type="password" autocomplete="new-password" minlength="12" maxlength="256" required /></label>
        {#if error}<p class="auth-error">{error}</p>{/if}
        <button class="primary" disabled={submitting}>{submitting ? '正在验证邀请…' : '继续绑定动态验证码'}</button>
      </form>
    {/if}
  </section>
</main>
