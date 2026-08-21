import axios from 'axios'

export interface Role {
  id: number
  name: string
  description: string
  // 管理员角色：拥有系统设置与全部数据权限
  is_admin: boolean
  // 内置角色不可删除、不可变更权限标识
  built_in: boolean
  created_at: string
  // 角色下成员数（列表接口返回）
  member_count?: number
}

export interface User {
  id: number
  casdoor_id: string
  name: string
  avatar: string
  email: string
  feishu_open_id: string
  is_admin: boolean
  role_id?: number | null
  role?: Role | null
  // 为 true 时登录后强制跳转到修改密码页
  must_change_password?: boolean
  created_at: string
  // 非 0 表示当前会话是管理员模拟该用户身份
  impersonated_by?: number
}

export interface Comment {
  id: number
  work_item_id: number
  // 非空表示回复，统一指向顶级评论（两层结构）
  parent_id?: number | null
  author_id: number
  author: User
  content: string
  created_at: string
}

export interface Project {
  id: number
  name: string
  description: string
  owner_id: number
  owner: User
  status: string
  created_at: string
}

export interface WorkItem {
  id: number
  title: string
  // 正文：任务总结（AI 分析与报表导出只取标题+正文）
  content: string
  // 详细内容：可选的第二层内容（细节/截图/日志），仅详情页展示，不提交给 AI
  detail?: string
  project_id: number
  project: Project
  author_id: number
  author: User
  assignee_id?: number | null
  assignee?: User | null
  participants?: User[]
  kind: 'plan' | 'work'
  status: 'todo' | 'doing' | 'done' | 'cancelled'
  priority: 'high' | 'medium' | 'low'
  // 开始日期；待办（未排期）任务为 null
  work_date: string | null
  due_date?: string | null
  // 到期提醒：勾选后截止日期当天 18:00 提醒作者与负责人
  due_remind?: boolean
  // 开始提醒：勾选后开始日期当天 12:00 提醒作者与负责人（仅开始日期为未来时可勾选）
  start_remind?: boolean
  done_at?: string | null
  created_at: string
  // 评论数（列表接口统计返回，看板卡片展示用）
  comment_count?: number
}

export interface Notification {
  id: number
  user_id: number
  work_item_id?: number
  work_item?: WorkItem
  // 提及类通知对应的评论，用于跳转后定位闪烁
  comment_id?: number
  type: string
  title: string
  content: string
  read: boolean
  created_at: string
}

export interface ReportData {
  period: string
  label: string
  from: string
  to: string
  by_user: { user: User; works: WorkItem[]; plans: WorkItem[] }[]
  by_project: { project: Project; works: WorkItem[]; plans: WorkItem[] }[]
  summary: {
    work_count: number
    plan_count: number
    plan_done_count: number
    plan_done_rate: number
    active_users: number
    active_projects: number
  }
}

export interface StatsOverview {
  by_user: { user_id: number; user_name: string; work_cnt: number; done_cnt: number }[]
  by_project: { project_id: number; project_name: string; cnt: number }[]
  daily_trend: { day: string; cnt: number }[]
  total_work: number
  total_projects: number
  total_users: number
}

// AI 模型配置（系统设置管理；api_key 仅管理员接口返回）
export interface AIModel {
  id: number
  name: string
  provider: string
  model_id: string
  api_key?: string
  base_url?: string
  enabled: boolean
  created_at?: string
}

// 阿里云 OSS 配置（系统设置管理，仅管理员可读写；附件存储用）
export interface OSSConfig {
  id?: number
  endpoint: string
  bucket: string
  access_key_id: string
  access_key_secret?: string
  dir?: string
  domain?: string
}

// 上传结果（图片/附件经服务端中继到 OSS）
export interface UploadResult {
  url: string
  name: string
  size: number
}

