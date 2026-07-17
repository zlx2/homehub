<script lang="ts">
  import { onMount } from 'svelte';

  type Health = {
    status: 'healthy' | 'unhealthy' | 'unknown';
    checked_at?: string;
    latency_ms?: number;
    message?: string;
  };

  type Service = {
    id: string;
    name: string;
    description: string;
    icon: string;
    route?: string;
    visibility: 'owner' | 'shared' | 'internal';
    share_enabled: boolean;
    health: Health;
  };

  type SystemInfo = {
    name: string;
    version: string;
    commit: string;
    environment: string;
    auth_enabled: boolean;
    time: string;
  };

  let services: Service[] = [];
  let system: SystemInfo | null = null;
  let loading = true;
  let error = '';
  let lastUpdated: Date | null = null;

  async function refresh() {
    try {
      const [systemResponse, servicesResponse] = await Promise.all([
        fetch('/api/v1/system', { cache: 'no-store' }),
        fetch('/api/v1/services', { cache: 'no-store' })
      ]);
      if (!systemResponse.ok || !servicesResponse.ok) {
        throw new Error(`API返回异常：${systemResponse.status}/${servicesResponse.status}`);
      }
      system = await systemResponse.json();
      const serviceData = await servicesResponse.json();
      services = serviceData.services;
      lastUpdated = new Date();
      error = '';
    } catch (cause) {
      error = cause instanceof Error ? cause.message : '无法连接HomeHub Control';
    } finally {
      loading = false;
    }
  }

  function healthLabel(status: Health['status']) {
    if (status === 'healthy') return '运行正常';
    if (status === 'unhealthy') return '服务异常';
    return '等待探测';
  }

  function visibilityLabel(visibility: Service['visibility']) {
    if (visibility === 'owner') return '仅自己';
    if (visibility === 'shared') return '可分享';
    return '内部组件';
  }

  function checkedAt(value?: string) {
    if (!value) return '尚未检查';
    return new Intl.DateTimeFormat('zh-CN', {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit'
    }).format(new Date(value));
  }

  onMount(() => {
    refresh();
    const timer = window.setInterval(refresh, 5000);
    return () => window.clearInterval(timer);
  });
</script>

<svelte:head>
  <meta
    name="description"
    content="HomeHub personal service control plane and health dashboard"
  />
</svelte:head>

<main class="shell">
  <header class="topbar">
    <div class="brand">
      <div class="brand-mark" aria-hidden="true">H</div>
      <div>
        <p class="eyebrow">PERSONAL SERVICE FABRIC</p>
        <h1>HomeHub</h1>
      </div>
    </div>

    <div class="header-meta">
      {#if system}
        <span class="environment">{system.environment}</span>
        <span>v{system.version}</span>
      {/if}
      <button class="refresh" type="button" on:click={refresh} aria-label="立即刷新">刷新</button>
    </div>
  </header>

  <section class="hero">
    <div>
      <p class="kicker">CONTROL PLANE</p>
      <h2>你的服务，保持清醒地运转。</h2>
      <p class="hero-copy">
        一个入口查看服务状态、访问范围和分享能力。统一鉴权将在下一阶段接入。
      </p>
    </div>

    <div class="summary" aria-label="系统概览">
      <div>
        <strong>{services.filter((service) => service.health.status === 'healthy').length}</strong>
        <span>健康服务</span>
      </div>
      <div>
        <strong>{services.length}</strong>
        <span>已登记</span>
      </div>
      <div>
        <strong>{services.filter((service) => service.share_enabled).length}</strong>
        <span>允许分享</span>
      </div>
    </div>
  </section>

  {#if system && !system.auth_enabled}
    <aside class="notice">
      <span class="notice-dot"></span>
      当前为本机开发入口，统一鉴权尚未启用，不得切换到公网443。
    </aside>
  {/if}

  <section class="section-heading">
    <div>
      <p class="kicker">SERVICE DIRECTORY</p>
      <h2>服务目录</h2>
    </div>
    <p>{lastUpdated ? `更新于 ${lastUpdated.toLocaleTimeString('zh-CN')}` : '正在读取状态'}</p>
  </section>

  {#if error}
    <div class="error-panel">
      <strong>状态读取失败</strong>
      <span>{error}</span>
    </div>
  {/if}

  {#if loading}
    <div class="service-grid" aria-label="加载中">
      {#each [1, 2] as item}
        <div class="service-card skeleton" aria-hidden="true" data-item={item}></div>
      {/each}
    </div>
  {:else}
    <div class="service-grid">
      {#each services as service}
        <article class="service-card" class:unhealthy={service.health.status === 'unhealthy'}>
          <div class="card-top">
            <div class="service-icon" aria-hidden="true">{service.icon || '◇'}</div>
            <div class="health" data-status={service.health.status}>
              <span></span>
              {healthLabel(service.health.status)}
            </div>
          </div>

          <div class="service-main">
            <p class="service-id">{service.id}</p>
            <h3>{service.name}</h3>
            <p>{service.description}</p>
          </div>

          <dl class="service-meta">
            <div>
              <dt>访问范围</dt>
              <dd>{visibilityLabel(service.visibility)}</dd>
            </div>
            <div>
              <dt>探测延迟</dt>
              <dd>{service.health.latency_ms ?? 0} ms</dd>
            </div>
            <div>
              <dt>最后检查</dt>
              <dd>{checkedAt(service.health.checked_at)}</dd>
            </div>
          </dl>

          <footer class="card-footer">
            <span class:enabled={service.share_enabled}>
              {service.share_enabled ? '允许分享' : '禁止分享'}
            </span>
            {#if service.route}
              <a href={service.route}>打开服务 <span aria-hidden="true">↗</span></a>
            {:else}
              <span class="no-route">内部组件</span>
            {/if}
          </footer>
        </article>
      {/each}
    </div>
  {/if}
</main>
