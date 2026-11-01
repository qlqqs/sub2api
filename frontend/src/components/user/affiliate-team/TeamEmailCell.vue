<template>
  <span v-if="user && user.email && !user.deleted" class="text-gray-600 dark:text-gray-300">{{ user.email }}</span>
  <span v-else class="text-gray-400 dark:text-dark-500">-</span>
</template>

<script setup lang="ts">
import type { AffiliateTeamUser } from '@/types'

// 团队表格的「邮箱」列，与 TeamUserCell 的「用户名」列配对。
defineProps<{ user?: AffiliateTeamUser | null }>()
</script>
