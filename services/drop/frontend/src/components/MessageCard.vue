<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from "vue";

import AppIcon from "@/components/AppIcon.vue";
import AttachmentCard from "@/components/AttachmentCard.vue";
import ImageLightbox from "@/components/ImageLightbox.vue";
import type { Attachment, DropItem, Role } from "@/types";
import { expiryText, formatBytes, formatTime, linkify, TTL_OPTIONS, ttlLabel } from "@/utils";

const props = defineProps<{ item: DropItem; role: Role }>();
const emit = defineEmits<{
  copy: [item: DropItem];
  expiry: [id: string, days: number];
  remove: [item: DropItem];
  toast: [message: string];
}>();

const menuOpen = ref(false);
const menuView = ref<"main" | "expiry">("main");
const menuPlacement = ref<"down" | "up">("down");
const lightboxIndex = ref<number | null>(null);
const card = ref<HTMLElement | null>(null);
const isOwner = computed(() => props.role === "owner" || props.role === "hermes");
const showMenu = computed(() => isOwner.value);
const textTokens = computed(() => linkify(props.item.text_preview || ""));
const imageAttachments = computed(() => props.item.attachments.filter((attachment) => attachment.previewable));

function closeOutside(event: PointerEvent): void {
  if (menuOpen.value && card.value && !card.value.contains(event.target as Node)) closeMenu();
}

function closeMenu(): void {
  menuOpen.value = false;
  menuView.value = "main";
}

function placeMenu(): void {
  if (!card.value || window.innerWidth > 720) {
    menuPlacement.value = "down";
    return;
  }
  const cardRect = card.value.getBoundingClientRect();
  const composerTop = document.querySelector(".composer-box")?.getBoundingClientRect().top ?? window.innerHeight;
  const expectedHeight = menuView.value === "expiry" ? 206 : 166;
  menuPlacement.value = cardRect.bottom + expectedHeight > composerTop - 10 ? "up" : "down";
}

async function toggleMenu(): Promise<void> {
  if (menuOpen.value) {
    closeMenu();
    return;
  }
  menuView.value = "main";
  menuOpen.value = true;
  await nextTick();
  placeMenu();
}

async function showExpiryMenu(): Promise<void> {
  menuView.value = "expiry";
  await nextTick();
  placeMenu();
}

function openImage(attachment: Attachment): void {
  const nextIndex = imageAttachments.value.findIndex((image) => image.id === attachment.id);
  lightboxIndex.value = nextIndex < 0 ? 0 : nextIndex;
}

function selectExpiry(days: number): void {
  closeMenu();
  emit("expiry", props.item.id, days);
}

function copyText(event?: Event): void {
  const target = event?.target as HTMLElement | null;
  if (target?.closest("a")) return;
  const selection = window.getSelection();
  if (selection && !selection.isCollapsed && selection.toString().trim()) return;
  emit("copy", props.item);
}

function textKeydown(event: KeyboardEvent): void {
  if (event.key !== "Enter" && event.key !== " ") return;
  event.preventDefault();
  copyText();
}

onMounted(() => document.addEventListener("pointerdown", closeOutside));
onBeforeUnmount(() => document.removeEventListener("pointerdown", closeOutside));
</script>

<template>
  <article ref="card" class="drop-card" :data-item-id="item.id">
    <div class="drop-card-content">
      <p
        v-if="item.has_text"
        class="drop-text drop-text--copyable"
        :class="{ 'drop-text--truncated': item.text_truncated }"
        role="button"
        tabindex="0"
        aria-label="复制全文"
        title="点击复制全文"
        @click="copyText"
        @keydown="textKeydown"
      >
        <template v-for="(token, index) in textTokens" :key="index">
          <a v-if="token.type === 'link'" :href="token.value" target="_blank" rel="noopener noreferrer" @click.stop>{{ token.value }}</a>
          <template v-else>{{ token.value }}</template>
        </template>
      </p>

      <div v-if="item.attachments?.length" class="attachment-grid" :class="{ 'attachment-grid--single': item.attachments.length === 1 }">
        <AttachmentCard
          v-for="attachment in item.attachments"
          :key="attachment.id"
          :attachment="attachment"
          :role="role"
          @open-image="openImage"
          @toast="emit('toast', $event)"
        />
      </div>

      <div class="drop-meta">
        <time :datetime="item.created_at">{{ formatTime(item.created_at) }}</time>
        <span>{{ expiryText(item.expires_at) }}</span>
        <span v-if="item.total_size && (item.has_text || item.attachments.length > 1)">{{ formatBytes(item.total_size) }}</span>
      </div>
    </div>

    <div v-if="showMenu" class="card-actions">
      <button
        class="quiet-icon-button"
        type="button"
        aria-label="更多操作"
        aria-haspopup="menu"
        :aria-expanded="menuOpen"
        @pointerdown.stop
        @click="toggleMenu"
      >
        <AppIcon name="more" />
      </button>
      <Transition name="menu-pop">
        <div v-if="menuOpen" class="card-menu" :class="`card-menu--${menuPlacement}`" role="menu" @pointerdown.stop>
          <template v-if="menuView === 'main'">
            <button v-if="item.has_text" type="button" role="menuitem" @click="closeMenu(); emit('copy', item)">
              <AppIcon name="copy" /><span>复制全文</span>
            </button>
            <button class="menu-expiry-action" type="button" role="menuitem" @click="showExpiryMenu">
              <AppIcon name="clock" />
              <span><strong>有效期</strong><small>{{ expiryText(item.expires_at) }}</small></span>
              <span class="menu-chevron" aria-hidden="true">›</span>
            </button>
            <div class="menu-separator"></div>
            <button class="danger-action" type="button" role="menuitem" @click="closeMenu(); emit('remove', item)">
              <AppIcon name="trash" /><span>彻底删除</span>
            </button>
          </template>
          <template v-else>
            <button class="menu-back" type="button" role="menuitem" @click="menuView = 'main'">
              <AppIcon name="back" /><span>调整有效期</span>
            </button>
            <div class="menu-separator"></div>
            <button v-for="days in TTL_OPTIONS" :key="days" type="button" role="menuitem" @click="selectExpiry(days)">
              <AppIcon name="clock" /><span>{{ ttlLabel(days) }}</span>
            </button>
          </template>
        </div>
      </Transition>
    </div>

    <ImageLightbox
      v-if="lightboxIndex !== null"
      :images="imageAttachments"
      :initial-index="lightboxIndex"
      @close="lightboxIndex = null"
    />
  </article>
</template>
