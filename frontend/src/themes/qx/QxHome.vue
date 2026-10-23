<template>
  <div class="qx">
    <QxSiteHeader />

    <div class="qx-hero qx-dots">
      <div class="qx-wrap qx-hero-body">
        <div>
          <h1>{{ siteName }}</h1>
          <p v-if="siteSubtitle" class="qx-hero-sub">{{ siteSubtitle }}</p>
          <p class="qx-hero-desc">{{ t('qxTheme.hero.description', { tools: toolNames }) }}</p>
          <div class="qx-hero-actions">
            <router-link v-if="isAuthenticated" :to="dashboardPath" class="qx-btn qx-btn-primary">
              {{ t('home.goToDashboard') }}
            </router-link>
            <template v-else>
              <router-link v-if="registrationEnabled" to="/register" class="qx-btn qx-btn-primary">
                {{ t('qxTheme.hero.register') }}
              </router-link>
              <router-link
                to="/login"
                class="qx-btn"
                :class="registrationEnabled ? 'qx-btn-outline' : 'qx-btn-primary'"
              >
                {{ t('home.login') }}
              </router-link>
            </template>
          </div>
          <!-- 小标题在上、数值在下，三项始终排在一行 -->
          <dl class="qx-stats qx-hero-stats">
            <div v-if="modelCount > 0" class="qx-stats-item">
              <dt>{{ t('qxTheme.hero.models') }}</dt>
              <dd><b>{{ modelCount }}</b></dd>
            </div>
            <div class="qx-stats-item">
              <dt>{{ t('qxTheme.hero.protocols') }}</dt>
              <dd><b v-for="p in protocols" :key="p">{{ QX_PROTOCOL_LABELS[p] }}</b></dd>
            </div>
            <div class="qx-stats-item">
              <dt>{{ t('qxTheme.hero.billing') }}</dt>
              <dd><b>{{ t('qxTheme.hero.billingValue') }}</b></dd>
            </div>
          </dl>
        </div>
        <QxConsole :protocols="protocols" :api-origin="apiOrigin" :sample-model="sampleModel" />
      </div>
    </div>

    <main>
      <section v-if="groups.length > 0" class="qx-section">
        <div class="qx-wrap">
          <span class="qx-kicker">{{ t('qxTheme.pricing.kicker') }}</span>
          <h2>{{ t('qxTheme.pricing.title') }}</h2>
          <p class="qx-lead">{{ t('qxTheme.pricing.lead') }}</p>
          <QxPricing :groups="groups" />
        </div>
      </section>

      <section class="qx-section">
        <div class="qx-wrap">
          <span class="qx-kicker">{{ t('qxTheme.steps.kicker') }}</span>
          <h2>{{ t('qxTheme.steps.title') }}</h2>
          <QxFlow :protocols="protocols" :api-origin="apiOrigin" :ccs-import-visible="ccsImportVisible" />
        </div>
      </section>
    </main>

    <footer class="qx-foot">
      <div class="qx-wrap qx-foot-inner">
        <span>&copy; {{ currentYear }} {{ siteName }}</span>
        <div class="qx-foot-links">
          <router-link v-if="modelPlazaVisible" to="/model-plaza">{{ t('nav.modelPlaza') }}</router-link>
          <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer">{{ t('home.docs') }}</a>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores'
import { getModelPlaza, type ModelPlazaGroup } from '@/api/modelPlaza'
import QxSiteHeader from './QxSiteHeader.vue'
import QxConsole from './QxConsole.vue'
import QxPricing from './QxPricing.vue'
import QxFlow from './QxFlow.vue'
import { QX_PROTOCOL_LABELS, QX_PROTOCOL_TOOLS, qxThemeConfig } from './config'
import { qxModelCount, qxSampleModel } from './pricing'
import { useQxSite } from './useQxSite'
import './theme.css'

const { t } = useI18n()
const authStore = useAuthStore()
const {
  siteName,
  siteSubtitle,
  docUrl,
  apiOrigin,
  isAuthenticated,
  dashboardPath,
  registrationEnabled,
  modelPlazaVisible,
  ccsImportVisible
} = useQxSite()

const protocols = qxThemeConfig.protocols
const separator = computed(() => t('qxTheme.listSeparator'))
const toolNames = computed(() => protocols.map((p) => QX_PROTOCOL_TOOLS[p]).join(separator.value))
const currentYear = new Date().getFullYear()

// 价格取自模型广场接口（登录后含用户专属倍率）；未开放或请求失败时不显示价格区块。
const groups = ref<ModelPlazaGroup[]>([])
const modelCount = computed(() => qxModelCount(groups.value))
const sampleModel = computed(() => qxSampleModel(groups.value))
let controller: AbortController | null = null

async function loadPlaza() {
  controller?.abort()
  if (!modelPlazaVisible.value) {
    groups.value = []
    return
  }
  const current = new AbortController()
  controller = current
  try {
    const data = await getModelPlaza({ signal: current.signal })
    groups.value = data.groups ?? []
  } catch {
    if (!current.signal.aborted) groups.value = []
  }
}

onMounted(() => {
  authStore.checkAuth()
  loadPlaza()
})
watch([isAuthenticated, modelPlazaVisible], loadPlaza)
onBeforeUnmount(() => controller?.abort())
</script>
