<template>
  <!-- CUSTOM: 合伙人列表：区域、比例、挂靠数、本月预估、累计亏损、待确认月数 -->
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <button
            type="button"
            class="btn btn-secondary px-2 md:px-3"
            :disabled="loading"
            :title="t('common.refresh')"
            :aria-label="t('common.refresh')"
            @click="load"
          >
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </button>
          <button type="button" class="btn btn-primary ml-auto" data-test="partners-create-open" @click="createOpen = true">
            + {{ t('admin.partners.create.title') }}
          </button>
        </div>
      </template>

      <template #table>
        <DataTable :columns="columns" :data="data ?? []" :loading="loading" :row-key="rowKey" data-test="partners-table">
          <template #cell-user="{ row }">
            <RouterLink
              :to="{ name: 'AdminAffiliatePartnerDetail', params: { userId: String(row.user.id) } }"
              class="block space-y-0.5"
              data-test="partner-detail-link"
            >
              <span class="block max-w-56 truncate text-sm font-medium text-primary-600 hover:underline dark:text-primary-400">{{ row.user.email || '-' }}</span>
              <span class="block text-xs text-gray-500 dark:text-dark-400">#{{ row.user.id }}<template v-if="row.user.username"> · {{ row.user.username }}</template></span>
            </RouterLink>
          </template>
          <template #cell-region_name="{ row }">
            <span class="text-sm font-medium text-gray-900 dark:text-white">{{ row.region_name }}</span>
          </template>
          <template #cell-share_rate="{ row }">
            <div class="text-sm text-gray-900 dark:text-white">{{ formatPartnerRate(row.share_rate) }}</div>
            <div v-if="row.next_share_rate" class="text-xs text-amber-600 dark:text-amber-400" data-test="partner-next-rate">
              {{ t('partner.nextRate', { month: row.next_rate_month, rate: formatPartnerRate(row.next_share_rate) }) }}
            </div>
          </template>
          <template #cell-agent_count="{ row }">
            <span class="text-sm tabular-nums">{{ row.agent_count }}</span>
          </template>
          <template #cell-member_count="{ row }">
            <span class="text-sm tabular-nums">{{ row.member_count }}</span>
          </template>
          <template #cell-share="{ row }">
            <FinanceMoney :value="row.current_month.share_amount" :color-negative="false" class="text-sm font-semibold" />
          </template>
          <template #cell-loss_carried="{ row }">
            <FinanceMoney :value="row.loss_carried" class="text-sm" />
          </template>
          <template #cell-unconfirmed_months="{ row }">
            <span :class="['text-sm tabular-nums', row.unconfirmed_months > 0 ? 'font-semibold text-amber-600 dark:text-amber-400' : 'text-gray-400 dark:text-dark-500']">
              {{ row.unconfirmed_months }}
            </span>
          </template>
          <template #empty>
            <div class="py-8 text-center text-sm text-gray-500 dark:text-dark-400">{{ t('admin.partners.empty') }}</div>
          </template>
        </DataTable>
        <p v-if="error" class="px-4 py-3 text-sm text-red-600" role="alert">{{ error }}</p>
      </template>
    </TablePageLayout>

    <PartnerCreateDialog :show="createOpen" @close="createOpen = false" @created="onCreated" />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Icon from '@/components/icons/Icon.vue'
import FinanceMoney from '@/components/admin/finance/FinanceMoney.vue'
import PartnerCreateDialog from '@/components/admin/affiliate-partner/PartnerCreateDialog.vue'
import type { Column } from '@/components/common/types'
import affiliatePartnersAPI, { type AdminPartnerItem } from '@/api/admin/affiliatePartners'
import { useFinanceLoad } from '@/composables/useFinanceLoad'
import { formatPartnerRate } from '@/utils/affiliatePartner'

const { t } = useI18n()

const { data, loading, error, load } = useFinanceLoad(
  (signal) => affiliatePartnersAPI.listPartners({ signal }),
  () => t('common.unknownError')
)

const createOpen = ref(false)
const rowKey = (row: AdminPartnerItem) => row.user.id

const columns = computed<Column[]>(() => [
  { key: 'user', label: t('admin.partners.columns.user') },
  { key: 'region_name', label: t('admin.partners.fields.regionName') },
  { key: 'share_rate', label: t('admin.partners.fields.shareRate') },
  { key: 'agent_count', label: t('admin.partners.columns.agentCount') },
  { key: 'member_count', label: t('admin.partners.columns.memberCount') },
  { key: 'share', label: t('admin.partners.columns.currentShare') },
  { key: 'loss_carried', label: t('admin.partners.columns.lossCarried') },
  { key: 'unconfirmed_months', label: t('admin.partners.columns.unconfirmedMonths') }
])

function onCreated() {
  createOpen.value = false
  void load()
}

onMounted(load)
</script>
