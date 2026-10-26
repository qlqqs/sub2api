<template>
  <!-- CUSTOM: 账号模型成本倍率。按发往上游的模型匹配，命中时替换账号倍率，只影响账号成本 -->
  <div class="space-y-2" data-testid="account-model-cost-editor">
    <div class="flex items-center justify-between gap-2">
      <label class="input-label mb-0">{{ t('admin.finance.accountModelCost.title') }}</label>
      <button
        type="button"
        class="btn btn-secondary btn-sm"
        :disabled="!editable || rows.length >= ACCOUNT_MODEL_COST_MAX_RULES"
        data-testid="account-model-cost-add"
        @click="addRow"
      >
        <Icon name="plus" size="sm" class="mr-1" />
        {{ t('admin.finance.accountModelCost.add') }}
      </button>
    </div>
    <p class="input-hint mt-0">{{ t('admin.finance.accountModelCost.hint') }}</p>

    <div v-if="loading" class="text-xs text-gray-500 dark:text-gray-400">{{ t('common.loading') }}</div>
    <div
      v-else-if="loadError"
      class="flex flex-wrap items-center gap-2 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-600 dark:border-red-500/30 dark:bg-red-500/10 dark:text-red-300"
      data-testid="account-model-cost-load-error"
    >
      <span>{{ t('admin.finance.accountModelCost.loadFailed', { error: loadError }) }}</span>
      <button type="button" class="font-medium underline" @click="load">{{ t('admin.finance.accountModelCost.retry') }}</button>
    </div>
    <template v-else>
      <div v-if="rows.length > 0" class="space-y-2">
        <div v-for="(row, index) in rows" :key="row.key" data-testid="account-model-cost-row">
          <div class="flex items-center gap-2">
            <input
              v-model="row.model"
              type="text"
              class="input min-w-0 flex-1 font-mono text-sm"
              :class="{ 'border-red-400 dark:border-red-500': showErrors && rowErrors[index] }"
              :list="suggestions.length > 0 ? datalistId : undefined"
              :placeholder="t('admin.finance.accountModelCost.modelPlaceholder')"
              :aria-label="t('admin.finance.accountModelCost.model')"
              maxlength="255"
              data-testid="account-model-cost-model"
            />
            <!-- 位于账号 <form> 内：用文本输入避免浏览器原生校验拦截提交，由下方校验提示 -->
            <input
              v-model="row.rate"
              type="text"
              inputmode="decimal"
              class="input w-32 shrink-0"
              :class="{ 'border-red-400 dark:border-red-500': showErrors && rowErrors[index] }"
              :placeholder="t('admin.finance.accountModelCost.ratePlaceholder')"
              :aria-label="t('admin.finance.accountModelCost.rate')"
              data-testid="account-model-cost-rate"
            />
            <button
              type="button"
              class="shrink-0 rounded-lg p-2 text-gray-400 transition-colors hover:bg-red-50 hover:text-red-500 dark:hover:bg-red-500/10"
              :title="t('admin.finance.accountModelCost.remove')"
              :aria-label="t('admin.finance.accountModelCost.remove')"
              data-testid="account-model-cost-remove"
              @click="removeRow(index)"
            >
              <Icon name="trash" size="sm" />
            </button>
          </div>
          <p v-if="showErrors && rowErrors[index]" class="mt-1 text-xs text-red-500" data-testid="account-model-cost-error">
            {{ errorMessage(rowErrors[index]) }}
          </p>
        </div>
      </div>
      <p v-else class="text-xs text-gray-400 dark:text-dark-500">{{ t('admin.finance.accountModelCost.empty') }}</p>
      <p v-if="showErrors && validation.tooMany" class="text-xs text-red-500" data-testid="account-model-cost-error">
        {{ t('admin.finance.accountModelCost.errors.tooMany', { max: ACCOUNT_MODEL_COST_MAX_RULES }) }}
      </p>
    </template>

    <datalist v-if="suggestions.length > 0" :id="datalistId">
      <option v-for="model in suggestions" :key="model" :value="model" />
    </datalist>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { adminAPI } from '@/api/admin'
