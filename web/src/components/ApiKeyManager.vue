<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  NButton,
  NDatePicker,
  NEmpty,
  NInput,
  NList,
  NListItem,
  NModal,
  NSpin,
  NTag,
  useMessage,
} from 'naive-ui'
import dayjs from 'dayjs'
import { api, type UserAPIKey } from '../api'
import ConfirmDialog from './ConfirmDialog.vue'

const message = useMessage()

const keys = ref<UserAPIKey[]>([])
const loading = ref(false)
const loadFailed = ref(false)

const showCreate = ref(false)
const name = ref('')
const expiresAt = ref<number | null>(null)
const creating = ref(false)
// 完整密钥只短暂保存在当前组件内存中，关闭创建结果弹窗后立即清除。
const createdKey = ref('')

const revokeTarget = ref<UserAPIKey | null>(null)
const revoking = ref(false)

const createModalTitle = computed(() => (createdKey.value ? 'API Key 创建成功' : '创建 API Key'))

function formatTime(value: string | null | undefined) {
  return value ? dayjs(value).format('YYYY-MM-DD HH:mm') : '从未使用'
}

function expirationMeta(item: UserAPIKey) {
  if (!item.expires_at) return { text: '永不过期', type: 'default' as const }
  if (dayjs(item.expires_at).isBefore(dayjs())) return { text: '已过期', type: 'error' as const }
  return { text: `${dayjs(item.expires_at).format('YYYY-MM-DD')} 过期`, type: 'warning' as const }
}

function disablePastDate(timestamp: number) {
  return dayjs(timestamp).isBefore(dayjs(), 'day')
}

async function load() {
  loading.value = true
  loadFailed.value = false
  try {
    const { data } = await api.apiKeys()
    keys.value = data
  } catch (e: any) {
    loadFailed.value = true
    message.error(e.response?.data?.error || 'API Key 加载失败')
  } finally {
    loading.value = false
  }
}

function openCreate() {
  name.value = ''
  expiresAt.value = null
  createdKey.value = ''
  showCreate.value = true
}

function closeCreate() {
  if (creating.value) return
  showCreate.value = false
  createdKey.value = ''
  name.value = ''
  expiresAt.value = null
}

async function createKey() {
  const trimmedName = name.value.trim()
  if (!trimmedName) return

  creating.value = true
  try {
    const payload: { name: string; expires_at?: string } = { name: trimmedName }
    if (expiresAt.value) payload.expires_at = dayjs(expiresAt.value).endOf('day').toISOString()

    const { data } = await api.createAPIKey(payload)
    createdKey.value = data.key
    keys.value = [data, ...keys.value.filter((item) => item.id !== data.id)]
    expiresAt.value = null
  } catch (e: any) {
    message.error(e.response?.data?.error || 'API Key 创建失败')
  } finally {
    creating.value = false
  }
}

async function copyKey() {
  try {
    await navigator.clipboard.writeText(createdKey.value)
    message.success('API Key 已复制')
  } catch {
    message.error('复制失败，请手动复制')
  }
}

async function revokeKey() {
  if (!revokeTarget.value) return

  revoking.value = true
  try {
    await api.deleteAPIKey(revokeTarget.value.id)
    keys.value = keys.value.filter((item) => item.id !== revokeTarget.value?.id)
    message.success('API Key 已吊销')
    revokeTarget.value = null
  } catch (e: any) {
    message.error(e.response?.data?.error || '吊销失败')
  } finally {
    revoking.value = false
  }
}

onMounted(load)
</script>

