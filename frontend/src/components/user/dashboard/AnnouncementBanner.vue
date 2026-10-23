<!--
  CUSTOM: 仪表盘顶部的公告轮播大横幅（fork 自有组件，DashboardView 里一行接入）。
  数据复用公告 store（登录后 App.vue 已拉取，这里再调一次走节流）。
  每条公告一屏：大标题 + 正文摘要 + 查看详情；多条时自动轮播，悬停/聚焦暂停，
  可点圆点、左右箭头或在手机上滑动切换。点详情标记已读。
  关掉后这一批公告不再显示，公告有增减时重新出现。
  variant="qx"：QxAPI 主题仪表盘用的极简线条样式：细边框、淡蓝渐变底，右侧是线描的铃铛圆环。
-->
<template>
  <section
    v-if="visible"
    class="announcement-banner relative overflow-hidden"
    :class="isQx ? 'qx-banner' : 'rounded-2xl bg-gradient-to-br from-primary-600 via-primary-500 to-sky-500 text-white shadow-lg shadow-primary-500/20 dark:from-primary-800 dark:via-primary-700 dark:to-sky-800 dark:shadow-none'"
    role="region"
    aria-roledescription="carousel"
    :aria-label="t('announcements.title')"
    data-testid="announcement-banner"
    @mouseenter="paused = true"
    @mouseleave="paused = false"
    @focusin="paused = true"
    @focusout="paused = false"
    @touchstart.passive="onTouchStart"
    @touchend="onTouchEnd"
  >
    <!-- 装饰：右侧大图标（qx 为两道细线圆环里的线描铃铛） -->
    <div v-if="isQx" class="qx-banner-mark pointer-events-none" aria-hidden="true" data-testid="announcement-banner-mark">
      <Icon name="bell" size="xl" :stroke-width="1.25" class="qx-banner-bell" />
    </div>
    <template v-else>
      <div class="pointer-events-none absolute -right-16 -top-24 h-72 w-72 rounded-full bg-white/10 blur-2xl" />
      <div class="pointer-events-none absolute -bottom-28 left-1/3 h-64 w-64 rounded-full bg-sky-300/20 blur-3xl" />
      <div class="pointer-events-none absolute right-10 top-1/2 hidden h-28 w-28 -translate-y-1/2 items-center justify-center rounded-full bg-white/10 ring-8 ring-white/5 md:flex">
        <Icon name="bell" size="xl" class="h-14 w-14 text-white/80" />
      </div>
    </template>

    <div class="banner-track flex" :style="{ transform: `translateX(-${index * 100}%)` }">
      <article
        v-for="(item, i) in items"
        :key="item.id"
        class="banner-slide relative flex w-full flex-shrink-0 flex-col justify-center"
        :class="isQx ? 'qx-banner-slide' : 'px-6 pb-12 pt-6 sm:px-10 sm:pb-14 sm:pt-9 md:pr-56'"
        role="group"
        aria-roledescription="slide"
        :aria-label="`${i + 1} / ${items.length}`"
        :aria-hidden="i !== index ? 'true' : undefined"
        :inert="i !== index || undefined"
      >
        <div v-if="isQx" class="qx-banner-row">
          <span class="qx-banner-tag">{{ t('announcements.title') }}</span>
          <span v-if="!item.read_at" class="qx-banner-new">NEW</span>
          <span class="qx-banner-date">{{ formatDateOnly(item.created_at) }}</span>
        </div>
        <div v-else class="flex items-center gap-2 text-xs font-semibold">
          <span class="rounded-full bg-white/20 px-2.5 py-1 tracking-wider">{{ t('announcements.title') }}</span>
          <span v-if="!item.read_at" class="rounded-full bg-amber-400 px-2 py-1 text-amber-950">NEW</span>
          <span class="text-white/70">{{ formatDateOnly(item.created_at) }}</span>
        </div>
        <h2 class="banner-title" :class="isQx ? 'qx-banner-title' : 'mt-3 text-xl font-bold leading-snug sm:text-3xl'">{{ item.title }}</h2>
        <p
          v-if="excerpt(item.content)"
          class="banner-excerpt"
          :class="isQx ? 'qx-banner-excerpt' : 'mt-2 max-w-3xl text-sm text-white/85 sm:text-base'"
        >
          {{ excerpt(item.content) }}
        </p>
        <button
          type="button"
          :class="isQx ? 'qx-banner-cta' : 'mt-5 inline-flex items-center self-start gap-1.5 rounded-lg bg-white px-4 py-2 text-sm font-semibold text-primary-700 shadow-sm transition hover:bg-white/90 dark:text-primary-800'"
          data-testid="announcement-banner-detail"
          @click="openDetail(item)"
        >
          {{ t('announcements.bannerDetail') }}
          <span v-if="isQx" aria-hidden="true">→</span>
          <Icon v-else name="arrowRight" size="sm" />
        </button>
      </article>
    </div>

    <!-- 多条时：左右箭头 + 圆点 -->
    <template v-if="items.length > 1">
      <button
        type="button"
        class="banner-arrow left-2"
        :aria-label="t('announcements.bannerPrev')"
        data-testid="announcement-banner-prev"
        @click="go(index - 1)"
      >
        <Icon name="chevronLeft" size="md" />
      </button>
      <button
        type="button"
        class="banner-arrow right-2"
        :aria-label="t('announcements.bannerNext')"
        data-testid="announcement-banner-next"
        @click="go(index + 1)"
      >
        <Icon name="chevronRight" size="md" />
      </button>
      <div :class="isQx ? 'qx-banner-dots' : 'absolute bottom-4 left-6 flex items-center gap-2 sm:left-10'">
        <button
          v-for="(item, i) in items"
          :key="item.id"
          type="button"
          :class="isQx
            ? ['qx-banner-dot', { 'qx-banner-dot-on': i === index }]
            : ['h-2 rounded-full transition-all', i === index ? 'w-7 bg-white' : 'w-2 bg-white/40 hover:bg-white/70']"
          :aria-label="`${i + 1} / ${items.length}`"
          :aria-current="i === index ? 'true' : undefined"
          data-testid="announcement-banner-dot"
          @click="go(i)"
        />
        <span v-if="isQx" class="qx-banner-progress" data-testid="announcement-banner-progress">{{ pad2(index + 1) }} / {{ pad2(items.length) }}</span>
      </div>
    </template>

    <button
      type="button"
      class="absolute right-3 top-3 rounded-lg p-1.5 transition"
      :class="isQx ? 'qx-banner-close' : 'text-white/70 hover:bg-white/15 hover:text-white'"
      :aria-label="t('common.close')"
      data-testid="announcement-banner-dismiss"
      @click="dismiss"
    >
      <Icon name="x" size="sm" />
    </button>

    <BaseDialog :show="selected !== null" :title="selected?.title ?? ''" width="wide" @close="selected = null">
      <template v-if="selected">
        <time class="block text-xs text-gray-500 dark:text-gray-400">
          {{ formatRelativeWithDateTime(selected.created_at) }}
        </time>
        <div
          class="markdown-body prose prose-sm mt-4 max-w-none dark:prose-invert"
          v-html="renderMarkdown(selected.content)"
        />
      </template>
    </BaseDialog>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { useAnnouncementStore } from '@/stores/announcements'
