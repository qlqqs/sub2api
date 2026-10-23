<template>
  <!-- CUSTOM: 分组内单个模型倍率。命中的模型按该倍率计费,用户专属倍率不再作用于该模型 -->
  <div class="space-y-2" data-testid="group-model-rates-editor">
    <div class="flex items-center justify-between gap-2">
      <label class="input-label mb-0">{{ t('admin.groups.modelRates.title') }}</label>
      <button
        type="button"
        class="btn btn-secondary btn-sm"
        :disabled="!editable || rows.length >= GROUP_MODEL_RATE_MAX_RULES"
        data-testid="group-model-rates-add"
        @click="addRow"
      >
        <Icon name="plus" size="sm" class="mr-1" />
        {{ t('admin.groups.modelRates.add') }}
      </button>
    </div>
    <p class="input-hint mt-0">{{ t('admin.groups.modelRates.hint') }}</p>

    <div v-if="loading" class="text-xs text-gray-500 dark:text-gray-400">{{ t('common.loading') }}</div>
    <div
      v-else-if="loadError"
      class="flex flex-wrap items-center gap-2 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-600 dark:border-red-500/30 dark:bg-red-500/10 dark:text-red-300"
      data-testid="group-model-rates-load-error"
    >
      <span>{{ t('admin.groups.modelRates.loadFailed', { error: loadError }) }}</span>
      <button type="button" class="font-medium underline" @click="load">{{ t('admin.groups.modelRates.retry') }}</button>
    </div>
    <template v-else>
      <div v-if="rows.length > 0" class="space-y-2">
        <div v-for="(row, index) in rows" :key="row.key" data-testid="group-model-rates-row">
          <div class="flex items-center gap-2">
            <input
              v-model="row.model"
              type="text"
              class="input min-w-0 flex-1 font-mono text-sm"
              :class="{ 'border-red-400 dark:border-red-500': showErrors && rowErrors[index] }"
              :list="suggestions.length > 0 ? datalistId : undefined"
              :placeholder="t('admin.groups.modelRates.modelPlaceholder')"
              :aria-label="t('admin.groups.modelRates.model')"
              maxlength="255"
              data-testid="group-model-rates-model"
            />
            <!-- 位于分组 <form> 内:不用 min/max/step 约束,避免浏览器原生校验拦截提交(如 4 位小数),由下方校验提示 -->
            <input
              v-model="row.rate"
              type="number"
              step="any"
              inputmode="decimal"
              class="input w-32 shrink-0"
              :class="{ 'border-red-400 dark:border-red-500': showErrors && rowErrors[index] }"
              :placeholder="t('admin.groups.modelRates.ratePlaceholder')"
              :aria-label="t('admin.groups.modelRates.rate')"
              data-testid="group-model-rates-rate"
            />
            <button
              type="button"
              class="shrink-0 rounded-lg p-2 text-gray-400 transition-colors hover:bg-red-50 hover:text-red-500 dark:hover:bg-red-500/10"
              :title="t('admin.groups.modelRates.remove')"
              :aria-label="t('admin.groups.modelRates.remove')"
              data-testid="group-model-rates-remove"
              @click="removeRow(index)"
            >
              <Icon name="trash" size="sm" />
            </button>
          </div>
          <p
            v-if="showErrors && rowErrors[index]"
            class="mt-1 text-xs text-red-500"
            data-testid="group-model-rates-error"
          >
            {{ errorMessage(rowErrors[index]) }}
          </p>
        </div>
      </div>
      <p v-else class="text-xs text-gray-400 dark:text-dark-500">{{ t('admin.groups.modelRates.empty') }}</p>
      <p v-if="showErrors && validation.tooMany" class="text-xs text-red-500" data-testid="group-model-rates-error">
        {{ t('admin.groups.modelRates.errors.tooMany', { max: GROUP_MODEL_RATE_MAX_RULES }) }}
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
import type { GroupModelRateRule } from '@/api/admin/groupModelRates'
import { extractApiErrorMessage } from '@/utils/apiError'
import {
  GROUP_MODEL_RATE_MAX,
  GROUP_MODEL_RATE_MAX_RULES,
  groupModelRatesSignature,
  validateGroupModelRates,
  type GroupModelRateRowError
} from '@/utils/groupModelRates'

const props = withDefaults(
  defineProps<{
    /** 编辑已有分组时传入,挂载后加载其规则;新建分组时为空。 */
    groupId?: number | null
    /** 模型名输入建议(如分组可用模型)。 */
    suggestions?: string[]
  }>(),
  { groupId: null, suggestions: () => [] }
)

