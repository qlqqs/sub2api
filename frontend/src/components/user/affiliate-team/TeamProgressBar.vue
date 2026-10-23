<template>
  <!-- 切换范围 / 排序 / 翻页时保留旧数据，只显示细进度条；占位高度固定，不推动下方表格。 -->
  <div class="h-0.5 overflow-hidden" aria-hidden="true">
    <div v-if="active" class="team-progress h-full w-1/3 bg-primary-500 dark:bg-primary-400"></div>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  active: boolean
}>()
</script>

<style scoped>
.team-progress {
  animation: team-progress-slide 1.2s ease-in-out infinite;
}

@keyframes team-progress-slide {
  0% {
    transform: translateX(-100%);
  }
  100% {
    transform: translateX(300%);
  }
}

@media (prefers-reduced-motion: reduce) {
  .team-progress {
    animation: none;
    width: 100%;
    opacity: 0.6;
  }
}
</style>
