<template>
  <div class="qx-console">
    <div v-if="protocols.length > 1" class="qx-console-tabs" role="tablist">
      <button
        v-for="p in protocols"
        :key="p"
        type="button"
        role="tab"
        class="qx-console-tab"
        :aria-selected="p === active"
        @click="active = p"
      >
        {{ QX_PROTOCOL_LABELS[p] }}
      </button>
    </div>
    <div class="qx-console-head">
      <div class="qx-console-ep">
        <span>{{ t('qxTheme.console.endpoint') }}</span>
        <code>{{ endpoint }}</code>
      </div>
      <button type="button" class="qx-copy" @click="copyToClipboard(endpoint)">
        {{ copied ? t('qxTheme.console.copied') : t('qxTheme.console.copy') }}
      </button>
    </div>
    <pre v-if="active === 'openai'"><span class="qx-c-muted"># {{ t('qxTheme.console.keyHint') }}</span>
<span class="qx-c-prompt">$</span> curl {{ endpoint }}/responses \
    -H <span class="qx-c-str">"Authorization: Bearer $API_KEY"</span> \
    -H <span class="qx-c-str">"Content-Type: application/json"</span> \
    -d <span class="qx-c-str">'{"model": "{{ model }}",
         "input": "{{ t('qxTheme.console.prompt') }}"}'</span></pre>
    <pre v-else><span class="qx-c-muted"># {{ t('qxTheme.console.keyHint') }}</span>
<span class="qx-c-prompt">$</span> curl {{ endpoint }}/v1/messages \
    -H <span class="qx-c-str">"x-api-key: $API_KEY"</span> \
    -H <span class="qx-c-str">"anthropic-version: 2023-06-01"</span> \
    -H <span class="qx-c-str">"Content-Type: application/json"</span> \
    -d <span class="qx-c-str">'{"model": "{{ model }}",
         "max_tokens": 1024,
         "messages": [{"role": "user",
                       "content": "{{ t('qxTheme.console.prompt') }}"}]}'</span></pre>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useClipboard } from '@/composables/useClipboard'
import { QX_PROTOCOL_LABELS, qxProtocolEndpoint, type QxProtocol } from './config'

const props = defineProps<{
  protocols: readonly QxProtocol[]
  apiOrigin: string
  /** 示例请求里的模型名；模型广场不可用时为空。 */
  sampleModel: string | null
}>()

const { t } = useI18n()
const { copied, copyToClipboard } = useClipboard()

const active = ref<QxProtocol>(props.protocols[0] ?? 'openai')
const endpoint = computed(() => qxProtocolEndpoint(active.value, props.apiOrigin))
const model = computed(() => props.sampleModel || 'MODEL')
</script>
