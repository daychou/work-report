<script setup lang="ts">
import { computed, ref } from 'vue'
import { NDrawer, NDrawerContent, NTag } from 'naive-ui'
import DOMPurify from 'dompurify'
import dayjs from 'dayjs'
import type { WorkItem } from '../api'
import { isHTMLContent, isEmptyRichText } from '../utils/richText'
import UserAvatar from './UserAvatar.vue'
import CommentSection from './CommentSection.vue'

const props = defineProps<{
  show: boolean
  item: WorkItem | null
  // 打开详情后需要定位并闪烁高亮的评论（来自 @提及 通知跳转）
  highlightCommentId?: number
}>()
const emit = defineEmits<{ close: [] }>()

const statusCN: Record<string, string> = { todo: '待办', doing: '进行中', done: '已完成', cancelled: '已取消' }
const priorityCN: Record<string, { label: string; type: 'error' | 'warning' | 'default' }> = {
  high: { label: '高优先级', type: 'error' },
  medium: { label: '中优先级', type: 'warning' },
  low: { label: '低优先级', type: 'default' },
}

const item = computed(() => props.item)

// 正文为富文本 HTML：消毒后渲染（防 XSS）；历史纯文本数据仍按原样换行展示
const contentIsHTML = computed(() => isHTMLContent(item.value?.content || ''))
const sanitizedContent = computed(() =>
  contentIsHTML.value ? DOMPurify.sanitize(item.value!.content) : '',
)

// 详细内容（第二层）：默认折叠，避免长内容刷屏；同样兼容历史纯文本
const hasDetail = computed(() => !isEmptyRichText(item.value?.detail || ''))
const detailExpanded = ref(false)
const detailIsHTML = computed(() => isHTMLContent(item.value?.detail || ''))
const sanitizedDetail = computed(() =>
  detailIsHTML.value ? DOMPurify.sanitize(item.value!.detail || '') : '',
)
</script>

<template>
  <!-- 侧边抽屉：看板卡片点击后滑出详情，避免整页跳转/遮罩式弹窗打断上下文 -->
  <n-drawer :show="show" :width="520" placement="right" @update:show="emit('close')">
    <n-drawer-content closable body-content-style="padding: 20px 24px">
      <template #header>
        <span class="text-sm">任务详情</span>
      </template>
      <template v-if="item">
        <div class="mb-3 flex flex-wrap items-center gap-1.5">
          <n-tag size="small" :bordered="false">{{ item.project?.name }}</n-tag>
          <n-tag size="small" :bordered="false" :type="priorityCN[item.priority]?.type">
            {{ priorityCN[item.priority]?.label }}
          </n-tag>
          <n-tag size="small" :bordered="false" type="warning">{{ statusCN[item.status] }}</n-tag>
        </div>

        <h3 class="mb-1 text-base font-bold">{{ item.title }}</h3>
        <div
          v-if="contentIsHTML"
          class="rich-content mb-3 text-sm text-slate-600 dark:text-[#c7cad1]"
          v-html="sanitizedContent"
        ></div>
        <p v-else-if="item.content" class="mb-3 text-sm whitespace-pre-wrap text-slate-600 dark:text-[#c7cad1]">{{ item.content }}</p>

        <!-- 详细内容（第二层）：默认折叠，点击展开 -->
        <div v-if="hasDetail" class="mb-3 rounded-lg border border-slate-100 dark:border-[#1d212b]">
          <button
            class="flex w-full items-center gap-1.5 px-3 py-2 text-xs font-medium text-slate-500 hover:text-slate-700 dark:hover:text-[#c7cad1]"
            @click="detailExpanded = !detailExpanded"
          >
            <span class="transition-transform" :class="detailExpanded ? 'rotate-90' : ''">▸</span>
            详细内容
            <span class="font-normal text-slate-400">（过程细节 / 附件，不提交给 AI 分析）</span>
          </button>
          <div v-show="detailExpanded" class="border-t border-slate-100 px-3 py-2 dark:border-[#1d212b]">
            <div v-if="detailIsHTML" class="rich-content text-sm text-slate-600 dark:text-[#c7cad1]" v-html="sanitizedDetail"></div>
            <p v-else class="text-sm whitespace-pre-wrap text-slate-600 dark:text-[#c7cad1]">{{ item!.detail }}</p>
          </div>
        </div>

        <div class="mb-4 flex flex-wrap items-center gap-3 border-b border-slate-100 pb-3 text-xs text-slate-400 dark:border-[#242730]">
          <span class="flex items-center gap-1"><UserAvatar :user="item.author" :size="5" />提交：{{ item.author?.name }}</span>
          <span v-if="item.assignee && item.assignee.id !== item.author_id" class="flex items-center gap-1">
            <UserAvatar :user="item.assignee" :size="5" />负责：{{ item.assignee.name }}
          </span>
          <span v-if="item.participants?.length" class="flex items-center gap-1">
            <span class="flex">
              <span v-for="u in item.participants" :key="u.id" class="-mr-1"><UserAvatar :user="u" :size="5" /></span>
            </span>
            参与：{{ item.participants.map((u) => u.name).join('、') }}
          </span>
          <span>工作日期 {{ dayjs(item.work_date).format('YYYY-MM-DD') }}</span>
          <span v-if="item.due_date" :class="dayjs(item.due_date).isBefore(dayjs(), 'day') && item.status !== 'done' ? 'text-red-500' : ''">
            截止 {{ dayjs(item.due_date).format('YYYY-MM-DD') }}
          </span>
          <span v-if="item.done_at">完成于 {{ dayjs(item.done_at).format('MM-DD HH:mm') }}</span>
        </div>

        <CommentSection :work-item-id="item.id" :highlight-comment-id="highlightCommentId" />
      </template>
    </n-drawer-content>
  </n-drawer>
</template>

<style scoped>
/* 富文本详情展示：图片自适应宽度，附件链接卡片化 */
.rich-content :deep(img) {
  max-width: 100%;
  border-radius: 8px;
  margin: 4px 0;
}
.rich-content :deep(a[data-w-e-type='attachment']) {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 10px;
  margin: 2px 0;
  border-radius: 6px;
  background: rgb(139 92 246 / 8%);
  color: #8b5cf6;
  text-decoration: none;
  font-size: 0.8125rem;
}
.rich-content :deep(a[data-w-e-type='attachment']::before) {
  content: '📎';
}
.rich-content :deep(p) {
  margin: 4px 0;
}
.rich-content :deep(table) {
  border-collapse: collapse;
}
.rich-content :deep(td),
.rich-content :deep(th) {
  border: 1px solid #cbd5e1;
  padding: 4px 8px;
}
</style>
