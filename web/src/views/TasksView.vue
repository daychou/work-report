<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { NEmpty, NRadioGroup, NRadioButton, NTag, useMessage } from 'naive-ui'
import dayjs from 'dayjs'
import { api, type Project, type User, type WorkItem } from '../api'
import { useAuthStore } from '../stores/auth'
import { stripHTML } from '../utils/richText'
import UserAvatar from '../components/UserAvatar.vue'
import TaskEditor from '../components/TaskEditor.vue'
import WorkItemDetail from '../components/WorkItemDetail.vue'
import ConfirmDialog from '../components/ConfirmDialog.vue'

const auth = useAuthStore()
const message = useMessage()

const projects = ref<Project[]>([])
const users = ref<User[]>([])
const tasks = ref<WorkItem[]>([])
const scope = ref<'mine' | 'team'>('mine')

async function load() {
  const params: Record<string, string | number> = {}
  if (scope.value === 'mine') params.author_id = auth.user!.id
  const [p, w, u] = await Promise.all([api.projects(), api.workItems(params), api.usersCached()])
  projects.value = p.data
  users.value = u.data
  // 按截止日期排序：有截止日的在前，逾期的最前
  tasks.value = w.data.sort((a, b) => {
    if (!a.due_date && !b.due_date) return 0
    if (!a.due_date) return 1
    if (!b.due_date) return -1
    return dayjs(a.due_date).unix() - dayjs(b.due_date).unix()
  })
}

onMounted(load)

function dueTag(item: WorkItem) {
  if (!item.due_date) return null
  if (item.status === 'done') return { text: '已完成', type: 'success' as const }
  const diff = dayjs(item.due_date).startOf('day').diff(dayjs().startOf('day'), 'day')
  if (diff < 0) return { text: `已逾期 ${-diff} 天`, type: 'error' as const }
  if (diff === 0) return { text: '今日到期', type: 'error' as const }
  if (diff <= 2) return { text: `${diff} 天后到期`, type: 'warning' as const }
  return { text: dayjs(item.due_date).format('MM-DD') + ' 截止', type: 'default' as const }
}

const statusMeta: Record<string, { label: string; type: 'default' | 'info' | 'success' | 'warning' }> = {
  todo: { label: '待办', type: 'default' },
  doing: { label: '进行中', type: 'warning' },
  done: { label: '已完成', type: 'success' },
}

async function markDone(item: WorkItem) {
  await api.updateStatus(item.id, 'done')
  message.success('已完成')
  load()
}

const showEditor = ref(false)
const editItem = ref<WorkItem | null>(null)
function openEdit(item: WorkItem) {
  editItem.value = item
  showEditor.value = true
}
const confirmRemove = ref<{ show: boolean; item: WorkItem | null; loading: boolean }>({
  show: false,
  item: null,
  loading: false,
})

function onRemove(item: WorkItem) {
  confirmRemove.value = { show: true, item, loading: false }
}

async function doRemove() {
  const item = confirmRemove.value.item
  if (!item) return
  confirmRemove.value.loading = true
  try {
    await api.deleteWorkItem(item.id)
    message.success('已删除')
    confirmRemove.value.show = false
  } catch (e: any) {
    message.error(e.response?.data?.error || '删除失败')
  } finally {
    confirmRemove.value.loading = false
  }
  load()
}

const pending = computed(() => tasks.value.filter((p) => p.status !== 'done'))
const done = computed(() => tasks.value.filter((p) => p.status === 'done'))

// 任务详情抽屉：点击进行中/已完成任务行，拉取完整数据后弹出（含评论）
const detailItem = ref<WorkItem | null>(null)
const showDetail = ref(false)
async function openDetail(item: WorkItem) {
  try {
    const { data } = await api.workItem(item.id)
    detailItem.value = data
    showDetail.value = true
  } catch (e: any) {
    message.error(e.response?.data?.error || '加载任务详情失败')
  }
}
</script>

