<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from "vue";

import { deleteItem, listItems, readFullText, updateExpiry } from "@/api";
import { serviceURL } from "@/paths";
import ComposerBox from "@/components/ComposerBox.vue";
import ConfirmDialog from "@/components/ConfirmDialog.vue";
import FloatingRefresh from "@/components/FloatingRefresh.vue";
import MessageCard from "@/components/MessageCard.vue";
import type { DropItem, Role } from "@/types";
import { dayKey, dayLabel, readableError, ttlLabel } from "@/utils";

const props = defineProps<{ role: Role }>();

const items = ref<DropItem[]>([]);
const initialLoading = ref(true);
const refreshing = ref(false);
const connected = ref(false);
const reconnecting = ref(false);
const online = ref(typeof navigator === "undefined" ? true : navigator.onLine);
const toast = ref("");
const pendingDelete = ref<DropItem | null>(null);
const deleting = ref(false);
let reloadTimer = 0;
let toastTimer = 0;
let events: EventSource | null = null;
let loadController: AbortController | null = null;

const owner = computed(() => props.role === "owner" || props.role === "hermes");
const connectionState = computed<"connected" | "connecting" | "disconnected" | "offline">(() => {
  if (!online.value) return "offline";
  if (connected.value) return "connected";
  if (reconnecting.value) return "disconnected";
  return "connecting";
});
const groups = computed(() => {
  const output: Array<{ key: string; label: string; items: DropItem[] }> = [];
  const chronologicalItems = [...items.value].sort((left, right) => {
    const timeDifference = new Date(left.created_at).getTime() - new Date(right.created_at).getTime();
    return timeDifference || left.id.localeCompare(right.id);
  });
  for (const item of chronologicalItems) {
    const key = dayKey(item.created_at);
    let group = output[output.length - 1];
    if (!group || group.key !== key) {
      group = { key, label: dayLabel(item.created_at), items: [] };
      output.push(group);
    }
    group.items.push(item);
  }
  return output;
});

function showToast(message: string): void {
  toast.value = message;
  window.clearTimeout(toastTimer);
  toastTimer = window.setTimeout(() => { toast.value = ""; }, 2400);
}

function nearTimelineBottom(): boolean {
  return document.documentElement.scrollHeight - window.innerHeight - window.scrollY < 180;
}

function scrollToLatest(behavior: ScrollBehavior = "auto"): void {
  window.scrollTo({ top: document.documentElement.scrollHeight, behavior });
}

async function load(showFeedback = false, forceLatest = false): Promise<void> {
  const keepLatestVisible = forceLatest || initialLoading.value || nearTimelineBottom();
  loadController?.abort();
  const controller = new AbortController();
  loadController = controller;
  if (showFeedback) refreshing.value = true;
  try {
    items.value = await listItems(controller.signal);
    initialLoading.value = false;
    if (keepLatestVisible) await nextTick(() => scrollToLatest());
    if (showFeedback) showToast("已刷新");
  } catch (reason) {
    if (controller.signal.aborted) return;
    showToast(readableError(reason));
  } finally {
    if (loadController === controller) {
      loadController = null;
      initialLoading.value = false;
      refreshing.value = false;
    }
  }
}

async function handleSent(): Promise<void> {
  await load(false, true);
}

function scheduleLoad(): void {
  window.clearTimeout(reloadTimer);
  reloadTimer = window.setTimeout(() => { void load(); }, 140);
}

async function copyItem(item: DropItem): Promise<void> {
  try {
    if (!item.full_text_url) throw new Error("没有可复制的文字");
    const response = await readFullText(item.full_text_url);
    if (!response.ok) throw new Error(`读取文字失败 (${response.status})`);
    await navigator.clipboard.writeText(await response.text());
    showToast("已复制全文");
  } catch (reason) {
    showToast(readableError(reason));
  }
}

async function changeExpiry(id: string, days: number): Promise<void> {
  try {
    await updateExpiry(id, days);
    showToast(`有效期已改为 ${ttlLabel(days)}`);
    scheduleLoad();
  } catch (reason) {
    showToast(readableError(reason));
  }
}

async function confirmDelete(): Promise<void> {
  if (!pendingDelete.value || deleting.value) return;
  deleting.value = true;
  try {
    const id = pendingDelete.value.id;
    await deleteItem(id);
    items.value = items.value.filter((item) => item.id !== id);
    pendingDelete.value = null;
    showToast("已彻底删除");
  } catch (reason) {
    showToast(readableError(reason));
  } finally {
    deleting.value = false;
  }
}

function connectEvents(): void {
  events?.close();
  connected.value = false;
  reconnecting.value = false;
  events = new EventSource(serviceURL("/api/v1/events"));
  events.addEventListener("open", () => {
    connected.value = true;
    reconnecting.value = false;
  });
  events.addEventListener("sync", scheduleLoad);
  events.addEventListener("items_changed", scheduleLoad);
  events.onerror = () => {
    connected.value = false;
    reconnecting.value = true;
  };
}

function handleOffline(): void {
  online.value = false;
  connected.value = false;
}

function handleOnline(): void {
  online.value = true;
  connectEvents();
}

onMounted(() => {
  void load();
  connectEvents();
  window.addEventListener("offline", handleOffline);
  window.addEventListener("online", handleOnline);
});

onBeforeUnmount(() => {
  events?.close();
  loadController?.abort();
  window.clearTimeout(reloadTimer);
  window.clearTimeout(toastTimer);
  window.removeEventListener("offline", handleOffline);
  window.removeEventListener("online", handleOnline);
});
</script>

<template>
  <main class="workspace">
    <FloatingRefresh :refreshing="refreshing" @refresh="load(true)" />

    <section class="timeline" aria-label="临时消息">
      <template v-if="initialLoading">
        <div class="day-divider"><span>正在读取</span></div>
        <div v-for="index in 3" :key="index" class="skeleton-card"><i></i><span></span><small></small></div>
      </template>

      <div v-else-if="groups.length" class="timeline-feed">
        <section v-for="group in groups" :key="group.key" class="day-group">
          <div class="day-divider"><span>{{ group.label }}</span></div>
          <div class="day-items">
            <MessageCard
              v-for="item in group.items"
              :key="item.id"
              :item="item"
              :role="role"
              @copy="copyItem"
              @expiry="changeExpiry"
              @remove="pendingDelete = $event"
              @toast="showToast"
            />
          </div>
        </section>
      </div>

      <section v-else class="empty-state">
        <div class="empty-mark"><span></span></div>
        <h1>这里还很安静</h1>
        <p>粘贴一段文字、截图，或者添加文件。</p>
      </section>
    </section>

    <ComposerBox
      :owner="owner"
      :connection-state="connectionState"
      @sent="handleSent"
      @toast="showToast"
    />
    <ConfirmDialog
      :open="Boolean(pendingDelete)"
      title="删除这条消息？"
      copy="消息和全部附件会立即永久删除，此操作无法恢复。"
      :busy="deleting"
      @cancel="pendingDelete = null"
      @confirm="confirmDelete"
    />

    <Transition name="toast-rise">
      <div v-if="toast" class="toast" role="status" aria-live="polite">{{ toast }}</div>
    </Transition>
  </main>
</template>
