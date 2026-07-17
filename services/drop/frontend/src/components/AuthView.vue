<script setup lang="ts">
import { onMounted, ref } from "vue";

import { redeemAuthCode } from "@/api";
import { readableError } from "@/utils";

const props = defineProps<{ scannedCode: string }>();

const code = ref("");
const error = ref("");
const submitting = ref(false);

async function redeem(): Promise<void> {
  if (!code.value.trim() || submitting.value) return;
  submitting.value = true;
  error.value = "";
  try {
    await redeemAuthCode(code.value.trim());
    location.replace("/");
  } catch (reason) {
    error.value = readableError(reason);
  } finally {
    submitting.value = false;
  }
}

onMounted(() => {
  if (props.scannedCode) {
    code.value = props.scannedCode;
    void redeem();
  }
});
</script>

<template>
  <main class="auth-page">
    <section class="auth-card" aria-labelledby="auth-title">
      <div class="brand-mark" aria-hidden="true"><span></span></div>
      <p class="eyebrow">PRIVATE DROP</p>
      <h1 id="auth-title">回到你的中转站</h1>
      <p class="auth-copy">输入电脑端生成的一次性授权码，或用相机扫码后在 Via / Safari 打开。</p>
      <form class="auth-form" @submit.prevent="redeem">
        <label for="auth-code">一次性授权码</label>
        <input
          id="auth-code"
          v-model="code"
          inputmode="text"
          autocomplete="one-time-code"
          autocapitalize="characters"
          spellcheck="false"
          placeholder="XXXX-XXXX-XXXX"
          autofocus
        >
        <p v-if="error" class="form-error" role="alert">{{ error }}</p>
        <button class="primary-button auth-submit" type="submit" :disabled="submitting || !code.trim()">
          <span>{{ submitting ? "正在验证…" : "进入 Drop" }}</span>
          <span aria-hidden="true">→</span>
        </button>
      </form>
      <p class="auth-footnote">授权码仅可使用一次，30 分钟后失效；授权成功后此浏览器会保持为受信任设备。</p>
    </section>
  </main>
</template>
