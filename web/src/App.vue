<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NConfigProvider, darkTheme, NPopover, NBadge, NMessageProvider, NDialogProvider, zhCN, dateZhCN } from 'naive-ui'
import { useAuthStore } from './stores/auth'
import { api } from './api'
import UserAvatar from './components/UserAvatar.vue'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()

const isDark = ref(localStorage.getItem('theme') === 'dark')
function toggleTheme() {
  isDark.value = !isDark.value
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
  applyTheme()
}
function applyTheme() {
  document.documentElement.classList.toggle('dark', isDark.value)
}
applyTheme()

const theme = computed(() => (isDark.value ? darkTheme : null))

const navs = [
  { path: '/board', label: '看板' },
  { path: '/tasks', label: '任务' },
  { path: '/ai-analysis', label: 'AI分析' },
  { path: '/stats', label: '统计' },
]

const isAuthPage = computed(() => ['/login', '/callback', '/change-password'].includes(route.path))

// 未读通知轮询
const unread = ref(0)
let timer: number | undefined
async function refreshUnread() {
  if (!auth.isLoggedIn) return
  try {
    const { data } = await api.unreadCount()
    unread.value = data.count
  } catch { /* ignore */ }
}
onMounted(() => {
  refreshUnread()
  timer = window.setInterval(refreshUnread, 30000)
})
onUnmounted(() => clearInterval(timer))
// 登录完成后（如 /callback 页 setSession）立即刷新一次未读数，不必等轮询
watch(() => auth.isLoggedIn, (v) => {
  if (v) refreshUnread()
})

const showNotify = ref(false)
const notifications = ref<any[]>([])
// 通知标签页：默认展示未读
const notifyTab = ref<'unread' | 'read'>('unread')
const filteredNotifications = computed(() =>
  notifications.value.filter((n) => (notifyTab.value === 'unread' ? !n.read : n.read)),
)
const unreadNotifications = computed(() => notifications.value.filter((n) => !n.read))
// trigger="click" 由 naive-ui 管理开关（含点击外部关闭），打开时加载并重置到未读页
watch(showNotify, async (v) => {
  if (!v) return
  notifyTab.value = 'unread'
  try {
    const { data } = await api.notifications()
    notifications.value = data
  } catch { /* ignore */ }
})

// 点击通知：新标签页打开对应任务详情（提及类通知带评论锚点，跳转后直接定位闪烁到该评论），并标记该条已读
async function goToItem(n: any) {
  if (!n.work_item_id) return
  showNotify.value = false
  const q = n.comment_id ? `&comment=${n.comment_id}` : ''
  window.open(`/board?item=${n.work_item_id}${q}`, '_blank')
  if (!n.read) {
    try {
      await api.markRead(n.id)
      n.read = true
      unread.value = Math.max(0, unread.value - 1)
    } catch { /* ignore */ }
  }
}

// 退出模拟身份，恢复管理员会话（store 内会整页刷新）
async function exitImpersonation() {
  await auth.exitImpersonation()
}
</script>

