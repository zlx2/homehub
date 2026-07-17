import { CanvasAddon } from '@xterm/addon-canvas';
import { FitAddon } from '@xterm/addon-fit';
import { Unicode11Addon } from '@xterm/addon-unicode11';
import { WebLinksAddon } from '@xterm/addon-web-links';
import { Terminal, type IDisposable, type ITheme } from '@xterm/xterm';
import '@xterm/xterm/css/xterm.css';

export type ConnectionState = 'connecting' | 'connected' | 'reconnecting' | 'disconnected';

const OUTPUT = '0';
const INPUT = '0';
const RESIZE = '1';
const PAUSE = '2';
const RESUME = '3';

export const terminalTheme: ITheme = {
  background: '#1b1f2e',
  foreground: '#e8e4d7',
  cursor: '#f0c674',
  cursorAccent: '#1b1f2e',
  selectionBackground: '#3a4058aa',
  selectionInactiveBackground: '#31364a88',
  black: '#171a24',
  red: '#e06c75',
  green: '#98c379',
  yellow: '#d7ba7d',
  blue: '#61afef',
  magenta: '#c678dd',
  cyan: '#56b6c2',
  white: '#d9dce3',
  brightBlack: '#73798d',
  brightRed: '#ef7f88',
  brightGreen: '#add58c',
  brightYellow: '#e8cc8a',
  brightBlue: '#7bbcf3',
  brightMagenta: '#d491e6',
  brightCyan: '#72cbd5',
  brightWhite: '#f5f2e9'
};

type Options = {
  wsUrl: string;
  tokenUrl: string;
  fontSize: number;
  onState: (state: ConnectionState) => void;
};

export class TtydClient {
  readonly terminal: Terminal;

  private readonly fitAddon = new FitAddon();
  private readonly encoder = new TextEncoder();
  private readonly disposables: IDisposable[] = [];
  private socket?: WebSocket;
  private resizeObserver?: ResizeObserver;
  private reconnectTimer?: number;
  private resizeTimer?: number;
  private pendingResize?: { columns: number; rows: number };
  private outputFrame?: number;
  private outputBytes = 0;
  private outputChunks: Uint8Array[] = [];
  private stopped = false;
  private fitFrame?: number;
  private written = 0;
  private pending = 0;

  constructor(private readonly options: Options) {
    this.terminal = new Terminal({
      allowProposedApi: true,
      convertEol: false,
      cursorBlink: true,
      cursorStyle: 'bar',
      fontFamily: '"Cascadia Mono", "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Noto Sans Mono CJK SC", monospace',
      fontSize: options.fontSize,
      fontWeight: '400',
      fontWeightBold: '600',
      letterSpacing: 0,
      lineHeight: 1.16,
      minimumContrastRatio: 1,
      scrollback: 6000,
      smoothScrollDuration: 100,
      theme: terminalTheme
    });
  }

  mount(element: HTMLElement) {
    this.terminal.loadAddon(this.fitAddon);
    this.terminal.loadAddon(new WebLinksAddon());
    const unicode = new Unicode11Addon();
    this.terminal.loadAddon(unicode);
    this.terminal.unicode.activeVersion = '11';
    this.terminal.open(element);

    try {
      this.terminal.loadAddon(new CanvasAddon());
    } catch {
      // The DOM renderer is a reliable fallback on older mobile browsers.
    }

    this.disposables.push(this.terminal.onData((data) => this.send(data)));
    this.disposables.push(this.terminal.onResize(({ cols, rows }) => this.queueResize(cols, rows)));

    this.resizeObserver = new ResizeObserver(() => this.scheduleFit());
    this.resizeObserver.observe(element);
    this.scheduleFit();
  }

  async connect() {
    if (this.socket?.readyState === WebSocket.OPEN || this.socket?.readyState === WebSocket.CONNECTING) return;
    this.clearReconnect();
    this.options.onState(this.socket ? 'reconnecting' : 'connecting');

    let token = '';
    try {
      const response = await fetch(this.options.tokenUrl, { cache: 'no-store', credentials: 'same-origin' });
      if (!response.ok) throw new Error(`token request failed: ${response.status}`);
      token = (await response.json()).token || '';
    } catch {
      this.options.onState('disconnected');
      this.scheduleReconnect();
      return;
    }

    if (this.stopped) return;
    const socket = new WebSocket(this.options.wsUrl, ['tty']);
    this.socket = socket;
    socket.binaryType = 'arraybuffer';

    socket.addEventListener('open', () => {
      socket.send(this.encoder.encode(JSON.stringify({ AuthToken: token, columns: this.terminal.cols, rows: this.terminal.rows })));
      this.options.onState('connected');
      if (matchMedia('(pointer: fine)').matches) this.terminal.focus();
    });
    socket.addEventListener('message', (event) => this.onMessage(event));
    socket.addEventListener('close', () => {
      if (this.socket === socket) this.socket = undefined;
      if (this.stopped) return;
      this.options.onState('reconnecting');
      this.scheduleReconnect();
    });
    socket.addEventListener('error', () => socket.close());
  }

