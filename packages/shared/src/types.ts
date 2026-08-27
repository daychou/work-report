export type WorkItemStatus = 'todo' | 'doing' | 'done' | 'cancelled'
export type WorkItemPriority = 'high' | 'medium' | 'low'

export interface Role {
  id: number
  name: string
  description: string
  is_admin: boolean
  built_in: boolean
  created_at?: string
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
  must_change_password?: boolean
  impersonated_by?: number
  created_at: string
}

export interface Project {
  id: number
  name: string
  description: string
  owner_id: number
  owner: User
  status: 'active' | 'archived' | string
  created_at: string
}

export interface WorkItem {
  id: number
  title: string
  content: string
  detail?: string
  project_id: number
  project: Project
  author_id: number
  author: User
  assignee_id?: number | null
  assignee?: User | null
  participants?: User[]
  kind: 'plan' | 'work'
  status: WorkItemStatus
  priority: WorkItemPriority
  work_date: string | null
  due_date?: string | null
  due_remind?: boolean
  start_remind?: boolean
  done_at?: string | null
  created_at: string
  updated_at?: string
  comment_count?: number
}

export interface Notification {
  id: number
  user_id: number
  work_item_id?: number
  work_item?: WorkItem
  comment_id?: number
  type: string
  title: string
  content: string
  read: boolean
  created_at: string
}

export interface UserAPIKey {
  id: number
  name: string
  key_prefix: string
  key?: string
  expires_at?: string | null
  last_used_at?: string | null
  created_at: string
}

export interface CreateWorkItemInput {
  title: string
  content?: string
  detail?: string
  project_id: number
  kind?: 'plan' | 'work'
  priority?: WorkItemPriority
  status?: WorkItemStatus
  work_date?: string
  due_date?: string
  due_remind?: boolean
  start_remind?: boolean
  assignee_id?: number
  participant_ids?: number[]
}
