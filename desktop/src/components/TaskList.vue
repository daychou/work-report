<script setup lang="ts">
import { computed } from 'vue'
import dayjs from 'dayjs'
import {
  CalendarClock,
  Check,
  ChevronRight,
  Circle,
  Flame,
  Inbox,
  Plus,
  Search,
  Sparkles,
  UserRound,
  X,
} from '@lucide/vue'
import type { Project, WorkItem, WorkItemStatus } from '@work-report/shared'

const props = defineProps<{
  tasks: WorkItem[]
  projects: Project[]
  selectedId?: number
  currentUserId: number
  search: string
  status: 'active' | WorkItemStatus
  projectId: number | 'all'
}>()
const emit = defineEmits<{
  select: [task: WorkItem]
  create: []
  'update:search': [value: string]
  'update:status': [value: 'active' | WorkItemStatus]
  'update:projectId': [value: number | 'all']
}>()

const statusTabs: Array<{ value: 'active' | WorkItemStatus; label: string }> = [
  { value: 'active', label: '进行中' },
  { value: 'todo', label: '待办' },
  { value: 'done', label: '已完成' },
  { value: 'cancelled', label: '已取消' },
]

const filtering = computed(() => Boolean(props.search.trim()) || props.projectId !== 'all')

function isOverdue(task: WorkItem) {
  return Boolean(task.due_date) && dayjs(task.due_date).isBefore(dayjs(), 'day') && task.status !== 'done'
}

// 逾期单独置顶：客户端最重要的价值是一眼看到已经失约的事。
const groupedTasks = computed(() => {
  const overdue: WorkItem[] = []
  const today: WorkItem[] = []
  const upcoming: WorkItem[] = []
  const unscheduled: WorkItem[] = []
  const completed: WorkItem[] = []
  props.tasks.forEach((task) => {
    if (task.status === 'done') completed.push(task)
    else if (isOverdue(task)) overdue.push(task)
    else if (!task.due_date) unscheduled.push(task)
    else if (!dayjs(task.due_date).isAfter(dayjs(), 'day')) today.push(task)
    else upcoming.push(task)
  })
  return [
    { label: '逾期', tasks: overdue, tone: 'danger' },
    { label: '今日聚焦', tasks: today, tone: 'accent' },
    { label: '接下来', tasks: upcoming, tone: 'plain' },
    { label: '未排期', tasks: unscheduled, tone: 'plain' },
    { label: '最近完成', tasks: completed, tone: 'plain' },
  ].filter((group) => group.tasks.length)
})

function dueText(task: WorkItem) {
  if (!task.due_date) return '待定'
  const due = dayjs(task.due_date)
  const diff = due.startOf('day').diff(dayjs().startOf('day'), 'day')
  if (task.status === 'done') return `${due.format('MM-DD')} 完成`
  if (diff < 0) return `逾期 ${Math.abs(diff)} 天`
  if (diff === 0) return '今日到期'
  if (diff === 1) return '明日到期'
  return due.format('MM月DD日')
}
</script>

<template>
  <section class="task-list-pane">
    <header class="list-header" data-tauri-drag-region>
      <div>
        <span class="eyebrow"><Sparkles :size="13" /> 专注视图</span>
        <h1>我的任务</h1>
      </div>
      <span class="list-count">{{ tasks.length }} 项</span>
    </header>

    <div class="search-box">
      <Search :size="16" />
      <input
        :value="search"
        placeholder="搜索任务、正文或项目…"
        @input="emit('update:search', ($event.target as HTMLInputElement).value)"
      />
      <button v-if="search" class="search-clear" title="清空搜索" @click="emit('update:search', '')">
        <X :size="14" />
      </button>
      <kbd v-else>⌘K</kbd>
    </div>

    <div class="filter-row">
      <div class="status-tabs">
        <button
          v-for="tab in statusTabs"
          :key="tab.value"
          :class="{ active: status === tab.value }"
          @click="emit('update:status', tab.value)"
        >
          {{ tab.label }}
        </button>
      </div>
      <select
        :value="projectId"
        aria-label="项目筛选"
        @change="emit('update:projectId', ($event.target as HTMLSelectElement).value === 'all' ? 'all' : Number(($event.target as HTMLSelectElement).value))"
      >
        <option value="all">全部项目</option>
        <option v-for="project in projects" :key="project.id" :value="project.id">{{ project.name }}</option>
      </select>
    </div>

    <div class="task-scroll">
      <template v-if="groupedTasks.length">
        <section v-for="group in groupedTasks" :key="group.label" class="task-group">
          <div class="group-heading" :class="group.tone">
            <span>{{ group.label }}</span>
            <b>{{ group.tasks.length }}</b>
          </div>
          <button
            v-for="task in group.tasks"
            :key="task.id"
            class="task-card"
            :class="{ selected: selectedId === task.id, completed: task.status === 'done' }"
            @click="emit('select', task)"
          >
            <span class="status-indicator" :class="[task.status, task.priority]">
              <Check v-if="task.status === 'done'" :size="12" />
              <Circle v-else :size="12" />
            </span>
            <span class="task-card-main">
              <strong>
                <Flame v-if="task.priority === 'high' && task.status !== 'done'" class="priority-flag" :size="13" />
                {{ task.title }}
              </strong>
              <span class="task-card-meta">
                <i class="project-dot" :style="{ '--project-hue': `${(task.project_id * 47) % 360}` }"></i>
                {{ task.project?.name || '未归类' }}
                <span>·</span>
                <CalendarClock :size="12" />
                <em :class="{ overdue: isOverdue(task) }">{{ dueText(task) }}</em>
                <template v-if="task.assignee && task.assignee.id !== currentUserId">
                  <span>·</span>
                  <UserRound :size="12" />
                  {{ task.assignee.name }}
                </template>
              </span>
            </span>
            <span v-if="task.comment_count" class="comment-count">{{ task.comment_count }}</span>
            <ChevronRight :size="15" class="task-chevron" />
          </button>
        </section>
      </template>
      <div v-else class="empty-state">
        <span><Inbox :size="28" /></span>
        <strong>这里很清爽</strong>
        <p v-if="filtering">没有符合当前筛选条件的任务。</p>
        <p v-else>这个视图暂时没有任务，记一件要推进的事吧。</p>
        <button v-if="filtering" class="soft-button" @click="emit('update:search', ''); emit('update:projectId', 'all')">
          <X :size="14" /> 清除筛选
        </button>
        <button v-else class="soft-button" @click="emit('create')"><Plus :size="14" /> 新建任务</button>
      </div>
    </div>
  </section>
</template>
