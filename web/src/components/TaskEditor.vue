<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { NModal, NInput, NSelect, NDatePicker, NButton, NCheckbox, useDialog } from 'naive-ui'
import dayjs from 'dayjs'
import { api, type Project, type User, type WorkItem } from '../api'
import { useAuthStore } from '../stores/auth'
import { isEmptyRichText, stripHTML } from '../utils/richText'
import RichTextEditor from './RichTextEditor.vue'

const props = defineProps<{
  show: boolean
  projects: Project[]
  users: User[]
  editItem?: WorkItem | null
}>()
const emit = defineEmits<{ close: []; saved: [item: WorkItem] }>()

const auth = useAuthStore()
const dialog = useDialog()

const title = ref('')
const content = ref('') // 正文：任务总结
const detail = ref('') // 详细内容：可选的第二层
const hasDetail = ref(false) // 「详细内容」勾选框：勾选后展开第二个编辑器
const projectId = ref<number | null>(null)
const priority = ref<'high' | 'medium' | 'low'>('medium')
const completed = ref(true) // 已完成勾选：勾选=done，取消=doing
const assigneeId = ref<number | null>(null)
const participantIds = ref<number[]>([])
const workDate = ref<number | null>(Date.now())
const dueDate = ref<number | null>(Date.now())
const isTodo = ref(false) // 待办勾选：尚未排期（清空开始/截止日期），仅新建时可勾
const dueRemind = ref(true) // 到期提醒：勾选后截止日当天 18:00 提醒（新建默认勾选）
const startRemind = ref(false) // 开始提醒：开始日期为未来时可勾选，当天 12:00 提醒
const saving = ref(false)

const isEdit = computed(() => !!props.editItem)
// 开始日期为未来时展示「开始提醒」勾选项
const isFutureStart = computed(() => workDate.value != null && dayjs(workDate.value).isAfter(dayjs(), 'day'))
// 编辑待办任务时：开始日期排到今天或之前，保存后将自动进入「进行中」（后端联动）
const willStartOnSave = computed(
  () =>
    isEdit.value &&
    props.editItem?.status === 'todo' &&
    workDate.value != null &&
    !dayjs(workDate.value).isAfter(dayjs(), 'day'),
)

// 「待办」勾选：清空开始/截止日期表示尚未排期；取消勾选恢复默认（今天 + 已完成勾选）
watch(isTodo, (v) => {
  if (v) {
    workDate.value = null
    dueDate.value = null
    completed.value = false
    startRemind.value = false
  } else {
    workDate.value = Date.now()
    dueDate.value = Date.now()
    completed.value = true
  }
})

// 新建时：开始日期选未来 → 默认勾选开始提醒，且任务将进入「待办」（取消已完成）；选回今天/过去 → 恢复
watch(workDate, (v) => {
  if (isEdit.value || v == null) return
  const future = dayjs(v).isAfter(dayjs(), 'day')
  startRemind.value = future
  if (future) completed.value = false
})

// 新建时：截止日期选未来 → 任务尚未到推进节点，主动取消「已完成」默认勾选
watch(dueDate, (v) => {
  if (isEdit.value || v == null) return
  if (dayjs(v).isAfter(dayjs(), 'day')) completed.value = false
})

watch(
  () => props.show,
  (v) => {
    if (!v) return
    if (props.editItem) {
      const it = props.editItem
      title.value = it.title
      // 正文现为纯文本；历史富文本数据（正文曾是富文本）编辑时转为纯文本，避免显示原始 HTML 标签
      content.value = stripHTML(it.content || '')
      detail.value = it.detail || ''
      hasDetail.value = !isEmptyRichText(it.detail || '')
      projectId.value = it.project_id
      priority.value = it.priority
      completed.value = it.status === 'done'
      assigneeId.value = it.assignee_id ?? it.author_id
      participantIds.value = (it.participants || []).map((u) => u.id)
      workDate.value = it.work_date ? dayjs(it.work_date).valueOf() : null
      dueDate.value = it.due_date ? dayjs(it.due_date).valueOf() : null
      dueRemind.value = !!it.due_remind
      startRemind.value = !!it.start_remind
      isTodo.value = false // 待办勾选仅新建时可用；编辑待办任务时日期为空即可排期
    } else {
      title.value = ''
      content.value = ''
      detail.value = ''
      hasDetail.value = false
      projectId.value = props.projects[0]?.id ?? null
      priority.value = 'medium'
      completed.value = true
      assigneeId.value = auth.user?.id ?? null
      participantIds.value = []
      workDate.value = Date.now()
      dueDate.value = Date.now()
      dueRemind.value = true
      startRemind.value = false
      isTodo.value = false
    }
  },
)

