<script setup lang="ts">
import AppIcon from "@/components/AppIcon.vue";

defineProps<{ open: boolean; title: string; copy: string; busy?: boolean }>();
const emit = defineEmits<{ cancel: []; confirm: [] }>();
</script>

<template>
  <Transition name="dialog-fade">
    <div v-if="open" class="dialog-backdrop" role="presentation" @pointerdown.self="emit('cancel')">
      <section class="confirm-dialog" role="alertdialog" aria-modal="true" aria-labelledby="confirm-title">
        <div class="confirm-icon"><AppIcon name="trash" /></div>
        <h2 id="confirm-title">{{ title }}</h2>
        <p>{{ copy }}</p>
        <div class="confirm-actions">
          <button class="secondary-button" type="button" :disabled="busy" @click="emit('cancel')">取消</button>
          <button class="danger-button" type="button" :disabled="busy" @click="emit('confirm')">{{ busy ? "正在删除…" : "彻底删除" }}</button>
        </div>
      </section>
    </div>
  </Transition>
</template>
