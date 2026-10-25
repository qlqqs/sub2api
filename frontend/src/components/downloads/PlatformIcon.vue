<template>
  <!-- Windows：四块窗格 -->
  <svg v-if="platform === 'windows'" :class="sizeClass" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
    <path d="M3 5.6 10.4 4.6v6.9H3zM11.4 4.4 21 3v8.5h-9.6zM3 12.5h7.4v6.9L3 18.4zM11.4 12.5H21V21l-9.6-1.4z" />
  </svg>
  <!-- macOS / iOS：苹果 -->
  <svg
    v-else-if="platform === 'macos' || platform === 'ios'"
    :class="sizeClass"
    viewBox="0 0 24 24"
    fill="currentColor"
    aria-hidden="true"
  >
    <path
      d="M12.152 6.896c-.948 0-2.415-1.078-3.96-1.04-2.04.027-3.91 1.183-4.961 3.014-2.117 3.675-.546 9.103 1.519 12.09 1.013 1.454 2.208 3.09 3.792 3.039 1.52-.065 2.09-.987 3.935-.987 1.831 0 2.35.987 3.96.948 1.637-.026 2.676-1.48 3.676-2.948 1.156-1.688 1.636-3.325 1.662-3.415-.039-.013-3.182-1.221-3.22-4.857-.026-3.04 2.48-4.494 2.597-4.559-1.429-2.09-3.623-2.324-4.39-2.376-2-.156-3.675 1.09-4.61 1.09zM15.53 3.83c.843-1.012 1.4-2.427 1.245-3.83-1.207.052-2.662.805-3.532 1.818-.78.896-1.454 2.338-1.273 3.714 1.338.104 2.715-.688 3.559-1.701"
    />
  </svg>
  <!-- Android：机器人头部 -->
  <svg
    v-else-if="platform === 'android'"
    :class="sizeClass"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="1.8"
    stroke-linecap="round"
    aria-hidden="true"
  >
    <path d="M4 17a8 8 0 0 1 16 0z" fill="currentColor" stroke="none" />
    <path d="m7 7.5-1.8-3M17 7.5l1.8-3" />
    <circle cx="9" cy="13" r="1" fill="#fff" stroke="none" />
    <circle cx="15" cy="13" r="1" fill="#fff" stroke="none" />
  </svg>
  <!-- Linux：终端 -->
  <svg
    v-else-if="platform === 'linux'"
    :class="sizeClass"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="1.8"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <rect x="3" y="4" width="18" height="16" rx="2" />
    <path d="m7 9 3 3-3 3M13 15h4" />
  </svg>
  <Icon v-else name="download" :size="size" />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import type { ClientDownloadPlatform } from '@/types'

const props = withDefaults(defineProps<{ platform: ClientDownloadPlatform | string; size?: 'sm' | 'md' | 'lg' }>(), {
  size: 'sm'
})

const SIZE_CLASSES = { sm: 'h-4 w-4 shrink-0', md: 'h-5 w-5 shrink-0', lg: 'h-6 w-6 shrink-0' }
const sizeClass = computed(() => SIZE_CLASSES[props.size])
</script>
