<script setup lang="ts">
import { computed } from 'vue'
import dayjs from 'dayjs'
import { ArrowUpRight, Bell, BellOff, CheckCheck, LoaderCircle, MessageSquare, X } from '@lucide/vue'
import type { Notification } from '@work-report/shared'

const props = defineProps<{
  notifications: Notification[]
  loading?: boolean
}>()
const emit = defineEmits<{
  close: []
  open: [item: Notification]
  readAll: []
  openWeb: []
}>()

const unread = computed(() => props.notifications.filter((item) => !item.read).length)

function relativeTime(value: string) {
  const time = dayjs(value)
  const minutes = dayjs().diff(time, 'minute')
  if (minutes < 1) return '刚刚'
  if (minutes < 60) return `${minutes} 分钟前`
  if (minutes < 60 * 24) return `${Math.floor(minutes / 60)} 小时前`
  if (minutes < 60 * 24 * 7) return `${Math.floor(minutes / (60 * 24))} 天前`
  return time.format('MM-DD HH:mm')
}
</script>

<template>
  <section class="create-pane">
    <header class="create-header" data-tauri-drag-region>
      <div>
        <span class="eyebrow"><Bell :size="13" /> 消息中心</span>
        <h2>通知<em v-if="unread"> · {{ unread }} 条未读</em></h2>
      </div>
      <div class="header-actions">
        <button class="soft-button" :disabled="!unread" @click="emit('readAll')">
          <CheckCheck :size="14" /> 全部已读
        </button>
        <button class="icon-button" title="关闭" @click="emit('close')"><X :size="18" /></button>
      </div>
    </header>

    <div class="notification-scroll">
      <div v-if="loading && !notifications.length" class="notification-empty">
        <LoaderCircle class="spin" :size="20" />
        <p>正在加载通知…</p>
      </div>
      <div v-else-if="!notifications.length" class="notification-empty">
        <BellOff :size="22" />
        <p>暂时没有通知，安心推进手上的事。</p>
      </div>
      <button
        v-for="item in notifications"
        v-else
        :key="item.id"
        class="notification-card"
        :class="{ unread: !item.read }"
        @click="emit('open', item)"
      >
        <span class="notification-dot"></span>
        <div class="notification-body">
          <strong>{{ item.title }}</strong>
          <p v-if="item.content">{{ item.content }}</p>
          <div class="notification-meta">
            <span>{{ relativeTime(item.created_at) }}</span>
            <span v-if="item.work_item?.title" class="truncate">
              <MessageSquare :size="12" /> {{ item.work_item.title }}
            </span>
          </div>
        </div>
      </button>
    </div>

    <div class="create-footer notification-footer">
      <p>点击通知可直接打开对应任务</p>
      <button class="soft-button" @click="emit('openWeb')">网页版查看 <ArrowUpRight :size="14" /></button>
    </div>
  </section>
</template>