<template>
  <div class="mx-auto max-w-4xl">
    <div class="mb-4 flex items-center justify-between">
      <h2 class="text-lg font-bold">任务</h2>
      <n-radio-group v-model:value="scope" size="small" @update:value="load">
        <n-radio-button value="mine">我的</n-radio-button>
        <n-radio-button value="team">团队</n-radio-button>
      </n-radio-group>
    </div>

    <div v-if="pending.length" class="mb-6">
      <h3 class="mb-2 text-sm font-bold text-slate-500">进行中（{{ pending.length }}）</h3>
      <div class="overflow-hidden rounded-xl border border-slate-200 bg-white dark:border-[#242730] dark:bg-[#12151b]">
        <div
          v-for="p in pending"
          :key="p.id"
          class="flex cursor-pointer items-center gap-3 border-b border-slate-100 px-4 py-3 last:border-0 hover:bg-slate-50 dark:border-[#1d212b] dark:hover:bg-[#171a20]"
          @click="openDetail(p)"
        >
          <UserAvatar :user="p.author" :size="7" />
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
              <span class="truncate text-sm font-semibold">{{ p.title }}</span>
              <span class="shrink-0 rounded bg-slate-100 px-1.5 py-0.5 text-[.65rem] text-slate-500 dark:bg-[#1d212b]">{{ p.project?.name }}</span>
            </div>
            <div v-if="p.content" class="mt-0.5 truncate text-xs text-slate-400">{{ stripHTML(p.content) }}</div>
          </div>
          <n-tag v-if="dueTag(p)" :type="dueTag(p)!.type" size="small" :bordered="false">{{ dueTag(p)!.text }}</n-tag>
          <n-tag :type="statusMeta[p.status]?.type" size="small" :bordered="false">{{ statusMeta[p.status]?.label }}</n-tag>
          <div class="flex shrink-0 gap-1">
            <button class="rounded px-2 py-1 text-xs text-emerald-600 hover:bg-emerald-50 dark:hover:bg-emerald-500/10" @click.stop="markDone(p)">完成</button>
            <button v-if="p.author_id === auth.user?.id" class="rounded px-2 py-1 text-xs text-slate-400 hover:bg-slate-100 dark:hover:bg-[#1d212b]" @click.stop="openEdit(p)">编辑</button>
            <button v-if="p.author_id === auth.user?.id || auth.user?.is_admin" class="rounded px-2 py-1 text-xs text-red-400 hover:bg-red-50 dark:hover:bg-red-500/10" @click.stop="onRemove(p)">删除</button>
          </div>
        </div>
      </div>
    </div>
    <n-empty v-else description="暂无进行中的任务" class="py-10" />

    <!-- 后端 limit 兜底（500 条）：超出时提示，避免误以为数据完整 -->
    <p v-if="tasks.length >= 500" class="mb-4 rounded-lg bg-amber-50 px-3 py-2 text-xs text-amber-600 dark:bg-amber-500/10">
      任务数量较多，仅显示最近 500 条；更早的历史任务请到「报表」中按周期查看
    </p>

    <div v-if="done.length">
      <h3 class="mb-2 text-sm font-bold text-slate-500">已完成（{{ done.length }}）</h3>
      <div class="overflow-hidden rounded-xl border border-slate-200 opacity-70 bg-white dark:border-[#242730] dark:bg-[#12151b]">
        <div
          v-for="p in done"
          :key="p.id"
          class="flex cursor-pointer items-center gap-3 border-b border-slate-100 px-4 py-2.5 last:border-0 hover:bg-slate-50 dark:border-[#1d212b] dark:hover:bg-[#171a20]"
          @click="openDetail(p)"
        >
          <UserAvatar :user="p.author" :size="6" />
          <span class="flex-1 truncate text-sm line-through">{{ p.title }}</span>
          <span class="text-xs text-slate-400">{{ p.project?.name }}</span>
          <span class="text-xs text-slate-400">{{ p.done_at ? dayjs(p.done_at).format('MM-DD') : '' }} 完成</span>
        </div>
      </div>
    </div>

    <TaskEditor :show="showEditor" :projects="projects" :users="users" :edit-item="editItem" @close="showEditor = false" @saved="load" />

    <!-- 任务详情抽屉（点击任务行弹出） -->
    <WorkItemDetail :show="showDetail" :item="detailItem" @close="showDetail = false" />

    <ConfirmDialog
      v-model:show="confirmRemove.show"
      title="删除确认"
      :content="`确定删除任务「${confirmRemove.item?.title}」吗？`"
      :loading="confirmRemove.loading"
      @confirm="doRemove"
    />
  </div>
</template>
