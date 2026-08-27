<script setup lang="ts">
import {
  Bell,
  CheckCircle2,
  CircleDot,
  ClipboardList,
  Cloud,
  Globe2,
  Plus,
  RefreshCw,
  Search,
  Settings,
  UserRound,
  UsersRound,
} from '@lucide/vue'
import type { User } from '@work-report/shared'
import type { TaskScope } from '../stores/app'

defineProps<{
  user: User
  scope: TaskScope
  pendingCount: number
  unreadCount: number
  refreshing: boolean
  serverUrl: string
  activePane: 'detail' | 'create' | 'settings' | 'notifications'
}>()

const emit = defineEmits<{
  scope: [scope: TaskScope]
  create: []
  refresh: []
  settings: []
  notifications: []
  openWeb: []
}>()

const scopes: Array<{ value: TaskScope; label: string; icon: typeof UserRound }> = [
  { value: 'assigned', label: '分配给我', icon: CircleDot },
  { value: 'created', label: '我创建的', icon: UserRound },
  { value: 'team', label: '团队可见', icon: UsersRound },
]
</script>

<template>
  <aside class="app-sidebar">
    <div class="drag-region" data-tauri-drag-region></div>
    <div class="brand-row">
      <div class="brand-mark">W</div>
      <div>
        <strong>Workline</strong>
        <span>快捷工作台</span>
      </div>
    </div>

    <button class="new-task-button" @click="emit('create')">
      <Plus :size="17" stroke-width="2.4" />
      新建任务
      <kbd>⌘N</kbd>
    </button>

    <nav class="sidebar-nav">
      <p class="nav-label">任务</p>
      <button
        v-for="item in scopes"
        :key="item.value"
        :class="{ active: scope === item.value }"
        @click="emit('scope', item.value)"
      >
        <component :is="item.icon" :size="17" />
        <span>{{ item.label }}</span>
        <b v-if="item.value === 'assigned'">{{ pendingCount }}</b>
      </button>

      <p class="nav-label nav-spacer">工作台</p>
      <button :class="{ active: activePane === 'notifications' }" @click="emit('notifications')">
        <Bell :size="17" />
        <span>通知</span>
        <b v-if="unreadCount" class="alert">{{ unreadCount }}</b>
      </button>
      <button @click="emit('openWeb')">
        <Globe2 :size="17" />
        <span>打开网页版</span>
      </button>
    </nav>

    <div class="sidebar-shortcuts">
      <div><ClipboardList :size="14" /><span>新建任务</span><kbd>⌘N</kbd></div>
      <div><Search :size="14" /><span>搜索任务</span><kbd>⌘K</kbd></div>
    </div>

    <div class="sidebar-footer">
      <div class="connection-line">
        <Cloud :size="14" />
        <span class="connection-pulse"></span>
        <span class="truncate">{{ serverUrl.replace(/^https?:\/\//, '') }}</span>
      </div>
      <div class="user-row">
        <div class="avatar">
          <img v-if="user.avatar" :src="user.avatar" alt="" />
          <span v-else>{{ user.name.slice(0, 1) }}</span>
          <i><CheckCircle2 :size="10" /></i>
        </div>
        <div class="user-meta">
          <strong>{{ user.name }}</strong>
          <span>{{ user.email || '已安全绑定' }}</span>
        </div>
        <button class="icon-button" title="刷新" @click="emit('refresh')">
          <RefreshCw :class="{ spin: refreshing }" :size="15" />
        </button>
        <button class="icon-button" title="设置" @click="emit('settings')">
          <Settings :size="15" />
        </button>
      </div>
    </div>
  </aside>
</template>