import { formatDateOnly, formatRelativeWithDateTime } from '@/utils/format'
import type { UserAnnouncement } from '@/types'
import Icon from '@/components/icons/Icon.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import '@/styles/announcement-markdown.css'

const props = withDefaults(defineProps<{ variant?: 'default' | 'qx' }>(), { variant: 'default' })

const DISMISS_KEY = 'dashboard_announcement_banner_dismissed'
const AUTOPLAY_MS = 6000

const { t } = useI18n()
const announcementStore = useAnnouncementStore()

const items = computed(() => announcementStore.announcements)
const signature = computed(() => items.value.map((a) => a.id).sort((a, b) => a - b).join(','))
const dismissedSignature = ref(readDismissed())
const visible = computed(() => items.value.length > 0 && signature.value !== dismissedSignature.value)

const index = ref(0)
const paused = ref(false)
const isQx = computed(() => props.variant === 'qx')

function pad2(n: number): string {
  return String(n).padStart(2, '0')
}
const selected = ref<UserAnnouncement | null>(null)

marked.setOptions({ breaks: true, gfm: true })

function renderMarkdown(content: string): string {
  if (!content) return ''
  return DOMPurify.sanitize(marked.parse(content) as string)
}

/** 正文去掉 Markdown 格式后的纯文本，横幅里最多显示两行。 */
function excerpt(content: string): string {
  const html = renderMarkdown(content)
  if (!html) return ''
  const doc = new DOMParser().parseFromString(html, 'text/html')
  return (doc.body.textContent ?? '').replace(/\s+/g, ' ').trim()
}

