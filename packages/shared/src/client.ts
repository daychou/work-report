import axios, { type AxiosError, type AxiosInstance } from 'axios'
import type {
  CreateWorkItemInput,
  Notification,
  Project,
  User,
  UserAPIKey,
  WorkItem,
  WorkItemStatus,
} from './types'

export interface APIClientOptions {
  baseURL: string
  getToken: () => string | null | Promise<string | null>
  onUnauthorized?: (error: AxiosError) => void | Promise<void>
  timeout?: number
}

export function normalizeServerURL(value: string): string {
  const trimmed = value.trim().replace(/\/+$/, '')
  if (!trimmed) return ''
  const withScheme = /^https?:\/\//i.test(trimmed) ? trimmed : `https://${trimmed}`
  return withScheme.endsWith('/api') ? withScheme : `${withScheme}/api`
}

export function createAPIClient(options: APIClientOptions) {
  const http: AxiosInstance = axios.create({
    baseURL: normalizeServerURL(options.baseURL),
    timeout: options.timeout ?? 15_000,
    headers: { Accept: 'application/json' },
  })

  http.interceptors.request.use(async (config) => {
    const token = await options.getToken()
    if (token) config.headers.Authorization = `Bearer ${token}`
    return config
  })

  http.interceptors.response.use(
    (response) => response,
    async (error: AxiosError) => {
      if (error.response?.status === 401) await options.onUnauthorized?.(error)
      return Promise.reject(error)
    },
  )

  return {
    http,
    me: () => http.get<User>('/auth/me'),
    users: () => http.get<User[]>('/users'),
    projects: (all = false) => http.get<Project[]>('/projects', { params: all ? { all: 1 } : {} }),
    workItems: (params: Record<string, string | number>) => http.get<WorkItem[]>('/work-items', { params }),
    workItem: (id: number) => http.get<WorkItem>(`/work-items/${id}`),
    createWorkItem: (input: CreateWorkItemInput) => http.post<WorkItem>('/work-items', input),
    updateWorkItem: (id: number, input: Partial<CreateWorkItemInput>) =>
      http.put<WorkItem>(`/work-items/${id}`, input),
    updateStatus: (id: number, status: WorkItemStatus) =>
      http.patch<WorkItem>(`/work-items/${id}/status`, { status }),
    notifications: () => http.get<Notification[]>('/notifications'),
    unreadCount: () => http.get<{ count: number }>('/notifications/unread-count'),
    markRead: (id: number | 'all') => http.patch(`/notifications/${id}/read`),
    apiKeys: () => http.get<UserAPIKey[]>('/api-keys'),
    createAPIKey: (input: { name: string; expires_at?: string }) =>
      http.post<UserAPIKey & { key: string }>('/api-keys', input),
    deleteAPIKey: (id: number) => http.delete(`/api-keys/${id}`),
  }
}

export type WorkReportAPIClient = ReturnType<typeof createAPIClient>
