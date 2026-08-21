<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { NButton, NMention, NEmpty, NPopover, useMessage } from 'naive-ui'
import dayjs from 'dayjs'
import { api, type Comment, type User } from '../api'
import { useAuthStore } from '../stores/auth'
import UserAvatar from './UserAvatar.vue'

const props = defineProps<{ workItemId: number; highlightCommentId?: number }>()

const auth = useAuthStore()
const message = useMessage()

const comments = ref<Comment[]>([])
const users = ref<User[]>([])
const draft = ref('')
const loading = ref(false)
const sending = ref(false)
// 闪烁高亮中的评论 id（来自通知跳转定位）
const flashingId = ref<number | null>(null)

// 顶级评论（正序）与按顶级评论分组的回复列表
const topComments = computed(() => comments.value.filter((c) => !c.parent_id))
const repliesMap = computed(() => {
  const m = new Map<number, Comment[]>()
  for (const c of comments.value) {
    if (c.parent_id) {
      if (!m.has(c.parent_id)) m.set(c.parent_id, [])
      m.get(c.parent_id)!.push(c)
    }
  }
  return m
})

// 回复状态：replyTo 为挂载回复的顶级评论；replyTarget 为直接被回复的那条（回复的回复时自动 @对方）
const replyTo = ref<Comment | null>(null)
const replyTarget = ref<Comment | null>(null)
const replyDraft = ref('')
const replySending = ref(false)

// @ 提及时可选的成员（排除自己）
const mentionOptions = computed(() =>
  users.value.filter((u) => u.id !== auth.user?.id).map((u) => ({ label: u.name, value: u.name })),
)

async function load() {
  loading.value = true
  try {
    const { data } = await api.comments(props.workItemId)
    comments.value = data
    await nextTick()
    scrollToHighlight()
  } finally {
    loading.value = false
  }
}

// 定位到目标评论：滚动到可视区域中央并闪烁高亮数秒。
// 弹窗打开有动画，稍作延迟等 DOM 稳定
function scrollToHighlight() {
  const id = props.highlightCommentId
  if (!id) return
  setTimeout(() => {
    const el = document.getElementById(`comment-${id}`)
    if (!el) return
    el.scrollIntoView({ behavior: 'smooth', block: 'center' })
    flashingId.value = id
    setTimeout(() => {
      flashingId.value = null
    }, 3300)
  }, 300)
}
onMounted(async () => {
  load()
  const { data } = await api.usersCached()
  users.value = data
})
watch(() => props.workItemId, load)

async function send() {
  const content = draft.value.trim()
  if (!content) return
  sending.value = true
  try {
    const { data } = await api.createComment(props.workItemId, content)
    comments.value.push(data)
    draft.value = ''
  } catch (e: any) {
    message.error(e.response?.data?.error || '评论失败')
  } finally {
    sending.value = false
  }
}

// NMention 候选面板打开时，Enter 用于选中成员而非发送：
// keydown 时记录面板是否打开，keyup 时若该次 Enter 是选人则不发送
let mentionPanelOpenOnKeydown = false

function onDraftKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter') {
    mentionPanelOpenOnKeydown = !!document.querySelector('.n-mention-menu')
  }
}

function onDraftEnterKeyup() {
  if (mentionPanelOpenOnKeydown) {
    mentionPanelOpenOnKeydown = false
    return
  }
  send()
}

function onReplyEnterKeyup() {
  if (mentionPanelOpenOnKeydown) {
    mentionPanelOpenOnKeydown = false
    return
  }
  sendReply()
}

// 打开回复框：回复顶级评论直接回复；回复某条回复时挂到同一顶级评论下并自动 @对方
function openReply(top: Comment, target?: Comment) {
  replyTo.value = top
  replyTarget.value = target ?? null
  replyDraft.value = target?.author?.name && target.author_id !== auth.user?.id ? `@${target.author.name} ` : ''
  nextTick(() => {
    document.querySelector<HTMLTextAreaElement>('.reply-box textarea')?.focus()
  })
}

function cancelReply() {
  replyTo.value = null
  replyTarget.value = null
  replyDraft.value = ''
}

async function sendReply() {
  const content = replyDraft.value.trim()
  if (!content || !replyTo.value) return
  replySending.value = true
  try {
    const { data } = await api.createComment(props.workItemId, content, replyTo.value.id)
    comments.value.push(data)
    cancelReply()
  } catch (e: any) {
    message.error(e.response?.data?.error || '回复失败')
  } finally {
    replySending.value = false
  }
}