function readDismissed(): string {
  try {
    return localStorage.getItem(DISMISS_KEY) ?? ''
  } catch {
    return ''
  }
}

function dismiss() {
  dismissedSignature.value = signature.value
  try {
    localStorage.setItem(DISMISS_KEY, signature.value)
  } catch {
    // 存不了就只在本次页面内隐藏
  }
}

function go(i: number) {
  const n = items.value.length
  if (n === 0) return
  index.value = (i + n) % n
}

function openDetail(item: UserAnnouncement) {
  selected.value = item
  if (!item.read_at) announcementStore.markAsRead(item.id)
}

let touchX: number | null = null
function onTouchStart(e: TouchEvent) {
  touchX = e.touches[0]?.clientX ?? null
}
function onTouchEnd(e: TouchEvent) {
  const endX = e.changedTouches[0]?.clientX
  if (touchX === null || endX === undefined) return
  const dx = endX - touchX
  touchX = null
  if (Math.abs(dx) > 40) go(index.value + (dx < 0 ? 1 : -1))
}

let timer: ReturnType<typeof setInterval> | null = null
function tick() {
  if (!paused.value && selected.value === null && items.value.length > 1) go(index.value + 1)
}

// 公告列表变化（新增 / 下架）时回到第一条，避免下标越界
watch(signature, () => {
  index.value = 0
})

onMounted(() => {
  announcementStore.fetchAnnouncements()
  timer = setInterval(tick, AUTOPLAY_MS)
})

onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})
</script>