interface RateRow {
  key: number
  model: string
  rate: string | number
}

const { t } = useI18n()

let rowSeq = 0
const datalistId = `group-model-rates-suggestions-${Math.random().toString(36).slice(2, 10)}`

const rows = ref<RateRow[]>([])
const loading = ref(false)
const loadError = ref('')
const showErrors = ref(false)
/** 已保存(或已加载)规则的比较键;与当前不同才需要提交。 */
const baseline = ref(groupModelRatesSignature([]))

const editable = computed(() => !loading.value && !loadError.value)
const validation = computed(() => validateGroupModelRates(rows.value))
const rowErrors = computed(() => validation.value.errors)

function toRows(rules: GroupModelRateRule[]): RateRow[] {
  return rules.map((rule) => ({ key: ++rowSeq, model: rule.model, rate: rule.rate_multiplier }))
}

function addRow() {
  rows.value.push({ key: ++rowSeq, model: '', rate: '' })
}

function removeRow(index: number) {
  rows.value.splice(index, 1)
}

let loadSeq = 0
async function load() {
  const groupId = props.groupId
  const seq = ++loadSeq
  showErrors.value = false
  if (groupId == null) {
    rows.value = []
    baseline.value = groupModelRatesSignature([])
    loadError.value = ''
    return
  }
  loading.value = true
  loadError.value = ''
  try {
    const rules = await adminAPI.groupModelRates.getForGroup(groupId)
    if (seq !== loadSeq) return
    rows.value = toRows(rules)
    baseline.value = groupModelRatesSignature(rules)
  } catch (error) {
    if (seq !== loadSeq) return
    rows.value = []
    loadError.value = extractApiErrorMessage(error, t('common.unknownError'))
  } finally {
    if (seq === loadSeq) loading.value = false
  }
}

watch(() => props.groupId, load, { immediate: true })

function errorMessage(error: GroupModelRateRowError | null): string {
  switch (error) {
    case 'modelRequired':
      return t('admin.groups.modelRates.errors.modelRequired')
    case 'modelTooLong':
      return t('admin.groups.modelRates.errors.modelTooLong')
    case 'modelWhitespace':
      return t('admin.groups.modelRates.errors.modelWhitespace')
    case 'wildcardPosition':
      return t('admin.groups.modelRates.errors.wildcardPosition')
    case 'duplicate':
      return t('admin.groups.modelRates.errors.duplicate')
    case 'rateRequired':
      return t('admin.groups.modelRates.errors.rateRequired')
    case 'rateInvalid':
      return t('admin.groups.modelRates.errors.rateInvalid')
    case 'rateTooLarge':
      return t('admin.groups.modelRates.errors.rateTooLarge', { max: GROUP_MODEL_RATE_MAX })
    case 'rateTooSmall':
      return t('admin.groups.modelRates.errors.rateTooSmall')
    default:
      return ''
  }
}

/** 有未保存的改动(加载失败时视为无改动,避免误清空已有规则)。 */
const hasChanges = computed(
  () => editable.value && groupModelRatesSignature(validation.value.rules) !== baseline.value
)

/**
 * 校验当前输入;不合法时显示每行错误并返回第一条错误文案,合法返回 null。
 * 父组件在提交分组前调用,避免「分组已保存、模型倍率不合法」的半成功状态。
 */
function validate(): string | null {
  if (!editable.value) return null
  const result = validation.value
  if (result.valid) return null
  showErrors.value = true
  if (result.tooMany) return t('admin.groups.modelRates.errors.tooMany', { max: GROUP_MODEL_RATE_MAX_RULES })
  return errorMessage(result.errors.find((e) => e !== null) ?? null)
}

/**
 * 分组保存成功后调用:整组替换该分组的模型倍率。没有改动时不发请求。
 * 失败时抛出带可读原因的 Error,由父组件提示「分组已保存,模型单独倍率保存失败:…」。
 */
async function save(groupId: number): Promise<void> {
  if (!editable.value) return
  // 先校验:不合法的行不会进入 rules,不能因此被当成「无改动」而静默丢弃
  const message = validate()
  if (message) throw new Error(message)
  if (!hasChanges.value) return
  const saved = await adminAPI.groupModelRates.replaceForGroup(groupId, validation.value.rules)
  rows.value = toRows(saved)
  baseline.value = groupModelRatesSignature(saved)
  showErrors.value = false
}

defineExpose({ save, validate })
</script>
