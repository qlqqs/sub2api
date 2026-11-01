<template>
  <!-- CUSTOM: 区域成本按分组、按模型拆分（不含账号与供应商） -->
  <div class="grid gap-4 lg:grid-cols-2" data-test="partner-costs">
    <div v-for="table in tables" :key="table.key" class="card overflow-hidden" :data-test="`partner-cost-${table.key}`">
      <h3 class="px-4 pb-3 pt-4 text-sm font-semibold text-gray-900 dark:text-white">{{ table.title }}</h3>
      <div class="overflow-x-auto">
        <table class="table min-w-max whitespace-nowrap">
          <thead>
            <tr>
              <th scope="col">{{ table.column }}</th>
              <th scope="col" class="!text-right">{{ t('partner.lines.consumption') }}</th>
              <th scope="col" class="!text-right">{{ t('partner.lines.account_cost') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="table.rows.length === 0">
              <td colspan="3" class="py-6 text-center text-gray-500 dark:text-dark-400">{{ t('partner.costs.empty') }}</td>
            </tr>
            <tr v-for="row in table.rows" :key="row.key">
              <td class="max-w-xs truncate" :title="row.label">{{ row.label }}</td>
              <td class="text-right"><FinanceMoney :value="row.consumption" /></td>
              <td class="text-right"><FinanceMoney :value="row.account_cost" /></td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import FinanceMoney from '@/components/admin/finance/FinanceMoney.vue'
import type { PartnerCostByGroup, PartnerCostByModel } from '@/api/affiliatePartner'

const props = defineProps<{
  byGroup: PartnerCostByGroup[]
  byModel: PartnerCostByModel[]
}>()

const { t } = useI18n()

const tables = computed(() => [
  {
    key: 'group',
    title: t('partner.costs.byGroup'),
    column: t('partner.costs.group'),
    rows: props.byGroup.map((row) => ({
      key: String(row.group_id),
      label: row.group_id === 0 || !row.group_name ? t('affiliate.team.filters.noGroup') : row.group_name,
      consumption: row.consumption,
      account_cost: row.account_cost
    }))
  },
  {
    key: 'model',
    title: t('partner.costs.byModel'),
    column: t('partner.costs.model'),
    rows: props.byModel.map((row) => ({ key: row.model, label: row.model || '-', consumption: row.consumption, account_cost: row.account_cost }))
  }
])
</script>