<style scoped>
.banner-track {
  transition: transform 0.6s cubic-bezier(0.22, 1, 0.36, 1);
}
.banner-title,
.banner-excerpt {
  display: -webkit-box;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.banner-title {
  -webkit-line-clamp: 2;
}
.banner-excerpt {
  -webkit-line-clamp: 2;
}
.banner-arrow {
  position: absolute;
  top: 50%;
  display: flex;
  height: 36px;
  width: 36px;
  align-items: center;
  justify-content: center;
  border-radius: 9999px;
  background: rgb(255 255 255 / 0.15);
  color: #fff;
  opacity: 0;
  transform: translateY(-50%);
  transition: opacity 0.2s, background-color 0.2s;
}
.banner-arrow:hover {
  background: rgb(255 255 255 / 0.3);
}
.announcement-banner:hover .banner-arrow,
.banner-arrow:focus-visible {
  opacity: 1;
}
/* ===== variant="qx"：QxAPI 主题的极简线条样式（1px 细线、淡蓝渐变底，颜色取 themes/qx/app/tokens.css） ===== */
.qx-banner {
  --qxb-mono: var(--qx-mono, ui-monospace, SFMono-Regular, Menlo, Consolas, 'Liberation Mono', monospace);
  --qxb-page: var(--qx-page, #fff);
  --qxb-blue: var(--qx-blue, #2340d0);
  --qxb-line: color-mix(in srgb, var(--qxb-blue) 18%, var(--qx-line, #e5e8ee));
  border: 1px solid var(--qxb-line);
  border-radius: 12px;
  background: linear-gradient(
    115deg,
    color-mix(in srgb, var(--qxb-blue) 9%, var(--qxb-page)) 0%,
    color-mix(in srgb, var(--qxb-blue) 5%, var(--qxb-page)) 55%,
    color-mix(in srgb, var(--qx-cyan, #0891b2) 7%, var(--qxb-page)) 100%
  );
  color: var(--qx-ink, #0f172a);
}
/* 右侧：两道细线圆环（外圈虚线）里的线描铃铛 */
.qx-banner-mark {
  position: absolute;
  top: 50%;
  right: 56px;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 132px;
  height: 132px;
  border: 1px dashed var(--qxb-line);
  border-radius: 999px;
  transform: translateY(-50%);
}
.qx-banner-mark::before {
  content: '';
  position: absolute;
  inset: 18px;
  border: 1px solid var(--qxb-line);
  border-radius: 999px;
}
.qx-banner-bell {
  width: 40px;
  height: 40px;
  color: var(--qxb-blue);
}
.qx-banner-slide {
  padding: 28px 240px 52px 32px;
}
.qx-banner-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  font-size: 12px;
  line-height: 1.5;
}
.qx-banner-tag,
.qx-banner-new {
  padding: 1px 10px;
  border: 1px solid currentColor;
  border-radius: 999px;
  font-weight: 600;
}
.qx-banner-tag {
  color: var(--qxb-blue);
}
.qx-banner-new {
  color: var(--qx-amber, #d97706);
  font-family: var(--qxb-mono);
  font-size: 11px;
}
.qx-banner-date {
  color: var(--qx-muted, #7a8699);
  font-family: var(--qxb-mono);
}
.qx-banner-title {
  margin-top: 14px;
  font-size: 28px;
  font-weight: 700;
  letter-spacing: -0.01em;
  line-height: 1.3;
  text-wrap: balance;
}
.qx-banner-excerpt {
  max-width: 46em;
  margin-top: 8px;
  color: var(--qx-text, #475569);
  font-size: 15px;
  line-height: 1.6;
}
.qx-banner-cta {
  display: inline-flex;
  align-items: center;
  align-self: flex-start;
  gap: 8px;
  height: 36px;
  margin-top: 20px;
  padding: 0 16px;
  border: 1px solid var(--qx-ink, #0f172a);
  border-radius: 8px;
  color: var(--qx-ink, #0f172a);
  font-size: 14px;
  font-weight: 600;
  transition: border-color 0.15s, color 0.15s;
}
.qx-banner-cta:hover {
  border-color: var(--qxb-blue);
  color: var(--qxb-blue);
}
/* 切换：一组细横线，当前一条加长变蓝 */
.qx-banner-dots {
  position: absolute;
  z-index: 1;
  left: 32px;
  bottom: 22px;
  display: flex;
  align-items: center;
  gap: 6px;
}
.qx-banner-dot {
  width: 14px;
  height: 2px;
  border-radius: 2px;
  background: var(--qxb-line);
  transition: width 0.3s, background-color 0.2s;
}
.qx-banner-dot:hover {
  background: var(--qx-muted, #7a8699);
}
.qx-banner-dot-on,
.qx-banner-dot-on:hover {
  width: 32px;
  background: var(--qxb-blue);
}
.qx-banner-progress {
  margin-left: 10px;
  color: var(--qx-muted, #7a8699);
  font-family: var(--qxb-mono);
  font-size: 11px;
}
.qx-banner .banner-arrow {
  z-index: 2;
  border: 1px solid var(--qxb-line);
  background: var(--qx-page, #fff);
  color: var(--qx-muted, #7a8699);
}
.qx-banner .banner-arrow:hover {
  border-color: var(--qxb-blue);
  background: var(--qx-page, #fff);
  color: var(--qxb-blue);
}
.qx-banner-close {
  z-index: 2;
  color: var(--qx-muted, #7a8699);
}
.qx-banner-close:hover {
  color: var(--qx-ink, #0f172a);
}
@media (max-width: 760px) {
  .qx-banner-mark {
    display: none;
  }
  .qx-banner-slide {
    padding: 22px 20px 44px;
  }
  .qx-banner-title {
    font-size: 21px;
  }
  .qx-banner-excerpt {
    font-size: 13.5px;
  }
  .qx-banner-dots {
    left: 20px;
    bottom: 18px;
  }
}
@media (hover: none) {
  .banner-arrow {
    display: none;
  }
}
@media (prefers-reduced-motion: reduce) {
  .banner-track {
    transition: none;
  }
}
</style>
