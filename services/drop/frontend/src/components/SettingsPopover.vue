<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from "vue";

import { loadStatus } from "@/api";
import AppIcon from "@/components/AppIcon.vue";
import type { StatusReport } from "@/types";
import { formatBytes, readableError, TTL_OPTIONS, ttlLabel } from "@/utils";

defineProps<{ modelValue: number }>();
const emit = defineEmits<{
  "update:modelValue": [days: number];
  toast: [message: string];
}>();

const root = ref<HTMLElement | null>(null);
const open = ref(false);
const loading = ref(false);
const status = ref<StatusReport | null>(null);

async function toggle(): Promise<void> {
  open.value = !open.value;
  if (open.value && !status.value) {
    loading.value = true;
    try {
      status.value = await loadStatus();
    } catch (reason) {
      emit("toast", readableError(reason));
    } finally {
      loading.value = false;
    }
  }
}

function choose(days: number): void {
  emit("update:modelValue", days);
  emit("toast", `新消息保留 ${ttlLabel(days)}`);
  open.value = false;
}

function closeOutside(event: PointerEvent): void {
  if (open.value && root.value && !root.value.contains(event.target as Node)) open.value = false;
}

onMounted(() => document.addEventListener("pointerdown", closeOutside));
onBeforeUnmount(() => document.removeEventListener("pointerdown", closeOutside));
</script>

<template>
  <div ref="root" class="settings-popover">
    <button class="composer-icon-button" type="button" aria-label="消息设置" :aria-expanded="open" @click="toggle">
      <AppIcon name="settings" />
    </button>
    <Transition name="menu-pop">
      <section v-if="open" class="settings-panel" aria-label="消息设置" @pointerdown.stop>
        <div class="settings-heading">
          <div><p class="menu-caption">设置</p><h2>保存期限</h2></div>
          <button class="panel-close" type="button" aria-label="关闭设置" @click="open = false"><AppIcon name="close" /></button>
        </div>

        <div class="ttl-list">
          <button v-for="days in TTL_OPTIONS" :key="days" type="button" :class="{ selected: modelValue === days }" @click="choose(days)">
            <span>{{ ttlLabel(days) }}</span><AppIcon v-if="modelValue === days" name="check" />
          </button>
        </div>

        <div class="status-block" :class="{ loading }">
          <template v-if="status">
            <div class="status-row"><span>存储空间</span><strong>{{ formatBytes(status.storage.used_bytes) }} / {{ formatBytes(status.storage.quota_bytes) }}</strong></div>
            <div class="storage-track"><i :style="{ width: `${Math.min(100, status.storage.used_bytes / status.storage.quota_bytes * 100)}%` }"></i></div>
            <div class="status-row"><span>近 24 小时流量</span><strong>{{ formatBytes(status.traffic.last_24_hours.total_bytes) }}</strong></div>
            <p>登录和分享权限由 HomeHub 统一管理</p>
          </template>
          <span v-else>{{ loading ? "正在读取状态…" : "暂时无法读取状态" }}</span>
        </div>
      </section>
    </Transition>
  </div>
</template>
