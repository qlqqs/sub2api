<template>
  <div class="card" data-testid="client-downloads-settings">
    <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
        {{ t('admin.settings.clientDownloads.title') }}
      </h2>
      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
        {{ t('admin.settings.clientDownloads.description') }}
      </p>
    </div>

    <div class="space-y-4 p-6">
      <div class="flex items-center justify-between gap-4">
        <div>
          <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.settings.clientDownloads.enabled') }}
          </label>
          <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.settings.clientDownloads.enabledHint') }}
          </p>
        </div>
        <Toggle v-model="enabled" data-testid="client-downloads-enabled" />
      </div>

      <template v-if="enabled">
        <p class="text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.settings.clientDownloads.recommendHint') }}
        </p>

        <p
          v-if="clients.length === 0"
          class="rounded-lg bg-gray-50 px-4 py-3 text-sm text-gray-500 dark:bg-dark-800 dark:text-gray-400"
        >
          {{ t('admin.settings.clientDownloads.empty') }}
        </p>

        <div
          v-for="(client, index) in clients"
          :key="index"
          class="rounded-lg border border-gray-200 p-4 dark:border-dark-600"
          data-testid="client-downloads-client"
        >
          <div class="mb-3 flex items-center justify-between">
            <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.settings.clientDownloads.clientLabel', { n: index + 1 }) }}
              <span v-if="client.name" class="ml-1 text-gray-500 dark:text-gray-400">· {{ client.name }}</span>
            </span>
            <div class="flex items-center gap-1">
              <button
                v-if="index > 0"
                type="button"
                class="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-700"
                :title="t('admin.settings.clientDownloads.moveUp')"
                @click="move(clients, index, -1)"
              >
                <Icon name="chevronUp" size="sm" />
              </button>
              <button
                v-if="index < clients.length - 1"
                type="button"
                class="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-700"
                :title="t('admin.settings.clientDownloads.moveDown')"
                @click="move(clients, index, 1)"
              >
                <Icon name="chevronDown" size="sm" />
              </button>
              <button
                type="button"
                class="rounded p-1 text-red-400 hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20"
                :title="t('admin.settings.clientDownloads.remove')"
                data-testid="client-downloads-remove-client"
                @click="clients.splice(index, 1)"
              >
                <Icon name="trash" size="sm" />
              </button>
            </div>
          </div>

          <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <div>
              <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                {{ t('admin.settings.clientDownloads.name') }}
              </label>
              <input
                v-model="client.name"
                type="text"
                maxlength="50"
                class="input text-sm"
                :placeholder="t('admin.settings.clientDownloads.namePlaceholder')"
              />
            </div>
            <div>
              <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                {{ t('admin.settings.clientDownloads.version') }}
              </label>
              <input
                v-model="client.version"
                type="text"
                maxlength="32"
                class="input font-mono text-sm"
                :placeholder="t('admin.settings.clientDownloads.versionPlaceholder')"
              />
            </div>
            <div class="sm:col-span-2">
              <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                {{ t('admin.settings.clientDownloads.descriptionLabel') }}
              </label>
              <input
                v-model="client.description"
                type="text"
                maxlength="300"
                class="input text-sm"
                :placeholder="t('admin.settings.clientDownloads.descriptionPlaceholder')"
              />
            </div>
            <div>
              <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                {{ t('admin.settings.clientDownloads.iconUrl') }}
              </label>
              <input
                v-model="client.icon_url"
                type="url"
                class="input font-mono text-sm"
                :placeholder="t('admin.settings.clientDownloads.iconUrlPlaceholder')"
              />
            </div>
            <div>
              <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                {{ t('admin.settings.clientDownloads.homepageUrl') }}
              </label>
              <input
                v-model="client.homepage_url"
                type="url"
                class="input font-mono text-sm"
                :placeholder="t('admin.settings.clientDownloads.homepageUrlPlaceholder')"
              />
            </div>
          </div>

          <div class="mt-4">
            <div class="mb-2 text-xs font-medium text-gray-600 dark:text-gray-400">
              {{ t('admin.settings.clientDownloads.links') }}
            </div>
            <div class="space-y-2">
              <div
                v-for="(link, linkIndex) in client.links"
                :key="linkIndex"
                class="grid grid-cols-2 items-center gap-2 rounded-lg bg-gray-50 p-2 dark:bg-dark-800 lg:grid-cols-[11rem_8.5rem_6.5rem_minmax(0,1fr)_5rem]"
                data-testid="client-downloads-link"
              >
                <input
                  v-model="link.label"
                  type="text"
                  maxlength="50"
                  class="input text-sm"
                  :aria-label="t('admin.settings.clientDownloads.linkLabel')"
                  :placeholder="t('admin.settings.clientDownloads.linkLabelPlaceholder')"
                />
                <select
                  v-model="link.platform"
                  class="input text-sm"
                  :aria-label="t('admin.settings.clientDownloads.platform')"
                >
                  <option v-for="platform in CLIENT_DOWNLOAD_PLATFORMS" :key="platform" :value="platform">
                    {{ t(`downloads.platforms.${platform}`) }}
                  </option>
                </select>
                <select v-model="link.arch" class="input text-sm" :aria-label="t('admin.settings.clientDownloads.arch')">
                  <option v-for="arch in CLIENT_DOWNLOAD_ARCHES" :key="arch" :value="arch">
                    {{ arch ? t(`downloads.arches.${arch}`) : t('admin.settings.clientDownloads.archAny') }}
                  </option>
                </select>
                <input
                  v-model="link.url"
                  type="url"
                  class="input col-span-2 font-mono text-sm lg:col-span-1"
                  :aria-label="t('admin.settings.clientDownloads.url')"
                  placeholder="https://"
                />
                <div class="col-span-2 flex items-center justify-end gap-1 lg:col-span-1">
                  <button
                    v-if="linkIndex > 0"
                    type="button"
                    class="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-700"
                    :title="t('admin.settings.clientDownloads.moveUp')"
                    @click="move(client.links, linkIndex, -1)"
                  >
                    <Icon name="chevronUp" size="sm" />
                  </button>
                  <button
                    v-if="linkIndex < client.links.length - 1"
                    type="button"
                    class="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-700"
                    :title="t('admin.settings.clientDownloads.moveDown')"
                    @click="move(client.links, linkIndex, 1)"
                  >
                    <Icon name="chevronDown" size="sm" />
                  </button>
                  <button
                    type="button"
                    class="rounded p-1 text-red-400 hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20"
                    :title="t('admin.settings.clientDownloads.remove')"
                    @click="client.links.splice(linkIndex, 1)"
                  >
                    <Icon name="trash" size="sm" />
                  </button>
                </div>
              </div>
            </div>
            <button
              type="button"
              class="mt-2 inline-flex items-center gap-1 text-sm text-primary-600 hover:text-primary-700 disabled:cursor-not-allowed disabled:opacity-50 dark:text-primary-400"
              :disabled="client.links.length >= MAX_CLIENT_DOWNLOAD_LINKS"
              data-testid="client-downloads-add-link"
              @click="addLink(client)"
            >
              <Icon name="plus" size="sm" />
              {{ t('admin.settings.clientDownloads.addLink') }}
            </button>
          </div>
        </div>

        <button
          type="button"
          class="flex w-full items-center justify-center gap-2 rounded-lg border-2 border-dashed border-gray-300 py-3 text-sm text-gray-500 transition-colors hover:border-primary-400 hover:text-primary-600 disabled:cursor-not-allowed disabled:opacity-50 dark:border-dark-600 dark:text-gray-400 dark:hover:border-primary-500 dark:hover:text-primary-400"
          :disabled="clients.length >= MAX_CLIENT_DOWNLOADS"
          data-testid="client-downloads-add-client"
          @click="addClient"
        >
          <Icon name="plus" size="sm" />
          {{ t('admin.settings.clientDownloads.addClient') }}
        </button>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
