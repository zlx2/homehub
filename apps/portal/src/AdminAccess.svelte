<script lang="ts">
  import { onMount } from 'svelte';

  type Service = { id: string; name: string; visibility: 'owner' | 'shared' | 'internal'; share_enabled: boolean };
  type Invitation = { id: string; service_ids: string[]; expires_at: string; consumed_at?: string; revoked_at?: string; created_at: string };

  export let services: Service[];

  let invitations: Invitation[] = [];
  let selectedServices: string[] = [];
  let linkHours = '24';
  let shareLink = '';
  let panelError = '';
  let notice = '';
  let loading = true;
  let submitting = false;
  $: shareableServices = services.filter((service) => service.visibility === 'shared' && service.share_enabled);

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
    loading = true;
    panelError = '';
    try {
      const data = await api('/api/v1/admin/invitations');
      invitations = data.invitations;
    } catch (cause) {
      panelError = cause instanceof Error ? cause.message : '无法读取分享链接';
    } finally {
      loading = false;
    }
  }

  async function createShareLink() {
    if (!selectedServices.length) return;
    submitting = true;
    panelError = '';
    notice = '';
    shareLink = '';
    try {
      const expiresAt = new Date(Date.now() + Number(linkHours) * 60 * 60 * 1000).toISOString();
      const created = await api('/api/v1/admin/invitations', mutation('POST', { service_ids: selectedServices, expires_at: expiresAt }));
      shareLink = `${location.origin}/#share=${created.token}`;
      notice = '分享链接已生成。对方打开后会直接进入，无需注册或绑定验证器。';
      selectedServices = [];
      await load();
    } catch (cause) {
      panelError = cause instanceof Error ? cause.message : '创建分享链接失败';
    } finally {
      submitting = false;
    }
  }

  async function copyShareLink() {
    if (!shareLink) return;
    await navigator.clipboard.writeText(shareLink);
    notice = '分享链接已复制。';
  }

  async function revoke(id: string) {
    submitting = true;
    panelError = '';
    try {
      await api(`/api/v1/admin/invitations/${id}`, mutation('DELETE'));
      notice = '分享链接及其现有访客会话已撤销。';
      await load();
    } catch (cause) {
      panelError = cause instanceof Error ? cause.message : '撤销分享链接失败';
    } finally {
      submitting = false;
    }
  }

  function formatDate(value: string) {
    return new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(new Date(value));
  }

  function state(invitation: Invitation) {
    if (invitation.revoked_at) return '已撤销';
    if (new Date(invitation.expires_at) <= new Date()) return '已过期';
    return `${invitation.consumed_at ? '已有人打开' : '尚未打开'} · 有效至 ${formatDate(invitation.expires_at)}`;
  }

  function active(invitation: Invitation) {
    return !invitation.revoked_at && new Date(invitation.expires_at) > new Date();
  }

  onMount(load);
</script>

<section class="access-panel">
  <div class="section-heading admin-heading"><div><p class="kicker">LINK ACCESS</p><h2>分享链接</h2></div><button class="refresh" type="button" on:click={load}>刷新</button></div>
  {#if panelError}<div class="error-panel"><strong>分享操作失败</strong><span>{panelError}</span></div>{/if}
  {#if notice}<aside class="notice secure"><span class="notice-dot"></span>{notice}</aside>{/if}

  <div class="admin-grid">
    <article class="admin-card">
      <p class="kicker">NEW SHARE LINK</p><h3>生成直接访问链接</h3>
      <p class="admin-copy">选择允许访问的服务和有效期。拿到链接的人打开即可使用，不需要账号、密码或 TOTP。</p>
      <form on:submit|preventDefault={createShareLink}>
        <fieldset><legend>允许访问</legend>
          {#if shareableServices.length}
            {#each shareableServices as service}<label class="check-row"><input type="checkbox" value={service.id} bind:group={selectedServices} /><span>{service.name}</span></label>{/each}
          {:else}<p class="empty-copy">当前没有允许分享的业务服务。</p>{/if}
        </fieldset>
        <label>有效期<select bind:value={linkHours}><option value="1">1 小时</option><option value="6">6 小时</option><option value="24">24 小时</option><option value="72">3 天</option><option value="168">7 天</option></select></label>
        <button class="primary compact" disabled={submitting || !selectedServices.length}>{submitting ? '正在生成…' : '生成分享链接'}</button>
      </form>
      {#if shareLink}<div class="secret-result"><label>链接仅在此处显示<input value={shareLink} readonly /></label><button class="refresh" type="button" on:click={copyShareLink}>复制链接</button></div>{/if}
    </article>
  </div>

  <div class="policy-lists">
    <article class="policy-card"><div class="list-title"><h3>分享记录</h3><span>{invitations.length}</span></div>
      {#if loading}<p class="empty-copy">正在读取…</p>
      {:else if invitations.length}<div class="rows">{#each invitations as invitation}<div class="policy-row"><div><strong>{invitation.service_ids.join('、')}</strong><span>{state(invitation)}</span></div>{#if active(invitation)}<button class="danger-button" type="button" disabled={submitting} on:click={() => revoke(invitation.id)}>撤销</button>{/if}</div>{/each}</div>
      {:else}<p class="empty-copy">还没有分享链接</p>{/if}
    </article>
  </div>
</section>