<template>
  <n-config-provider :theme="theme" :locale="zhCN" :date-locale="dateZhCN">
    <n-message-provider>
      <n-dialog-provider>
        <div v-if="isAuthPage" class="min-h-screen">
          <router-view />
        </div>

    <div v-else class="min-h-screen text-slate-950 dark:text-slate-100">
      <!-- 模拟身份提示横幅 -->
      <div
        v-if="auth.impersonating"
        class="flex items-center justify-center gap-3 bg-amber-400 px-4 py-1.5 text-xs font-medium text-amber-950"
      >
        <span>正在以「{{ auth.user?.name }}」的身份访问平台，操作将以该成员身份记录</span>
        <button class="rounded bg-amber-950/10 px-2 py-0.5 font-bold hover:bg-amber-950/20" @click="exitImpersonation">
          退出模拟身份
        </button>
      </div>

      <!-- 顶栏：参考 Iris 的 sticky 毛玻璃风格 -->
      <header
        class="sticky top-0 z-20 flex h-14 items-center gap-4 border-b border-slate-200 bg-white/90 px-4 backdrop-blur-xl sm:px-6 dark:border-[#242730] dark:bg-[#0b0d11]/90"
      >
        <router-link to="/board" class="flex items-center gap-2.5 font-bold tracking-tight no-underline text-inherit">
          <span
            class="grid size-7 place-items-center rounded-lg border border-violet-400/55 bg-gradient-to-br from-violet-500 to-violet-800 text-[1rem] font-bold text-white italic shadow-[inset_0_1px_rgb(255_255_255/.25),0_0_22px_rgb(124_58_237/.28)]"
            >W</span
          >
          <span class="hidden sm:inline">工作日志</span>
        </router-link>

        <div class="mx-1 hidden h-6 w-px bg-slate-200 sm:block dark:bg-[#2b2e37]"></div>

        <nav class="flex items-center gap-1 overflow-x-auto">
          <router-link
            v-for="n in navs"
            :key="n.path"
            :to="n.path"
            class="rounded-md px-2.5 py-1.5 text-sm font-medium whitespace-nowrap transition-colors"
            :class="
              route.path === n.path
                ? 'bg-violet-500/10 text-violet-600 dark:text-violet-400'
                : 'text-slate-600 hover:bg-slate-100 dark:text-[#c7cad1] dark:hover:bg-[#171a20]'
            "
            >{{ n.label }}</router-link
          >
        </nav>

        <div class="flex-1"></div>

        <!-- 暗色切换 -->
        <button
          class="grid size-8 place-items-center rounded-lg border border-slate-200 bg-slate-100 text-slate-600 dark:border-[#2b2f38] dark:bg-[#171a20] dark:text-[#c7cad1]"
          :title="isDark ? '切换亮色' : '切换暗色'"
          @click="toggleTheme"
        >
          <span>{{ isDark ? '☀' : '☾' }}</span>
        </button>

        <!-- 通知铃铛 -->
        <n-popover v-model:show="showNotify" trigger="click" placement="bottom-end" style="max-width: 360px">
          <template #trigger>
            <button
              class="relative grid size-8 place-items-center rounded-lg border border-slate-200 bg-slate-100 text-slate-600 dark:border-[#2b2f38] dark:bg-[#171a20] dark:text-[#c7cad1]"
              title="通知"
            >
              <n-badge :value="unread" :max="99" :show="unread > 0">
                <span class="text-sm">🔔</span>
              </n-badge>
            </button>
          </template>
          <div style="width: 320px">
            <!-- 未读 / 已读 标签页 -->
            <div class="flex gap-1 border-b border-slate-100 p-1.5 dark:border-[#242730]">
              <button
                v-for="t in [
                  { key: 'unread', label: `未读${unreadNotifications.length ? ` (${unreadNotifications.length})` : ''}` },
                  { key: 'read', label: '已读' },
                ]"
                :key="t.key"
                class="flex-1 rounded-md px-2 py-1 text-xs font-medium transition-colors"
                :class="
                  notifyTab === t.key
                    ? 'bg-violet-500/10 text-violet-600 dark:text-violet-400'
                    : 'text-slate-500 hover:bg-slate-100 dark:text-[#8a8f9c] dark:hover:bg-[#171a20]'
                "
                @click="notifyTab = t.key as 'unread' | 'read'"
              >
                {{ t.label }}
              </button>
            </div>
            <div class="max-h-96 overflow-y-auto">
              <div v-if="filteredNotifications.length === 0" class="py-6 text-center text-sm text-slate-400">
                {{ notifyTab === 'unread' ? '暂无未读通知' : '暂无已读通知' }}
              </div>
              <div
                v-for="n in filteredNotifications"
                :key="n.id"
                class="border-b border-slate-100 px-1 py-2 last:border-0 dark:border-[#242730]"
                :class="n.work_item_id ? 'cursor-pointer rounded transition-colors hover:bg-slate-50 dark:hover:bg-[#171a20]' : ''"
                @click="goToItem(n)"
              >
                <div class="flex items-start gap-1.5">
                  <span v-if="!n.read" class="mt-1.5 size-1.5 shrink-0 rounded-full bg-red-500"></span>
                  <div class="min-w-0 flex-1">
                    <div class="text-sm font-medium" :class="!n.read ? '' : 'text-slate-500 dark:text-[#9aa0ab]'">{{ n.title }}</div>
                    <div class="mt-0.5 text-xs text-slate-500">{{ n.content }}</div>
                    <div class="mt-0.5 flex items-center gap-1 text-xs text-slate-400">
                      {{ n.created_at?.slice(0, 16).replace('T', ' ') }}
                      <span v-if="n.work_item_id" class="text-violet-400">查看任务 →</span>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </n-popover>

        <!-- 用户 -->
        <n-popover trigger="click" placement="bottom-end">
          <template #trigger>
            <button class="cursor-pointer">
              <UserAvatar :user="auth.user" :size="8" />
            </button>
          </template>
          <div class="w-44">
            <div class="border-b border-slate-100 px-2 py-2 dark:border-[#242730]">
              <div class="flex items-center gap-1.5 text-sm font-semibold">
                {{ auth.user?.name }}
                <span v-if="auth.user?.is_admin" class="rounded bg-violet-500/10 px-1 py-0.5 text-[.6rem] font-bold text-violet-500">管理员</span>
              </div>
              <div class="text-xs text-slate-400">{{ auth.user?.email || '—' }}</div>
            </div>
            <button
              v-if="auth.impersonating"
              class="block w-full px-2 py-2 text-left text-sm font-semibold text-amber-600 hover:bg-slate-100 dark:text-amber-400 dark:hover:bg-[#171a20]"
              @click="exitImpersonation"
            >
              退出模拟身份
            </button>
            <button
              class="block w-full px-2 py-2 text-left text-sm hover:bg-slate-100 dark:hover:bg-[#171a20]"
              @click="router.push('/settings')"
            >
              个人设置
            </button>
            <button
              v-if="auth.user?.is_admin"
              class="block w-full px-2 py-2 text-left text-sm hover:bg-slate-100 dark:hover:bg-[#171a20]"
              @click="router.push('/system')"
            >
              系统设置
            </button>
            <button
              class="block w-full px-2 py-2 text-left text-sm text-red-500 hover:bg-slate-100 dark:hover:bg-[#171a20]"
              @click="auth.logout()"
            >
              退出登录
            </button>
          </div>
        </n-popover>
      </header>

      <main class="mx-auto max-w-[1400px] px-4 py-5 sm:px-6">
        <router-view />
      </main>
        </div>
      </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>
