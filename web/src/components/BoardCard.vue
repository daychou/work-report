<script setup lang="ts">
import { computed, onUnmounted, ref } from 'vue'
import dayjs from 'dayjs'
import type { WorkItem } from '../api'
import { useAuthStore } from '../stores/auth'
import UserAvatar from './UserAvatar.vue'

const props = withDefaults(
  defineProps<{
    item: WorkItem
    // 已完成列降低视觉权重（灰化 + 标题删除线）
    dimmed?: boolean
    // 多选模式下的选中态
    selected?: boolean
    // 拖入「已完成」列时播放轻微完成动画
    celebrating?: boolean
    // 移动端单列模式：显示状态快速切换按钮（桌面端用拖拽代替）
    mobile?: boolean
  }>(),
  { dimmed: false, selected: false, celebrating: false, mobile: false },
)

const emit = defineEmits<{
  open: [item: WorkItem]
  edit: [item: WorkItem]
  remove: [item: WorkItem]
  toggleSelect: [item: WorkItem]
  toggleDone: [item: WorkItem]
  changeStatus: [item: WorkItem, status: string]
}>()

const auth = useAuthStore()
const canOperate = computed(() => props.item.author_id === auth.user?.id || !!auth.user?.is_admin)
// 状态切换权限：发布者/负责人/管理员（与后端 UpdateStatus 校验一致）
const canChangeStatus = computed(
  () =>
    props.item.author_id === auth.user?.id ||
    props.item.assignee?.id === auth.user?.id ||
    !!auth.user?.is_admin,
)

const priorityMeta: Record<string, { label: string; cls: string }> = {
  high: { label: '高优', cls: 'bg-red-500/10 text-red-500' },
  medium: { label: '中优', cls: 'bg-slate-500/10 text-slate-400' },
  low: { label: '低优', cls: 'bg-sky-500/10 text-sky-500' },
}

const dueInfo = computed(() => {
  const it = props.item
  if (!it.due_date) return null
  const due = dayjs(it.due_date)
  const diff = due.diff(dayjs().startOf('day'), 'day')
  if (it.status === 'done') return { text: due.format('MM-DD') + ' 截止', cls: 'text-slate-500 dark:text-[#9aa0ad]' }
  if (diff < 0) return { text: `逾期 ${-diff} 天`, cls: 'text-red-500 font-semibold' }
  if (diff === 0) return { text: '今日到期', cls: 'text-red-500 font-semibold' }
  if (diff <= 2) return { text: `${diff} 天后到期`, cls: 'text-amber-500' }
  return { text: due.format('MM-DD') + ' 截止', cls: 'text-slate-500 dark:text-[#9aa0ad]' }
})

// 卡片内状态切换菜单（当前状态置灰不可重复选）；自绘菜单而非 NDropdown：
// 嵌在卡片里的 NDropdown trigger 与 @click.stop 组合下 select 事件不稳定
const showStatusMenu = ref(false)
const statusOptions = computed(() => [
  { label: '待办', key: 'todo', disabled: props.item.status === 'todo' },
  { label: '进行中', key: 'doing', disabled: props.item.status === 'doing' },
  { label: '已完成', key: 'done', disabled: props.item.status === 'done' },
])

function onSelectStatus(key: string) {
  showStatusMenu.value = false
  if (key !== props.item.status) emit('changeStatus', props.item, key)
}

// 点击任意处关闭菜单（trigger/菜单容器的 @click.stop 保证不会立即触发）
function closeStatusMenu() {
  showStatusMenu.value = false
}
document.addEventListener('click', closeStatusMenu)
onUnmounted(() => document.removeEventListener('click', closeStatusMenu))
</script>

