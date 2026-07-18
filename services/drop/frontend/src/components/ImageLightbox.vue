<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";

import AppIcon from "@/components/AppIcon.vue";
import type { Attachment } from "@/types";

const props = defineProps<{
  images: Attachment[];
  initialIndex: number;
}>();
const emit = defineEmits<{ close: [] }>();

const index = ref(props.initialIndex);
const loading = ref(true);
const failed = ref(false);
const stage = ref<HTMLElement | null>(null);
let pointerStartX: number | null = null;
let previousOverflow = "";

const current = computed(() => props.images[index.value]);
const multiple = computed(() => props.images.length > 1);

function resetImageState(): void {
  loading.value = true;
  failed.value = false;
}

function goTo(nextIndex: number): void {
  if (!props.images.length) return;
  index.value = (nextIndex + props.images.length) % props.images.length;
  resetImageState();
}

function previous(): void {
  goTo(index.value - 1);
}

function next(): void {
  goTo(index.value + 1);
}

function handleKeydown(event: KeyboardEvent): void {
  if (event.key === "Escape") emit("close");
  if (event.key === "ArrowLeft" && multiple.value) previous();
  if (event.key === "ArrowRight" && multiple.value) next();
}

function handlePointerDown(event: PointerEvent): void {
  if (!multiple.value) return;
  if ((event.target as HTMLElement).closest("button, a")) return;
  pointerStartX = event.clientX;
  stage.value?.setPointerCapture(event.pointerId);
}

function handlePointerUp(event: PointerEvent): void {
  if (pointerStartX === null) return;
  const distance = event.clientX - pointerStartX;
  pointerStartX = null;
  if (Math.abs(distance) < 44) return;
  distance > 0 ? previous() : next();
}

watch(() => props.initialIndex, (value) => {
  index.value = value;
  resetImageState();
});

watch(index, () => { void nextTick(() => stage.value?.focus()); });

onMounted(() => {
  previousOverflow = document.body.style.overflow;
  document.body.style.overflow = "hidden";
  window.addEventListener("keydown", handleKeydown);
  void nextTick(() => stage.value?.focus());
});

onBeforeUnmount(() => {
  document.body.style.overflow = previousOverflow;
  window.removeEventListener("keydown", handleKeydown);
});
</script>

<template>
  <Teleport to="body">
    <div class="image-lightbox" role="dialog" aria-modal="true" aria-label="图片预览" @click.self="emit('close')">
      <header class="lightbox-header">
        <div class="lightbox-title">
          <strong :title="current.original_name">{{ current.original_name }}</strong>
          <span v-if="multiple">{{ index + 1 }} / {{ images.length }} · 左右滑动切换</span>
        </div>
        <a
          class="lightbox-action lightbox-action--download"
          :href="`${current.download_url}?download=1`"
          :download="current.original_name"
          :aria-label="`下载 ${current.original_name}`"
        ><AppIcon name="download" /></a>
        <button class="lightbox-action" type="button" aria-label="关闭图片预览" @click="emit('close')"><AppIcon name="close" /></button>
      </header>

      <div
        ref="stage"
        class="lightbox-stage"
        tabindex="-1"
        @pointerdown="handlePointerDown"
        @pointerup="handlePointerUp"
        @pointercancel="pointerStartX = null"
      >
        <button v-if="multiple" class="lightbox-nav lightbox-nav--previous" type="button" aria-label="上一张" @pointerdown.stop @pointerup.stop @click.stop="previous">‹</button>
        <div v-if="loading" class="lightbox-loading"><i></i><span>正在加载预览</span></div>
        <div v-if="failed" class="lightbox-failed">
          <strong>这张图片暂时无法预览</strong>
          <a :href="`${current.download_url}?download=1`" :download="current.original_name">下载原图</a>
        </div>
        <img
          v-show="!failed"
          :key="current.id"
          :src="current.download_url"
          :alt="current.original_name"
          draggable="false"
          @load="loading = false"
          @error="loading = false; failed = true"
        >
        <button v-if="multiple" class="lightbox-nav lightbox-nav--next" type="button" aria-label="下一张" @pointerdown.stop @pointerup.stop @click.stop="next">›</button>
      </div>

      <div v-if="multiple" class="lightbox-dots" aria-hidden="true">
        <i v-for="imageIndex in images.length" :key="imageIndex" :class="{ active: imageIndex - 1 === index }"></i>
      </div>
    </div>
  </Teleport>
</template>