<template>
  <div>
    <div class="mb-4 flex items-center justify-between gap-3">
      <p class="text-xs leading-5 text-slate-400">
        为桌面客户端或 API 集成创建独立凭证。请勿与他人共享。
      </p>
      <n-button type="primary" size="small" @click="openCreate">创建 API Key</n-button>
    </div>

    <n-spin :show="loading">
      <n-list v-if="keys.length" :show-divider="false" class="bg-transparent!">
        <n-list-item
          v-for="item in keys"
          :key="item.id"
          class="mb-2 rounded-xl! border! border-slate-200! px-4! py-3! dark:border-[#2b2f39]!"
        >
          <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2">
                <span class="truncate text-sm font-semibold">{{ item.name }}</span>
                <n-tag size="small" :type="expirationMeta(item).type" :bordered="false">
                  {{ expirationMeta(item).text }}
                </n-tag>
              </div>
              <div class="mt-1 font-mono text-xs text-slate-500 dark:text-[#9aa0ad]">
                {{ item.key_prefix }}••••••••
              </div>
              <div class="mt-1.5 flex flex-wrap gap-x-4 gap-y-1 text-xs text-slate-400">
                <span>创建于 {{ dayjs(item.created_at).format('YYYY-MM-DD HH:mm') }}</span>
                <span>最近使用 {{ formatTime(item.last_used_at) }}</span>
              </div>
            </div>
            <n-button size="small" type="error" secondary @click="revokeTarget = item">吊销</n-button>
          </div>
        </n-list-item>
      </n-list>

      <div v-else-if="!loading" class="rounded-xl border border-dashed border-slate-200 py-8 dark:border-[#2b2f39]">
        <n-empty :description="loadFailed ? '加载失败，请重试' : '尚未创建 API Key'">
          <template v-if="loadFailed" #extra>
            <n-button size="small" @click="load">重新加载</n-button>
          </template>
        </n-empty>
      </div>
    </n-spin>

    <n-modal
      :show="showCreate"
      preset="card"
      :title="createModalTitle"
      style="width: min(460px, calc(100vw - 32px))"
      :mask-closable="!creating"
      :closable="!creating"
      @update:show="!$event && closeCreate()"
    >
      <div v-if="createdKey">
        <div class="rounded-xl border border-amber-200 bg-amber-50 p-3 dark:border-amber-500/30 dark:bg-amber-500/10">
          <p class="text-sm font-semibold text-amber-700 dark:text-amber-300">请立即复制并妥善保存</p>
          <p class="mt-1 text-xs leading-5 text-amber-600 dark:text-amber-300/80">
            完整 API Key 仅显示这一次。关闭窗口后将无法再次查看，只能吊销并重新创建。
          </p>
        </div>
        <n-input
          class="mt-4 font-mono"
          :value="createdKey"
          type="textarea"
          readonly
          autosize
          @focus="($event.target as HTMLTextAreaElement).select()"
        />
        <div class="mt-5 flex justify-end gap-2">
          <n-button @click="closeCreate">我已保存，关闭</n-button>
          <n-button type="primary" @click="copyKey">复制 API Key</n-button>
        </div>
      </div>

      <div v-else class="flex flex-col gap-4">
        <div>
          <label class="mb-1 block text-xs font-medium text-slate-500">名称 *</label>
          <n-input
            v-model:value="name"
            maxlength="100"
            placeholder="如：我的桌面客户端"
            @keyup.enter="createKey"
          />
          <p class="mt-1.5 text-xs text-slate-400">用于区分不同设备或集成。</p>
        </div>
        <div>
          <label class="mb-1 block text-xs font-medium text-slate-500">过期日期（可选）</label>
          <n-date-picker
            v-model:value="expiresAt"
            type="date"
            class="w-full"
            clearable
            :is-date-disabled="disablePastDate"
            placeholder="不设置则永不过期"
          />
        </div>
        <div class="flex justify-end gap-2">
          <n-button :disabled="creating" @click="closeCreate">取消</n-button>
          <n-button type="primary" :loading="creating" :disabled="!name.trim()" @click="createKey">创建</n-button>
        </div>
      </div>
    </n-modal>

    <ConfirmDialog
      :show="Boolean(revokeTarget)"
      title="吊销 API Key"
      :content="`确定吊销「${revokeTarget?.name || ''}」吗？使用此 Key 的客户端将立即无法访问，且该操作不可撤销。`"
      positive-text="确认吊销"
      :loading="revoking"
      @update:show="!$event && !revoking && (revokeTarget = null)"
      @confirm="revokeKey"
    />
  </div>
</template>
