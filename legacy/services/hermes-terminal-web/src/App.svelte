<script lang="ts">
  import { onMount } from 'svelte';
  import { TtydClient, type ConnectionState } from './lib/ttyd';

  let terminalElement: HTMLDivElement;
  let client: TtydClient | undefined;
  let connection: ConnectionState = 'connecting';
  let fontSize = 15;
  let notice = '';
  let noticeTimer: number | undefined;
  let viewportTimer: number | undefined;
  let lastViewportHeight = 0;
  let composerElement: HTMLTextAreaElement;
  let mobileMode = false;
  let composerFocused = false;
  let draft = '';
  let lastPointerSendAt = 0;

  const isiOS = /iPad|iPhone|iPod/.test(navigator.userAgent) ||
    (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1);

  const wsProtocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
  const wsUrl = `${wsProtocol}//${location.host}/hermes/ws`;
  const tokenUrl = `${location.origin}/hermes/token`;

  const connectionLabels: Record<ConnectionState, string> = {
    connecting: '连接中',
    connected: '已连接',
    reconnecting: '重连中',
    disconnected: '已断开'
  };

  function showNotice(message: string) {
    notice = message;
    if (noticeTimer) clearTimeout(noticeTimer);
    noticeTimer = window.setTimeout(() => (notice = ''), 1800);
  }

  function send(data: string) {
    client?.send(data);
  }

  function changeFont(delta: number) {
    fontSize = Math.min(20, Math.max(12, fontSize + delta));
    localStorage.setItem('homehub-hermes-font-size', String(fontSize));
    client?.setFontSize(fontSize);
    showNotice(`字号 ${fontSize}px`);
  }

  function openSessions() {
    // Give Hermes enough time to process the cleared input before submitting
    // the command. A second Enter would immediately select the first session.
    send('\x15');
    window.setTimeout(() => send('/sessions'), 80);
    window.setTimeout(() => send('\r'), 420);
  }

  function sendDraft() {
    // Read from the native control at tap time. During IME composition the DOM
    // value can be newer than the framework binding by one render turn.
    const value = composerElement?.value ?? draft;
    if (!value.trim()) return;
    if (connection !== 'connected') return showNotice('终端尚未连接');

    // Keep IME composition entirely inside the native textarea. Hermes only
    // receives the final committed string, so xterm never sees partial mobile
    // composition events that some keyboards drop.
    draft = '';
    send('\x15');
    window.setTimeout(() => send(value), 50);
    window.setTimeout(() => send('\r'), 100);
    if (mobileMode) requestAnimationFrame(() => composerElement?.focus());
  }

  function handleComposerKeydown(event: KeyboardEvent) {
    if (event.key !== 'Enter' || event.shiftKey || event.isComposing || event.keyCode === 229) return;
    event.preventDefault();
    sendDraft();
  }

  function sendOnPointerDown(event: PointerEvent) {
    // iOS may suppress the later `click` while an IME composition is active.
    // Send on the earliest pointer event and keep the textarea focused so the
    // button never moves between touch-down and touch-up.
    event.preventDefault();
    lastPointerSendAt = performance.now();
    sendDraft();
  }

  function sendOnClick() {
    // Keyboard activation has no pointerdown. A touch/mouse click follows its
    // pointerdown and is ignored here to prevent two interleaved submissions.
    if (performance.now() - lastPointerSendAt > 500) sendDraft();
  }

  function focusInput() {
    if (mobileMode) composerElement?.focus();
    else client?.focus();
  }

  function handleComposerFocus() {
    composerFocused = true;
    updateViewport();
  }

  function handleComposerBlur() {
    composerFocused = false;
    updateViewport();
  }

  async function pasteClipboard() {
    try {
      const value = await navigator.clipboard.readText();
      if (!value) return showNotice('剪贴板为空');
      if (mobileMode) {
        draft += value;
        composerElement?.focus();
        showNotice('已粘贴到输入框');
      } else {
        send(value);
        showNotice('已粘贴');
      }
    } catch {
      focusInput();
      showNotice('请长按输入区粘贴');
    }
  }

  async function copySelection() {
    try {
      showNotice((await client?.copySelection()) ? '已复制选中内容' : '请先选择终端文字');
    } catch {
      showNotice('复制失败');
    }
  }

  async function toggleFullscreen() {
    try {
      if (document.fullscreenElement) await document.exitFullscreen();
      else await document.documentElement.requestFullscreen();
      setTimeout(() => client?.fit(), 80);
    } catch {
      showNotice('当前浏览器不支持全屏');
    }
  }

  function updateViewport() {
    mobileMode = matchMedia('(max-width: 720px), (pointer: coarse)').matches;
    const viewport = window.visualViewport;
    const visualHeight = Math.round(viewport?.height || window.innerHeight);
    const layoutHeight = Math.round(window.innerHeight);
    const viewportGap = layoutHeight - visualHeight;
    // Safari 26 can resize the layout viewport for the keyboard and then
    // subtract its form-assistant row once more from VisualViewport. In that
    // narrow-gap case the layout height is the actual top of the assistant.
    const height = isiOS && composerFocused && viewportGap > 0 && viewportGap < 120
      ? layoutHeight
      : visualHeight;
    const offsetTop = Math.round(viewport?.offsetTop || 0);
    const keyboardOpen = composerFocused || layoutHeight - visualHeight > 120;

    document.documentElement.style.setProperty('--app-offset-top', `${offsetTop}px`);
    document.documentElement.style.setProperty(
      '--bottom-safe-area',
      keyboardOpen ? '0px' : 'env(safe-area-inset-bottom)'
    );
    if (height === lastViewportHeight) return;
    lastViewportHeight = height;
    document.documentElement.style.setProperty('--app-height', `${height}px`);
    if (viewportTimer) clearTimeout(viewportTimer);
    viewportTimer = window.setTimeout(() => client?.fit(), 100);
  }

  onMount(() => {
    const stored = Number(localStorage.getItem('homehub-hermes-font-size'));
    const coarsePointer = matchMedia('(pointer: coarse)').matches;
    fontSize = Number.isFinite(stored) && stored >= 12 && stored <= 20 ? stored : coarsePointer ? 13 : 15;

    client = new TtydClient({ wsUrl, tokenUrl, fontSize, onState: (state) => (connection = state) });
    client.mount(terminalElement);
    void client.connect();
    updateViewport();

    window.addEventListener('resize', updateViewport);
    window.visualViewport?.addEventListener('resize', updateViewport);
    window.visualViewport?.addEventListener('scroll', updateViewport);

    return () => {
      if (noticeTimer) clearTimeout(noticeTimer);
      if (viewportTimer) clearTimeout(viewportTimer);
      window.removeEventListener('resize', updateViewport);
      window.visualViewport?.removeEventListener('resize', updateViewport);
      window.visualViewport?.removeEventListener('scroll', updateViewport);
      client?.dispose();
    };
  });
