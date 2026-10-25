<template>
  <div class="mx-auto max-w-4xl" data-testid="client-downloads">
    <!-- 标题 -->
    <div class="text-center">
      <h1 class="text-3xl font-bold tracking-tight text-gray-900 dark:text-white sm:text-4xl">
        {{ t('downloads.title') }}
      </h1>
      <p class="mt-3 text-sm text-gray-500 dark:text-gray-400 sm:text-base">{{ t('downloads.lead') }}</p>
      <p
        v-if="deviceUnsupported"
        class="mt-3 text-xs text-amber-600 dark:text-amber-400"
        data-testid="client-downloads-unsupported"
      >
        {{ t('downloads.deviceUnsupported') }}
      </p>
    </div>

    <div
      v-if="platforms.length === 0"
      class="card mt-10 px-6 py-16 text-center text-sm text-gray-500 dark:text-gray-400"
    >
      {{ t('downloads.empty') }}
    </div>

    <template v-else>
      <!-- 系统标签：默认选中访客的系统。手机上等分整行（图标在上、文字在下，最多三列，四个时排成 2×2），sm 起为居中横排 -->
      <div class="mt-10 flex justify-center">
        <div
          role="tablist"
          :aria-label="t('downloads.platformTabs')"
          class="grid w-full gap-1.5 rounded-2xl bg-gray-100 p-1.5 dark:bg-dark-800 sm:inline-flex sm:w-auto sm:flex-wrap sm:justify-center"
          :style="{ gridTemplateColumns: `repeat(${mobileTabColumns}, minmax(0, 1fr))` }"
        >
          <button
            v-for="platform in platforms"
            :key="platform"
            type="button"
            role="tab"
            :aria-selected="platform === selected"
            class="flex min-w-0 flex-col items-center justify-center gap-1 rounded-xl px-1 py-2.5 text-sm font-semibold transition-colors min-[360px]:px-2 min-[360px]:text-[15px] sm:flex-row sm:gap-2.5 sm:px-8 sm:py-3.5 sm:text-lg"
            :class="
              platform === selected
                ? 'bg-primary-600 text-white shadow-sm'
                : 'text-gray-600 hover:bg-white hover:text-gray-900 dark:text-gray-300 dark:hover:bg-dark-700 dark:hover:text-white'
            "
            :data-testid="`client-downloads-tab-${platform}`"
            @click="selected = platform"
          >
            <PlatformIcon :platform="platform" size="lg" />
            <span class="max-w-full truncate">{{ t(`downloads.platforms.${platform}`) }}</span>
          </button>
        </div>
      </div>
      <p class="mt-3 text-center text-sm text-gray-400 dark:text-gray-500">
        {{ t('downloads.switchHint') }}
      </p>

      <div class="mt-6 space-y-4" role="tabpanel">
        <article
          v-for="client in cards"
          :key="client.id || client.name"
          class="card flex flex-col gap-5 p-5 sm:flex-row sm:items-center sm:p-6"
          data-testid="client-download-card"
        >
          <div class="flex min-w-0 flex-1 items-start gap-4">
            <span
              class="flex h-12 w-12 shrink-0 items-center justify-center overflow-hidden rounded-xl bg-gray-100 text-lg font-semibold text-gray-700 ring-1 ring-gray-200 dark:bg-dark-700 dark:text-gray-200 dark:ring-dark-600"
            >
              <img v-if="client.icon_url" :src="client.icon_url" alt="" class="h-full w-full object-contain" />
              <template v-else>{{ initial(client.name) }}</template>
            </span>
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2">
                <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ client.name }}</h2>
                <span
                  v-if="client.version"
                  class="rounded-md bg-gray-100 px-1.5 py-0.5 font-mono text-xs text-gray-600 dark:bg-dark-700 dark:text-gray-300"
                >
                  {{ client.version }}
                </span>
                <a
                  v-if="client.homepage_url"
                  :href="client.homepage_url"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="inline-flex items-center gap-0.5 text-xs text-gray-500 hover:text-primary-600 dark:text-gray-400 dark:hover:text-primary-400"
                >
                  {{ t('downloads.homepage') }}
                  <Icon name="externalLink" size="xs" />
                </a>
              </div>
              <p v-if="client.description" class="mt-1 text-sm leading-relaxed text-gray-500 dark:text-gray-400">
                {{ client.description }}
              </p>
            </div>
          </div>

          <div v-if="client.primary" class="flex shrink-0 flex-col gap-2 sm:items-end">
            <a
              :href="client.primary.url"
              target="_blank"
              rel="noopener noreferrer"
              class="btn btn-primary justify-center gap-2 px-5"
              data-testid="client-download-recommended"
            >
              <Icon name="download" size="sm" />
              <span>{{ t('downloads.downloadFor', { platform: platformName }) }}</span>
              <span v-if="archName(client.primary)" class="font-normal opacity-80">· {{ archName(client.primary) }}</span>
            </a>
            <div v-if="client.others.length > 0" class="flex flex-wrap items-center gap-1.5 sm:justify-end">
              <span class="text-xs text-gray-400 dark:text-gray-500">{{ t('downloads.otherBuilds') }}</span>
              <a
                v-for="link in client.others"
                :key="link.url"
                :href="link.url"
                target="_blank"
                rel="noopener noreferrer"
                class="inline-flex items-center gap-1 rounded-md border border-gray-200 px-2 py-1 text-xs text-gray-600 transition-colors hover:border-primary-300 hover:text-primary-600 dark:border-dark-600 dark:text-gray-300 dark:hover:border-primary-500/60 dark:hover:text-primary-300"
                data-testid="client-download-other"
              >
                <Icon name="download" size="xs" />
                {{ link.label }}
              </a>
            </div>
          </div>
          <p
            v-else
            class="shrink-0 text-sm text-gray-400 dark:text-gray-500"
            data-testid="client-download-unavailable"
          >
            {{ t('downloads.notAvailable', { platform: platformName }) }}
          </p>
        </article>
      </div>

      <p v-if="archHint" class="mt-4 text-center text-xs text-gray-500 dark:text-gray-400" data-testid="client-downloads-hint">
        {{ archHint }}
      </p>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from './PlatformIcon.vue'
