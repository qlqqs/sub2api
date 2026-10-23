<template>
  <span class="inline-flex min-w-0 items-center gap-2">
    <span v-if="user.deleted" class="text-gray-400 dark:text-dark-500" data-team-row-disabled>
      {{ t('affiliate.team.deleted') }}
    </span>
    <template v-else>
      <RouterLink
        v-if="to"
        :to="to"
        class="font-medium text-gray-900 transition-colors hover:text-primary-600 dark:text-white dark:hover:text-primary-400"
        @click.stop
      >
        {{ user.nickname || '-' }}
      </RouterLink>
      <span v-else class="font-medium text-gray-900 dark:text-white">{{ user.nickname || '-' }}</span>
      <span v-if="user.email" class="text-xs text-gray-500 dark:text-dark-400">{{ user.email }}</span>
      <slot />
    </template>
  </span>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { RouteLocationRaw } from 'vue-router'
import type { AffiliateTeamUser } from '@/types'

defineProps<{
  user: AffiliateTeamUser
  /** 可下钻时传入；已删除用户不可点击。 */
  to?: RouteLocationRaw
}>()

const { t } = useI18n()
</script>

<style>
/* 可点击行里的已删除用户：DataTable 给整行加了 cursor-pointer，这里还原。 */
tr:has([data-team-row-disabled]) {
  cursor: default;
}
</style>