function canDelete(c: Comment) {
  return c.author_id === auth.user?.id || auth.user?.is_admin
}

// 把评论内容拆成文本/提及片段：@用户名 渲染为彩色文本，hover 展示个人信息
interface Segment {
  text: string
  user?: User
}

function isNameChar(ch: string) {
  return /[\u4e00-\u9fff\w-]/.test(ch)
}

function segmentsOf(content: string): Segment[] {
  const segs: Segment[] = []
  const sorted = users.value.filter((u) => u.name).sort((a, b) => b.name.length - a.name.length)
  let rest = content
  while (rest.length) {
    const at = rest.indexOf('@')
    if (at < 0) {
      segs.push({ text: rest })
      break
    }
    if (at > 0) segs.push({ text: rest.slice(0, at) })
    const after = rest.slice(at + 1)
    const hit = sorted.find((u) => {
      if (!after.startsWith(u.name)) return false
      const next = after[u.name.length]
      return next === undefined || !isNameChar(next)
    })
    if (hit) {
      segs.push({ text: '@' + hit.name, user: hit })
      rest = after.slice(hit.name.length)
    } else {
      segs.push({ text: '@' })
      rest = after
    }
  }
  return segs
}

async function remove(c: Comment) {
  try {
    await api.deleteComment(c.id)
    // 删除顶级评论时其回复也被后端级联删除，前端同步移除
    comments.value = comments.value.filter((x) => x.id !== c.id && x.parent_id !== c.id)
    message.success('已删除')
  } catch (e: any) {
    message.error(e.response?.data?.error || '删除失败')
  }
}
</script>

