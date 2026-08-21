<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { NButton, NSelect, NDatePicker, NInput, NEmpty, NTag, NModal, useMessage } from 'naive-ui'
import dayjs from 'dayjs'
import { api, type AIModel, type AIReport, type User, type WorkItem } from '../api'
import { useAuthStore } from '../stores/auth'
import UserAvatar from '../components/UserAvatar.vue'
import WorkItemDetail from '../components/WorkItemDetail.vue'
import ConfirmDialog from '../components/ConfirmDialog.vue'

const auth = useAuthStore()
const message = useMessage()

// 与后端 handler.DefaultAIPrompt 保持一致，用户可改
const DEFAULT_PROMPT = `你是一名擅长帮助技术人员撰写年度工作总结的职业总结顾问。

我会提供我一段时间的工作日报，请你不要简单地按照日期罗列工作，而是从全年日报中提炼我的工作成果、核心贡献、解决的问题、能力成长和明年的工作方向等。`

const users = ref<User[]>([])
const models = ref<AIModel[]>([])
const reports = ref<AIReport[]>([])

// 生成表单
const targetUserId = ref<number | null>(null)
const reportType = ref<'week' | 'year'>('week')
const range = ref<[number, number]>([
  dayjs().startOf('week').valueOf(),
  dayjs().endOf('week').valueOf(),
])
const modelId = ref<number | null>(null)
const extraPrompt = ref(DEFAULT_PROMPT)
const creating = ref(false)

const userOptions = computed(() => users.value.map((u) => ({ label: u.name, value: u.id })))
const modelOptions = computed(() => models.value.map((m) => ({ label: `${m.name}（${m.model_id}）`, value: m.id })))

// 切换报告类型时联动默认时间范围
function pickType(t: 'week' | 'year') {
  reportType.value = t
  range.value =
    t === 'week'
      ? [dayjs().startOf('week').valueOf(), dayjs().endOf('week').valueOf()]
      : [dayjs().startOf('year').valueOf(), dayjs().endOf('year').valueOf()]
}

async function load() {
  const [u, m, r] = await Promise.all([api.usersCached(), api.aiModelsEnabled(), api.aiReports()])
  users.value = u.data
  models.value = Array.isArray(m.data) ? m.data : []
  reports.value = Array.isArray(r.data) ? r.data : []
  if (!targetUserId.value) targetUserId.value = auth.user?.id ?? null
  if (!modelId.value && models.value.length) modelId.value = models.value[0].id
}

// 有生成中的报告时 5s 轮询（生成在后端异步执行，刷新页面不影响）
const hasRunning = computed(() => reports.value.some((r) => r.status === 'running'))
let timer: number | undefined
function resetTimer() {
  if (timer) window.clearInterval(timer)
  timer = undefined
  if (hasRunning.value) {
    timer = window.setInterval(load, 5000)
  }
}

onMounted(async () => {
  await load()
  resetTimer()
})
onUnmounted(() => timer && window.clearInterval(timer))

// 生成门槛：必须先预览数据，且预览结果多于 1 条才允许生成（后端 Create 同样校验）
const MIN_ITEMS = 2
const canGenerate = computed(
  () =>
    !!targetUserId.value &&
    !!modelId.value &&
    !!range.value &&
    models.value.length > 0 &&
    previewItems.value !== null &&
    previewItems.value.length >= MIN_ITEMS,
)

async function create() {
  if (!targetUserId.value || !modelId.value || !range.value) return
  if (previewItems.value === null) {
    message.warning('请先预览数据，确认后再生成报告')
    return
  }
  if (previewItems.value.length < MIN_ITEMS) {
    message.warning('数据过少无法生成报告')
    return
  }
  creating.value = true
  try {
    await api.createAIReport({
      user_id: targetUserId.value,
      ai_model_id: modelId.value,
      report_type: reportType.value,
      date_from: dayjs(range.value[0]).format('YYYY-MM-DD'),
      date_to: dayjs(range.value[1]).format('YYYY-MM-DD'),
      extra_prompt: extraPrompt.value,
    })
    message.success('已提交，AI 正在分析中…')
    await load()
    resetTimer()
  } catch (e: any) {
    message.error(e.response?.data?.error || '创建失败')
  } finally {
    creating.value = false
  }
}

