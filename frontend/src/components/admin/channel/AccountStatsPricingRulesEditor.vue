<template>
  <section class="space-y-4" data-testid="account-stats-rules-editor">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <h4 class="text-sm font-medium">{{ t('admin.channels.form.accountStatsPricingRules') }}</h4>
      <button type="button" class="btn btn-secondary btn-sm" @click="addRule">{{ t('admin.channels.form.addRule') }}</button>
    </div>
    <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.finance.costSettings.ruleOrderHint') }}</p>
    <p class="text-xs text-amber-700 dark:text-amber-300" data-testid="stats-pricing-support-warning">{{ t('admin.finance.costSettings.advancedSupport') }}</p>
    <p v-if="!modelValue.length" class="text-xs text-gray-500">{{ t('admin.channels.form.noRulesConfigured') }}</p>
    <section v-for="(rule, index) in modelValue" :key="index" class="space-y-3 rounded-lg border border-gray-200 p-3 dark:border-dark-600" data-testid="stats-rule">
      <div class="flex flex-wrap items-center gap-2">
        <span class="text-xs">#{{ index + 1 }}</span>
        <input :value="rule.name" class="input min-w-0 flex-1" :aria-label="t('admin.channels.form.ruleName')" :placeholder="t('admin.channels.form.ruleName')" @input="patch(index, { name: ($event.target as HTMLInputElement).value })" />
        <button type="button" class="btn btn-secondary btn-sm" :disabled="index === 0" :aria-label="t('admin.finance.costSettings.moveUp')" @click="move(index, -1)">↑</button>
        <button type="button" class="btn btn-secondary btn-sm" :disabled="index === modelValue.length - 1" :aria-label="t('admin.finance.costSettings.moveDown')" @click="move(index, 1)">↓</button>
        <button type="button" class="btn btn-secondary btn-sm" @click="remove(index)">{{ t('common.delete') }}</button>
      </div>
      <fieldset>
        <legend class="input-label">{{ t('admin.channels.form.ruleGroups') }}</legend>
        <div class="flex flex-wrap gap-2">
          <label v-for="group in scopeGroups(rule)" :key="group.id" class="inline-flex items-center gap-1 text-xs">
            <input type="checkbox" :checked="rule.group_ids.includes(group.id)" @change="patch(index, { group_ids: toggle(rule.group_ids, group.id) })" />
            #{{ group.id }} {{ group.name }}
          </label>
        </div>
      </fieldset>
      <div>
        <p class="input-label">{{ t('admin.channels.form.ruleAccounts') }}</p>
        <div class="flex flex-wrap gap-2">
          <button v-for="id in rule.account_ids" :key="id" type="button" class="badge badge-gray" :aria-label="`${t('common.remove')} #${id}`" @click="patch(index, { account_ids: toggle(rule.account_ids, id) })">#{{ id }} {{ accountNames[id] || '' }} ×</button>
        </div>
        <div class="mt-2 flex gap-2" @keydown.enter.prevent="search(index)">
          <input v-model="keywords[index]" class="input min-w-0 flex-1" :aria-label="t('admin.channels.form.searchAccountPlaceholder')" :placeholder="t('admin.channels.form.searchAccountPlaceholder')" />
          <button type="button" class="btn btn-secondary" @click="search(index)">{{ t('common.search') }}</button>
        </div>
        <p v-if="searchErrors[index]" class="text-xs text-red-600" role="alert">{{ searchErrors[index] }}</p>
        <div class="mt-1 max-h-40 overflow-auto">
          <button v-for="account in results[index] || []" :key="account.id" type="button" class="block w-full rounded p-2 text-left text-xs hover:bg-gray-100 dark:hover:bg-dark-700" :disabled="rule.account_ids.includes(account.id)" @click="patch(index, { account_ids: [...rule.account_ids, account.id] })">#{{ account.id }} {{ account.name }} · {{ account.platform }}</button>
        </div>
        <p class="mt-1 text-xs text-gray-500">{{ t('admin.channels.form.ruleAccountsHint') }}</p>
      </div>
      <div v-for="(pricing, pIndex) in rule.pricing" :key="pIndex" class="space-y-2">
        <label class="flex items-center gap-2 text-xs">
          {{ t('admin.finance.costSettings.platform') }}
          <select :value="pricing.platform" class="input w-auto" @change="updatePlatform(index, pIndex, ($event.target as HTMLSelectElement).value)">
            <option v-for="platform in platformOptions(pricing.platform)" :key="platform" :value="platform">{{ platform }}</option>
          </select>
        </label>
        <PricingEntryCard :entry="formPricing(index, pIndex, pricing)" :platform="pricing.platform" compact-header enable-time-pricing enable-tier-multipliers @update="updatePricing(index, pIndex, $event)" @remove="removePricing(index, pIndex)" />
      </div>
      <button type="button" class="btn btn-secondary btn-sm" @click="patch(index, { pricing: [...rule.pricing, emptyStatsPricing(platforms[0] || 'openai')] })">{{ t('admin.channels.form.ruleModelPricing') }} +</button>
    </section>
  </section>