  reconnect() {
    this.socket?.close();
    this.socket = undefined;
    void this.connect();
  }

  dispose() {
    this.stopped = true;
    this.clearReconnect();
    if (this.resizeTimer) clearTimeout(this.resizeTimer);
    if (this.outputFrame) cancelAnimationFrame(this.outputFrame);
    if (this.fitFrame) cancelAnimationFrame(this.fitFrame);
    this.resizeObserver?.disconnect();
    this.socket?.close(1000);
    for (const disposable of this.disposables) disposable.dispose();
    this.terminal.dispose();
  }

  send(data: string | Uint8Array) {
    const socket = this.socket;
    if (socket?.readyState !== WebSocket.OPEN) return;
    const encoded = typeof data === 'string' ? this.encoder.encode(data) : data;
    const payload = new Uint8Array(encoded.length + 1);
    payload[0] = INPUT.charCodeAt(0);
    payload.set(encoded, 1);
    socket.send(payload);
  }

  focus() {
    this.terminal.focus();
  }

  fit() {
    this.scheduleFit();
  }

  setFontSize(value: number) {
    this.terminal.options.fontSize = value;
    this.scheduleFit();
  }

  async copySelection() {
    const text = this.terminal.getSelection();
    if (!text) return false;
    await navigator.clipboard.writeText(text);
    return true;
  }

  private onMessage(event: MessageEvent<ArrayBuffer>) {
    const bytes = new Uint8Array(event.data);
    if (!bytes.length) return;
    const command = String.fromCharCode(bytes[0]);
    const body = bytes.subarray(1);

    if (command === OUTPUT) {
      this.queueOutput(body);
    }
  }

  private queueOutput(data: Uint8Array) {
    this.outputChunks.push(data);
    this.outputBytes += data.length;
    if (this.outputFrame) return;

    // Ink may emit clear-screen and redraw sequences in separate WebSocket
    // messages. Flush them together just before paint so xterm never renders
    // the temporary blank state between those messages.
    this.outputFrame = requestAnimationFrame(() => this.flushOutput());
  }

  private flushOutput() {
    this.outputFrame = undefined;
    if (!this.outputBytes) return;

    const output = new Uint8Array(this.outputBytes);
    let offset = 0;
    for (const chunk of this.outputChunks) {
      output.set(chunk, offset);
      offset += chunk.length;
    }
    this.outputChunks = [];
    this.outputBytes = 0;
    this.write(output);
  }

  private write(data: Uint8Array) {
    this.written += data.length;
    if (this.written <= 100_000) {
      this.terminal.write(data);
      return;
    }

    this.terminal.write(data, () => {
      this.pending = Math.max(0, this.pending - 1);
      if (this.pending < 4) this.sendControl(RESUME);
    });
    this.pending += 1;
    this.written = 0;
    if (this.pending > 10) this.sendControl(PAUSE);
  }

  private sendResize(columns: number, rows: number) {
    const socket = this.socket;
    if (socket?.readyState !== WebSocket.OPEN) return;
    socket.send(this.encoder.encode(RESIZE + JSON.stringify({ columns, rows })));
  }

  private queueResize(columns: number, rows: number) {
    this.pendingResize = { columns, rows };
    if (this.resizeTimer) clearTimeout(this.resizeTimer);
    // Mobile virtual keyboards animate the visual viewport through many
    // intermediate heights. Sending every size to Hermes forces repeated
    // full-screen TUI renders and competes with keystroke processing.
    this.resizeTimer = window.setTimeout(() => {
      this.resizeTimer = undefined;
      const size = this.pendingResize;
      this.pendingResize = undefined;
      if (size) this.sendResize(size.columns, size.rows);
    }, 100);
  }

  private sendControl(command: string) {
    const socket = this.socket;
    if (socket?.readyState === WebSocket.OPEN) socket.send(this.encoder.encode(command));
  }

  private scheduleFit() {
    if (this.fitFrame) cancelAnimationFrame(this.fitFrame);
    this.fitFrame = requestAnimationFrame(() => {
      this.fitFrame = undefined;
      try {
        this.fitAddon.fit();
      } catch {
        // Layout can briefly be zero-sized while the mobile keyboard animates.
      }
    });
  }

  private scheduleReconnect() {
    if (this.stopped || this.reconnectTimer) return;
    this.reconnectTimer = window.setTimeout(() => {
      this.reconnectTimer = undefined;
      void this.connect();
    }, 1500);
  }

  private clearReconnect() {
    if (!this.reconnectTimer) return;
    clearTimeout(this.reconnectTimer);
    this.reconnectTimer = undefined;
  }
}