const statusMeta: Record<string, { label: string; type: 'default' | 'info' | 'success' | 'error' }> = {
  running: { label: '分析中', type: 'info' },
  done: { label: '已完成', type: 'success' },
  failed: { label: '失败', type: 'error' },
}

// 结果查看弹窗
const viewReport = ref<AIReport | null>(null)
async function copyResult(r: AIReport) {
  try {
    await navigator.clipboard.writeText(r.result || '')
    message.success('已复制 Markdown')
  } catch {
    message.error('复制失败')
  }
}

// ---- 删除报告：发起人或管理员可删，生成中不可删（后端同样校验）----
const confirmDelete = ref<{ show: boolean; target: AIReport | null; loading: boolean }>({
  show: false,
  target: null,
  loading: false,
})

function canDeleteReport(r: AIReport) {
  return auth.user?.is_admin || r.requester_id === auth.user?.id
}

function onDeleteReport(r: AIReport) {
  confirmDelete.value = { show: true, target: r, loading: false }
}

async function doDeleteReport() {
  const target = confirmDelete.value.target
  if (!target) return
  confirmDelete.value.loading = true
  try {
    await api.deleteAIReport(target.id)
    message.success('报告已删除')
    confirmDelete.value.show = false
    load()
  } catch (e: any) {
    message.error(e.response?.data?.error || '删除失败')
  } finally {
    confirmDelete.value.loading = false
  }
}

// ---- 数据预览：生成前先看会喂给 AI 哪些工作数据（与后端生成取数逻辑一致）----
const previewItems = ref<WorkItem[] | null>(null) // null = 尚未预览
const previewLoading = ref(false)
const previewMeta = ref<{ userName: string; from: string; to: string } | null>(null)

async function preview() {
  if (!targetUserId.value || !range.value) return
  previewLoading.value = true
  try {
    const from = dayjs(range.value[0]).format('YYYY-MM-DD')
    const to = dayjs(range.value[1]).format('YYYY-MM-DD')
    const { data } = await api.aiReportPreview({ user_id: targetUserId.value, date_from: from, date_to: to })
    previewItems.value = Array.isArray(data) ? data : []
    previewMeta.value = {
      userName: users.value.find((u) => u.id === targetUserId.value)?.name ?? '',
      from,
      to,
    }
    if (previewItems.value.length < MIN_ITEMS) {
      message.warning('数据过少无法生成报告（该时间范围内已完成任务不足 2 条）')
    }
  } catch (e: any) {
    message.error(e.response?.data?.error || '预览查询失败')
  } finally {
    previewLoading.value = false
  }
}

// 执行人/时间范围变化后，旧预览结果作废，避免误导
watch([targetUserId, range], () => {
  previewItems.value = null
  previewMeta.value = null
})

// 任务详情抽屉：点击预览列表条目，拉取完整数据后弹出
const detailItem = ref<WorkItem | null>(null)
const showDetail = ref(false)
async function openDetail(it: WorkItem) {
  try {
    const { data } = await api.workItem(it.id)
    detailItem.value = data
    showDetail.value = true
  } catch (e: any) {
    message.error(e.response?.data?.error || '加载任务详情失败')
  }
}

const priorityCN: Record<string, { label: string; type: 'error' | 'warning' | 'default' }> = {
  high: { label: '高', type: 'error' },
  medium: { label: '中', type: 'warning' },
  low: { label: '低', type: 'default' },
}
</script>

