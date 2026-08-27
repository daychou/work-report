<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import dayjs from 'dayjs'
import {
  BellRing,
  CalendarDays,
  Check,
  CheckCircle2,
  ChevronDown,
  CircleDot,
  FileText,
  Flag,
  Inbox,
  LoaderCircle,
  NotebookPen,
  Plus,
  UserRound,
  UsersRound,
  X,
} from '@lucide/vue'
import type { CreateWorkItemInput, Project, User, WorkItemPriority, WorkItemStatus } from '@work-report/shared'

const props = defineProps<{
  projects: Project[]
  users: User[]
  currentUser: User
  recentProjectId?: number | null
  saving?: boolean
}>()
const emit = defineEmits<{ close: []; create: [input: CreateWorkItemInput] }>()

const today = dayjs().format('YYYY-MM-DD')
const titleInput = ref<HTMLInputElement | null>(null)
const title = ref('')
const content = ref('')
const detail = ref('')
const showDetail = ref(false)
const projectID = ref<number | null>(props.recentProjectId || props.projects[0]?.id || null)
const priority = ref<WorkItemPriority>('medium')
const assigneeID = ref(props.currentUser.id)
const participantIDs = ref<number[]>([])
const status = ref<WorkItemStatus>('doing')
const workDate = ref(today)
const dueDate = ref(today)
const dueRemind = ref(true)
const startRemind = ref(false)
const showMore = ref(false)

const statusChoices: Array<{ value: WorkItemStatus; label: string; hint: string }> = [
  { value: 'todo', label: '待办', hint: '还没决定什么时候开始，日后再排期' },
  { value: 'doing', label: '进行中', hint: '已经在推进，按开始与截止日期跟进' },
  { value: 'done', label: '已完成', hint: '补记一件已经做完的事' },
]

const canSubmit = computed(() => title.value.trim() && projectID.value)
const isTodo = computed(() => status.value === 'todo')
const isFutureStart = computed(() => Boolean(workDate.value && dayjs(workDate.value).isAfter(dayjs(), 'day')))
const statusHint = computed(() => {
  if (isTodo.value) return statusChoices[0].hint
  if (isFutureStart.value) return '开始日期在未来，任务会先进入「待办」，到期当天自动转入「进行中」'
  return statusChoices.find((choice) => choice.value === status.value)?.hint ?? ''
})

// 「待办」表示尚未排期：日期清空为待定，切回其他状态时恢复今天。
watch(status, (value) => {
  if (value === 'todo') {
    workDate.value = ''
    dueDate.value = ''
    startRemind.value = false
    return
  }
  if (!workDate.value) workDate.value = today
  if (!dueDate.value) dueDate.value = today
})

// 与网页版一致：开始日期排到未来时默认打开开始日提醒，改回今天或更早则关闭。
// 同时保证开始不晚于截止——改动其中一个日期时，把另一个顺带带过去。
watch(workDate, (value) => {
  if (!value) return
  startRemind.value = dayjs(value).isAfter(dayjs(), 'day')
  if (dueDate.value && dayjs(value).isAfter(dayjs(dueDate.value), 'day')) dueDate.value = value
})

watch(dueDate, (value) => {
  if (!value) return
  if (workDate.value && dayjs(value).isBefore(dayjs(workDate.value), 'day')) workDate.value = value
})

onMounted(() => nextTick(() => titleInput.value?.focus()))

async function submit() {
  if (!canSubmit.value || !projectID.value || props.saving) return
  const input: CreateWorkItemInput = {
    title: title.value.trim(),
    content: content.value.trim(),
    detail: detail.value.trim(),
    project_id: projectID.value,
    priority: priority.value,
    status: status.value,
    work_date: isTodo.value ? '' : workDate.value,
    due_date: isTodo.value ? '' : dueDate.value,
    due_remind: !isTodo.value && dueRemind.value,
    start_remind: !isTodo.value && isFutureStart.value && startRemind.value,
    assignee_id: assigneeID.value,
    participant_ids: participantIDs.value,
  }
  emit('create', input)
}

function onKeydown(event: KeyboardEvent) {
  if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') {
    event.preventDefault()
    submit()
  }
}

function toggleParticipant(id: number) {
  participantIDs.value = participantIDs.value.includes(id)
    ? participantIDs.value.filter((participantID) => participantID !== id)
    : [...participantIDs.value, id]
}
</script>

