<template>
  <!-- CUSTOM: 设为合伙人：选人 → 预览 → 填区域名和比例 → 二次确认 -->
  <BaseDialog :show="show" :title="t('admin.partners.create.title')" width="normal" @close="close">
    <div class="space-y-4" data-test="partner-create-form">
      <AffiliateUserLookup
        input-id="partner-create-user"
        :label="t('admin.partners.create.user')"
        :placeholder="t('admin.affiliates.agents.addDialog.userPlaceholder')"
        :clear-label="t('admin.affiliates.agents.addDialog.changeUser')"
        :empty-text="t('admin.affiliates.agents.addDialog.noUserFound')"
        :selected="user"
        test-id="partner-create-user"
        @select="selectUser"
        @clear="selectUser(null)"
      />
      <p v-if="previewLoading" class="text-xs text-gray-500 dark:text-dark-400">{{ t('common.loading') }}</p>
      <PartnerPreviewInfo
        v-else-if="preview"
        :agent-level="preview.agent_level"
        :team="preview.team"
        :current-region="preview.current_region"
        :reason="preview.can_create ? null : preview.reason"
        region-hint-key="admin.partners.create.currentRegion"
      />
      <div>
        <label for="partner-create-region" class="input-label">{{ t('admin.partners.fields.regionName') }}</label>
        <input
          id="partner-create-region"
          v-model="regionName"
          type="text"
          class="input"
          maxlength="64"
          data-test="partner-create-region"
        />
      </div>
      <div>
        <label for="partner-create-rate" class="input-label">{{ t('admin.partners.fields.shareRate') }}</label>
        <div class="relative">
          <input
            id="partner-create-rate"
            v-model="ratePercent"
            type="text"
            inputmode="decimal"
            class="input pr-8"
            data-test="partner-create-rate"
          />
          <span class="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-gray-400">%</span>
        </div>
        <p v-if="'error' in rate" class="mt-1 text-xs text-red-500">{{ t('admin.partners.errors.rate') }}</p>
      </div>
      <p v-if="formError" role="alert" class="text-sm text-red-600">{{ formError }}</p>
    </div>
    <template #footer>
      <button type="button" class="btn btn-secondary" :disabled="saving" @click="close">{{ t('common.cancel') }}</button>
      <button
        type="button"
        class="btn btn-primary"
        :disabled="!canSubmit"
        data-test="partner-create-submit"
        @click="askConfirm"
      >
        {{ saving ? t('common.saving') : t('admin.partners.create.submit') }}
      </button>
    </template>
  </BaseDialog>

  <ConfirmDialog
    :show="confirming"
    :title="t('admin.partners.create.confirmTitle')"
    :message="confirmMessage"
    :confirm-text="t('admin.partners.create.submit')"
    danger
    @confirm="submit"
    @cancel="confirming = false"
  />
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import AffiliateUserLookup from '@/views/admin/affiliates/AffiliateUserLookup.vue'
import type { SimpleUser } from '@/api/admin/affiliates'
import affiliatePartnersAPI, { type AdminPartnerItem, type PartnerCreatePreview } from '@/api/admin/affiliatePartners'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { partnerRateFromPercent } from '@/utils/affiliatePartner'
import PartnerPreviewInfo from './PartnerPreviewInfo.vue'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ close: []; created: [item: AdminPartnerItem] }>()

const DEFAULT_RATE_PERCENT = '40'

const { t } = useI18n()
const appStore = useAppStore()

const user = ref<SimpleUser | null>(null)
const preview = ref<PartnerCreatePreview | null>(null)
const previewLoading = ref(false)
const regionName = ref('')
const ratePercent = ref(DEFAULT_RATE_PERCENT)
const formError = ref('')
const confirming = ref(false)
const saving = ref(false)
let previewController: AbortController | null = null

watch(
  () => props.show,
  (show) => {
    if (!show) return
    selectUser(null)
    regionName.value = ''
    ratePercent.value = DEFAULT_RATE_PERCENT
  }
)

async function selectUser(next: SimpleUser | null) {
  previewController?.abort()
  previewController = null
  user.value = next
  preview.value = null
  formError.value = ''
  if (!next) return
  const controller = new AbortController()
  previewController = controller
  previewLoading.value = true
  try {
    const result = await affiliatePartnersAPI.previewPartner(next.id, { signal: controller.signal })
    if (previewController === controller) preview.value = result
  } catch (error) {
    if (previewController === controller) formError.value = extractApiErrorMessage(error, t('common.error'))
  } finally {
    if (previewController === controller) previewLoading.value = false
  }
}

const rate = computed(() => partnerRateFromPercent(ratePercent.value))

const canSubmit = computed(
  () => !saving.value && !!preview.value?.can_create && regionName.value.trim() !== '' && 'value' in rate.value
)

const confirmMessage = computed(() =>
  t('admin.partners.create.confirmMessage', {
    user: user.value?.email ?? '',
    region: regionName.value.trim(),
    rate: ratePercent.value.trim()
  })
)

function askConfirm() {
  if (canSubmit.value) confirming.value = true
}

async function submit() {
  confirming.value = false
  const target = user.value
  const r = rate.value
  if (!target || !('value' in r) || saving.value) return
  saving.value = true
  try {
    const item = await affiliatePartnersAPI.createPartner({
      user_id: target.id,
      region_name: regionName.value.trim(),
      share_rate: r.value
    })
    appStore.showSuccess(t('admin.partners.create.success'))
    emit('created', item)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  } finally {
    saving.value = false
  }
}

function close() {
  if (!saving.value) emit('close')
}
</script>