// AI 生成的总结报告（后端异步生成，前端轮询状态）
export interface AIReport {
  id: number
  requester_id: number
  requester?: User
  user_id: number
  user?: User
  ai_model_id: number
  ai_model?: AIModel
  report_type: 'week' | 'year'
  date_from: string
  date_to: string
  extra_prompt?: string
  status: 'running' | 'done' | 'failed'
  result?: string
  error?: string
  work_item_id?: number | null
  created_at: string
}

const http = axios.create({ baseURL: '/api' })

http.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

http.interceptors.response.use(
  (resp) => resp,
  (err) => {
    if (err.response?.status === 401) {
      localStorage.removeItem('token')
      // /callback 页的 401 由 CallbackView 自行展示错误，不整页跳转
      if (location.pathname !== '/login' && location.pathname !== '/callback') location.href = '/login'
    }
    // 初始密码未修改：后端拦截其他接口，统一跳转强制改密页
    if (err.response?.status === 403 && err.response?.data?.code === 'must_change_password') {
      if (location.pathname !== '/change-password') location.href = '/change-password'
    }
    return Promise.reject(err)
  },
)

// 用户列表前端缓存（60s）：评论弹窗、看板轮询等高频场景复用，
// 成员变更不敏感；系统设置页等需要实时数据的地方用 api.users() 并调 invalidateUsers
let usersCache: { data: User[]; at: number } | null = null

export function invalidateUsersCache() {
  usersCache = null
}

