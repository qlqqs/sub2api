<template>
  <!-- CUSTOM: 分润：期间新产生（支出）、去向（转入余额 / 提现）、当前未提与冻结、受益人排行 -->
  <FinanceLoadState :loading="loading" :error="error" :has-data="!!data" @retry="load">
    <div v-if="data" class="space-y-4">
      <div class="grid grid-cols-1 gap-4 lg:grid-cols-3">
        <section class="card p-4">
          <h3 class="mb-3 text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.finance.affiliate.accruedTitle') }}</h3>
          <dl class="space-y-2 text-sm">
            <div
              v-for="key in ACCRUED_KEYS"
              :key="key"
              class="flex items-center justify-between gap-3"
              :class="{ 'border-t border-gray-100 pt-2 font-semibold dark:border-dark-700': key === 'total' }"
            >
              <dt class="text-gray-600 dark:text-gray-300">{{ t(`admin.finance.affiliate.accrued.${key}`) }}</dt>
              <dd class="text-gray-900 dark:text-white"><FinanceMoney :value="data.accrued[key]" /></dd>
            </div>
          </dl>
        </section>
        <section class="card p-4">
          <h3 class="mb-3 text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.finance.affiliate.flowTitle') }}</h3>
          <dl class="space-y-2 text-sm">
            <div class="flex items-center justify-between gap-3">
              <dt class="text-gray-600 dark:text-gray-300">{{ t('admin.finance.affiliate.transferred') }}</dt>
              <dd class="text-gray-900 dark:text-white"><FinanceMoney :value="data.transferred" /></dd>
            </div>
            <div class="flex items-center justify-between gap-3">
              <dt class="text-gray-600 dark:text-gray-300">{{ t('admin.finance.affiliate.withdrawn') }}</dt>
              <dd class="text-gray-900 dark:text-white"><FinanceMoney :value="data.withdrawn" /></dd>
            </div>
          </dl>
        </section>
        <section class="card p-4">
          <h3 class="mb-3 text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.finance.affiliate.snapshotTitle') }}</h3>
          <dl class="space-y-2 text-sm">
            <div class="flex items-center justify-between gap-3">
              <dt class="text-gray-600 dark:text-gray-300">{{ t('admin.finance.affiliate.available') }}</dt>
              <dd class="text-gray-900 dark:text-white"><FinanceMoney :value="data.available" /></dd>
            </div>
            <div class="flex items-center justify-between gap-3">
              <dt class="text-gray-600 dark:text-gray-300">{{ t('admin.finance.affiliate.frozen') }}</dt>
              <dd class="text-gray-900 dark:text-white"><FinanceMoney :value="data.frozen" /></dd>
            </div>
          </dl>
        </section>
      </div>

      <section class="card overflow-hidden">
        <h3 class="px-4 pt-4 text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.finance.affiliate.topTitle') }}</h3>
        <div class="mt-3 overflow-x-auto">
          <table class="table min-w-max whitespace-nowrap">
            <thead>
              <tr>
                <th scope="col">{{ t('admin.finance.affiliate.columns.user') }}</th>
                <th scope="col" class="text-right">{{ t('admin.finance.affiliate.columns.accrued') }}</th>
                <th scope="col" class="text-right">{{ t('admin.finance.affiliate.columns.transferred') }}</th>
                <th scope="col" class="text-right">{{ t('admin.finance.affiliate.columns.withdrawn') }}</th>
                <th scope="col" class="text-right">{{ t('admin.finance.affiliate.columns.available') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="data.top_beneficiaries.length === 0">
                <td colspan="5" class="py-8 text-center text-gray-500 dark:text-gray-400">{{ t('admin.finance.noData') }}</td>
              </tr>
              <tr v-for="row in data.top_beneficiaries" :key="row.user_id">
                <td>
                  {{ row.user_email || `#${row.user_id}` }}
                  <span class="ml-1 text-xs text-gray-400">#{{ row.user_id }}</span>
                </td>
                <td class="text-right"><FinanceMoney :value="row.accrued" /></td>
                <td class="text-right"><FinanceMoney :value="row.transferred" /></td>
                <td class="text-right"><FinanceMoney :value="row.withdrawn" /></td>
                <td class="text-right"><FinanceMoney :value="row.available" /></td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </div>
  </FinanceLoadState>
</template>

<script setup lang="ts">
import { watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { FinanceAffiliate } from '@/api/admin/finance'
import { useFinanceLoad } from '@/composables/useFinanceLoad'
import FinanceLoadState from './FinanceLoadState.vue'
import FinanceMoney from './FinanceMoney.vue'

const props = defineProps<{ start: string; end: string; refreshToken: number }>()

const { t } = useI18n()

const ACCRUED_KEYS: Array<keyof FinanceAffiliate['accrued']> = [
  'agent_direct',
  'agent_diff',
  'invite_percent',
  'invite_fixed',
  'total'
]

const { data, loading, error, load } = useFinanceLoad(
  (signal) => adminAPI.finance.getAffiliate({ start: props.start, end: props.end }, { signal }),
  () => t('common.unknownError')
)

watch(() => [props.start, props.end, props.refreshToken], load, { immediate: true })
</script>