<template>
  <div
    class="group relative cursor-pointer rounded-[10px] border bg-white p-3 shadow-sm transition-all dark:bg-[#12151b]"
    :class="[
      selected
        ? 'border-violet-400 ring-2 ring-violet-500/40 dark:border-violet-500'
        : 'border-slate-200 hover:border-violet-300 hover:shadow-md dark:border-[#242730] dark:hover:border-violet-500/40',
      dimmed ? 'opacity-60 hover:opacity-90' : '',
      // 无状态切换权限的卡片禁止拖拽（draggable 的 filter 按此 class 排除）
      canChangeStatus ? '' : 'no-drag',
    ]"
    @click="emit('open', item)"
  >
    <!-- 完成动画覆盖层 -->
    <div v-if="celebrating" class="pointer-events-none absolute inset-0 z-10 grid place-items-center rounded-[10px] bg-emerald-500/15">
      <span class="celebrate-pop grid size-9 place-items-center rounded-full bg-emerald-500 text-base text-white shadow-lg">✓</span>
    </div>

    <!-- 多选框：hover 或已选中时显示 -->
    <button
      class="absolute -top-1.5 -left-1.5 z-10 grid size-5 place-items-center rounded-full border text-[10px] shadow-sm transition-opacity"
      :class="
        selected
          ? 'border-violet-500 bg-violet-500 text-white opacity-100'
          : 'border-slate-300 bg-white text-transparent opacity-0 group-hover:opacity-100 dark:border-[#3a3e48] dark:bg-[#1d212b]'
      "
      :title="selected ? '取消选择' : '选择'"
      @click.stop="emit('toggleSelect', item)"
    >
      ✓
    </button>

    <div class="flex items-start justify-between gap-2">
      <div class="flex min-w-0 flex-1 flex-wrap items-center gap-1.5">
        <span class="rounded px-1.5 py-0.5 text-[.65rem] font-bold" :class="priorityMeta[item.priority]?.cls">
          {{ priorityMeta[item.priority]?.label }}
        </span>
        <span class="rounded bg-slate-100 px-1.5 py-0.5 text-[.65rem] font-medium text-slate-500 dark:bg-[#1d212b] dark:text-[#9aa0ad]">
          {{ item.project?.name }}
        </span>
      </div>
      <div class="flex shrink-0 gap-0.5 opacity-0 transition-opacity group-hover:opacity-100">
        <!-- 状态切换：拖拽之外的另一种方式；仅发布者/负责人/管理员可用 -->
        <div v-if="canChangeStatus" class="relative" @click.stop>
          <button
            class="grid size-6 place-items-center rounded text-slate-400 hover:bg-slate-100 hover:text-slate-600 dark:hover:bg-[#1d212b]"
            :class="showStatusMenu ? 'bg-slate-100 text-slate-600 dark:bg-[#1d212b]' : ''"
            title="切换状态"
            @click.stop="showStatusMenu = !showStatusMenu"
          >
            ⇄
          </button>
          <div
            v-if="showStatusMenu"
            class="absolute top-7 right-0 z-20 w-24 overflow-hidden rounded-lg border border-slate-200 bg-white py-1 shadow-lg dark:border-[#2b2e37] dark:bg-[#1d212b]"
          >
            <button
              v-for="opt in statusOptions"
              :key="opt.key"
              class="block w-full px-3 py-1.5 text-left text-xs"
              :class="
                opt.disabled
                  ? 'cursor-default text-slate-300 dark:text-[#4a4e58]'
                  : 'text-slate-600 hover:bg-slate-100 dark:text-[#c7cad1] dark:hover:bg-[#23262f]'
              "
              @click.stop="onSelectStatus(opt.key)"
            >
              {{ opt.label }}
            </button>
          </div>
        </div>
        <button
          v-if="canOperate"
          class="grid size-6 place-items-center rounded text-slate-400 hover:bg-slate-100 hover:text-slate-600 dark:hover:bg-[#1d212b]"
          title="编辑"
          @click.stop="emit('edit', item)"
        >
          ✎
        </button>
        <button
          v-if="canOperate"
          class="grid size-6 place-items-center rounded text-slate-400 hover:bg-red-50 hover:text-red-500 dark:hover:bg-red-500/10"
          title="删除"
          @click.stop="emit('remove', item)"
        >
          ×
        </button>
      </div>
    </div>

    <h4
      class="mt-1.5 text-sm leading-snug font-semibold wrap-break-word"
      :class="dimmed ? 'text-slate-500 line-through dark:text-[#8a8f9c]' : ''"
    >
      {{ item.title }}
    </h4>

    <div class="mt-2.5 flex items-center justify-between">
      <UserAvatar :user="item.assignee || item.author" :size="6" />
      <div class="flex items-center gap-2 text-[.68rem]">
        <span v-if="item.comment_count" class="flex items-center gap-0.5 text-slate-400" :title="`${item.comment_count} 条评论`">
          💬 {{ item.comment_count }}
        </span>
        <span v-if="dueInfo" :class="dueInfo.cls">{{ dueInfo.text }}</span>
        <span v-else class="text-slate-500 dark:text-[#9aa0ad]">{{ item.work_date ? dayjs(item.work_date).format('MM-DD') : '待定' }}</span>
      </div>
    </div>

    <!-- 移动端状态快速切换（桌面端拖拽卡片代替）；仅发布者/负责人/管理员可用 -->
    <button
      v-if="mobile && canChangeStatus"
      class="mt-2 w-full rounded-lg border border-slate-200 py-1 text-xs text-slate-500 hover:bg-slate-50 dark:border-[#2b2e37] dark:hover:bg-[#171a20]"
      @click.stop="emit('toggleDone', item)"
    >
      {{ item.status === 'done' ? '↩ 标记为进行中' : '✓ 标记完成' }}
    </button>
  </div>
</template>

<style scoped>
/* 拖入完成列的轻微完成动画：对勾弹出后稳定 */
@keyframes celebrate-pop {
  0% {
    transform: scale(0);
    opacity: 0;
  }
  60% {
    transform: scale(1.15);
    opacity: 1;
  }
  100% {
    transform: scale(1);
    opacity: 1;
  }
}
.celebrate-pop {
  animation: celebrate-pop 0.5s ease-out both;
}
</style>