<template>
  <div>
    <div class="mb-2 text-xs font-bold text-slate-500">评论（{{ comments.length }}）</div>

    <div v-if="comments.length" class="mb-3 flex max-h-64 flex-col gap-3 overflow-y-auto pr-1">
      <div v-for="c in topComments" :key="c.id">
        <!-- 顶级评论 -->
        <div
          :id="'comment-' + c.id"
          class="group -mx-1 flex items-start gap-2 rounded-lg px-1"
          :class="{ 'comment-flash': flashingId === c.id }"
        >
          <UserAvatar :user="c.author" :size="6" />
          <div class="min-w-0 flex-1">
            <div class="flex items-baseline gap-2">
              <span class="text-xs font-bold">{{ c.author?.name }}</span>
              <span class="text-[.65rem] text-slate-400">{{ dayjs(c.created_at).format('MM-DD HH:mm') }}</span>
              <button
                class="text-[.65rem] text-slate-300 opacity-0 transition-opacity group-hover:opacity-100 hover:text-violet-500 dark:text-slate-600 dark:hover:text-violet-400"
                @click="openReply(c)"
              >
                回复
              </button>
              <button
                v-if="canDelete(c)"
                class="text-[.65rem] text-slate-300 opacity-0 transition-opacity group-hover:opacity-100 hover:text-red-500 dark:text-slate-600"
                @click="remove(c)"
              >
                删除
              </button>
            </div>
            <p class="mt-0.5 text-sm break-words whitespace-pre-wrap text-slate-700 dark:text-[#c7cad1]"><template v-for="(seg, i) in segmentsOf(c.content)" :key="i"><n-popover v-if="seg.user" trigger="hover" :delay="200" placement="top"><template #trigger><span class="cursor-default font-medium text-violet-500 dark:text-violet-400">{{ seg.text }}</span></template><div class="flex items-center gap-2.5 py-0.5"><UserAvatar :user="seg.user" :size="9" /><div><div class="text-sm font-semibold">{{ seg.user.name }}<span v-if="seg.user.is_admin" class="ml-1 rounded bg-violet-500/10 px-1 py-0.5 text-[.6rem] font-bold text-violet-500">管理员</span></div><div class="mt-0.5 text-xs text-slate-400">{{ seg.user.email || '未填写邮箱' }}</div></div></div></n-popover><span v-else>{{ seg.text }}</span></template></p>
          </div>
        </div>

        <!-- 回复列表（缩进一层） -->
        <div v-if="repliesMap.get(c.id)?.length" class="mt-2 ml-7 flex flex-col gap-2 border-l-2 border-slate-100 pl-3 dark:border-[#242730]">
          <div
            v-for="r in repliesMap.get(c.id)"
            :key="r.id"
            :id="'comment-' + r.id"
            class="group flex items-start gap-2 rounded-lg"
            :class="{ 'comment-flash': flashingId === r.id }"
          >
            <UserAvatar :user="r.author" :size="5" />
            <div class="min-w-0 flex-1">
              <div class="flex items-baseline gap-2">
                <span class="text-xs font-bold">{{ r.author?.name }}</span>
                <span class="text-[.65rem] text-slate-400">{{ dayjs(r.created_at).format('MM-DD HH:mm') }}</span>
                <button
                  class="text-[.65rem] text-slate-300 opacity-0 transition-opacity group-hover:opacity-100 hover:text-violet-500 dark:text-slate-600 dark:hover:text-violet-400"
                  @click="openReply(c, r)"
                >
                  回复
                </button>
                <button
                  v-if="canDelete(r)"
                  class="text-[.65rem] text-slate-300 opacity-0 transition-opacity group-hover:opacity-100 hover:text-red-500 dark:text-slate-600"
                  @click="remove(r)"
                >
                  删除
                </button>
              </div>
              <p class="mt-0.5 text-sm break-words whitespace-pre-wrap text-slate-700 dark:text-[#c7cad1]"><template v-for="(seg, i) in segmentsOf(r.content)" :key="i"><n-popover v-if="seg.user" trigger="hover" :delay="200" placement="top"><template #trigger><span class="cursor-default font-medium text-violet-500 dark:text-violet-400">{{ seg.text }}</span></template><div class="flex items-center gap-2.5 py-0.5"><UserAvatar :user="seg.user" :size="9" /><div><div class="text-sm font-semibold">{{ seg.user.name }}<span v-if="seg.user.is_admin" class="ml-1 rounded bg-violet-500/10 px-1 py-0.5 text-[.6rem] font-bold text-violet-500">管理员</span></div><div class="mt-0.5 text-xs text-slate-400">{{ seg.user.email || '未填写邮箱' }}</div></div></div></n-popover><span v-else>{{ seg.text }}</span></template></p>
            </div>
          </div>
        </div>

        <!-- 内联回复框（同一时刻只展开一个） -->
        <div v-if="replyTo?.id === c.id" class="reply-box mt-2 ml-7 flex items-start gap-2">
          <n-mention
            v-model:value="replyDraft"
            type="textarea"
            :options="mentionOptions"
            :autosize="{ minRows: 1, maxRows: 4 }"
            :placeholder="`回复 ${replyTarget?.author?.name || c.author?.name}…`"
            @keydown="onDraftKeydown"
            @keyup.enter.exact="onReplyEnterKeyup"
          />
          <div class="flex shrink-0 flex-col gap-1">
            <n-button type="primary" size="tiny" :loading="replySending" :disabled="!replyDraft.trim()" @click="sendReply">发送</n-button>
            <n-button size="tiny" quaternary @click="cancelReply">取消</n-button>
          </div>
        </div>
      </div>
    </div>
    <n-empty v-else-if="!loading" description="暂无评论" size="small" class="py-4" />

    <div class="flex items-start gap-2">
      <n-mention
        v-model:value="draft"
        type="textarea"
        :options="mentionOptions"
        :autosize="{ minRows: 1, maxRows: 4 }"
        placeholder="写下你的评论，输入 @ 可以提及成员…"
        @keydown="onDraftKeydown"
        @keyup.enter.exact="onDraftEnterKeyup"
      />
      <n-button type="primary" size="small" :loading="sending" :disabled="!draft.trim()" @click="send">发送</n-button>
    </div>
    <p class="mt-1.5 text-xs text-slate-400">评论此任务涉及到的成员都会收到站内通知；对方绑定飞书 open_id 后还会收到飞书消息，也可支持@任务之外的其他人员</p>
  </div>
</template>

<style scoped>
/* 通知跳转定位后的闪烁高亮：violet 背景呼吸 3 次 */
@keyframes comment-flash {
  0%,
  100% {
    background-color: transparent;
  }
  50% {
    background-color: rgb(124 58 237 / 0.16);
  }
}
.comment-flash {
  animation: comment-flash 1.1s ease-in-out 3;
}
</style>