/** CUSTOM: 系统设置「通用」页的客户端下载配置，随整页设置一起保存，校验由后端负责。 */
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import Toggle from '@/components/common/Toggle.vue'
import type { ClientDownload } from '@/types'
import {
  CLIENT_DOWNLOAD_ARCHES,
  CLIENT_DOWNLOAD_PLATFORMS,
  MAX_CLIENT_DOWNLOADS,
  MAX_CLIENT_DOWNLOAD_LINKS
} from '@/utils/clientDownloads'

const enabled = defineModel<boolean>('enabled', { required: true })
const clients = defineModel<ClientDownload[]>({ required: true })

const { t } = useI18n()

function move<T>(list: T[], index: number, delta: -1 | 1) {
  const target = index + delta
  if (target < 0 || target >= list.length) return
  ;[list[index], list[target]] = [list[target], list[index]]
}

function addLink(client: ClientDownload) {
  const previous = client.links[client.links.length - 1]
  client.links.push({ label: '', platform: previous?.platform ?? 'windows', arch: '', url: '' })
}

function addClient() {
  // id 留空由后端生成
  clients.value.push({
    id: '',
    name: '',
    description: '',
    icon_url: '',
    version: '',
    homepage_url: '',
    links: [{ label: '', platform: 'windows', arch: '', url: '' }]
  })
}
</script>
