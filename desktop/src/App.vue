<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { ArrowRight, LoaderCircle, Plus, Sparkles } from '@lucide/vue'
import type { CreateWorkItemInput, Notification, WorkItem, WorkItemStatus } from '@work-report/shared'
import AppSidebar from './components/AppSidebar.vue'
import ConnectionSetup from './components/ConnectionSetup.vue'
import NotificationPanel from './components/NotificationPanel.vue'
import QuickCreate from './components/QuickCreate.vue'
import SettingsPane from './components/SettingsPane.vue'
import TaskDetail from './components/TaskDetail.vue'
import TaskList from './components/TaskList.vue'
import {
  hideWindow,
  openExternal,
  setLaunchAtStartup,
  type DesktopPreferences,
} from './lib/runtime'
import { useAppStore, type TaskScope } from './stores/app'

const app = useAppStore()
const state = app.state
const search = ref('')
const status = ref<'active' | WorkItemStatus>('active')
const projectID = ref<number | 'all'>('all')
const activePane = ref<'detail' | 'create' | 'settings' | 'notifications'>('detail')
const createSaving = ref(false)
const toast = ref<{ message: string; tone: 'success' | 'error' } | null>(null)
const theme = ref<'light' | 'dark'>(
  (localStorage.getItem('work-report-desktop-theme') as 'light' | 'dark') ||
    (matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'),
)

const filteredTasks = computed(() => {
  const query = search.value.trim().toLocaleLowerCase()
  return state.tasks
    .filter((task) => {
      if (status.value === 'active' && !['todo', 'doing'].includes(task.status)) return false
      if (status.value !== 'active' && task.status !== status.value) return false
      if (projectID.value !== 'all' && task.project_id !== projectID.value) return false
      if (
        query &&
        ![task.title, task.content, task.project?.name, task.assignee?.name]
          .filter(Boolean)
          .some((value) => value?.toLocaleLowerCase().includes(query))
      ) {
        return false
      }
      return true
    })
    .sort((left, right) => {
      if (!left.due_date && !right.due_date) return right.id - left.id
      if (!left.due_date) return 1
      if (!right.due_date) return -1
      return left.due_date.localeCompare(right.due_date)
    })
})

let refreshTimer: number | undefined
let toastTimer: number | undefined

function showToast(message: string, tone: 'success' | 'error' = 'success') {
  toast.value = { message, tone }
  window.clearTimeout(toastTimer)
  toastTimer = window.setTimeout(() => (toast.value = null), 2800)
}

function chooseTask(task: WorkItem) {
  state.selectedTask = task
  activePane.value = 'detail'
  app.loadTask(task.id)
}

function openCreate() {
  activePane.value = 'create'
}

async function openNotifications() {
  activePane.value = 'notifications'
  try {
    await app.loadNotifications()
  } catch (error) {
    showToast(app.errorMessage(error), 'error')
  }
}

async function readAllNotifications() {
  try {
    await app.readNotification('all')
    showToast('已全部标记为已读')
  } catch (error) {
    showToast(app.errorMessage(error), 'error')
  }
}

// 点击通知：标记已读并跳到对应任务，看不到任务时退回网页版。
async function openNotification(item: Notification) {
  try {
    if (!item.read) await app.readNotification(item.id)
    if (!item.work_item_id) return
    const task = state.tasks.find((candidate) => candidate.id === item.work_item_id)
    activePane.value = 'detail'
    if (task) {
      chooseTask(task)
      return
    }
    await app.loadTask(item.work_item_id)
  } catch (error) {
    showToast(app.errorMessage(error), 'error')
  }
}

async function connect(serverURL: string, apiKey: string) {
  try {
    await app.connect(serverURL, apiKey)
    state.selectedTask = filteredTasks.value[0] || null
    showToast(`欢迎回来，${state.user?.name}`)
  } catch {
    // 连接错误由连接页直接展示
  }
}

async function refresh() {
  try {
    await app.refresh()
  } catch (error) {
    showToast(app.errorMessage(error), 'error')
  }
}

async function changeScope(scope: TaskScope) {
  try {
    await app.changeScope(scope)
    state.selectedTask = filteredTasks.value[0] || null
  } catch (error) {
    showToast(app.errorMessage(error), 'error')
  }
}

async function createTask(input: CreateWorkItemInput) {
  createSaving.value = true
  try {
    const created = await app.createTask(input)
    activePane.value = 'detail'
    showToast(`已创建「${created.title}」`)
  } catch (error) {
    showToast(app.errorMessage(error), 'error')
  } finally {
    createSaving.value = false
  }
}

async function changeTaskStatus(nextStatus: WorkItemStatus) {
  if (!state.selectedTask) return
  try {
    await app.updateTaskStatus(state.selectedTask, nextStatus)
    showToast(nextStatus === 'done' ? '任务已完成' : '任务已更新')
  } catch (error) {
    showToast(app.errorMessage(error), 'error')
  }
}

function webURL(path = '/board') {
  return `${state.preferences.server_url.replace(/\/+$/, '')}${path}`
}

async function saveSettings(preferences: DesktopPreferences) {
  try {
    await app.savePreferences(preferences)
    await setLaunchAtStartup(Boolean(preferences.launch_at_startup))
    activePane.value = 'detail'
    showToast('设置已保存')
  } catch (error) {
    showToast(app.errorMessage(error), 'error')
  }
}

async function disconnect() {
  await app.disconnect()
  activePane.value = 'detail'
}

function setTheme(value: 'light' | 'dark') {
  theme.value = value
  localStorage.setItem('work-report-desktop-theme', value)
}

function onKeyboard(event: KeyboardEvent) {
  const target = event.target as HTMLElement
  const editing = ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName) || target.isContentEditable
  if (event.key === 'Escape') {
    if (activePane.value !== 'detail') activePane.value = 'detail'
    else hideWindow()
    return
  }
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'n') {
    event.preventDefault()
    openCreate()
    return
  }
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
    event.preventDefault()
    document.querySelector<HTMLInputElement>('.search-box input')?.focus()
    return
  }
  if ((event.metaKey || event.ctrlKey) && event.key === 'Enter' && activePane.value === 'detail' && state.selectedTask) {
    event.preventDefault()
    changeTaskStatus(state.selectedTask.status === 'done' ? 'doing' : 'done')
    return
  }
  if (editing || activePane.value !== 'detail' || !['j', 'k', 'ArrowDown', 'ArrowUp'].includes(event.key)) return
  const currentIndex = filteredTasks.value.findIndex((task) => task.id === state.selectedTask?.id)
  const direction = ['j', 'ArrowDown'].includes(event.key) ? 1 : -1
  const nextIndex = Math.min(Math.max(currentIndex + direction, 0), filteredTasks.value.length - 1)
  const nextTask = filteredTasks.value[nextIndex]
  if (nextTask) chooseTask(nextTask)
}

