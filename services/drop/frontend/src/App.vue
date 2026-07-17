<script setup lang="ts">
import DropWorkspace from "@/components/DropWorkspace.vue";
import type { Role } from "@/types";

defineProps<{
  page: string;
  role: Role;
}>();
</script>

<template>
  <DropWorkspace :role="role" />
</template>
