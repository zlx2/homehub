<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from "vue";

import AppIcon from "@/components/AppIcon.vue";
import SettingsPopover from "@/components/SettingsPopover.vue";
import { serviceURL } from "@/paths";
import { fileExtension, formatBytes, formatDuration } from "@/utils";

const props = defineProps<{
  owner: boolean;
  connectionState: "connected" | "connecting" | "disconnected" | "offline";
}>();
const emit = defineEmits<{
  sent: [];
  toast: [message: string];
}>();

const MAX_FILES = 10;
const MAX_ATTACHMENT_BYTES = 500 * 1024 * 1024;
const MAX_ITEM_BYTES = 1024 * 1024 * 1024;
const MAX_TEXT_BYTES = 50 * 1024 * 1024;

type UploadPhase = "idle" | "preparing" | "uploading" | "processing";

const text = ref("");
const files = ref<File[]>([]);
const ttlDays = ref(1);
const fileInput = ref<HTMLInputElement | null>(null);
const textarea = ref<HTMLTextAreaElement | null>(null);
const dragging = ref(false);
const phase = ref<UploadPhase>("idle");
const progress = ref(0);
const uploadSpeed = ref(0);
const remainingSeconds = ref(0);
const error = ref("");
let activeRequest: XMLHttpRequest | null = null;
let dragDepth = 0;
let uploadCancelled = false;
let retryTimer = 0;
let progressStartedAt = 0;
let lastProgressAt = 0;
let lastLoaded = 0;

const busy = computed(() => phase.value !== "idle");
const canCancel = computed(() => phase.value === "preparing" || phase.value === "uploading");
const actionIcon = computed<"send" | "stop" | "check">(() => {
  if (phase.value === "processing") return "check";
  return busy.value ? "stop" : "send";
});
const totalFileBytes = computed(() => files.value.reduce((sum, file) => sum + file.size, 0));
const phaseLabel = computed(() => {
  if (phase.value === "preparing") return "正在准备文件";
  if (phase.value === "uploading") return `正在上传 · ${progress.value}%`;
  if (phase.value === "processing") return "上传完成 · 服务器正在保存";
  return "";
});
const uploadDetail = computed(() => {
  if (phase.value === "preparing") return "等待上传";
  if (phase.value === "processing") return "请稍候";
  if (uploadSpeed.value <= 0) return "正在估算速度";
  return `${formatBytes(uploadSpeed.value)}/s · ${formatDuration(remainingSeconds.value)}`;
});
const connectionLabel = computed(() => ({
  connected: "实时连接正常",
  connecting: "正在建立实时连接",
  disconnected: "实时连接已断开，正在重试",
  offline: "设备当前没有网络",
})[props.connectionState]);

function carriesFiles(event: DragEvent): boolean {
  return Array.from(event.dataTransfer?.types || []).includes("Files");
}

function handlePageDragEnter(event: DragEvent): void {
  if (!carriesFiles(event) || busy.value) return;
  event.preventDefault();
  dragDepth += 1;
  dragging.value = true;
}

function handlePageDragOver(event: DragEvent): void {
  if (!carriesFiles(event) || busy.value) return;
  event.preventDefault();
  if (event.dataTransfer) event.dataTransfer.dropEffect = "copy";
  dragging.value = true;
}

function handlePageDragLeave(event: DragEvent): void {
  if (!dragging.value) return;
  event.preventDefault();
  dragDepth = Math.max(0, dragDepth - 1);
  if (dragDepth === 0) dragging.value = false;
}

function resetDragging(): void {
  dragDepth = 0;
  dragging.value = false;
}

function openFilePicker(): void {
  if (!busy.value) fileInput.value?.click();
}

function addFiles(collection: FileList | File[]): void {
  const additions = Array.from(collection);
  if (!additions.length) return;
  error.value = "";
  const next = [...files.value];
  for (const file of additions) {
    if (next.length >= MAX_FILES) {
      error.value = "一条消息最多添加 10 个文件";
      break;
    }
    if (file.size > MAX_ATTACHMENT_BYTES) {
      error.value = `${file.name} 超过单文件 500 MB 限制`;
      continue;
    }
    const duplicate = next.some((item) => item.name === file.name && item.size === file.size && item.lastModified === file.lastModified);
    if (duplicate) continue;
    if (next.reduce((sum, item) => sum + item.size, 0) + file.size > MAX_ITEM_BYTES) {
      error.value = "本次文件总量超过 1 GB 限制";
      break;
    }
    next.push(file);
  }
  files.value = next;
}

function handleFileInput(): void {
  if (fileInput.value?.files) addFiles(fileInput.value.files);
  if (fileInput.value) fileInput.value.value = "";
}

