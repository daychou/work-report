<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import dayjs from 'dayjs'
import {
  ArrowUpRight,
  Ban,
  CalendarDays,
  Check,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  CircleDot,
  Clock3,
  FileText,
  Flag,
  Inbox,
  LoaderCircle,
  RotateCcw,
  Sparkles,
  UserRound,
  UsersRound,
} from '@lucide/vue'
import type { WorkItem, WorkItemStatus } from '@work-report/shared'
import { isEmptyRichText, isHTMLContent, sanitizeRichText } from '../lib/richText'

const props = defineProps<{ task: WorkItem }>()
const emit = defineEmits<{
  status: [status: WorkItemStatus]
  openWeb: [path: string]
}>()
const changing = ref(false)
const detailExpanded = ref(false)
const statusMenuOpen = ref(false)

const statusOptions: Array<{ value: WorkItemStatus; label: string; icon: typeof CircleDot }> = [
  { value: 'todo', label: '待办', icon: Inbox },
  { value: 'doing', label: '进行中', icon: CircleDot },
  { value: 'done', label: '已完成', icon: CheckCircle2 },
  { value: 'cancelled', label: '已取消', icon: Ban },
]

const statusLabel = computed(
  () => statusOptions.find((option) => option.value === props.task.status)?.label ?? props.task.status,
)

const priorityLabel = computed(() => ({ high: '高优先级', medium: '中优先级', low: '低优先级' })[props.task.priority])
const contentIsHTML = computed(() => isHTMLContent(props.task.content))
const sanitizedContent = computed(() => sanitizeRichText(props.task.content))
const hasDetail = computed(() => !isEmptyRichText(props.task.detail))
const detailIsHTML = computed(() => isHTMLContent(props.task.detail))
const sanitizedDetail = computed(() => sanitizeRichText(props.task.detail))

async function changeStatus(status: WorkItemStatus) {
  statusMenuOpen.value = false
  if (status === props.task.status) return
  changing.value = true
  emit('status', status)
  window.setTimeout(() => (changing.value = false), 450)
}

function closeStatusMenu(event: MouseEvent) {
  if (!(event.target as HTMLElement).closest('.detail-status-switch')) statusMenuOpen.value = false
}

onMounted(() => window.addEventListener('click', closeStatusMenu))
onBeforeUnmount(() => window.removeEventListener('click', closeStatusMenu))
</script>

<template>
  <article class="detail-pane">
    <header class="detail-toolbar" data-tauri-drag-region>
      <div class="detail-status-switch">
        <button class="detail-status" :class="task.status" :disabled="changing" @click="statusMenuOpen = !statusMenuOpen">
          <CircleDot :size="14" />
          {{ statusLabel }}
          <ChevronDown :class="{ rotated: statusMenuOpen }" :size="13" />
        </button>
        <div v-if="statusMenuOpen" class="status-menu">
          <button
            v-for="option in statusOptions"
            :key="option.value"
            :class="{ current: option.value === task.status }"
            @click="changeStatus(option.value)"
          >
            <component :is="option.icon" :size="15" />
            {{ option.label }}
            <Check v-if="option.value === task.status" :size="14" />
          </button>
        </div>
      </div>
      <button class="soft-button" @click="emit('openWeb', `/board?item=${task.id}`)">
        网页中打开 <ArrowUpRight :size="14" />
      </button>
    </header>

    <div class="detail-scroll">
      <div class="detail-heading">
        <span class="project-chip">
          <i :style="{ '--project-hue': `${(task.project_id * 47) % 360}` }"></i>
          {{ task.project?.name || '未归类' }}
        </span>
        <h2>{{ task.title }}</h2>
        <p class="task-id">TASK-{{ String(task.id).padStart(4, '0') }}</p>
      </div>

      <div class="detail-actions">
        <button v-if="task.status !== 'done'" class="complete-button" :disabled="changing" @click="changeStatus('done')">
          <LoaderCircle v-if="changing" class="spin" :size="17" />
          <CheckCircle2 v-else :size="17" />
          标记完成
          <kbd>⌘↵</kbd>
        </button>
        <button v-else class="reopen-button" :disabled="changing" @click="changeStatus('doing')">
          <RotateCcw :size="16" />
          重新打开
        </button>
      </div>

      <section class="property-grid">
        <div>
          <span><UserRound :size="14" /> 负责人</span>
          <strong>
            <i class="mini-avatar">{{ (task.assignee?.name || task.author?.name || '我').slice(0, 1) }}</i>
            {{ task.assignee?.name || task.author?.name || '未指定' }}
          </strong>
        </div>
        <div>
          <span><Flag :size="14" /> 优先级</span>
          <strong class="priority-value" :class="task.priority"><i></i>{{ priorityLabel }}</strong>
        </div>
        <div>
          <span><CalendarDays :size="14" /> 开始日期</span>
          <strong>{{ task.work_date ? dayjs(task.work_date).format('YYYY年MM月DD日') : '尚未排期' }}</strong>
        </div>
        <div>
          <span><Clock3 :size="14" /> 截止日期</span>
          <strong>{{ task.due_date ? dayjs(task.due_date).format('YYYY年MM月DD日') : '未设置' }}</strong>
        </div>
        <div class="property-wide">
          <span><UsersRound :size="14" /> 参与人</span>
          <strong v-if="task.participants?.length" class="participant-list">
            <i v-for="participant in task.participants.slice(0, 4)" :key="participant.id" :title="participant.name">
              {{ participant.name.slice(0, 1) }}
            </i>
            <em>{{ task.participants.map((item) => item.name).join('、') }}</em>
          </strong>
          <strong v-else>暂无参与人</strong>
        </div>
      </section>

      <section class="content-section">
        <h3><FileText :size="15" /> 任务正文</h3>
        <div
          v-if="sanitizedContent"
          class="task-content rich-content"
          :class="{ 'is-html': contentIsHTML }"
          v-html="sanitizedContent"
        ></div>
        <div v-else class="content-empty">还没有正文描述。</div>
      </section>

      <section v-if="hasDetail" class="detail-content-section">
        <button class="detail-content-toggle" @click="detailExpanded = !detailExpanded">
          <ChevronRight :class="{ expanded: detailExpanded }" :size="16" />
          <span>
            <strong>详细内容</strong>
            <small>过程细节、截图与附件</small>
          </span>
          <em>{{ detailExpanded ? '收起' : '展开' }}</em>
        </button>
        <Transition name="detail-expand">
          <div v-show="detailExpanded" class="detail-content-body">
            <div class="rich-content" :class="{ 'is-html': detailIsHTML }" v-html="sanitizedDetail"></div>
          </div>
        </Transition>
      </section>

      <section class="activity-section">
        <h3><Sparkles :size="15" /> 最近动态</h3>
        <div class="activity-item">
          <span class="activity-icon"><Check :size="13" /></span>
          <div>
            <strong>{{ task.author?.name || '创建者' }} 创建了任务</strong>
            <p>{{ dayjs(task.created_at).format('MM月DD日 HH:mm') }}</p>
          </div>
        </div>
        <button class="text-button" @click="emit('openWeb', `/board?item=${task.id}`)">查看评论与完整动态</button>
      </section>
    </div>
  </article>
</template>
