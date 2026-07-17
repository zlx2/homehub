<script setup lang="ts">
import { computed } from "vue";

import AppIcon from "@/components/AppIcon.vue";
import type { AuthCode } from "@/types";
import { formatTime } from "@/utils";

const props = defineProps<{ code: AuthCode | null }>();
const emit = defineEmits<{ close: []; toast: [message: string] }>();

const trustedDays = computed(() => Math.max(1, Math.round((props.code?.session_ttl_seconds || 0) / 86400)));

async function copy(value: string, message: string): Promise<void> {
  await navigator.clipboard.writeText(value);
  emit("toast", message);
}
</script>

<template>
  <Transition name="dialog-fade">
    <div v-if="code" class="dialog-backdrop" role="presentation" @pointerdown.self="emit('close')">
      <section class="auth-code-dialog" role="dialog" aria-modal="true" aria-labelledby="code-title">
        <button class="dialog-close" type="button" aria-label="关闭" @click="emit('close')"><AppIcon name="close" /></button>
        <p class="eyebrow">NEW DEVICE</p>
        <h2 id="code-title">授权受信任设备</h2>
        <p class="dialog-copy">用相机扫描，或复制链接在 Via / Safari 打开；授权后保持登录 {{ trustedDays }} 天。</p>
        <div v-if="code.qr_data_url" class="qr-frame"><img :src="code.qr_data_url" alt="一次性授权二维码"></div>
        <button class="code-button" type="button" @click="copy(code.code, '授权码已复制')">{{ code.code }}</button>
        <div class="code-footer">
          <span>{{ formatTime(code.expires_at) }} 失效</span>
          <button v-if="code.redeem_url" type="button" @click="copy(code.redeem_url, '授权链接已复制')">复制授权链接</button>
        </div>
      </section>
    </div>
  </Transition>
</template>