const projectOptions = computed(() => props.projects.map((p) => ({ label: p.name, value: p.id })))
const userOptions = computed(() => props.users.map((u) => ({ label: u.name, value: u.id })))
const participantOptions = computed(() =>
  props.users.filter((u) => u.id !== assigneeId.value).map((u) => ({ label: u.name, value: u.id })),
)

// 「详细内容」勾选切换：取消勾选且已有内容时弹确认，防止误清空
function onToggleDetail(checked: boolean) {
  if (checked) {
    hasDetail.value = true
    return
  }
  if (isEmptyRichText(detail.value)) {
    hasDetail.value = false
    return
  }
  dialog.warning({
    title: '清除详细内容',
    content: '取消勾选将清空已填写的详细内容，确定吗？',
    positiveText: '清除',
    negativeText: '保留',
    onPositiveClick: () => {
      detail.value = ''
      hasDetail.value = false
    },
  })
}

async function save() {
  if (!title.value.trim() || !projectId.value) return
  saving.value = true
  try {
    const payload: Record<string, unknown> = {
      title: title.value.trim(),
      content: content.value.trim(),
      // 详细内容为富文本：空占位 HTML（<p><br></p> 等）归一化为空串；
      // 未勾选时显式清空（编辑场景后端按指针字段处理）
      detail: hasDetail.value && !isEmptyRichText(detail.value) ? detail.value : '',
      project_id: projectId.value,
      priority: priority.value,
      work_date: workDate.value ? dayjs(workDate.value).format('YYYY-MM-DD') : '',
      due_date: dueDate.value ? dayjs(dueDate.value).format('YYYY-MM-DD') : '',
      due_remind: !isTodo.value && dueRemind.value,
      // 开始提醒仅当开始日期为未来时生效（后端创建时同样校验）
      start_remind: !isTodo.value && isFutureStart.value && startRemind.value,
      assignee_id: assigneeId.value ?? undefined,
      participant_ids: participantIds.value,
    }
    if (!isEdit.value) {
      // 未排期（待办勾选）或未来开始的任务进入「待办」；否则按「已完成」勾选
      payload.status = isTodo.value || isFutureStart.value ? 'todo' : completed.value ? 'done' : 'doing'
    }
    const { data } = isEdit.value
      ? await api.updateWorkItem(props.editItem!.id, payload)
      : await api.createWorkItem(payload as Parameters<typeof api.createWorkItem>[0])
    emit('saved', data)
    emit('close')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <n-modal
    :show="show"
    preset="card"
    :title="isEdit ? '编辑任务' : '新建任务'"
    style="width: 760px; max-width: calc(100vw - 32px)"
    @update:show="emit('close')"
  >
    <div class="flex flex-col gap-4">
      <!-- 标题与正文是主体，全宽置顶 -->
      <div>
        <label class="mb-1 block text-xs font-medium text-slate-500">标题 *</label>
        <n-input v-model:value="title" placeholder="要做什么" />
      </div>

      <div>
        <label class="mb-1 block text-xs font-medium text-slate-500">正文</label>
        <n-input
          v-model:value="content"
          type="textarea"
          placeholder="任务总结：想怎么做、做了什么、结果如何"
          :autosize="{ minRows: 3, maxRows: 8 }"
        />
        <p class="mt-1 text-xs text-slate-400">精炼总结即可，AI 分析与报表只取标题和正文</p>
      </div>

      <!-- 详细内容：可选第二层，勾选后展开富文本编辑器；仅详情页展示，不提交给 AI -->
      <div>
        <n-checkbox :checked="hasDetail" @update:checked="onToggleDetail">
          <span class="text-sm">详细内容</span>
          <span class="ml-1 text-xs text-slate-400">有较多细节时补充在这里，支持插入图片和附件（单个不超过 500M）</span>
        </n-checkbox>
        <div v-if="hasDetail" class="mt-2">
          <RichTextEditor v-model="detail" placeholder="详细内容：过程细节、截图、日志等（仅详情页展示，不会提交给 AI 分析）" />
        </div>
      </div>

      <!-- 属性区：两列紧凑排布 -->
      <div class="grid grid-cols-2 gap-3">
        <div>
          <label class="mb-1 block text-xs font-medium text-slate-500">所属项目 *</label>
          <n-select v-model:value="projectId" :options="projectOptions" placeholder="选择项目" />
        </div>
        <div>
          <label class="mb-1 block text-xs font-medium text-slate-500">优先级</label>
          <n-select
            v-model:value="priority"
            :options="[
              { label: '高', value: 'high' },
              { label: '中', value: 'medium' },
              { label: '低', value: 'low' },
            ]"
          />
        </div>
      </div>

      <div class="grid grid-cols-2 gap-3">
        <div>
          <label class="mb-1 block text-xs font-medium text-slate-500">负责人</label>
          <n-select v-model:value="assigneeId" :options="userOptions" placeholder="默认自己" clearable />
        </div>
        <div>
          <label class="mb-1 block text-xs font-medium text-slate-500">参与人（可多选）</label>
          <n-select
            v-model:value="participantIds"
            :options="participantOptions"
            multiple
            clearable
            placeholder="可选"
            :max-tag-count="2"
          />
        </div>
      </div>

      <div class="grid grid-cols-2 gap-3">
        <div>
          <label class="mb-1 block text-xs font-medium text-slate-500">开始日期</label>
          <n-date-picker v-model:value="workDate" type="date" class="w-full" :disabled="isTodo" placeholder="待定" />
          <p v-if="willStartOnSave" class="mt-1 text-xs text-amber-500">保存后任务将进入「进行中」</p>
        </div>
        <div>
          <label class="mb-1 block text-xs font-medium text-slate-500">截止日期</label>
          <n-date-picker v-model:value="dueDate" type="date" class="w-full" :disabled="isTodo" placeholder="待定" />
        </div>
      </div>

      <!-- 勾选项合并为一行，减少纵向占用 -->
      <div class="flex flex-wrap items-center gap-x-6 gap-y-2 rounded-lg bg-slate-50 px-3 py-2.5 dark:bg-[#171a20]">
        <!-- 待办：尚未排期（清空日期），日后再定开始/截止时间 -->
        <n-checkbox v-if="!isEdit" v-model:checked="isTodo">
          <span class="text-sm">待办</span>
          <span class="ml-1 text-xs text-slate-400">还没决定什么时候开始，日后再排期</span>
        </n-checkbox>
        <template v-if="!isTodo">
          <n-checkbox v-model:checked="dueRemind">
            <span class="text-sm">到期提醒</span>
            <span class="ml-1 text-xs text-slate-400">截止日当天 18:00 提醒（站内通知，绑定飞书后同步推送）</span>
          </n-checkbox>
          <!-- 开始日期为未来时出现：开始当天 12:00 提醒 -->
          <n-checkbox v-if="isFutureStart" v-model:checked="startRemind">
            <span class="text-sm">开始提醒</span>
            <span class="ml-1 text-xs text-slate-400">开始日当天 12:00 提醒</span>
          </n-checkbox>
          <n-checkbox v-if="!isEdit" v-model:checked="completed" :disabled="isFutureStart">
            <span class="text-sm">已完成</span>
            <span class="ml-1 text-xs text-slate-400">
              {{ isFutureStart ? '未来开始的任务将进入「待办」，开始当天自动转入「进行中」' : '取消勾选后任务将进入「进行中」' }}
            </span>
          </n-checkbox>
        </template>
      </div>

      <div class="flex justify-end gap-2">
        <n-button @click="emit('close')">取消</n-button>
        <n-button type="primary" :loading="saving" :disabled="!title.trim() || !projectId" @click="save">
          {{ isEdit ? '保存' : '提交' }}
        </n-button>
      </div>
    </div>
  </n-modal>
</template>