export const api = {
  // auth
  authConfig: () => http.get<{ casdoor_enabled: boolean; authorize_url: string }>('/auth/config'),
  callback: (code: string, state: string) =>
    http.post<{ token: string; user: User }>('/auth/callback', { code, state }),
  devLogin: (name: string) => http.post<{ token: string; user: User }>('/auth/dev-login', { name }),
  // 本地账号密码登录
  login: (username: string, password: string) =>
    http.post<{ token: string; user: User }>('/auth/login', { username, password }),
  changePassword: (oldPassword: string, newPassword: string) =>
    http.post<User>('/auth/change-password', { old_password: oldPassword, new_password: newPassword }),
  me: () => http.get<User>('/auth/me'),
  updateMe: (data: { feishu_open_id?: string; avatar?: string }) => http.put<User>('/auth/me', data),

  // users & projects
  users: () => http.get<User[]>('/users'),
  // 带 60s 缓存的用户列表（返回与 axios 响应同构的对象，便于替换现有调用）
  usersCached: async () => {
    if (usersCache && Date.now() - usersCache.at < 60_000) {
      return { data: usersCache.data }
    }
    const resp = await http.get<User[]>('/users')
    usersCache = { data: resp.data, at: Date.now() }
    return resp
  },
  createUser: (data: { name: string; email?: string }) => http.post<User>('/users', data),
  updateUser: (id: number, data: { name: string; email?: string; role_id?: number }) =>
    http.put<User>(`/users/${id}`, data),
  impersonate: (id: number) => http.post<{ token: string; user: User }>(`/users/${id}/impersonate`),

  // roles
  roles: () => http.get<Role[]>('/roles'),
  createRole: (data: { name: string; description?: string; is_admin?: boolean }) =>
    http.post<Role>('/roles', data),
  updateRole: (id: number, data: { name: string; description?: string; is_admin?: boolean }) =>
    http.put<Role>(`/roles/${id}`, data),
  deleteRole: (id: number) => http.delete(`/roles/${id}`),
  projects: (all = false) => http.get<Project[]>('/projects', { params: all ? { all: 1 } : {} }),
  createProject: (data: { name: string; description?: string; owner_id: number }) =>
    http.post<Project>('/projects', data),
  updateProject: (id: number, data: Partial<Project>) => http.put<Project>(`/projects/${id}`, data),
  deleteProject: (id: number) => http.delete(`/projects/${id}`),

  // work items
  workItems: (params: Record<string, string | number>) => http.get<WorkItem[]>('/work-items', { params }),
  workItem: (id: number) => http.get<WorkItem>(`/work-items/${id}`),
  createWorkItem: (data: {
    title: string
    content?: string
    detail?: string
    project_id: number
    kind?: 'plan' | 'work'
    priority?: string
    status?: string
    work_date?: string
    due_date?: string
    due_remind?: boolean
    start_remind?: boolean
    assignee_id?: number
    participant_ids?: number[]
  }) => http.post<WorkItem>('/work-items', data),
  updateWorkItem: (id: number, data: Record<string, unknown>) =>
    http.put<WorkItem>(`/work-items/${id}`, data),
  updateStatus: (id: number, status: string) => http.patch<WorkItem>(`/work-items/${id}/status`, { status }),
  deleteWorkItem: (id: number) => http.delete(`/work-items/${id}`),
  // 恢复软删除（看板删除撤销用）
  restoreWorkItem: (id: number) => http.post<WorkItem>(`/work-items/${id}/restore`),

  // reports & stats
  report: (period: string, date: string) =>
    http.get<ReportData>('/reports', { params: { period, date } }),
  reportMarkdown: (period: string, date: string) =>
    http.get<string>('/reports', { params: { period, date, format: 'markdown' } }),
  stats: (days = 30) => http.get<StatsOverview>('/stats', { params: { days } }),

  // AI 模型（管理操作仅管理员；enabled 列表所有登录用户可用）
  aiModelsEnabled: () => http.get<AIModel[]>('/ai-models/enabled'),
  aiModels: () => http.get<AIModel[]>('/ai-models'),
  createAIModel: (data: Partial<AIModel>) => http.post<AIModel>('/ai-models', data),
  updateAIModel: (id: number, data: Partial<AIModel>) => http.put<AIModel>(`/ai-models/${id}`, data),
  deleteAIModel: (id: number) => http.delete(`/ai-models/${id}`),

  // AI 分析报告
  aiReports: () => http.get<AIReport[]>('/ai-reports'),
  // 预览将提交给 AI 的工作数据（按执行人+时间范围，与生成取数逻辑一致）
  aiReportPreview: (params: { user_id: number; date_from: string; date_to: string }) =>
    http.get<WorkItem[]>('/ai-reports/preview', { params }),
  createAIReport: (data: {
    user_id: number
    ai_model_id: number
    report_type: 'week' | 'year'
    date_from: string
    date_to: string
    extra_prompt?: string
  }) => http.post<AIReport>('/ai-reports', data),
  aiReport: (id: number) => http.get<AIReport>(`/ai-reports/${id}`),
  // 删除报告（发起人或管理员；生成中不可删除）
  deleteAIReport: (id: number) => http.delete(`/ai-reports/${id}`),

  // OSS 附件存储（配置仅管理员；上传为登录用户，单文件上限 500M）
  ossConfig: () => http.get<OSSConfig>('/oss-config'),
  saveOSSConfig: (data: OSSConfig) => http.put<OSSConfig>('/oss-config', data),
  uploadFile: (file: File, onProgress?: (percent: number) => void) => {
    const form = new FormData()
    form.append('file', file)
    return http.post<UploadResult>('/uploads', form, {
      // 大文件上传不限超时；进度回调供编辑器展示
      timeout: 0,
      onUploadProgress: (e) => onProgress?.(e.total ? Math.round((e.loaded / e.total) * 100) : 0),
    })
  },

  // comments
  comments: (workItemId: number) => http.get<Comment[]>(`/work-items/${workItemId}/comments`),
  createComment: (workItemId: number, content: string, parentId?: number) =>
    http.post<Comment>(`/work-items/${workItemId}/comments`, { content, parent_id: parentId }),
  deleteComment: (id: number) => http.delete(`/comments/${id}`),

  // notifications
  notifications: () => http.get<Notification[]>('/notifications'),
  unreadCount: () => http.get<{ count: number }>('/notifications/unread-count'),
  markRead: (id: number | 'all') => http.patch(`/notifications/${id}/read`),
}
