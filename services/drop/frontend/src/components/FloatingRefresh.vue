<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from "vue";

import AppIcon from "@/components/AppIcon.vue";

defineProps<{ refreshing: boolean }>();
const emit = defineEmits<{ refresh: [] }>();

const STORAGE_KEY = "drop.refresh-position.v1";
const SIZE = 42;
const position = ref({ x: 20, y: 18 });
const dragging = ref(false);
let side: "left" | "right" = "left";
let pointerID: number | null = null;
let startX = 0;
let startY = 0;
let originX = 0;
let originY = 0;
let moved = false;
let suppressClick = false;
let composerObserver: ResizeObserver | null = null;

const style = computed(() => ({ left: `${position.value.x}px`, top: `${position.value.y}px` }));

function margin(): number {
  return window.innerWidth <= 720 ? 11 : 20;
}

function bounds(): { minX: number; maxX: number; minY: number; maxY: number } {
  const edge = margin();
  const composer = document.querySelector<HTMLElement>(".composer-box");
  const composerTop = composer?.getBoundingClientRect().top ?? window.innerHeight;
  return {
    minX: edge,
    maxX: Math.max(edge, window.innerWidth - SIZE - edge),
    minY: edge,
    maxY: Math.max(edge, Math.min(window.innerHeight - SIZE - edge, composerTop - SIZE - 12)),
  };
}

function clamp(value: number, minimum: number, maximum: number): number {
  return Math.min(maximum, Math.max(minimum, value));
}

function restorePosition(): void {
  try {
    const stored = JSON.parse(localStorage.getItem(STORAGE_KEY) || "null") as { side?: unknown; y?: unknown } | null;
    if (stored?.side === "right") side = "right";
    if (stored?.side === "left") side = "left";
    if (typeof stored?.y === "number" && Number.isFinite(stored.y)) position.value.y = stored.y;
  } catch {
    // A restrictive browser may make localStorage unavailable.
  }
  const area = bounds();
  position.value = {
    x: side === "left" ? area.minX : area.maxX,
    y: clamp(position.value.y, area.minY, area.maxY),
  };
}

function savePosition(): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ side, y: Math.round(position.value.y) }));
  } catch {
    // Dragging still works for the current page when storage is unavailable.
  }
}

function pointerDown(event: PointerEvent): void {
  if (event.button !== 0) return;
  pointerID = event.pointerId;
  startX = event.clientX;
  startY = event.clientY;
  originX = position.value.x;
  originY = position.value.y;
  moved = false;
  (event.currentTarget as HTMLElement).setPointerCapture(event.pointerId);
}

function pointerMove(event: PointerEvent): void {
  if (pointerID !== event.pointerId) return;
  const deltaX = event.clientX - startX;
  const deltaY = event.clientY - startY;
  if (!moved && Math.hypot(deltaX, deltaY) < 5) return;
  moved = true;
  dragging.value = true;
  const area = bounds();
  position.value = {
    x: clamp(originX + deltaX, area.minX, area.maxX),
    y: clamp(originY + deltaY, area.minY, area.maxY),
  };
}

function pointerUp(event: PointerEvent): void {
  if (pointerID !== event.pointerId) return;
  pointerID = null;
  if (!moved) return;
  const area = bounds();
  side = position.value.x + SIZE / 2 < window.innerWidth / 2 ? "left" : "right";
  position.value.x = side === "left" ? area.minX : area.maxX;
  position.value.y = clamp(position.value.y, area.minY, area.maxY);
  dragging.value = false;
  suppressClick = true;
  savePosition();
  window.setTimeout(() => { suppressClick = false; }, 0);
}

function click(event: MouseEvent): void {
  if (suppressClick) {
    event.preventDefault();
    return;
  }
  emit("refresh");
}

function resize(): void {
  restorePosition();
}

onMounted(() => {
  void nextTick(() => {
    restorePosition();
    const composer = document.querySelector<HTMLElement>(".composer-box");
    if (composer && "ResizeObserver" in window) {
      composerObserver = new ResizeObserver(restorePosition);
      composerObserver.observe(composer);
    }
  });
  window.addEventListener("resize", resize);
});
onBeforeUnmount(() => {
  composerObserver?.disconnect();
  window.removeEventListener("resize", resize);
});
</script>

<template>
  <button
    class="floating-refresh"
    :class="{ spinning: refreshing, dragging }"
    :style="style"
    type="button"
    aria-label="刷新消息，可拖动位置"
    title="刷新 · 可拖动"
    @pointerdown="pointerDown"
    @pointermove="pointerMove"
    @pointerup="pointerUp"
    @pointercancel="pointerUp"
    @click="click"
  >
    <AppIcon name="refresh" />
  </button>
</template>
