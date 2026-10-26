<template>
  <!-- CUSTOM: 充值与赠送：期间现金、通道费、退款、本金、各类赠送，订单明细可跳到订单管理 -->
  <FinanceLoadState :loading="loading" :error="error" :has-data="!!data" @retry="load">
    <div v-if="data" class="space-y-4">
      <div class="grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-6">
        <div v-for="card in cards" :key="card.key" class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ card.label }}</p>
          <p class="mt-1 text-lg font-bold text-gray-900 dark:text-white"><FinanceMoney :value="card.value" /></p>
          <p v-if="card.hint" class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ card.hint }}</p>
        </div>
      </div>

      <section class="card p-4">
        <h3 class="mb-3 text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.finance.recharge.giftTitle') }}</h3>
        <dl class="grid grid-cols-2 gap-3 sm:grid-cols-4 xl:grid-cols-7">
          <div v-for="key in GIFT_KEYS" :key="key" class="rounded-lg bg-gray-50 px-3 py-2 dark:bg-dark-800/60">
            <dt
              class="text-xs text-gray-500 dark:text-gray-400"
              :title="key === 'admin_adjust_out' ? t('admin.finance.recharge.adminAdjustOutHint') : undefined"
            >{{ t(`admin.finance.recharge.gift.${key}`) }}</dt>
            <dd class="mt-0.5 font-semibold text-gray-900 dark:text-white" :class="{ 'text-primary-700 dark:text-primary-300': key === 'total' }">
              <FinanceMoney :value="data.totals.gift[key]" />
            </dd>
          </div>
        </dl>
        <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.finance.recharge.adminAdjustOutHint') }}</p>
      </section>

      <section class="card overflow-hidden">
        <div class="flex flex-wrap items-center justify-between gap-2 px-4 pt-4">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.finance.recharge.ordersTitle') }}</h3>
          <p v-if="data.orders_truncated" class="text-xs text-amber-600 dark:text-amber-400">
            {{ t('admin.finance.recharge.ordersTruncated') }}
          </p>
        </div>
        <div class="mt-3 overflow-x-auto">
          <table class="table min-w-max whitespace-nowrap">
            <thead>
              <tr>
                <th scope="col">{{ t('admin.finance.recharge.columns.id') }}</th>
                <th scope="col">{{ t('admin.finance.recharge.columns.user') }}</th>
                <th scope="col">{{ t('admin.finance.recharge.columns.completedAt') }}</th>
                <th scope="col">{{ t('admin.finance.recharge.columns.paymentType') }}</th>
                <th scope="col" class="text-right">{{ t('admin.finance.recharge.columns.payAmount') }}</th>
                <th scope="col" class="text-right">{{ t('admin.finance.recharge.columns.channelFee') }}</th>
                <th scope="col" class="text-right">{{ t('admin.finance.recharge.columns.principal') }}</th>
                <th scope="col" class="text-right">{{ t('admin.finance.recharge.columns.bonus') }}</th>
                <th scope="col" class="text-right">{{ t('admin.finance.recharge.columns.refund') }}</th>
                <th scope="col">{{ t('admin.finance.recharge.columns.status') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="data.orders.length === 0">
                <td colspan="10" class="py-8 text-center text-gray-500 dark:text-gray-400">{{ t('admin.finance.noData') }}</td>
              </tr>
              <tr v-for="order in data.orders" :key="order.id">
                <td class="font-mono text-xs">#{{ order.id }}</td>
                <td>
                  <router-link
                    :to="{ path: '/admin/orders', query: { keyword: order.user_email } }"
                    class="text-primary-600 hover:underline dark:text-primary-400"
                    :title="t('admin.finance.recharge.viewInOrders')"
                  >{{ order.user_email || `#${order.user_id}` }}</router-link>
                </td>
                <td class="text-xs">{{ formatDateTime(order.completed_at) }}</td>
                <td>{{ t('payment.methods.' + order.payment_type, order.payment_type) }}</td>
                <td class="text-right"><FinanceMoney :value="order.pay_amount" /></td>
                <td class="text-right">
                  <FinanceMoney :value="order.channel_fee" />
                  <span class="ml-1 text-xs text-gray-400">({{ formatRatio(order.channel_fee_rate) }})</span>
                </td>
                <td class="text-right"><FinanceMoney :value="order.principal" :currency="false" /></td>
                <td class="text-right"><FinanceMoney :value="order.bonus" :currency="false" /></td>
                <td class="text-right">
                  <FinanceMoney :value="order.refund_cash" /> /
                  <FinanceMoney :value="order.refund_balance_deducted" :currency="false" />
                </td>
                <td>{{ t('payment.status.' + order.status.toLowerCase(), order.status) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </div>
  </FinanceLoadState>
</template>

<script setup lang="ts">
import { computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { FinanceGiftTotals } from '@/api/admin/finance'
import { useFinanceLoad } from '@/composables/useFinanceLoad'
import { formatDateTime } from '@/utils/format'
import { formatRatio } from '@/utils/financeFormat'
import FinanceLoadState from './FinanceLoadState.vue'
import FinanceMoney from './FinanceMoney.vue'

const props = defineProps<{ start: string; end: string; refreshToken: number }>()

const { t } = useI18n()

const GIFT_KEYS: Array<keyof FinanceGiftTotals> = [
  'recharge_bonus',
  'invitee_bonus',
  'promo',
  'admin_adjust_in',
  'redeem_code',
  'total',
  'admin_adjust_out'
]

const { data, loading, error, load } = useFinanceLoad(
  (signal) => adminAPI.finance.getRecharge({ start: props.start, end: props.end }, { signal }),
  () => t('common.unknownError')
)

watch(() => [props.start, props.end, props.refreshToken], load, { immediate: true })

const cards = computed(() => {
  const s = data.value?.totals
  if (!s) return []
  return [
    {
      key: 'cash_received',
      label: t('admin.finance.recharge.cards.cashReceived'),
      value: s.cash_received,
      hint: t('admin.finance.recharge.cards.orderCount', { count: s.order_count })
    },
    { key: 'channel_fee', label: t('admin.finance.recharge.cards.channelFee'), value: s.channel_fee },
    { key: 'refund_cash', label: t('admin.finance.recharge.cards.refundCash'), value: s.refund_cash },
    {
      key: 'net_cash',
      label: t('admin.finance.recharge.cards.netCash'),
      value: s.net_cash,
      hint: t('admin.finance.recharge.cards.netCashHint')
    },
    { key: 'principal', label: t('admin.finance.recharge.cards.principal'), value: s.principal },
    {
      key: 'commission_transfer_in',
      label: t('admin.finance.recharge.cards.commissionTransferIn'),
      value: s.commission_transfer_in
    }
  ] as Array<{ key: string; label: string; value: string; hint?: string }>
})
</script>
