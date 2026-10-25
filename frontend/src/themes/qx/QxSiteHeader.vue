<template>
  <header class="qx-top" :class="{ 'qx-top-plain': plain }">
    <div class="qx-wrap qx-top-inner">
      <router-link to="/" class="qx-brand">
        <img :src="siteLogo || '/logo.svg'" alt="" />
        <span>{{ siteName }}</span>
      </router-link>
      <nav class="qx-nav">
        <template v-if="!plain">
          <router-link v-if="modelPlazaVisible" to="/model-plaza" class="qx-nav-link qx-hide-sm">
            {{ t('nav.modelPlaza') }}
          </router-link>
          <router-link v-if="downloadsVisible" to="/downloads" class="qx-nav-link qx-hide-sm">
            {{ t('nav.downloads') }}
          </router-link>
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="qx-nav-link qx-hide-sm"
          >
            {{ t('home.docs') }}
          </a>
        </template>
        <LocaleSwitcher />
        <button
          type="button"
          class="qx-nav-link qx-nav-icon"
          :aria-label="isDark ? t('home.switchToLight') : t('home.switchToDark')"
          :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
          @click="toggle"
        >
          <Icon v-if="isDark" name="sun" size="md" />
          <Icon v-else name="moon" size="md" />
        </button>
        <template v-if="!plain">
          <router-link v-if="isAuthenticated" :to="dashboardPath" class="qx-btn qx-btn-sm qx-btn-primary">
            {{ t('home.dashboard') }}
          </router-link>
          <template v-else>
            <router-link to="/login" class="qx-nav-link">{{ t('home.login') }}</router-link>
            <router-link v-if="registrationEnabled" to="/register" class="qx-btn qx-btn-sm qx-btn-primary">
              {{ t('auth.signUp') }}
            </router-link>
          </template>
        </template>
      </nav>
    </div>
  </header>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import { useQxColorScheme, useQxSite } from './useQxSite'

withDefaults(defineProps<{
  /** 登录等页面只保留品牌、语言和深浅色切换。 */
  plain?: boolean
}>(), { plain: false })

const { t } = useI18n()
const {
  siteName,
  siteLogo,
  docUrl,
  isAuthenticated,
  dashboardPath,
  registrationEnabled,
  modelPlazaVisible,
  downloadsVisible
} = useQxSite()
const { isDark, init, toggle } = useQxColorScheme()

onMounted(init)
</script>
