<template>
  <!-- CUSTOM: 挂靠渠道经理：选人 → 预览 → 二次确认（不能移出或改挂） -->
  <BaseDialog :show="show" :title="t('admin.partners.attach.title')" width="normal" @close="close">
    <div class="space-y-4" data-test="partner-attach-form">
      <AffiliateUserLookup
        input-id="partner-attach-agent"
        :label="t('admin.partners.attach.agent')"
        :placeholder="t('admin.affiliates.agents.upline.selectPlaceholder')"
        :clear-label="t('admin.affiliates.agents.upline.changeSelection')"
        :empty-text="t('admin.affiliates.agents.upline.noAgentFound')"
        :selected="agent"
        :search="searchTopAgents"
        test-id="partner-attach-agent"
        @select="selectAgent"
        @clear="selectAgent(null)"
      />
      <p v-if="previewLoading" class="text-xs text-gray-500 dark:text-dark-400">{{ t('common.loading') }}</p>
      <PartnerPreviewInfo
        v-else-if="preview"
        :agent-level="preview.agent_level"
        :team="preview.team"
        :current-region="preview.current_region"
        :reason="preview.can_attach ? null : preview.reason"
        region-hint-key="admin.partners.attach.currentRegion"
      />
      <p v-if="error" role="alert" class="text-sm text-red-600">{{ error }}</p>
    </div>
    <template #footer>
      <button type="button" class="btn btn-secondary" :disabled="saving" @click="close">{{ t('common.cancel') }}</button>
      <button
        type="button"
        class="btn btn-primary"
        :disabled="saving || !preview?.can_attach"
        data-test="partner-attach-submit"
        @click="confirming = true"
      >
        {{ saving ? t('common.saving') : t('admin.partners.attach.submit') }}
      </button>
    </template>
  </BaseDialog>

  <ConfirmDialog
    :show="confirming"
    :title="t('admin.partners.attach.confirmTitle')"
    :message="t('admin.partners.attach.confirmMessage', { agent: agent?.email ?? '', region: regionName })"
    :confirm-text="t('admin.partners.attach.submit')"
    danger
    @confirm="submit"
    @cancel="confirming = false"
  />
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import AffiliateUserLookup from '@/views/admin/affiliates/AffiliateUserLookup.vue'
import { affiliatesAPI, type SimpleUser } from '@/api/admin/affiliates'
import affiliatePartnersAPI, { type AdminPartnerAgentItem, type PartnerAttachPreview } from '@/api/admin/affiliatePartners'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import PartnerPreviewInfo from './PartnerPreviewInfo.vue'

const props = defineProps<{
  show: boolean
  partnerId: number
  regionName: string
}>()
const emit = defineEmits<{ close: []; attached: [item: AdminPartnerAgentItem] }>()

const { t } = useI18n()
const appStore = useAppStore()

const agent = ref<SimpleUser | null>(null)
const preview = ref<PartnerAttachPreview | null>(null)
const previewLoading = ref(false)
const error = ref('')
const confirming = ref(false)
const saving = ref(false)
let previewController: AbortController | null = null

const searchTopAgents = (keyword: string) => affiliatesAPI.lookupTopAgents(keyword)

watch(
  () => props.show,
  (show) => {
    if (show) selectAgent(null)
  }
)

async function selectAgent(next: SimpleUser | null) {
  previewController?.abort()
  previewController = null
  agent.value = next
  preview.value = null
  error.value = ''
  if (!next) return
  const controller = new AbortController()
  previewController = controller
  previewLoading.value = true
  try {
    const result = await affiliatePartnersAPI.previewAttach(props.partnerId, next.id, { signal: controller.signal })
    if (previewController === controller) preview.value = result
  } catch (err) {
    if (previewController === controller) error.value = extractApiErrorMessage(err, t('common.error'))
  } finally {
    if (previewController === controller) previewLoading.value = false
  }
}

async function submit() {
  confirming.value = false
  if (!agent.value || saving.value) return
  saving.value = true
  try {
    const item = await affiliatePartnersAPI.attachAgent(props.partnerId, agent.value.id)
    appStore.showSuccess(t('admin.partners.attach.success'))
    emit('attached', item)
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  } finally {
    saving.value = false
  }
}

function close() {
  if (!saving.value) emit('close')
}
</script>