import type { ClientDownload, ClientDownloadLink, ClientDownloadPlatform } from '@/types'
import {
  availablePlatforms,
  defaultPlatform,
  detectClientEnv,
  pickRecommendedLink,
  refineClientArch,
  type ClientEnv
} from '@/utils/clientDownloads'

const props = defineProps<{ clients: ClientDownload[] }>()

const { t, te } = useI18n()

const env = ref<ClientEnv>(detectClientEnv())
const platforms = computed(() => availablePlatforms(props.clients))
const selected = ref<ClientDownloadPlatform | null>(defaultPlatform(platforms.value, env.value))

onMounted(async () => {
  env.value = await refineClientArch(env.value)
})

// 设置晚到（公共设置异步加载）或被管理员改掉时，重新按访客系统选中
watch(platforms, (list) => {
  if (!selected.value || !list.includes(selected.value)) selected.value = defaultPlatform(list, env.value)
})

const deviceUnsupported = computed(
  () => env.value.platform !== null && platforms.value.length > 0 && !platforms.value.includes(env.value.platform)
)
// 手机上的标签列数：四个系统时 2×2，否则最多三列一行
const mobileTabColumns = computed(() => (platforms.value.length === 4 ? 2 : Math.min(platforms.value.length, 3)))
const platformName = computed(() => (selected.value ? t(`downloads.platforms.${selected.value}`) : ''))

const cards = computed(() =>
  props.clients.map((client) => {
    const links = client.links.filter((link) => link.platform === selected.value)
    // 只有选中的就是访客自己的系统时，已识别的架构才有意义
    const target: ClientEnv = {
      platform: selected.value,
      arch: selected.value === env.value.platform ? env.value.arch : null
    }
    const index = pickRecommendedLink(links, target)
    const primary = index >= 0 ? links[index] : null
    return { ...client, primary, others: links.filter((_, i) => i !== index) }
  })
)

/** 选中的系统里有客户端提供多个架构时，提示怎么选。 */
const archHint = computed(() => {
  const key = `downloads.archHints.${selected.value}`
  return cards.value.some((card) => card.others.length > 0) && te(key) ? t(key) : ''
})

function archLabel(platform: ClientDownloadPlatform, arch: string): string {
  const macKey = `downloads.archesMac.${arch}`
  return platform === 'macos' && te(macKey) ? t(macKey) : t(`downloads.arches.${arch}`)
}

function archName(link: ClientDownloadLink): string {
  return link.arch ? archLabel(link.platform, link.arch) : ''
}

function initial(name: string): string {
  return Array.from(name.trim())[0]?.toUpperCase() ?? '?'
}
</script>
