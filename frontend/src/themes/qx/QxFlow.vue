<template>
  <div class="qx-panel qx-flow" :class="{ 'qx-flow-single': !ccsImportVisible }">
    <div class="qx-flow-main">
      <div class="qx-flow-box">
        <span class="qx-kicker">{{ stepLabel(1) }}</span>
        <h3>{{ t('qxTheme.steps.register.title') }}</h3>
        <p>{{ t('qxTheme.steps.register.description') }}</p>
      </div>
      <div class="qx-flow-link" aria-hidden="true" />
      <div class="qx-flow-box">
        <span class="qx-kicker">{{ stepLabel(2) }}</span>
        <h3>{{ t('qxTheme.steps.key.title') }}</h3>
        <p>{{ t('qxTheme.steps.key.description') }}</p>
      </div>

      <!-- 第 3 步从这里分叉：每条支线一行，行高与右侧终点方框对齐 -->
      <div class="qx-flow-fork">
        <div v-if="ccsImportVisible" class="qx-flow-branch qx-tone-cyan">
          <span class="qx-pill">
            {{ t('qxTheme.steps.ccs.label') }}<b>{{ t('qxTheme.steps.ccs.badge') }}</b>
          </span>
        </div>
        <div class="qx-flow-branch qx-tone-amber">
          <span class="qx-pill">{{ t('qxTheme.steps.manual.label') }}</span>
        </div>
      </div>

      <div class="qx-flow-ends">
        <div v-if="ccsImportVisible" class="qx-flow-box qx-flow-end qx-tone-cyan">
          <span class="qx-pill qx-flow-pill-inline">
            {{ t('qxTheme.steps.ccs.label') }}<b>{{ t('qxTheme.steps.ccs.badge') }}</b>
          </span>
          <span class="qx-kicker">{{ stepLabel(3) }}</span>
          <h3>{{ t('qxTheme.steps.ccs.title') }}</h3>
          <p>{{ t('qxTheme.steps.ccs.description') }}</p>
          <div class="qx-flow-links">
            <router-link to="/keys">{{ t('qxTheme.steps.ccs.keys') }} →</router-link>
            <a :href="CC_SWITCH_DOWNLOAD_URL" target="_blank" rel="noopener noreferrer">
              {{ t('qxTheme.steps.ccs.download') }} ↗
            </a>
          </div>
        </div>
        <div class="qx-flow-box qx-flow-end qx-tone-amber">
          <span class="qx-pill qx-flow-pill-inline">{{ t('qxTheme.steps.manual.label') }}</span>
          <span class="qx-kicker">{{ stepLabel(3) }}</span>
          <h3>{{ t('qxTheme.steps.manual.title') }}</h3>
          <p>{{ t('qxTheme.steps.manual.description') }}</p>
          <dl class="qx-flow-endpoints">
            <template v-for="p in protocols" :key="p">
              <dt>{{ QX_PROTOCOL_TOOLS[p] }}</dt>
              <dd>{{ qxProtocolEndpoint(p, apiOrigin) }}</dd>
            </template>
          </dl>
        </div>
      </div>
    </div>

    <div class="qx-stats qx-flow-foot">
      <span class="qx-flow-foot-label">{{ t('qxTheme.steps.paths') }}</span>
      <span v-for="p in protocols" :key="p" class="qx-stats-item">
        {{ QX_PROTOCOL_LABELS[p] }}
        <code v-for="path in QX_PROTOCOL_PATHS[p]" :key="path">{{ path }}</code>
      </span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import {
  QX_PROTOCOL_LABELS,
  QX_PROTOCOL_PATHS,
  QX_PROTOCOL_TOOLS,
  qxProtocolEndpoint,
  type QxProtocol
} from './config'

defineProps<{
  protocols: readonly QxProtocol[]
  apiOrigin: string
  /** 后台隐藏了「导入到 CCS」按钮时只剩手动配置一条线。 */
  ccsImportVisible: boolean
}>()

const { t } = useI18n()

const CC_SWITCH_DOWNLOAD_URL = 'https://github.com/farion1231/cc-switch/releases'

function stepLabel(n: number): string {
  return `STEP ${String(n).padStart(2, '0')}`
}
</script>
