<script setup lang="ts">
import { computed, onMounted, ref } from "vue";

import AppIcon from "@/components/AppIcon.vue";
import type { Attachment, Role } from "@/types";
import { fileExtension, formatBytes, readPreviewHistory, rememberPreview } from "@/utils";

const props = defineProps<{ attachment: Attachment; role: Role }>();
const emit = defineEmits<{
  toast: [message: string];
  openImage: [attachment: Attachment];
}>();

const showingImage = ref(false);
const imageLoaded = ref(false);
const imageFailed = ref(false);
const rememberOnLoad = ref(false);
const showingVideo = ref(false);
const videoFailed = ref(false);
const isVideo = computed(() => props.attachment.mime_type.startsWith("video/"));

function showPreview(remember: boolean): void {
  showingImage.value = true;
  imageFailed.value = false;
  rememberOnLoad.value = remember;
}

function handleImageLoad(): void {
  imageLoaded.value = true;
  if (rememberOnLoad.value) rememberPreview(props.attachment.id);
}

function handleImageError(): void {
  showingImage.value = false;
  imageLoaded.value = false;
  imageFailed.value = true;
  emit("toast", "图片预览加载失败");
}

function showVideo(): void {
  showingVideo.value = true;
  videoFailed.value = false;
}

function handleVideoError(): void {
  showingVideo.value = false;
  videoFailed.value = true;
  emit("toast", "视频加载失败，可尝试右侧下载按钮");
}

onMounted(() => {
  const isOwner = props.role === "owner" || props.role === "hermes";
  if (props.attachment.previewable && props.attachment.preview_url && (isOwner || readPreviewHistory().has(props.attachment.id))) {
    showPreview(false);
  }
});
</script>

<template>
  <div class="attachment" :class="{ 'attachment--image': attachment.previewable, 'attachment--video': isVideo }">
    <div v-if="attachment.previewable" class="attachment-visual">
      <button
        v-if="showingImage"
        class="image-link"
        :class="{ 'is-loaded': imageLoaded }"
        type="button"
        :aria-label="`预览 ${attachment.original_name}`"
        @click="emit('openImage', attachment)"
      >
        <img
          :src="attachment.preview_url || attachment.download_url"
          :alt="attachment.original_name"
          loading="lazy"
          decoding="async"
          @load="handleImageLoad"
          @error="handleImageError"
        >
        <span v-if="!imageLoaded" class="image-loading"><i></i>正在加载预览</span>
      </button>
      <button v-else class="preview-trigger" type="button" @click="showPreview(true)">
        <span class="preview-glyph">IMG</span>
        <span>{{ imageFailed ? "加载失败，点按重试" : attachment.preview_url ? "点按加载省流量预览" : "点按加载图片" }}</span>
      </button>
    </div>
    <div v-else-if="isVideo" class="attachment-visual attachment-video">
      <video
        v-if="showingVideo"
        :src="attachment.download_url"
        controls
        playsinline
        preload="metadata"
        @error="handleVideoError"
      ></video>
      <button v-else class="preview-trigger video-trigger" type="button" @click="showVideo">
        <span class="video-play" aria-hidden="true">▶</span>
        <span>{{ videoFailed ? "加载失败，点按重试" : "点按在页面内播放" }}</span>
      </button>
    </div>
    <div v-else class="file-glyph" aria-hidden="true">
      <span>{{ fileExtension(attachment.original_name) }}</span>
    </div>

    <div class="attachment-details">
      <span class="attachment-name" :title="attachment.original_name">{{ attachment.original_name }}</span>
      <span class="attachment-size">{{ formatBytes(attachment.size) }}</span>
    </div>
    <a
      class="attachment-download"
      :href="`${attachment.download_url}?download=1`"
      :download="attachment.original_name"
      :aria-label="`下载 ${attachment.original_name}`"
    >
      <AppIcon name="download" />
    </a>
  </div>
</template>