</script>

<svelte:head>
  <meta name="description" content="HomeHub Hermes native web terminal" />
</svelte:head>

<main class="terminal-app" class:composing={composerFocused}>
  <header class="topbar">
    <a class="icon-button home-button" href="/" aria-label="返回 HomeHub">‹</a>
    <div class="identity">
      <span class="hermes-mark">H</span>
      <span class="title">Hermes</span>
    </div>
    <button class="connection" class:connected={connection === 'connected'} onclick={() => client?.reconnect()}>
      <span class="status-dot"></span>{connectionLabels[connection]}
    </button>
    <div class="desktop-actions">
      <button onclick={openSessions}>会话</button>
      <button aria-label="减小字号" onclick={() => changeFont(-1)}>A−</button>
      <button aria-label="增大字号" onclick={() => changeFont(1)}>A+</button>
      <button aria-label="切换全屏" onclick={toggleFullscreen}>全屏</button>
    </div>
    <button class="icon-button fullscreen-mobile" aria-label="切换全屏" onclick={toggleFullscreen}>⛶</button>
  </header>

  <section class="terminal-stage">
    <div class="terminal-frame" class:disconnected={connection === 'disconnected'}>
      <div class="terminal-host" bind:this={terminalElement}></div>
      {#if connection !== 'connected'}
        <button class="connection-overlay" onclick={() => client?.reconnect()}>
          <span class="spinner" class:still={connection === 'disconnected'}></span>
          <strong>{connectionLabels[connection]}</strong>
          <small>{connection === 'disconnected' ? '点按重新连接' : '正在恢复终端'}</small>
        </button>
      {/if}
    </div>
  </section>

  <form class="mobile-composer" onsubmit={(event) => { event.preventDefault(); sendDraft(); }}>
    <textarea
      bind:this={composerElement}
      bind:value={draft}
      aria-label="消息输入"
      enterkeyhint="send"
      maxlength="12000"
      placeholder="输入消息或命令…"
      rows="1"
      onblur={handleComposerBlur}
      onfocus={handleComposerFocus}
      onkeydown={handleComposerKeydown}
    ></textarea>
    <button
      type="button"
      disabled={connection !== 'connected'}
      onpointerdown={sendOnPointerDown}
      onclick={sendOnClick}
    >发送</button>
  </form>

  <nav class="keybar" aria-label="终端快捷键">
    <button onclick={() => send('\x1b')}>Esc</button>
    <button onclick={() => send('\t')}>Tab</button>
    <button onclick={() => send('\x03')}>Ctrl+C</button>
    <button class="accent" onclick={openSessions}>会话</button>
    <button class="arrow" aria-label="方向上" onclick={() => send('\x1b[A')}>↑</button>
    <button class="arrow" aria-label="方向下" onclick={() => send('\x1b[B')}>↓</button>
    <button onclick={pasteClipboard}>粘贴</button>
    <button class="arrow" aria-label="方向左" onclick={() => send('\x1b[D')}>←</button>
    <button class="arrow" aria-label="方向右" onclick={() => send('\x1b[C')}>→</button>
    <button onclick={copySelection}>复制</button>
    <button onclick={focusInput}>⌨</button>
  </nav>

  {#if notice}<div class="toast">{notice}</div>{/if}
</main>
