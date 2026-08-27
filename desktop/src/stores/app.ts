import { computed, reactive, ref } from 'vue'
import axios from 'axios'
import {
  createAPIClient,
  normalizeServerURL,
  type CreateWorkItemInput,
  type Notification,
  type Project,
  type User,
  type WorkItem,
  type WorkItemStatus,
  type WorkReportAPIClient,
} from '@work-report/shared'
import {
  clearAPIKey,
  hasAPIKey,
  loadAPIKey,
  loadPreferences,
  saveAPIKey,
  savePreferences,
  type DesktopPreferences,
} from '../lib/runtime'

export type TaskScope = 'assigned' | 'created' | 'team'

function errorMessage(error: unknown) {
  if (axios.isAxiosError(error)) {
    if (!error.response) return '无法连接服务器，请检查地址和网络'
    return (error.response.data as { error?: string })?.error || `请求失败（${error.response.status}）`
  }
  return error instanceof Error ? error.message : '发生未知错误'
}

const preferences = ref<DesktopPreferences>({
  server_url: '',
  recent_project_id: null,
  launch_at_startup: false,
})
const user = ref<User | null>(null)
const projects = ref<Project[]>([])
const users = ref<User[]>([])
const tasks = ref<WorkItem[]>([])
const unreadCount = ref(0)
const notifications = ref<Notification[]>([])
const notificationsLoading = ref(false)
const selectedTask = ref<WorkItem | null>(null)
const scope = ref<TaskScope>('assigned')
const loading = ref(false)
const refreshing = ref(false)
const connectionRequired = ref(true)
const connectionError = ref('')
const lastUpdatedAt = ref<Date | null>(null)
let client: WorkReportAPIClient | null = null

function makeClient(
  serverURL: string,
  getToken: () => string | null | Promise<string | null> = loadAPIKey,
) {
  return createAPIClient({
    baseURL: serverURL,
    getToken,
    onUnauthorized: () => {
      user.value = null
      connectionRequired.value = true
      connectionError.value = 'API Key 已失效或被吊销，请重新绑定'
    },
  })
}

function taskParams(): Record<string, string | number> {
  if (!user.value) return { limit: 200 }
  if (scope.value === 'assigned') return { assignee_id: user.value.id, limit: 200 }
  if (scope.value === 'created') return { author_id: user.value.id, limit: 200 }
  return { visible: 1, limit: 300 }
}

async function loadCoreData() {
  if (!client) return
  const [projectResponse, userResponse, taskResponse, unreadResponse] = await Promise.all([
    client.projects(),
    client.users(),
    client.workItems(taskParams()),
    client.unreadCount(),
  ])
  projects.value = projectResponse.data
  users.value = userResponse.data
  tasks.value = taskResponse.data
  unreadCount.value = unreadResponse.data.count
  if (selectedTask.value) {
    selectedTask.value = tasks.value.find((task) => task.id === selectedTask.value?.id) ?? null
  }
  lastUpdatedAt.value = new Date()
}

async function initialize() {
  loading.value = true
  connectionError.value = ''
  try {
    preferences.value = await loadPreferences()
    if (!preferences.value.server_url || !(await hasAPIKey())) {
      connectionRequired.value = true
      return
    }
    client = makeClient(preferences.value.server_url)
    user.value = (await client.me()).data
    connectionRequired.value = false
    await loadCoreData()
  } catch (error) {
    connectionRequired.value = true
    connectionError.value = errorMessage(error)
  } finally {
    loading.value = false
  }
}

async function connect(serverURL: string, apiKey: string) {
  loading.value = true
  connectionError.value = ''
  try {
    const normalizedURL = normalizeServerURL(serverURL)
    const verificationClient = makeClient(normalizedURL, () => apiKey.trim())
    const verifiedUser = (await verificationClient.me()).data
    const baseServerURL = normalizedURL.replace(/\/api$/, '')
    preferences.value = { ...preferences.value, server_url: baseServerURL }
    await savePreferences(preferences.value)
    await saveAPIKey(apiKey.trim())
    client = makeClient(baseServerURL)
    user.value = verifiedUser
    connectionRequired.value = false
    await loadCoreData()
  } catch (error) {
    connectionError.value = errorMessage(error)
    throw error
  } finally {
    loading.value = false
  }
}

async function disconnect() {
  await clearAPIKey()
  client = null
  user.value = null
  tasks.value = []
  projects.value = []
  users.value = []
  notifications.value = []
  unreadCount.value = 0
  selectedTask.value = null
  connectionRequired.value = true
}

async function refresh() {
  if (!client || refreshing.value) return
  refreshing.value = true
  try {
    await loadCoreData()
  } finally {
    refreshing.value = false
  }
}

async function changeScope(nextScope: TaskScope) {
  scope.value = nextScope
  await refresh()
}

async function createTask(input: CreateWorkItemInput) {
  if (!client) throw new Error('客户端尚未绑定')
  const created = (await client.createWorkItem(input)).data
  preferences.value = { ...preferences.value, recent_project_id: input.project_id }
  await savePreferences(preferences.value)
  await refresh()
  selectedTask.value = tasks.value.find((task) => task.id === created.id) ?? created
  return created
}

async function updateTaskStatus(item: WorkItem, status: WorkItemStatus) {
  if (!client) throw new Error('客户端尚未绑定')
  const updated = (await client.updateStatus(item.id, status)).data
  const index = tasks.value.findIndex((task) => task.id === updated.id)
  if (index >= 0) tasks.value[index] = updated
  if (selectedTask.value?.id === updated.id) selectedTask.value = updated
  return updated
}

async function loadTask(id: number) {
  if (!client) return
  selectedTask.value = (await client.workItem(id)).data
}

async function loadNotifications() {
  if (!client || notificationsLoading.value) return
  notificationsLoading.value = true
  try {
    notifications.value = (await client.notifications()).data
    unreadCount.value = notifications.value.filter((item) => !item.read).length
  } finally {
    notificationsLoading.value = false
  }
}

async function readNotification(id: number | 'all') {
  if (!client) return
  await client.markRead(id)
  notifications.value = notifications.value.map((item) =>
    id === 'all' || item.id === id ? { ...item, read: true } : item,
  )
  unreadCount.value = notifications.value.filter((item) => !item.read).length
}

const pendingCount = computed(() => tasks.value.filter((task) => task.status !== 'done').length)
const appState = reactive({
  preferences,
  user,
  projects,
  users,
  tasks,
  unreadCount,
  notifications,
  notificationsLoading,
  selectedTask,
  scope,
  loading,
  refreshing,
  connectionRequired,
  connectionError,
  lastUpdatedAt,
  pendingCount,
})

export function useAppStore() {
  return {
    state: appState,
    initialize,
    connect,
    disconnect,
    refresh,
    changeScope,
    createTask,
    updateTaskStatus,
    loadTask,
    loadNotifications,
    readNotification,
    savePreferences: async (next: DesktopPreferences) => {
      preferences.value = next
      await savePreferences(next)
    },
    errorMessage,
  }
}
