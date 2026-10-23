<template>
  <div class="qx-panel qx-price-panel">
    <div class="qx-price-groups" :role="groups.length > 1 ? 'tablist' : undefined">
      <template v-if="groups.length > 1">
        <button
          v-for="(g, i) in groups"
          :key="g.id"
          type="button"
          role="tab"
          class="qx-pill qx-tab"
          :class="{ 'qx-tone-blue': i === activeIndex }"
          :aria-selected="i === activeIndex"
          @click="activeIndex = i"
        >
          <span class="qx-tab-name">{{ g.name }}</span><b>{{ g.models.length }}</b>
        </button>
      </template>
      <span v-else-if="activeGroup" class="qx-pill qx-tone-blue">
        {{ activeGroup.name }}<b>{{ rows.length }}</b>
      </span>
    </div>
    <template v-if="activeGroup">
      <p v-if="activeGroup.description" class="qx-group-desc">{{ activeGroup.description }}</p>
      <div class="qx-table">
        <table>
          <thead>
            <tr>
              <th scope="col">{{ t('qxTheme.pricing.model') }}</th>
              <th v-if="hasTiers" scope="col">{{ t('qxTheme.pricing.context') }}</th>
              <th scope="col" class="qx-num">{{ t('qxTheme.pricing.input') }}</th>
              <th scope="col" class="qx-num">{{ t('qxTheme.pricing.output') }}</th>
              <th scope="col" class="qx-num qx-hide-sm-col">{{ t('qxTheme.pricing.cacheRead') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in rows" :key="row.name">
              <td><span class="qx-model">{{ row.name }}</span></td>
              <td v-if="hasTiers" class="qx-tier">
                <div v-for="(tier, i) in row.tiers" :key="i">{{ tier.label }}</div>
              </td>
              <template v-if="row.unitPrice">
                <td class="qx-num">
                  <div>{{ row.unitPrice }} <span class="qx-unit">{{ unitSuffix(row.mode) }}</span></div>
                </td>
                <td class="qx-num"><div>-</div></td>
                <td class="qx-num qx-hide-sm-col"><div>-</div></td>
              </template>
              <template v-else>
                <td class="qx-num"><div v-for="(tier, i) in row.tiers" :key="i">{{ tier.input }}</div></td>
                <td class="qx-num"><div v-for="(tier, i) in row.tiers" :key="i">{{ tier.output }}</div></td>
                <td class="qx-num qx-hide-sm-col">
                  <div v-for="(tier, i) in row.tiers" :key="i">{{ tier.cacheRead }}</div>
                </td>
              </template>
            </tr>
          </tbody>
        </table>
      </div>
    </template>
    <div class="qx-stats qx-price-foot">
      <span v-if="hasTimePricing">{{ t('qxTheme.pricing.timePricingNote') }}</span>
      <router-link to="/model-plaza" class="qx-more">{{ t('qxTheme.pricing.more') }} →</router-link>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ModelPlazaGroup } from '@/api/modelPlaza'
import { BILLING_MODE_IMAGE, type BillingMode } from '@/constants/channel'
import { buildQxPriceRows } from './pricing'

const props = defineProps<{
  groups: ModelPlazaGroup[]
}>()

const { t } = useI18n()

const activeIndex = ref(0)
watch(
  () => props.groups.length,
  (n) => {
    if (activeIndex.value >= n) activeIndex.value = 0
  }
)

const activeGroup = computed(() => props.groups[activeIndex.value] ?? null)
const rows = computed(() => (activeGroup.value ? buildQxPriceRows(activeGroup.value) : []))
const hasTiers = computed(() => rows.value.some((r) => r.tiers.some((tier) => tier.label)))
const hasTimePricing = computed(() => rows.value.some((r) => r.hasTimePricing))

function unitSuffix(mode: BillingMode): string {
  return mode === BILLING_MODE_IMAGE ? t('qxTheme.pricing.perImage') : t('qxTheme.pricing.perRequest')
}
</script>