import type { AccountModelCostRule } from '@/api/admin/accountModelCost'
import { extractApiErrorMessage } from '@/utils/apiError'
import {
  ACCOUNT_MODEL_COST_MAX,
  ACCOUNT_MODEL_COST_MAX_RULES,
  accountModelCostSignature,
  validateAccountModelCosts,
  type AccountModelCostRowError
} from '@/utils/accountModelCost'

const props = withDefaults(
  defineProps<{
    accountId: number | null
    /** 模型名输入建议（如账号的模型映射）。 */
    suggestions?: string[]
  }>(),
  { suggestions: () => [] }
)

interface CostRow {
  key: number
  model: string
  rate: string
}

const { t } = useI18n()

let rowSeq = 0
const datalistId = `account-model-cost-suggestions-${Math.random().toString(36).slice(2, 10)}`

const rows = ref<CostRow[]>([])
const loading = ref(false)
const loadError = ref('')
const showErrors = ref(false)
const baseline = ref(accountModelCostSignature([]))

const editable = computed(() => props.accountId != null && !loading.value && !loadError.value)
const validation = computed(() => validateAccountModelCosts(rows.value))
const rowErrors = computed(() => validation.value.errors)

function toRows(rules: AccountModelCostRule[]): CostRow[] {
  return rules.map((rule) => ({ key: ++rowSeq, model: rule.model, rate: String(rule.rate_multiplier) }))
}

function addRow() {
  rows.value.push({ key: ++rowSeq, model: '', rate: '' })
}

function removeRow(index: number) {
  rows.value.splice(index, 1)
}

let loadSeq = 0
async function load() {
  const accountId = props.accountId
  const seq = ++loadSeq
  showErrors.value = false
  loadError.value = ''
  if (accountId == null) {
    rows.value = []
    baseline.value = accountModelCostSignature([])
    return
  }
  loading.value = true
  try {
    const rules = await adminAPI.accountModelCost.getForAccount(accountId)
    if (seq !== loadSeq) return
    rows.value = toRows(rules)
    baseline.value = accountModelCostSignature(rules)
  } catch (error) {
    if (seq !== loadSeq) return
    rows.value = []
    loadError.value = extractApiErrorMessage(error, t('common.unknownError'))
  } finally {
    if (seq === loadSeq) loading.value = false
  }
}

watch(() => props.accountId, load, { immediate: true })

function errorMessage(error: AccountModelCostRowError | null): string {
  if (!error) return ''
  if (error === 'rateTooLarge') {
    return t('admin.finance.accountModelCost.errors.rateTooLarge', { max: ACCOUNT_MODEL_COST_MAX })
  }
  return t(`admin.finance.accountModelCost.errors.${error}`)
}

/** 有未保存的改动（加载失败时视为无改动，避免误清空已有规则）。 */
const hasChanges = computed(
  () => editable.value && accountModelCostSignature(validation.value.rules) !== baseline.value
)

/** 校验当前输入；不合法时显示每行错误并返回第一条错误文案，合法返回 null。 */
function validate(): string | null {
  if (!editable.value) return null
  const result = validation.value
  if (result.valid) return null
  showErrors.value = true
  if (result.tooMany) {
    return t('admin.finance.accountModelCost.errors.tooMany', { max: ACCOUNT_MODEL_COST_MAX_RULES })
  }
  return errorMessage(result.errors.find((e) => e !== null) ?? null)
}

/**
 * 账号保存成功后调用：整组替换该账号的模型成本倍率，没有改动时不发请求。
 * 失败时抛出带可读原因的 Error，由父组件提示「账号已保存，模型成本倍率保存失败：…」。
 */
async function save(): Promise<void> {
  const accountId = props.accountId
  if (accountId == null || !editable.value) return
  const message = validate()
  if (message) throw new Error(message)
  if (!hasChanges.value) return
  const saved = await adminAPI.accountModelCost.replaceForAccount(accountId, validation.value.rules)
  rows.value = toRows(saved)
  baseline.value = accountModelCostSignature(saved)
  showErrors.value = false
}

defineExpose({ save, validate, hasChanges })
</script>