</template>
<script setup lang="ts">
import { onBeforeUnmount, ref, watch, toRaw } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { AccountStatsPricingRule, ChannelModelPricing } from '@/api/admin/channels'
import { listPlatformIds } from '@/constants/platformCatalog'
import { extractApiErrorMessage } from '@/utils/apiError'
import PricingEntryCard from './PricingEntryCard.vue'
import { emptyStatsPricing, statsPricingToAPI, statsPricingToForm } from './accountStatsPricing'
import { validateIntervals, validateReasoningEffortMultipliers, validateTimePricing, isValidPositiveMultiplier, type PricingFormEntry } from './types'
const props = defineProps<{ modelValue: AccountStatsPricingRule[]; groups: { id: number; name: string }[] }>()
const emit = defineEmits<{ 'update:modelValue': [value: AccountStatsPricingRule[]] }>()
const { t } = useI18n()
const platforms = listPlatformIds()
const keywords = ref<Record<number, string>>({})
const results = ref<Record<number, { id: number; name: string; platform: string }[]>>({})
const searchErrors = ref<Record<number, string>>({})
const accountNames = ref<Record<number, string>>({})
// Keep numeric input drafts (e.g. `0.`) until the user finishes typing.
const pricingDrafts = ref<Record<string, PricingFormEntry>>({})
let localEmission: AccountStatsPricingRule[] | null = null
watch(() => props.modelValue, value => {
  if (toRaw(value) !== localEmission) pricingDrafts.value = {}
  localEmission = null
})
function publish(value: AccountStatsPricingRule[]) { localEmission = value; emit('update:modelValue', value) }
function formPricing(index: number, pIndex: number, pricing: ChannelModelPricing) { return pricingDrafts.value[`${index}:${pIndex}`] ?? statsPricingToForm(pricing) }
let searchGeneration = 0
let controller: AbortController | undefined
onBeforeUnmount(() => { searchGeneration++; controller?.abort() })
function clearSearch() { pricingDrafts.value = {}; searchGeneration++; controller?.abort(); keywords.value = {}; results.value = {}; searchErrors.value = {} }
function platformOptions(current: string) { return [...new Set([...platforms, current].filter(Boolean))] }
function scopeGroups(rule: AccountStatsPricingRule) {
  return [...props.groups, ...rule.group_ids.filter(id => !props.groups.some(g => g.id === id)).map(id => ({ id, name: '' }))]
}
function toggle(ids: number[], id: number) { return ids.includes(id) ? ids.filter(value => value !== id) : [...ids, id] }
function patch(index: number, changes: Partial<AccountStatsPricingRule>) { publish(props.modelValue.map((rule, i) => i === index ? { ...rule, ...changes } : rule)) }
function addRule() { publish([...props.modelValue, { name: '', group_ids: [], account_ids: [], pricing: [] }]) }
function remove(index: number) { clearSearch(); publish(props.modelValue.filter((_, i) => i !== index)) }
function move(index: number, delta: number) {
  const rules = [...props.modelValue]; [rules[index], rules[index + delta]] = [rules[index + delta], rules[index]]
  clearSearch(); publish(rules)
}
function updatePricing(index: number, pIndex: number, entry: PricingFormEntry) {
  pricingDrafts.value[`${index}:${pIndex}`] = entry
  patch(index, { pricing: props.modelValue[index].pricing.map((p, i) => i === pIndex ? statsPricingToAPI(entry, p) : p) })
}
function updatePlatform(index: number, pIndex: number, platform: string) {
  patch(index, { pricing: props.modelValue[index].pricing.map((p, i) => i === pIndex ? { ...p, platform } : p) })
}
function removePricing(index: number, pIndex: number) { pricingDrafts.value = {}; patch(index, { pricing: props.modelValue[index].pricing.filter((_, i) => i !== pIndex) }) }
async function search(index: number) {
  controller?.abort(); controller = new AbortController(); const generation = ++searchGeneration
  searchErrors.value[index] = ''
  try {
    const response = await adminAPI.accounts.list(1, 50, { search: keywords.value[index] || '', lite: 'true' }, { signal: controller.signal })
    if (generation !== searchGeneration) return
    results.value[index] = response.items
    for (const account of response.items) accountNames.value[account.id] = account.name
  } catch (error) { if (generation === searchGeneration) searchErrors.value[index] = extractApiErrorMessage(error, t('common.unknownError')) }
}
function validate(): string | null {
  for (const [index, rule] of props.modelValue.entries()) for (const [pIndex, api] of rule.pricing.entries()) {
    const p = formPricing(index, pIndex, api)
    if (!p.models.length) return t('admin.channels.form.noModels')
    const error = validateIntervals(p.intervals, p.billing_mode, t) || validateTimePricing(p.time_pricing, t) || validateReasoningEffortMultipliers(p.reasoning_effort_multipliers, t)
    if (error) return error
    if (!isValidPositiveMultiplier(p.fast_multiplier) || !isValidPositiveMultiplier(p.flex_multiplier)) return t('admin.channels.form.multiplierPositive')
    if (p.billing_mode !== 'token' && p.per_request_price == null && !p.intervals.length) return t('admin.channels.form.perRequestPriceRequired')
    for (const interval of p.intervals) {
      if (!Number.isFinite(Number(interval.min_tokens)) || (interval.max_tokens != null && !Number.isFinite(Number(interval.max_tokens)))) return t('admin.finance.costSettings.invalidPrice')
      for (const value of [interval.input_price, interval.output_price, interval.cache_write_price, interval.cache_write_1h_price, interval.cache_read_price, interval.per_request_price]) {
        if (value != null && value !== '' && (!Number.isFinite(Number(value)) || Number(value) < 0)) return t('admin.finance.costSettings.invalidPrice')
      }
    }
    for (const value of [p.input_price, p.output_price, p.cache_write_price, p.cache_write_1h_price, p.cache_read_price, p.image_input_price, p.image_output_price, p.per_request_price]) {
      if (value != null && value !== '' && (!Number.isFinite(Number(value)) || Number(value) < 0)) return t('admin.finance.costSettings.invalidPrice')
    }
  }
  return null
}
defineExpose({ validate })
</script>