function handlePaste(event: ClipboardEvent): void {
  const pasted = event.clipboardData?.files;
  if (pasted?.length) addFiles(pasted);
}

function handleDrop(event: DragEvent): void {
  event.preventDefault();
  resetDragging();
  if (busy.value) return;
  if (event.dataTransfer?.files) addFiles(event.dataTransfer.files);
}

function removeFile(index: number): void {
  if (!busy.value) files.value.splice(index, 1);
}

function autoGrow(): void {
  const node = textarea.value;
  if (!node) return;
  node.style.height = "auto";
  node.style.height = `${Math.min(node.scrollHeight, 180)}px`;
}

function handleKeydown(event: KeyboardEvent): void {
  if (event.key === "Enter" && !event.shiftKey && !event.isComposing) {
    event.preventDefault();
    void send();
  }
}

function clearComposer(): void {
  text.value = "";
  files.value = [];
  progress.value = 0;
  error.value = "";
  void nextTick(autoGrow);
}

function resetUploadMetrics(): void {
  uploadSpeed.value = 0;
  remainingSeconds.value = 0;
  progressStartedAt = 0;
  lastProgressAt = 0;
  lastLoaded = 0;
}

function finishRequest(): void {
  phase.value = "idle";
  activeRequest = null;
}

function sendRequest(form: FormData, idempotencyKey: string, attempt = 0): void {
  const request = new XMLHttpRequest();
  activeRequest = request;
    request.open("POST", serviceURL("/api/v1/items"));
  request.responseType = "json";
  request.setRequestHeader("Idempotency-Key", idempotencyKey);

  request.upload.addEventListener("loadstart", () => {
    resetUploadMetrics();
    progressStartedAt = performance.now();
    lastProgressAt = progressStartedAt;
    phase.value = "uploading";
  });
  request.upload.addEventListener("progress", (event) => {
    phase.value = "uploading";
    if (!event.lengthComputable) return;
    progress.value = Math.min(100, Math.round(event.loaded / event.total * 100));
    const now = performance.now();
    const elapsed = Math.max(.001, (now - progressStartedAt) / 1000);
    const sampleElapsed = Math.max(.001, (now - lastProgressAt) / 1000);
    const average = event.loaded / elapsed;
    const instant = (event.loaded - lastLoaded) / sampleElapsed;
    const sample = lastLoaded > 0 && sampleElapsed >= .08 ? instant : average;
    if (sample > 0 && Number.isFinite(sample)) {
      uploadSpeed.value = uploadSpeed.value > 0 ? uploadSpeed.value * .72 + sample * .28 : sample;
      remainingSeconds.value = Math.max(0, (event.total - event.loaded) / uploadSpeed.value);
    }
    lastProgressAt = now;
    lastLoaded = event.loaded;
  });
  request.upload.addEventListener("load", () => {
    progress.value = 100;
    phase.value = "processing";
  });
  request.addEventListener("load", () => {
    if (request.status >= 200 && request.status < 300) {
      clearComposer();
      finishRequest();
      emit("toast", "已发送");
      emit("sent");
      return;
    }
    error.value = request.response?.error?.message || `发送失败 (${request.status})，请重试`;
    finishRequest();
    if (request.status === 401) location.reload();
  });
  request.addEventListener("error", () => {
    if (attempt === 0 && !uploadCancelled && navigator.onLine) {
      activeRequest = null;
      phase.value = "preparing";
      progress.value = 0;
      resetUploadMetrics();
      error.value = "连接波动，正在自动重试一次…";
      retryTimer = window.setTimeout(() => sendRequest(form, idempotencyKey, 1), 700);
      return;
    }
    error.value = "网络连接中断，文件仍保留在发送区，可再次发送";
    finishRequest();
  });
  request.addEventListener("abort", () => {
    if (uploadCancelled) {
      error.value = "已取消上传，文件仍保留在发送区";
      finishRequest();
    }
  });
  request.send(form);
}

async function send(): Promise<void> {
  if (busy.value) return;
  const textBytes = new TextEncoder().encode(text.value).byteLength;
  if (!text.value.length && !files.value.length) {
    error.value = "输入文字，或者添加一个文件";
    textarea.value?.focus();
    return;
  }
  if (textBytes > MAX_TEXT_BYTES) {
    error.value = "文字内容超过 50 MB 限制";
    return;
  }
  if (textBytes + totalFileBytes.value > MAX_ITEM_BYTES) {
    error.value = "本次内容总量超过 1 GB 限制";
    return;
  }

  const form = new FormData();
  if (text.value.length) form.append("text", text.value);
  if (props.owner) form.append("ttl_days", String(ttlDays.value));
  for (const file of files.value) form.append("files", file, file.name);

  error.value = "";
  progress.value = 0;
  resetUploadMetrics();
  phase.value = "preparing";
  uploadCancelled = false;
  await nextTick();
  sendRequest(form, crypto.randomUUID());
}