<template>
  <section class="create-pane" @keydown="onKeydown">
    <header class="create-header" data-tauri-drag-region>
      <div>
        <span class="eyebrow"><Plus :size="13" /> 快速记录</span>
        <h2>新建任务</h2>
      </div>
      <button class="icon-button" title="关闭" @click="emit('close')"><X :size="18" /></button>
    </header>

    <form class="create-form" @submit.prevent="submit">
      <label class="title-field">
        <span>任务标题</span>
        <input ref="titleInput" v-model="title" placeholder="要推进什么？" maxlength="256" />
      </label>

      <label>
        <span><FileText :size="14" /> 正文</span>
        <textarea v-model="content" rows="4" placeholder="精炼总结：想怎么做、做了什么、结果如何"></textarea>
        <small class="field-note">AI 分析与报表只取标题和正文</small>
      </label>

      <div class="detail-field">
        <button class="detail-field-toggle" type="button" @click="showDetail = !showDetail">
          <NotebookPen :size="15" />
          <span>
            <strong>详细内容</strong>
            <small>过程细节、日志等，仅在详情中展示</small>
          </span>
          <ChevronDown :class="{ rotated: showDetail }" :size="15" />
        </button>
        <div v-if="showDetail" class="detail-field-body">
          <textarea v-model="detail" rows="5" placeholder="展开记录过程细节，换行会原样保留…"></textarea>
          <small class="field-note">需要插入图片或附件时，请在网页版补充</small>
        </div>
      </div>

      <div class="status-field">
        <span class="field-label"><CircleDot :size="14" /> 初始状态</span>
        <div class="status-choice">
          <button
            v-for="choice in statusChoices"
            :key="choice.value"
            type="button"
            :class="{ active: status === choice.value }"
            @click="status = choice.value"
          >
            <Inbox v-if="choice.value === 'todo'" :size="15" />
            <CircleDot v-else-if="choice.value === 'doing'" :size="15" />
            <CheckCircle2 v-else :size="15" />
            {{ choice.label }}
          </button>
        </div>
        <small class="field-note">{{ statusHint }}</small>
      </div>

      <div class="form-grid">
        <label>
          <span><CalendarDays :size="14" /> 开始日期</span>
          <input v-if="!isTodo" v-model="workDate" type="date" />
          <p v-else class="pending-date"><Inbox :size="14" /> 待定</p>
        </label>
        <label>
          <span><CalendarDays :size="14" /> 截止日期</span>
          <input v-if="!isTodo" v-model="dueDate" type="date" />
          <p v-else class="pending-date"><Inbox :size="14" /> 待定</p>
        </label>
      </div>

      <div class="remind-field">
        <span class="field-label"><BellRing :size="14" /> 提醒</span>
        <p v-if="isTodo" class="remind-idle"><Inbox :size="14" /> 待办任务暂不排期，排期后再设置提醒</p>
        <div v-else class="remind-row">
          <label>
            <input v-model="dueRemind" type="checkbox" />
            <b>截止日提醒</b>
            <small>截止日当天 18:00</small>
          </label>
          <label v-if="isFutureStart">
            <input v-model="startRemind" type="checkbox" />
            <b>开始日提醒</b>
            <small>开始日当天 12:00</small>
          </label>
        </div>
      </div>

      <div class="form-grid">
        <label>
          <span><CircleDot :size="14" /> 所属项目</span>
          <div class="select-shell">
            <select v-model="projectID">
              <option v-for="project in projects" :key="project.id" :value="project.id">{{ project.name }}</option>
            </select>
            <ChevronDown :size="14" />
          </div>
        </label>
        <label>
          <span><Flag :size="14" /> 优先级</span>
          <div class="select-shell">
            <select v-model="priority">
              <option value="high">高优先级</option>
              <option value="medium">中优先级</option>
              <option value="low">低优先级</option>
            </select>
            <ChevronDown :size="14" />
          </div>
        </label>
      </div>

      <label>
        <span><UserRound :size="14" /> 负责人</span>
        <div class="select-shell">
          <select v-model="assigneeID">
            <option v-for="person in users" :key="person.id" :value="person.id">{{ person.name }}</option>
          </select>
          <ChevronDown :size="14" />
        </div>
      </label>

      <button class="more-toggle" type="button" @click="showMore = !showMore">
        <UsersRound :size="15" />
        {{ showMore ? '收起参与人' : `参与人${participantIDs.length ? `（已选 ${participantIDs.length} 人）` : ''}` }}
        <ChevronDown :class="{ rotated: showMore }" :size="15" />
      </button>

      <div v-if="showMore" class="advanced-options">
        <div class="participant-picker">
          <button
            v-for="person in users.filter((item) => item.id !== assigneeID)"
            :key="person.id"
            type="button"
            :class="{ selected: participantIDs.includes(person.id) }"
            @click="toggleParticipant(person.id)"
          >
            <i>{{ person.name.slice(0, 1) }}</i>
            {{ person.name }}
            <Check v-if="participantIDs.includes(person.id)" :size="12" />
          </button>
        </div>
      </div>

      <div class="create-footer">
        <p><kbd>⌘ ↵</kbd> 提交任务</p>
        <button class="primary-button" type="submit" :disabled="!canSubmit || saving">
          <LoaderCircle v-if="saving" class="spin" :size="16" />
          <Plus v-else :size="16" />
          {{ saving ? '正在创建…' : '创建任务' }}
        </button>
      </div>
    </form>
  </section>
</template>
