<script lang="ts">
  import { onMount } from 'svelte';

  type Service = { id: string; name: string; visibility: 'owner' | 'shared' | 'internal'; share_enabled: boolean };
  type Principal = { id: string; username: string; display_name: string; status: string; scopes: string[]; created_at: string };
  type Grant = { id: string; principal_id: string; username: string; service_id: string; expires_at?: string; created_at: string };
  type Invitation = { id: string; service_ids: string[]; expires_at: string; consumed_at?: string; revoked_at?: string; created_at: string };

  export let services: Service[];

  let principals: Principal[] = [];
  let grants: Grant[] = [];
  let invitations: Invitation[] = [];
  let selectedInviteServices: string[] = [];
  let inviteHours = '24';
  let inviteLink = '';
  let selectedPrincipal = '';
  let selectedService = '';
  let panelError = '';
  let notice = '';
  let loading = true;
  let submitting = false;
  $: shareableServices = services.filter((service) => service.visibility === 'shared' && service.share_enabled);
  $: friends = principals.filter((principal) => !principal.scopes.includes('admin'));

  async function api(path: string, init?: RequestInit) {
    const response = await fetch(path, { cache: 'no-store', ...init });
    const body = response.status === 204 ? null : await response.json().catch(() => null);
    if (!response.ok) throw new Error(body?.message || body?.error || `请求失败（${response.status}）`);
    return body;
  }

  function cookie(name: string) {
    const prefix = `${encodeURIComponent(name)}=`;
    return document.cookie.split('; ').find((value) => value.startsWith(prefix))?.slice(prefix.length) || '';
  }

  function mutation(method: string, body?: unknown): RequestInit {
    return {
      method,
      headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': cookie('homehub_csrf') || cookie('__Host-homehub_csrf') },
      ...(body === undefined ? {} : { body: JSON.stringify(body) })
    };
  }

  async function load() {
    loading = true; panelError = '';
    try {
      const [principalData, grantData, invitationData] = await Promise.all([
        api('/api/v1/admin/principals'), api('/api/v1/admin/service-grants'), api('/api/v1/admin/invitations')
      ]);
      principals = principalData.principals; grants = grantData.grants; invitations = invitationData.invitations;
      const availableFriends = principals.filter((principal) => !principal.scopes.includes('admin'));
      const availableServices = services.filter((service) => service.visibility === 'shared' && service.share_enabled);
      if (!selectedPrincipal && availableFriends.length) selectedPrincipal = availableFriends[0].id;
      if (!selectedService && availableServices.length) selectedService = availableServices[0].id;
    } catch (cause) { panelError = cause instanceof Error ? cause.message : '无法读取访问策略'; }
    finally { loading = false; }
  }

  async function createInvitation() {
    submitting = true; panelError = ''; notice = ''; inviteLink = '';
    try {
      const expiresAt = new Date(Date.now() + Number(inviteHours) * 60 * 60 * 1000).toISOString();
      const created = await api('/api/v1/admin/invitations', mutation('POST', { service_ids: selectedInviteServices, expires_at: expiresAt }));
      inviteLink = `${location.origin}/#invite=${created.token}`;
      notice = '邀请已创建。原始链接只显示这一次，请立即复制。';
      selectedInviteServices = [];
      await load();
    } catch (cause) { panelError = cause instanceof Error ? cause.message : '创建邀请失败'; }
    finally { submitting = false; }
  }

  async function copyInvite() {
    if (!inviteLink) return;
    await navigator.clipboard.writeText(inviteLink);
    notice = '邀请链接已复制到剪贴板。';
  }

  async function revokeInvitation(id: string) {
    submitting = true; panelError = '';
    try { await api(`/api/v1/admin/invitations/${id}`, mutation('DELETE')); await load(); }
    catch (cause) { panelError = cause instanceof Error ? cause.message : '撤销邀请失败'; }
    finally { submitting = false; }
  }

  async function createGrant() {
    if (!selectedPrincipal || !selectedService) return;
    submitting = true; panelError = '';
    try {
      await api('/api/v1/admin/service-grants', mutation('POST', { principal_id: selectedPrincipal, service_id: selectedService, expires_at: null }));
      notice = '服务权限已更新。'; await load();
    } catch (cause) { panelError = cause instanceof Error ? cause.message : '授权失败'; }
    finally { submitting = false; }
  }

  async function revokeGrant(id: string) {
    submitting = true; panelError = '';
    try { await api(`/api/v1/admin/service-grants/${id}`, mutation('DELETE')); await load(); }
    catch (cause) { panelError = cause instanceof Error ? cause.message : '撤销权限失败'; }
    finally { submitting = false; }
  }

  function formatDate(value?: string) {
    return value ? new Intl.DateTimeFormat('zh-CN', { year: '2-digit', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(new Date(value)) : '永久';
  }

  onMount(load);
</script>

<section class="access-panel">
  <div class="section-heading admin-heading"><div><p class="kicker">ACCESS CONTROL</p><h2>朋友与授权</h2></div><button class="refresh" type="button" on:click={load}>刷新权限</button></div>
  {#if panelError}<div class="error-panel"><strong>访问策略操作失败</strong><span>{panelError}</span></div>{/if}
  {#if notice}<aside class="notice secure"><span class="notice-dot"></span>{notice}</aside>{/if}

  <div class="admin-grid">
    <article class="admin-card">
      <p class="kicker">NEW INVITATION</p><h3>创建朋友邀请</h3>
      <p class="admin-copy">邀请默认只创建账号。勾选的服务会在朋友完成 TOTP 后自动授权。</p>
      <form on:submit|preventDefault={createInvitation}>
        <fieldset><legend>初始服务</legend>
          {#if shareableServices.length}
            {#each shareableServices as service}<label class="check-row"><input type="checkbox" value={service.id} bind:group={selectedInviteServices} /><span>{service.name}</span></label>{/each}
          {:else}<p class="empty-copy">当前还没有标记为“可分享”的业务服务，可先邀请账号，之后再授权。</p>{/if}
        </fieldset>
        <label>有效期<select bind:value={inviteHours}><option value="24">24 小时</option><option value="72">3 天</option><option value="168">7 天</option></select></label>
        <button class="primary compact" disabled={submitting}>{submitting ? '正在创建…' : '生成一次性邀请'}</button>
      </form>
      {#if inviteLink}<div class="secret-result"><label>仅显示一次<input value={inviteLink} readonly /></label><button class="refresh" type="button" on:click={copyInvite}>复制链接</button></div>{/if}
    </article>

    <article class="admin-card">
      <p class="kicker">DIRECT GRANT</p><h3>调整服务权限</h3>
      <p class="admin-copy">只会列出普通朋友账号和明确允许分享的服务。</p>
      <form on:submit|preventDefault={createGrant}>
        <label>朋友<select bind:value={selectedPrincipal} disabled={!friends.length}>{#each friends as friend}<option value={friend.id}>{friend.username}</option>{/each}</select></label>
        <label>服务<select bind:value={selectedService} disabled={!shareableServices.length}>{#each shareableServices as service}<option value={service.id}>{service.name}</option>{/each}</select></label>
        <button class="primary compact" disabled={submitting || !friends.length || !shareableServices.length}>授予永久访问</button>
      </form>
    </article>
  </div>

  <div class="policy-lists">
    <article class="policy-card"><div class="list-title"><h3>账号</h3><span>{principals.length}</span></div>
      {#if loading}<p class="empty-copy">正在读取…</p>{:else if principals.length}
        <div class="rows">{#each principals as item}<div class="policy-row"><div><strong>{item.username}</strong><span>{item.scopes.includes('admin') ? '管理员' : '朋友账号'} · {item.status}</span></div><time>{formatDate(item.created_at)}</time></div>{/each}</div>
      {:else}<p class="empty-copy">暂无账号</p>{/if}
    </article>

    <article class="policy-card"><div class="list-title"><h3>有效授权</h3><span>{grants.length}</span></div>
      {#if grants.length}<div class="rows">{#each grants as grant}<div class="policy-row"><div><strong>{grant.username} → {grant.service_id}</strong><span>{grant.expires_at ? `有效至 ${formatDate(grant.expires_at)}` : '永久访问'}</span></div><button class="danger-button" type="button" disabled={submitting} on:click={() => revokeGrant(grant.id)}>撤销</button></div>{/each}</div>
      {:else}<p class="empty-copy">暂无朋友服务授权</p>{/if}
    </article>

    <article class="policy-card"><div class="list-title"><h3>邀请记录</h3><span>{invitations.length}</span></div>
      {#if invitations.length}<div class="rows">{#each invitations as invitation}<div class="policy-row"><div><strong>{invitation.service_ids.length ? invitation.service_ids.join('、') : '仅创建账号'}</strong><span>{invitation.consumed_at ? '已使用' : invitation.revoked_at ? '已撤销' : new Date(invitation.expires_at) < new Date() ? '已过期' : `有效至 ${formatDate(invitation.expires_at)}`}</span></div>{#if !invitation.consumed_at && !invitation.revoked_at && new Date(invitation.expires_at) > new Date()}<button class="danger-button" type="button" disabled={submitting} on:click={() => revokeInvitation(invitation.id)}>撤销</button>{/if}</div>{/each}</div>
      {:else}<p class="empty-copy">暂无邀请</p>{/if}
    </article>
  </div>
</section>