function cancelUpload(): void {
  uploadCancelled = true;
  window.clearTimeout(retryTimer);
  activeRequest?.abort();
  if (!activeRequest) finishRequest();
}

onMounted(() => {
  window.addEventListener("dragenter", handlePageDragEnter);
  window.addEventListener("dragover", handlePageDragOver);
  window.addEventListener("dragleave", handlePageDragLeave);
  window.addEventListener("drop", handleDrop);
  window.addEventListener("dragend", resetDragging);
  window.addEventListener("blur", resetDragging);
});

onBeforeUnmount(() => {
  window.clearTimeout(retryTimer);
  activeRequest?.abort();
  window.removeEventListener("dragenter", handlePageDragEnter);
  window.removeEventListener("dragover", handlePageDragOver);
  window.removeEventListener("dragleave", handlePageDragLeave);
  window.removeEventListener("drop", handleDrop);
  window.removeEventListener("dragend", resetDragging);
  window.removeEventListener("blur", resetDragging);
});
</script>

<template>
  <div class="composer-dock">
    <form
      class="composer-box"
      :class="{ 'is-dragging': dragging, 'is-busy': busy }"
      aria-label="发送消息"
      @submit.prevent="send"
    >
      <Teleport to="body">
        <Transition name="drag-overlay">
          <div v-if="dragging" class="page-drop-overlay" aria-hidden="true">
            <div class="page-drop-prompt"><AppIcon name="plus" /><strong>松开以添加文件</strong><span>可放到页面任意位置</span></div>
          </div>
        </Transition>
      </Teleport>
      <label class="sr-only" for="message-input">消息内容</label>
      <textarea
        id="message-input"
        ref="textarea"
        v-model="text"
        rows="1"
        maxlength="52428800"
        placeholder="粘贴文字、网址或截图…"
        :disabled="busy"
        @input="autoGrow"
        @keydown="handleKeydown"
        @paste="handlePaste"
      ></textarea>

      <div v-if="files.length" class="selected-files" aria-label="待上传文件">
        <div v-for="(file, index) in files" :key="`${file.name}-${file.lastModified}-${index}`" class="selected-file">
          <span class="selected-file-type">{{ fileExtension(file.name) }}</span>
          <span class="selected-file-copy"><strong :title="file.name">{{ file.name }}</strong><small>{{ formatBytes(file.size) }}</small></span>
          <button type="button" :disabled="busy" :aria-label="`移除 ${file.name}`" @click="removeFile(index)"><AppIcon name="close" /></button>
        </div>
      </div>

      <div v-if="busy" class="upload-progress" aria-live="polite">
        <div class="upload-progress-copy"><strong>{{ phaseLabel }}</strong><span>{{ uploadDetail }}</span></div>
        <div class="upload-track"><i :style="{ width: `${phase === 'preparing' ? 2 : progress}%` }"></i></div>
      </div>

      <div class="composer-footer">
        <div class="composer-tools">
          <input ref="fileInput" type="file" multiple hidden @change="handleFileInput">
          <button class="composer-icon-button" type="button" :disabled="busy" aria-label="添加文件" @click="openFilePicker">
            <AppIcon name="plus" />
          </button>
          <SettingsPopover
            v-if="owner"
            v-model="ttlDays"
            @toast="emit('toast', $event)"
          />
          <span v-if="files.length && !busy" class="file-total">{{ files.length }} 个文件 · {{ formatBytes(totalFileBytes) }}</span>
        </div>
        <div class="composer-actions">
          <span
            class="connection-light"
            :class="`connection-light--${connectionState}`"
            role="status"
            :aria-label="connectionLabel"
            :title="connectionLabel"
          ></span>
          <button
            class="send-button"
            :class="{ 'send-button--stop': canCancel, 'send-button--processing': phase === 'processing' }"
            type="button"
            :disabled="phase === 'processing' || (!busy && !text.length && !files.length)"
            :aria-label="phase === 'processing' ? '服务器正在保存' : canCancel ? '取消上传' : '发送'"
            :title="phase === 'processing' ? '服务器正在保存' : canCancel ? '取消上传' : '发送'"
            @click="canCancel ? cancelUpload() : send()"
          >
            <AppIcon :name="actionIcon" />
          </button>
        </div>
      </div>
      <p v-if="error" class="composer-error" role="alert">{{ error }}</p>
    </form>
  </div>
</template>