watch(theme, (value) => document.documentElement.classList.toggle('dark', value === 'dark'), { immediate: true })
watch(filteredTasks, (tasks) => {
  if (!state.selectedTask && tasks.length && activePane.value === 'detail') state.selectedTask = tasks[0]
})

onMounted(async () => {
  window.addEventListener('keydown', onKeyboard)
  window.addEventListener('focus', refresh)
  await app.initialize()
  if (!state.connectionRequired) state.selectedTask = filteredTasks.value[0] || null
  refreshTimer = window.setInterval(() => {
    if (!document.hidden && !state.connectionRequired) refresh()
  }, 60_000)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeyboard)
  window.removeEventListener('focus', refresh)
  window.clearInterval(refreshTimer)
  window.clearTimeout(toastTimer)
})
</script>

<template>
  <div class="window-drag-strip" data-tauri-drag-region></div>

  <ConnectionSetup
    v-if="state.connectionRequired"
    :loading="state.loading"
    :error="state.connectionError"
    @connect="connect"
  />

  <main v-else-if="state.user" class="desktop-shell">
    <AppSidebar
      :user="state.user"
      :scope="state.scope"
      :pending-count="state.pendingCount"
      :unread-count="state.unreadCount"
      :refreshing="state.refreshing"
      :server-url="state.preferences.server_url"
      :active-pane="activePane"
      @scope="changeScope"
      @create="openCreate"
      @refresh="refresh"
      @settings="activePane = 'settings'"
      @notifications="openNotifications"
      @open-web="openExternal(webURL())"
    />

    <TaskList
      :tasks="filteredTasks"
      :projects="state.projects"
      :selected-id="state.selectedTask?.id"
      :current-user-id="state.user.id"
      :search="search"
      :status="status"
      :project-id="projectID"
      @select="chooseTask"
      @create="openCreate"
      @update:search="search = $event"
      @update:status="status = $event"
      @update:project-id="projectID = $event"
    />

    <Transition name="pane" mode="out-in">
      <QuickCreate
        v-if="activePane === 'create'"
        key="create"
        :projects="state.projects"
        :users="state.users"
        :current-user="state.user"
        :recent-project-id="state.preferences.recent_project_id"
        :saving="createSaving"
        @close="activePane = 'detail'"
        @create="createTask"
      />
      <NotificationPanel
        v-else-if="activePane === 'notifications'"
        key="notifications"
        :notifications="state.notifications"
        :loading="state.notificationsLoading"
        @close="activePane = 'detail'"
        @open="openNotification"
        @read-all="readAllNotifications"
        @open-web="openExternal(webURL('/board'))"
      />
      <SettingsPane
        v-else-if="activePane === 'settings'"
        key="settings"
        :preferences="state.preferences"
        :user="state.user"
        :theme="theme"
        @close="activePane = 'detail'"
        @save="saveSettings"
        @disconnect="disconnect"
        @open-web="openExternal(webURL('/settings'))"
        @theme="setTheme"
      />
      <TaskDetail
        v-else-if="state.selectedTask"
        :key="state.selectedTask.id"
        :task="state.selectedTask"
        @status="changeTaskStatus"
        @open-web="openExternal(webURL($event))"
      />
      <section v-else key="empty" class="detail-empty">
        <div class="detail-empty-illustration">
          <span></span><span></span><span></span>
          <Sparkles :size="20" />
        </div>
        <h2>今天从一件小事开始</h2>
        <p>选择左侧任务查看详情，或快速记录一个新任务。</p>
        <button class="primary-button" @click="openCreate"><Plus :size="16" /> 新建任务 <ArrowRight :size="15" /></button>
      </section>
    </Transition>

    <Transition name="toast">
      <div v-if="toast" class="toast" :class="toast.tone">{{ toast.message }}</div>
    </Transition>
  </main>

  <div v-else class="boot-screen">
    <LoaderCircle class="spin" :size="26" />
    <span>正在打开工作台…</span>
  </div>
</template>
