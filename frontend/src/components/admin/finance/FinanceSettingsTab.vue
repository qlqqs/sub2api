<template>
  <!-- CUSTOM: 财务设置：固定成本账号（单次请求成本记 0，成本来自其他支出） -->
  <FinanceLoadState :loading="loading" :error="error" :has-data="!!data" @retry="load">
    <section v-if="data" class="card p-4">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.finance.settings.fixedCostTitle') }}</h3>
          <p class="mt-1 max-w-2xl text-xs text-gray-500 dark:text-gray-400">{{ t('admin.finance.settings.fixedCostHint') }}</p>
        </div>
        <button
          type="button"
          class="btn btn-primary"
          :disabled="saving || !dirty"
          data-testid="finance-settings-save"
          @click="save"
        >
          {{ saving ? t('common.saving') : t('admin.finance.settings.save') }}
        </button>
      </div>

      <div class="mt-4 flex flex-wrap items-center gap-3">
        <input
          v-model="keyword"
          type="search"
          class="input w-full sm:w-72"
          :placeholder="t('admin.finance.settings.searchPlaceholder')"
          :aria-label="t('admin.finance.settings.searchPlaceholder')"
        />
        <span class="text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.finance.settings.selectedCount', { count: selected.size }) }}
        </span>
      </div>

      <ul
        v-if="filteredAccounts.length > 0"
        class="mt-3 grid max-h-[28rem] grid-cols-1 gap-1 overflow-y-auto sm:grid-cols-2 xl:grid-cols-3"
        data-testid="finance-settings-accounts"
      >
        <li v-for="account in filteredAccounts" :key="account.id">
          <label
            class="flex cursor-pointer items-center gap-2 rounded-lg px-2 py-1.5 text-sm hover:bg-gray-50 dark:hover:bg-dark-700"
          >
            <input
              type="checkbox"
              class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-500"
              :checked="selected.has(account.id)"
              @change="toggle(account.id)"
            />
            <span class="font-mono text-xs text-gray-400">#{{ account.id }}</span>
            <span class="min-w-0 flex-1 truncate text-gray-800 dark:text-gray-200" :title="account.name">{{ account.name }}</span>
            <span class="text-xs text-gray-400">{{ account.platform }}</span>
            <span v-if="account.deleted" class="badge badge-gray">{{ t('admin.finance.settings.deletedTag') }}</span>
          </label>
        </li>
      </ul>
      <p v-else class="mt-3 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.finance.settings.noAccounts') }}</p>
    </section>
  </FinanceLoadState>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { useFinanceLoad } from '@/composables/useFinanceLoad'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import FinanceLoadState from './FinanceLoadState.vue'

const props = defineProps<{ refreshToken: number }>()
/** 固定成本账号变化会影响账号成本与利润，通知页面刷新汇总。 */
const emit = defineEmits<{ changed: [] }>()

const { t } = useI18n()
const appStore = useAppStore()

const { data, loading, error, load } = useFinanceLoad(
  () => adminAPI.finance.getSettings(),
  () => t('common.unknownError')
)

const selected = ref(new Set<number>())
const keyword = ref('')
const saving = ref(false)

watch(data, (value) => {
  selected.value = new Set(value?.fixed_cost_account_ids ?? [])
})
watch(() => props.refreshToken, load, { immediate: true })

const signature = (ids: Iterable<number>) => [...ids].sort((a, b) => a - b).join(',')
const dirty = computed(() => signature(selected.value) !== signature(data.value?.fixed_cost_account_ids ?? []))

/** 已保存的固定成本账号排前面，便于确认；按保存值排序，勾选时列表不跳动。 */
const filteredAccounts = computed(() => {
  const saved = new Set(data.value?.fixed_cost_account_ids ?? [])
  const q = keyword.value.trim().toLowerCase()
  const list = (data.value?.accounts ?? []).filter(
    (a) => !q || a.name.toLowerCase().includes(q) || String(a.id) === q.replace(/^#/, '')
  )
  return [...list].sort((a, b) => Number(saved.has(b.id)) - Number(saved.has(a.id)) || a.id - b.id)
})

function toggle(id: number) {
  const next = new Set(selected.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  selected.value = next
}

async function save() {
  if (saving.value) return
  saving.value = true
  try {
    const ids = [...selected.value].sort((a, b) => a - b)
    const saved = await adminAPI.finance.updateSettings({ fixed_cost_account_ids: ids })
    // PUT 可能不回账号列表，沿用 GET 拿到的
    data.value = {
      fixed_cost_account_ids: saved.fixed_cost_account_ids,
      accounts: saved.accounts.length > 0 ? saved.accounts : (data.value?.accounts ?? [])
    }
    appStore.showSuccess(t('admin.finance.settings.saved'))
    emit('changed')
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('common.unknownError')))
  } finally {
    saving.value = false
  }
}
</script>