<template>
  <div class="mx-auto max-w-4xl">
    <h2 class="mb-4 text-lg font-bold">AI 分析</h2>

    <!-- 生成表单 -->
    <div class="rounded-2xl border border-slate-200 bg-white p-5 dark:border-[#242730] dark:bg-[#12151b]">
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="mb-1 block text-xs font-medium text-slate-500">执行人（分析哪位同事的工作数据）</label>
          <n-select v-model:value="targetUserId" :options="userOptions" />
        </div>
        <div>
          <label class="mb-1 block text-xs font-medium text-slate-500">AI 模型</label>
          <n-select v-model:value="modelId" :options="modelOptions" placeholder="先在系统设置启用模型" />
        </div>
      </div>

      <div class="mt-4 grid grid-cols-2 gap-4">
        <div>
          <label class="mb-1 block text-xs font-medium text-slate-500">报告类型</label>
          <div class="grid grid-cols-2 gap-1 rounded-xl bg-slate-100 p-1 dark:bg-[#171a20]">
            <button
              v-for="t in [
                { key: 'week', label: '周报' },
                { key: 'year', label: '年度报告' },
              ]"
              :key="t.key"
              class="rounded-lg px-3 py-1.5 text-sm transition-all"
              :class="
                reportType === t.key
                  ? 'bg-white font-semibold text-violet-600 shadow-sm dark:bg-[#23262f] dark:text-violet-400'
                  : 'text-slate-500 hover:bg-white/50 dark:text-[#9aa0ad] dark:hover:bg-[#23262f]/50'
              "
              @click="pickType(t.key as 'week' | 'year')"
            >
              {{ t.label }}
            </button>
          </div>
        </div>
        <div>
          <label class="mb-1 block text-xs font-medium text-slate-500">时间范围（取该范围内已完成任务）</label>
          <n-date-picker v-model:value="range" type="daterange" class="w-full" :clearable="false" />
        </div>
      </div>

      <div class="mt-4">
        <label class="mb-1 block text-xs font-medium text-slate-500">额外提示词（告诉 AI 以什么角度和要求生成）</label>
        <n-input v-model:value="extraPrompt" type="textarea" :rows="4" />
      </div>

      <div class="mt-4 flex items-center justify-end gap-3">
        <span v-if="!models.length" class="text-xs text-amber-500">
          暂无已启用的 AI 模型，请管理员先到「系统设置 → AI 模型」配置并启用
        </span>
        <template v-else>
          <span v-if="previewItems === null" class="text-xs text-slate-400">
            请先预览数据，确认后再生成报告
          </span>
          <span v-else-if="previewItems.length < MIN_ITEMS" class="text-xs text-amber-500">
            数据过少无法生成报告（至少 2 条已完成任务）
          </span>
        </template>
        <n-button :loading="previewLoading" :disabled="!targetUserId || !range" @click="preview">
          预览数据
        </n-button>
        <n-button type="primary" :loading="creating" :disabled="!canGenerate" @click="create">
          生成报告
        </n-button>
      </div>
    </div>

    <!-- 数据预览：生成前确认将提交给 AI 的工作数据，点击条目可查看任务详情 -->
    <div
      v-if="previewItems !== null"
      class="mt-4 rounded-2xl border border-slate-200 bg-white p-5 dark:border-[#242730] dark:bg-[#12151b]"
    >
      <div class="mb-3 flex items-center gap-2">
        <h3 class="text-sm font-bold">
          将提交给 AI 的数据（{{ previewItems.length }} 条）
        </h3>
        <span v-if="previewMeta" class="text-xs text-slate-400">
          {{ previewMeta.userName }} · {{ previewMeta.from }} ~ {{ previewMeta.to }} · 仅统计已完成任务 · 只取标题与正文（详细内容不提交）
        </span>
      </div>

      <div v-if="previewItems.length" class="flex max-h-80 flex-col gap-1.5 overflow-y-auto pr-1">
        <button
          v-for="it in previewItems"
          :key="it.id"
          class="flex items-center gap-2 rounded-lg border border-slate-100 px-3 py-2 text-left transition-colors hover:border-violet-300 hover:bg-violet-50/50 dark:border-[#1d212b] dark:hover:border-violet-500/40 dark:hover:bg-violet-500/5"
          @click="openDetail(it)"
        >
          <span class="shrink-0 text-xs text-slate-400 tabular-nums">{{ dayjs(it.work_date).format('MM-DD ddd') }}</span>
          <n-tag size="small" :bordered="false" class="shrink-0">{{ it.project?.name }}</n-tag>
          <span class="min-w-0 flex-1 truncate text-sm">{{ it.title }}</span>
          <n-tag size="small" :bordered="false" :type="priorityCN[it.priority]?.type" class="shrink-0">
            {{ priorityCN[it.priority]?.label }}
          </n-tag>
          <span class="shrink-0 text-xs text-violet-500">详情 →</span>
        </button>
      </div>
      <n-empty v-else description="该时间范围内没有已完成的工作数据，换个范围试试" class="py-6" />
    </div>

    <!-- 报告列表 -->
    <h3 class="mt-6 mb-3 text-sm font-bold text-slate-500">生成记录（{{ reports.length }}）</h3>
    <div v-if="reports.length" class="flex flex-col gap-2.5">
      <div
        v-for="r in reports"
        :key="r.id"
        class="rounded-xl border border-slate-200 bg-white px-4 py-3 dark:border-[#242730] dark:bg-[#12151b]"
      >
        <div class="flex items-center gap-2">
          <n-tag size="small" :bordered="false" :type="r.report_type === 'year' ? 'warning' : 'info'">
            {{ r.report_type === 'year' ? '年度报告' : '周报' }}
          </n-tag>
          <span class="text-sm font-semibold">{{ r.user?.name }}</span>
          <span class="text-xs text-slate-400">
            {{ dayjs(r.date_from).format('YYYY-MM-DD') }} ~ {{ dayjs(r.date_to).format('YYYY-MM-DD') }}
          </span>
          <span class="text-xs text-slate-400">{{ r.ai_model?.name }}</span>
          <div class="flex-1"></div>
          <n-tag :type="statusMeta[r.status]?.type" size="small" :bordered="false">
            {{ statusMeta[r.status]?.label }}
          </n-tag>
        </div>

        <!-- 生成中状态：转圈动画提示 -->
        <div v-if="r.status === 'running'" class="mt-2 flex items-center gap-2 text-xs text-violet-500">
          <span class="ai-spin inline-block size-3.5 rounded-full border-2 border-violet-300 border-t-violet-600"></span>
          AI 正在分析中，可离开或刷新页面，完成后会自动更新…
        </div>
        <div v-else-if="r.status === 'failed'" class="mt-2 text-xs text-red-500">{{ r.error }}</div>

        <div class="mt-2 flex items-center gap-2 text-xs text-slate-400">
          <UserAvatar :user="r.requester" :size="5" />
          <span>{{ r.requester?.name }} 发起</span>
          <span>{{ dayjs(r.created_at).format('MM-DD HH:mm') }}</span>
          <div class="flex-1"></div>
          <template v-if="r.status === 'done'">
            <button class="rounded px-2 py-1 text-xs text-violet-600 hover:bg-violet-50 dark:hover:bg-violet-500/10" @click="viewReport = r">
              查看报告
            </button>
            <button class="rounded px-2 py-1 text-xs text-slate-400 hover:bg-slate-100 dark:hover:bg-[#1d212b]" @click="copyResult(r)">
              复制 Markdown
            </button>
          </template>
          <button
            v-if="r.status !== 'running' && canDeleteReport(r)"
            class="rounded px-2 py-1 text-xs text-red-400 hover:bg-red-50 dark:hover:bg-red-500/10"
            @click="onDeleteReport(r)"
          >
            删除
          </button>
        </div>
      </div>
    </div>
    <n-empty v-else description="还没有生成过报告" class="py-10" />

    <!-- 报告结果弹窗 -->
    <n-modal
      :show="!!viewReport"
      preset="card"
      :title="`${viewReport?.user?.name} 的${viewReport?.report_type === 'year' ? '年度报告' : '周报'}`"
      style="width: 720px; max-width: calc(100vw - 32px)"
      @update:show="viewReport = null"
    >
      <div class="max-h-[60vh] overflow-y-auto text-sm leading-relaxed whitespace-pre-wrap">{{ viewReport?.result }}</div>
      <div class="mt-4 flex justify-end">
        <n-button v-if="viewReport" @click="copyResult(viewReport)">复制 Markdown</n-button>
      </div>
    </n-modal>

    <!-- 任务详情抽屉（预览列表点击条目弹出） -->
    <WorkItemDetail :show="showDetail" :item="detailItem" @close="showDetail = false" />

    <ConfirmDialog
      v-model:show="confirmDelete.show"
      title="删除报告"
      :content="`确定删除「${confirmDelete.target?.user?.name}」的${confirmDelete.target?.report_type === 'year' ? '年度报告' : '周报'}（${dayjs(confirmDelete.target?.date_from).format('YYYY-MM-DD')} ~ ${dayjs(confirmDelete.target?.date_to).format('YYYY-MM-DD')}）吗？已写入任务的【AI生成】条目不受影响。`"
      positive-text="删除"
      :danger="true"
      :loading="confirmDelete.loading"
      @confirm="doDeleteReport"
    />
  </div>
</template>

<style scoped>
.ai-spin {
  animation: ai-spin 0.8s linear infinite;
}
@keyframes ai-spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
