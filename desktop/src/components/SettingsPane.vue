<script setup lang="ts">
import { reactive } from 'vue'
import {
  Check,
  Command,
  ExternalLink,
  KeyRound,
  Laptop,
  Link2,
  LogOut,
  Moon,
  Rocket,
  Save,
  ShieldCheck,
  Sun,
  X,
} from '@lucide/vue'
import type { User } from '@work-report/shared'
import type { DesktopPreferences } from '../lib/runtime'

const props = defineProps<{ preferences: DesktopPreferences; user: User; theme: 'light' | 'dark' }>()
const emit = defineEmits<{
  close: []
  save: [preferences: DesktopPreferences]
  disconnect: []
  openWeb: []
  theme: [theme: 'light' | 'dark']
}>()

const form = reactive<DesktopPreferences>({ ...props.preferences })
</script>

<template>
  <section class="settings-pane">
    <header class="create-header" data-tauri-drag-region>
      <div>
        <span class="eyebrow"><ShieldCheck :size="13" /> 本机偏好</span>
        <h2>客户端设置</h2>
      </div>
      <button class="icon-button" title="关闭" @click="emit('close')"><X :size="18" /></button>
    </header>

    <div class="settings-scroll">
      <section class="settings-section account-card">
        <div class="settings-user-avatar">{{ user.name.slice(0, 1) }}</div>
        <div>
          <strong>{{ user.name }}</strong>
          <span>{{ user.email || 'work-report 用户' }}</span>
        </div>
        <span class="secure-badge"><Check :size="12" /> 已绑定</span>
      </section>

      <section class="settings-section">
        <h3><Link2 :size="15" /> 连接</h3>
        <label>
          <span>平台地址</span>
          <input v-model="form.server_url" type="url" />
        </label>
        <div class="key-status">
          <KeyRound :size="15" />
          <div><strong>API Key 本机加密存储</strong><span>与本机绑定，仅当前用户可读</span></div>
          <ShieldCheck :size="17" />
        </div>
        <button class="soft-button" @click="emit('openWeb')">管理网页版 API Key <ExternalLink :size="14" /></button>
      </section>

      <section class="settings-section">
        <h3><Command :size="15" /> 快捷键</h3>
        <div class="shortcut-setting">
          <div><strong>新建任务</strong><span>窗口激活时可用</span></div>
          <kbd>⌘N</kbd>
        </div>
        <div class="shortcut-setting">
          <div><strong>搜索任务</strong><span>聚焦左侧搜索框</span></div>
          <kbd>⌘K</kbd>
        </div>
        <div class="shortcut-setting">
          <div><strong>唤起窗口</strong><span>请交给 Manico 等第三方启动器</span></div>
          <kbd>自定义</kbd>
        </div>
      </section>

      <section class="settings-section">
        <h3><Laptop :size="15" /> 外观与启动</h3>
        <div class="theme-choice">
          <button :class="{ active: theme === 'light' }" @click="emit('theme', 'light')"><Sun :size="16" /> 浅色</button>
          <button :class="{ active: theme === 'dark' }" @click="emit('theme', 'dark')"><Moon :size="16" /> 深色</button>
        </div>
        <label class="switch-setting">
          <div><Rocket :size="16" /><span><strong>登录时自动启动</strong><small>在菜单栏静默待命</small></span></div>
          <input v-model="form.launch_at_startup" type="checkbox" />
        </label>
      </section>

      <section class="settings-section danger-section">
        <button @click="emit('disconnect')"><LogOut :size="15" /> 解除本机绑定</button>
        <span>会删除本机的加密凭据，不会删除网页版中的 Key。</span>
      </section>
    </div>

    <footer class="settings-footer">
      <button class="soft-button" @click="emit('close')">取消</button>
      <button class="primary-button" @click="emit('save', { ...form })"><Save :size="15" /> 保存设置</button>
    </footer>
  </section>
</template>
