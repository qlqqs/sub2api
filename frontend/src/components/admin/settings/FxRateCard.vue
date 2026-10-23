<template>
  <!-- CUSTOM: 美元汇率。只用于显示「相比官方价」折扣,不参与计费 -->
  <div class="card" data-testid="fx-rate-card">
    <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
        {{ t('admin.settings.fxRate.title') }}
      </h2>
      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
        {{ t('admin.settings.fxRate.description') }}
      </p>
    </div>

    <div v-if="loading" class="p-6 text-sm text-gray-500 dark:text-gray-400">{{ t('common.loading') }}</div>
    <div
      v-else-if="loadError"
      class="flex flex-wrap items-center gap-2 p-6 text-sm text-red-600 dark:text-red-300"
      data-testid="fx-rate-load-error"
    >
      <span>{{ t('admin.settings.fxRate.loadFailed', { error: loadError }) }}</span>
      <button type="button" class="btn btn-secondary btn-sm" @click="load">{{ t('admin.settings.fxRate.retry') }}</button>
    </div>
    <div v-else-if="view" class="space-y-5 p-6">
      <div
        v-if="view.stale"
        class="flex items-start gap-2 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-700 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-300"
        data-testid="fx-rate-stale"
      >
        <Icon name="exclamationTriangle" size="sm" class="mt-0.5 shrink-0" />
        <!-- 从未成功获取(如容器无外网)与「超过 3 天未更新」分开说明 -->
        <span>{{
          t(view.fetched_at ? 'admin.settings.fxRate.stale' : 'admin.settings.fxRate.staleNever', {
            rate: formatRate(view.effective_rate)
          })
        }}</span>
      </div>

      <!-- 当前生效值 -->
      <dl class="grid grid-cols-1 gap-3 text-sm sm:grid-cols-2">
        <div>
          <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.settings.fxRate.effective') }}</dt>
          <dd class="mt-0.5 font-mono text-base font-semibold text-gray-900 dark:text-white" data-testid="fx-rate-effective">
            {{ formatFxRate(view.effective_rate) }}
            <span class="ml-1 font-sans text-xs font-normal text-gray-500 dark:text-gray-400">{{
              view.effective_from === 'fetched' ? t('admin.settings.fxRate.fromFetched') : t('admin.settings.fxRate.fromDefault')
            }}</span>
          </dd>
        </div>
        <div>
          <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.settings.fxRate.lastFetched') }}</dt>
          <dd class="mt-0.5 text-gray-700 dark:text-gray-300" data-testid="fx-rate-fetched">
            <template v-if="view.fetched_rate != null && view.fetched_at">
              <span class="font-mono">{{ formatRate(view.fetched_rate) }}</span>
              · {{ view.source || '—' }} · {{ formatDateTime(view.fetched_at) }}
            </template>
            <template v-else>{{ t('admin.settings.fxRate.neverFetched') }}</template>
          </dd>
        </div>
        <div v-if="view.last_error" class="sm:col-span-2">
          <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.settings.fxRate.lastError') }}</dt>
          <dd class="mt-0.5 break-words text-xs text-red-600 dark:text-red-300" data-testid="fx-rate-last-error">
            {{ view.last_error }}<template v-if="view.last_attempt_at"> · {{ formatDateTime(view.last_attempt_at) }}</template>
          </dd>
        </div>
      </dl>

      <!-- 自动更新 -->
      <div class="flex items-center justify-between gap-4">
        <div>
          <label class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.settings.fxRate.autoEnabled') }}</label>
          <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.settings.fxRate.autoEnabledHint') }}</p>
        </div>
        <Toggle v-model="form.autoEnabled" />
      </div>

      <!-- 默认汇率 -->
      <div>
        <label class="text-sm font-medium text-gray-700 dark:text-gray-300" for="fx-rate-default">
          {{ t('admin.settings.fxRate.defaultRate') }}
        </label>
        <p class="mb-2 mt-0.5 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.settings.fxRate.defaultRateHint', { min: FX_MIN, max: FX_MAX }) }}
        </p>
        <input
          id="fx-rate-default"
          v-model="form.defaultRate"
          type="number"
          :min="FX_MIN"
          :max="FX_MAX"
          step="0.0001"
          class="input w-40"
          :class="{ 'border-red-400 dark:border-red-500': defaultRateError }"
          data-testid="fx-rate-default-input"
        />
        <p v-if="defaultRateError" class="mt-1 text-xs text-red-500" data-testid="fx-rate-default-error">
          {{ defaultRateError }}
        </p>
      </div>

      <div class="flex flex-wrap items-center gap-3">
        <button
          type="button"
          class="btn btn-primary btn-sm"
          :disabled="saving || !dirty || !!defaultRateError"
          data-testid="fx-rate-save"
          @click="save"
        >
          {{ saving ? t('common.saving') : t('common.save') }}
        </button>
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          :disabled="refreshing"
          data-testid="fx-rate-refresh"
          @click="refresh"
        >
          <Icon name="refresh" size="sm" class="mr-1" :class="{ 'animate-spin': refreshing }" />
          {{ t('admin.settings.fxRate.refreshNow') }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import Toggle from '@/components/common/Toggle.vue'
// 与 SettingsView 使用相同的入口(@/api、@/stores),便于页面级测试统一 mock
import { adminAPI } from '@/api'
import type { FxRateView } from '@/api/admin/fxRate'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'
import { formatFxRate } from '@/utils/priceDiscount'
import { formatRateValue } from '@/utils/balanceCurrency'

/** 默认汇率允许范围,与后端校验一致。 */
const FX_MIN = 5
const FX_MAX = 9

const { t } = useI18n()
const appStore = useAppStore()

const view = ref<FxRateView | null>(null)
const loading = ref(false)
const loadError = ref('')
const saving = ref(false)
const refreshing = ref(false)
const form = reactive<{ autoEnabled: boolean; defaultRate: string | number }>({
  autoEnabled: true,
  defaultRate: ''
})

function formatRate(rate: number): string {
  return formatRateValue(rate)
}

function applyView(next: FxRateView) {
  view.value = next
  form.autoEnabled = next.auto_enabled
  form.defaultRate = next.default_rate
}

const parsedDefaultRate = computed(() => {
  const text = String(form.defaultRate ?? '').trim()
  return text === '' ? Number.NaN : Number(text)
})

const defaultRateError = computed(() => {
  const n = parsedDefaultRate.value
  if (!Number.isFinite(n) || n < FX_MIN || n > FX_MAX) {
    return t('admin.settings.fxRate.defaultRateInvalid', { min: FX_MIN, max: FX_MAX })
  }
  return ''
})

/** 生效汇率变化后刷新公开设置缓存,本页会话里的折扣(模型广场、使用记录)随之更新。 */
function syncPublicRate() {
  void appStore.fetchPublicSettings(true)
}

const dirty = computed(() => {
  if (!view.value) return false
  return form.autoEnabled !== view.value.auto_enabled || parsedDefaultRate.value !== view.value.default_rate
})

async function load() {
  loading.value = true
  loadError.value = ''
  try {
    applyView(await adminAPI.fxRate.get())
  } catch (error) {
    loadError.value = extractApiErrorMessage(error, t('common.unknownError'))
  } finally {
    loading.value = false
  }
}

async function save() {
  if (saving.value || defaultRateError.value) return
  saving.value = true
  try {
    applyView(
      await adminAPI.fxRate.update({ auto_enabled: form.autoEnabled, default_rate: parsedDefaultRate.value })
    )
    appStore.showSuccess(t('admin.settings.fxRate.saved'))
    syncPublicRate()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.settings.fxRate.saveFailed')))
  } finally {
    saving.value = false
  }
}

async function refresh() {
  if (refreshing.value) return
  refreshing.value = true
  try {
    const result = await adminAPI.fxRate.refresh()
    if (result?.rate) {
      // 只刷新只读信息,保留尚未保存的表单输入
      const pending = dirty.value ? { ...form } : null
      applyView(result.rate)
      if (pending) Object.assign(form, pending)
    }
    if (result?.ok) {
      // 关闭自动更新时生效值仍是默认汇率,提示里给出本次获取到的值
      const fetched = result.rate?.fetched_rate ?? result.rate?.effective_rate ?? 0
      appStore.showSuccess(t('admin.settings.fxRate.refreshed', { rate: formatRate(fetched) }))
      syncPublicRate()
    } else {
      appStore.showError(
        t('admin.settings.fxRate.refreshFailed', { error: result?.error || t('common.unknownError') })
      )
    }
  } catch (error) {
    appStore.showError(
      t('admin.settings.fxRate.refreshFailed', { error: extractApiErrorMessage(error, t('common.unknownError')) })
    )
  } finally {
    refreshing.value = false
  }
}

onMounted(load)
</script>
