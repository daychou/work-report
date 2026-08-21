<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NButton, NModal, NInput, NSelect, NCheckbox, NSwitch, NEmpty, NTag, useMessage } from 'naive-ui'
import dayjs from 'dayjs'
import { api, invalidateUsersCache, type AIModel, type OSSConfig, type Role, type User } from '../api'
import { useAuthStore } from '../stores/auth'
import UserAvatar from '../components/UserAvatar.vue'
import ConfirmDialog from '../components/ConfirmDialog.vue'
import ProjectsView from './ProjectsView.vue'

const auth = useAuthStore()
const message = useMessage()
const route = useRoute()
const router = useRouter()

// 标签页：成员 / 角色 / 项目 / AI 模型 / 附件存储（支持 ?tab= 深链接）
type TabKey = 'members' | 'roles' | 'projects' | 'ai' | 'oss'
const tab = ref<TabKey>(['members', 'roles', 'projects', 'ai', 'oss'].includes(route.query.tab as string) ? (route.query.tab as TabKey) : 'members')
watch(tab, (v) => {
  router.replace({ query: v === 'members' ? {} : { tab: v } })
})

const users = ref<User[]>([])
const roles = ref<Role[]>([])
const aiModels = ref<AIModel[]>([])
const loading = ref(false)

const showCreate = ref(false)
const createName = ref('')
const createEmail = ref('')
const saving = ref(false)

const showEdit = ref(false)
const editTarget = ref<User | null>(null)
const editName = ref('')
const editEmail = ref('')
const editRoleId = ref<number | null>(null)

const isAdmin = computed(() => !!auth.user?.is_admin)

const roleOptions = computed(() => roles.value.map((r) => ({ label: r.name, value: r.id })))

function roleName(u: User) {
  return u.role?.name || '未分配'
}

async function load() {
  loading.value = true
  try {
    const [u, r, m] = await Promise.all([
      api.users(),
      // 后端未升级（无 roles 接口）时回退空数组，保证成员编辑等基础功能可用
      api.roles().catch(() => ({ data: [] as Role[] })),
      api.aiModels().catch(() => ({ data: [] as AIModel[] })),
    ])
    users.value = Array.isArray(u.data) ? u.data : []
    roles.value = Array.isArray(r.data) ? r.data : []
    aiModels.value = Array.isArray(m.data) ? m.data : []
  } finally {
    loading.value = false
  }
}
onMounted(load)

// 账号来源：manual: 手动创建（待关联）/ local: 本地账号 / dev: 开发模式 / 其他为统一认证已绑定
function sourceTag(u: User) {
  if (u.casdoor_id.startsWith('manual:')) return { text: '手动创建 · 待关联', type: 'warning' as const }
  if (u.casdoor_id.startsWith('local:')) return { text: '本地账号', type: 'info' as const }
  if (u.casdoor_id.startsWith('dev:')) return { text: '开发模式', type: 'default' as const }
  return { text: '统一认证已绑定', type: 'success' as const }
}

async function create() {
  if (!createName.value.trim()) return
  saving.value = true
  try {
    await api.createUser({ name: createName.value.trim(), email: createEmail.value.trim() })
    invalidateUsersCache()
    message.success('成员已创建；对方用统一认证登录且名称相同时会自动关联')
    showCreate.value = false
    createName.value = ''
    createEmail.value = ''
    load()
  } catch (e: any) {
    message.error(e.response?.data?.error || '创建失败')
  } finally {
    saving.value = false
  }
}

function openEdit(u: User) {
  editTarget.value = u
  editName.value = u.name
  editEmail.value = u.email
  editRoleId.value = u.role_id ?? null
  showEdit.value = true
}

// 模拟身份（排查问题用）：确认后切换到目标用户视角，右上角可退出
const confirmImpersonate = ref<{ show: boolean; target: User | null; loading: boolean }>({
  show: false,
  target: null,
  loading: false,
})

function openImpersonate(u: User) {
  confirmImpersonate.value = { show: true, target: u, loading: false }
}

async function doImpersonate() {
  const target = confirmImpersonate.value.target
  if (!target) return
  confirmImpersonate.value.loading = true
  try {
    // 成功后 store 内会整页跳转到看板
    await auth.impersonate(target.id)
  } catch (e: any) {
    message.error(e.response?.data?.error || '模拟身份失败')
    confirmImpersonate.value.loading = false
  }
}

async function saveEdit() {
  if (!editTarget.value || !editName.value.trim()) return
  saving.value = true
  try {
    const payload: { name: string; email: string; role_id?: number } = {
      name: editName.value.trim(),
      email: editEmail.value.trim(),
    }
    if (editRoleId.value) payload.role_id = editRoleId.value
    await api.updateUser(editTarget.value.id, payload)
    invalidateUsersCache()
    message.success('已保存')
    showEdit.value = false
    load()
  } catch (e: any) {
    message.error(e.response?.data?.error || '保存失败')
  } finally {
    saving.value = false
  }
}

// ---- 角色管理 ----
const showRoleEditor = ref(false)
const roleEditTarget = ref<Role | null>(null) // null = 新建
const roleName_ = ref('')
const roleDesc = ref('')
const roleIsAdmin = ref(false)

function openRoleCreate() {
  roleEditTarget.value = null
  roleName_.value = ''
  roleDesc.value = ''
  roleIsAdmin.value = false
  showRoleEditor.value = true
}

function openRoleEdit(r: Role) {
  roleEditTarget.value = r
  roleName_.value = r.name
  roleDesc.value = r.description
  roleIsAdmin.value = r.is_admin
  showRoleEditor.value = true
}

async function saveRole() {
  if (!roleName_.value.trim()) return
  saving.value = true
  try {
    const payload = { name: roleName_.value.trim(), description: roleDesc.value.trim(), is_admin: roleIsAdmin.value }
    if (roleEditTarget.value) {
      await api.updateRole(roleEditTarget.value.id, payload)
    } else {
      await api.createRole(payload)
    }
    invalidateUsersCache()
    message.success('已保存')
    showRoleEditor.value = false
    load()
  } catch (e: any) {
    message.error(e.response?.data?.error || '保存失败')
  } finally {
    saving.value = false
  }
}

const confirmDeleteRole = ref<{ show: boolean; target: Role | null; loading: boolean }>({
  show: false,
  target: null,
  loading: false,
})

function openDeleteRole(r: Role) {
  confirmDeleteRole.value = { show: true, target: r, loading: false }
}

async function doDeleteRole() {
  const target = confirmDeleteRole.value.target
  if (!target) return
  confirmDeleteRole.value.loading = true
  try {
    await api.deleteRole(target.id)
    message.success('角色已删除')
    confirmDeleteRole.value.show = false
    load()
  } catch (e: any) {
    message.error(e.response?.data?.error || '删除失败')
  } finally {
    confirmDeleteRole.value.loading = false
  }
}

// ---- AI 模型管理 ----
const showAIEditor = ref(false)
const aiEditTarget = ref<AIModel | null>(null) // null = 新建
const aiName = ref('')
const aiProvider = ref('deepseek')
const aiModelId = ref('')
const aiApiKey = ref('')
const aiBaseURL = ref('')
const aiEnabled = ref(false)

function openAICreate() {
  aiEditTarget.value = null
  aiName.value = ''
  aiProvider.value = 'deepseek'
  aiModelId.value = ''
  aiApiKey.value = ''
  aiBaseURL.value = 'https://api.deepseek.com/v1'
  aiEnabled.value = false
  showAIEditor.value = true
}

function openAIEdit(m: AIModel) {
  aiEditTarget.value = m
  aiName.value = m.name
  aiProvider.value = m.provider
  aiModelId.value = m.model_id
  aiApiKey.value = m.api_key || ''
  aiBaseURL.value = m.base_url || ''
  aiEnabled.value = m.enabled
  showAIEditor.value = true
}

async function saveAIModel() {
  if (!aiName.value.trim() || !aiModelId.value.trim()) return
  saving.value = true
  try {
    const payload = {
      name: aiName.value.trim(),
      provider: aiProvider.value.trim(),
      model_id: aiModelId.value.trim(),
      api_key: aiApiKey.value.trim(),
      base_url: aiBaseURL.value.trim(),
      enabled: aiEnabled.value,
    }
    if (aiEditTarget.value) {
      await api.updateAIModel(aiEditTarget.value.id, payload)
    } else {
      await api.createAIModel(payload)
    }
    message.success('已保存')
    showAIEditor.value = false
    load()
  } catch (e: any) {
    message.error(e.response?.data?.error || '保存失败')
  } finally {
    saving.value = false
  }
}

// 列表行内快速切换启用状态
async function toggleAIEnabled(m: AIModel, enabled: boolean) {
  try {
    await api.updateAIModel(m.id, {
      name: m.name,
      provider: m.provider,
      model_id: m.model_id,
      api_key: m.api_key || '',
      base_url: m.base_url || '',
      enabled,
    })
    message.success(enabled ? `已启用 ${m.name}` : `已停用 ${m.name}`)
    load()
  } catch (e: any) {
    message.error(e.response?.data?.error || '操作失败')
  }
}

// ---- 附件存储（阿里云 OSS）配置：单行配置，任务富文本的图片/附件经服务端中继上传 ----
const ossForm = ref<OSSConfig>({ endpoint: '', bucket: '', access_key_id: '', access_key_secret: '', dir: 'work-report', domain: '' })
const ossLoading = ref(false)
const ossSaving = ref(false)

async function loadOSSConfig() {
  ossLoading.value = true
  try {
    const { data } = await api.ossConfig()
    if (data && data.id) {
      ossForm.value = { ...data, access_key_secret: '' } // Secret 不回显，留空表示不修改
    }
  } catch {
    // 非管理员或接口不可用时静默（页面本身仅管理员可见）
  } finally {
    ossLoading.value = false
  }
}

async function saveOSSConfig() {
  ossSaving.value = true
  try {
    await api.saveOSSConfig(ossForm.value)
    message.success('OSS 配置已保存')
    loadOSSConfig()
  } catch (e: any) {
    message.error(e.response?.data?.error || '保存失败')
  } finally {
    ossSaving.value = false
  }
}

watch(tab, (v) => {
  if (v === 'oss') loadOSSConfig()
})

const confirmDeleteAI = ref<{ show: boolean; target: AIModel | null; loading: boolean }>({
  show: false,
  target: null,
  loading: false,
})

function openDeleteAI(m: AIModel) {
  confirmDeleteAI.value = { show: true, target: m, loading: false }
}

async function doDeleteAI() {
  const target = confirmDeleteAI.value.target
  if (!target) return
  confirmDeleteAI.value.loading = true
  try {
    await api.deleteAIModel(target.id)
    message.success('模型已删除')
    confirmDeleteAI.value.show = false
    load()
  } catch (e: any) {
    message.error(e.response?.data?.error || '删除失败')
  } finally {
    confirmDeleteAI.value.loading = false
  }
}
</script>

<template>
  <div class="mx-auto max-w-4xl">
    <h2 class="mb-1 text-lg font-bold">系统设置</h2>
    <p class="mb-4 text-xs text-slate-400">仅管理员可见。权限由平台内部角色决定，统一认证（Casdoor）仅用于登录。</p>

    <!-- 标签页：成员 / 角色 / 项目 / AI 模型 / 附件存储 -->
    <div class="mb-4 flex gap-1 rounded-xl bg-slate-100 p-1 dark:bg-[#171a20] w-fit">
      <button
        v-for="t in [
          { key: 'members', label: '成员管理' },
          { key: 'roles', label: '角色管理' },
          { key: 'projects', label: '项目管理' },
          { key: 'ai', label: 'AI 模型' },
          { key: 'oss', label: '附件存储' },
        ]"
        :key="t.key"
        class="rounded-lg px-3 py-1.5 text-sm font-medium transition-all"
        :class="
          tab === t.key
            ? 'bg-white text-violet-600 shadow-sm ring-1 ring-violet-500/30 dark:bg-[#23262f] dark:text-violet-400'
            : 'text-slate-500 hover:text-slate-700 dark:text-[#8a8f9c] dark:hover:text-[#c7cad1]'
        "
        @click="tab = t.key as TabKey"
      >
        {{ t.label }}
      </button>
    </div>

    <ProjectsView v-if="tab === 'projects'" />

    <!-- 附件存储（阿里云 OSS）配置 -->
    <template v-else-if="tab === 'oss'">
      <div class="mb-3">
        <h3 class="text-sm font-bold text-slate-500">阿里云 OSS 配置</h3>
      </div>
      <p class="mb-3 rounded-lg bg-slate-50 px-3 py-2 text-xs text-slate-400 dark:bg-[#171a20]">
        任务详细内容中的图片和附件经服务端中继上传到该 Bucket（AccessKey 不下发浏览器），单个附件不超过 500M。
        Bucket 需开启「公共读」，或配置自定义访问域名（如 CDN 域名），否则前端无法展示图片和下载附件。
        经 Nginx 反代部署时，需设置 client_max_body_size 500m 放行大文件。
      </p>

      <div class="rounded-xl border border-slate-200 bg-white p-5 dark:border-[#242730] dark:bg-[#12151b]">
        <div class="grid grid-cols-2 gap-4">
          <div>
            <label class="mb-1 block text-xs font-medium text-slate-500">Endpoint *</label>
            <n-input v-model:value="ossForm.endpoint" placeholder="如 oss-cn-hangzhou.aliyuncs.com" :disabled="ossLoading" />
          </div>
          <div>
            <label class="mb-1 block text-xs font-medium text-slate-500">Bucket 名称 *</label>
            <n-input v-model:value="ossForm.bucket" placeholder="如 my-work-report" :disabled="ossLoading" />
          </div>
        </div>
        <div class="mt-4 grid grid-cols-2 gap-4">
          <div>
            <label class="mb-1 block text-xs font-medium text-slate-500">AccessKey ID *</label>
            <n-input v-model:value="ossForm.access_key_id" placeholder="LTAI..." :disabled="ossLoading" />
          </div>
          <div>
            <label class="mb-1 block text-xs font-medium text-slate-500">AccessKey Secret</label>
            <n-input
              v-model:value="ossForm.access_key_secret"
              type="password"
              show-password-on="click"
              placeholder="留空表示保持不变"
              :disabled="ossLoading"
            />
          </div>
        </div>
        <div class="mt-4 grid grid-cols-2 gap-4">
          <div>
            <label class="mb-1 block text-xs font-medium text-slate-500">存储目录前缀</label>
            <n-input v-model:value="ossForm.dir" placeholder="如 work-report（对象 key 前缀，可选）" :disabled="ossLoading" />
          </div>
          <div>
            <label class="mb-1 block text-xs font-medium text-slate-500">自定义访问域名</label>
            <n-input v-model:value="ossForm.domain" placeholder="如 https://cdn.example.com（可选）" :disabled="ossLoading" />
          </div>
        </div>
        <div class="mt-5 flex justify-end">
          <n-button
            type="primary"
            :loading="ossSaving"
            :disabled="ossLoading || !ossForm.endpoint.trim() || !ossForm.bucket.trim() || !ossForm.access_key_id.trim()"
            @click="saveOSSConfig"
          >
            保存配置
          </n-button>
        </div>
      </div>
    </template>

    <!-- AI 模型管理 -->
    <template v-else-if="tab === 'ai'">
      <div class="mb-3 flex items-center justify-between">
        <h3 class="text-sm font-bold text-slate-500">AI 模型（{{ aiModels.length }}）</h3>
        <n-button v-if="isAdmin" type="primary" size="small" @click="openAICreate">+ 添加模型</n-button>
      </div>
      <p class="mb-3 rounded-lg bg-slate-50 px-3 py-2 text-xs text-slate-400 dark:bg-[#171a20]">
        只有「启用」的模型才能在 AI 分析页被选用；API Key 仅管理员可见。
      </p>

      <div v-if="aiModels.length" class="overflow-hidden rounded-xl border border-slate-200 bg-white dark:border-[#242730] dark:bg-[#12151b]">
        <div
          v-for="m in aiModels"
          :key="m.id"
          class="flex items-center gap-3 border-b border-slate-100 px-4 py-3 last:border-0 dark:border-[#1d212b]"
        >
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
              <span class="text-sm font-semibold">{{ m.name }}</span>
              <n-tag size="tiny" :bordered="false">{{ m.provider }}</n-tag>
              <span class="rounded bg-slate-100 px-1.5 py-0.5 text-[.65rem] text-slate-500 dark:bg-[#1d212b]">{{ m.model_id }}</span>
            </div>
            <div class="mt-0.5 text-xs text-slate-400">
              {{ m.api_key ? '已配置 API Key' : '未配置 API Key' }}<span v-if="m.base_url"> · {{ m.base_url }}</span>
            </div>
          </div>
          <n-switch :value="m.enabled" size="small" @update:value="(v: boolean) => toggleAIEnabled(m, v)" />
          <div v-if="isAdmin" class="flex shrink-0 gap-1">
            <button class="rounded px-2 py-1 text-xs text-violet-500 hover:bg-violet-500/10" @click="openAIEdit(m)">编辑</button>
            <button class="rounded px-2 py-1 text-xs text-red-500 hover:bg-red-500/10" @click="openDeleteAI(m)">删除</button>
          </div>
        </div>
      </div>
      <n-empty v-else :description="loading ? '加载中…' : '暂无 AI 模型'" class="py-16" />
    </template>

    <!-- 角色管理 -->
    <template v-else-if="tab === 'roles'">
      <div class="mb-3 flex items-center justify-between">
        <h3 class="text-sm font-bold text-slate-500">角色（{{ roles.length }}）</h3>
        <n-button v-if="isAdmin" type="primary" size="small" @click="openRoleCreate">+ 新建角色</n-button>
      </div>

      <div v-if="roles.length" class="overflow-hidden rounded-xl border border-slate-200 bg-white dark:border-[#242730] dark:bg-[#12151b]">
        <div
          v-for="r in roles"
          :key="r.id"
          class="flex items-center gap-3 border-b border-slate-100 px-4 py-3 last:border-0 dark:border-[#1d212b]"
        >
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
              <span class="text-sm font-semibold">{{ r.name }}</span>
              <span v-if="r.is_admin" class="rounded bg-violet-500/10 px-1 py-0.5 text-[.6rem] font-bold text-violet-500">管理员</span>
              <n-tag v-if="r.built_in" size="tiny" :bordered="false">内置</n-tag>
            </div>
            <div class="mt-0.5 text-xs text-slate-400">
              {{ r.description || '—' }} · {{ r.member_count ?? 0 }} 名成员
            </div>
          </div>
          <div v-if="isAdmin" class="flex shrink-0 gap-1">
            <button class="rounded px-2 py-1 text-xs text-violet-500 hover:bg-violet-500/10" @click="openRoleEdit(r)">编辑</button>
            <button
              v-if="!r.built_in"
              class="rounded px-2 py-1 text-xs text-red-500 hover:bg-red-500/10"
              @click="openDeleteRole(r)"
            >
              删除
            </button>
          </div>
        </div>
      </div>
      <n-empty v-else :description="loading ? '加载中…' : '暂无角色'" class="py-16" />
    </template>

    <!-- 成员管理 -->
    <template v-else>
    <div class="mb-3 flex items-center justify-between">
      <h3 class="text-sm font-bold text-slate-500">成员（{{ users.length }}）</h3>
      <n-button v-if="isAdmin" type="primary" size="small" @click="showCreate = true">+ 添加成员</n-button>
    </div>

    <div v-if="users.length" class="overflow-hidden rounded-xl border border-slate-200 bg-white dark:border-[#242730] dark:bg-[#12151b]">
      <div
        v-for="u in users"
        :key="u.id"
        class="flex items-center gap-3 border-b border-slate-100 px-4 py-3 last:border-0 dark:border-[#1d212b]"
      >
        <UserAvatar :user="u" :size="8" />
        <div class="min-w-0 flex-1">
          <div class="flex items-center gap-2">
            <span class="text-sm font-semibold">{{ u.name }}</span>
            <span v-if="u.is_admin" class="rounded bg-violet-500/10 px-1 py-0.5 text-[.6rem] font-bold text-violet-500">管理员</span>
            <n-tag size="tiny" :bordered="false" type="default">{{ roleName(u) }}</n-tag>
            <n-tag :type="sourceTag(u).type" size="tiny" :bordered="false">{{ sourceTag(u).text }}</n-tag>
          </div>
          <div class="mt-0.5 text-xs text-slate-400">
            {{ u.email || '—' }} · 加入于 {{ dayjs(u.created_at).format('YYYY-MM-DD') }}
            <span v-if="u.feishu_open_id"> · 已绑定飞书</span>
          </div>
        </div>
        <div v-if="isAdmin" class="flex shrink-0 gap-1">
          <button
            v-if="u.id !== auth.user?.id"
            class="rounded px-2 py-1 text-xs text-amber-600 hover:bg-amber-500/10 dark:text-amber-400"
            title="以该成员的身份访问平台，用于排查问题"
            @click="openImpersonate(u)"
          >
            模拟身份
          </button>
          <button class="rounded px-2 py-1 text-xs text-violet-500 hover:bg-violet-500/10" @click="openEdit(u)">编辑</button>
        </div>
      </div>
    </div>
    <n-empty v-else :description="loading ? '加载中…' : '暂无成员'" class="py-16" />
    </template>

    <n-modal :show="showCreate" preset="card" title="添加成员" style="width: 420px" @update:show="showCreate = false">
      <div class="flex flex-col gap-4">
        <div>
          <label class="mb-1 block text-xs font-medium text-slate-500">名称 *</label>
          <n-input v-model:value="createName" placeholder="与统一认证显示名一致即可自动关联" />
        </div>
        <div>
          <label class="mb-1 block text-xs font-medium text-slate-500">邮箱</label>
          <n-input v-model:value="createEmail" placeholder="可选" />
        </div>
        <p class="rounded-lg bg-slate-50 px-3 py-2 text-xs text-slate-400 dark:bg-[#171a20]">
          手动创建的成员可以先被指派任务；之后本人通过统一认证登录且名称相同时，账号自动关联，历史数据保留。新成员默认为「普通用户」角色。
        </p>
        <div class="flex justify-end gap-2">
          <n-button @click="showCreate = false">取消</n-button>
          <n-button type="primary" :loading="saving" :disabled="!createName.trim()" @click="create">创建</n-button>
        </div>
      </div>
    </n-modal>

    <n-modal :show="showEdit" preset="card" title="编辑成员" style="width: 420px" @update:show="showEdit = false">
      <div class="flex flex-col gap-4">
        <div>
          <label class="mb-1 block text-xs font-medium text-slate-500">名称 *</label>
          <n-input v-model:value="editName" />
        </div>
        <div>
          <label class="mb-1 block text-xs font-medium text-slate-500">邮箱</label>
          <n-input v-model:value="editEmail" placeholder="可选" />
        </div>
        <div>
          <label class="mb-1 block text-xs font-medium text-slate-500">角色</label>
          <n-select v-model:value="editRoleId" :options="roleOptions" placeholder="选择角色" />
          <p class="mt-1 text-xs text-slate-400">角色决定成员权限（普通用户 / 管理员）</p>
        </div>
        <div class="flex justify-end gap-2">
          <n-button @click="showEdit = false">取消</n-button>
          <n-button type="primary" :loading="saving" :disabled="!editName.trim()" @click="saveEdit">保存</n-button>
        </div>
      </div>
    </n-modal>

    <!-- 角色新建 / 编辑 -->
    <n-modal
      :show="showRoleEditor"
      preset="card"
      :title="roleEditTarget ? '编辑角色' : '新建角色'"
      style="width: 420px"
      @update:show="showRoleEditor = false"
    >
      <div class="flex flex-col gap-4">
        <div>
          <label class="mb-1 block text-xs font-medium text-slate-500">角色名称 *</label>
          <n-input v-model:value="roleName_" :disabled="!!roleEditTarget?.built_in" placeholder="如：组长、观察员" />
        </div>
        <div>
          <label class="mb-1 block text-xs font-medium text-slate-500">描述</label>
          <n-input v-model:value="roleDesc" placeholder="可选" />
        </div>
        <n-checkbox v-model:checked="roleIsAdmin" :disabled="!!roleEditTarget?.built_in">
          管理员权限（可访问系统设置、管理全部数据）
        </n-checkbox>
        <p v-if="roleEditTarget?.built_in" class="rounded-lg bg-slate-50 px-3 py-2 text-xs text-slate-400 dark:bg-[#171a20]">
          内置角色仅允许修改描述。
        </p>
        <div class="flex justify-end gap-2">
          <n-button @click="showRoleEditor = false">取消</n-button>
          <n-button type="primary" :loading="saving" :disabled="!roleName_.trim()" @click="saveRole">保存</n-button>
        </div>
      </div>
    </n-modal>

    <ConfirmDialog
      v-model:show="confirmDeleteRole.show"
      title="删除角色"
      :content="`确定删除角色「${confirmDeleteRole.target?.name}」吗？该操作不可恢复。`"
      positive-text="删除"
      :danger="true"
      :loading="confirmDeleteRole.loading"
      @confirm="doDeleteRole"
    />

    <ConfirmDialog
      v-model:show="confirmImpersonate.show"
      title="模拟身份"
      :content="`确定以「${confirmImpersonate.target?.name}」的身份访问平台吗？期间你的操作将以该成员身份记录。排查完成后可通过右上角头像菜单退出。`"
      positive-text="开始模拟"
      :danger="false"
      :loading="confirmImpersonate.loading"
      @confirm="doImpersonate"
    />

    <!-- AI 模型新建 / 编辑 -->
    <n-modal
      :show="showAIEditor"
      preset="card"
      :title="aiEditTarget ? '编辑 AI 模型' : '添加 AI 模型'"
      style="width: 460px"
      @update:show="showAIEditor = false"
    >
      <div class="flex flex-col gap-4">
        <div>
          <label class="mb-1 block text-xs font-medium text-slate-500">显示名称 *</label>
          <n-input v-model:value="aiName" placeholder="如：DeepSeek V4 Flash" />
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="mb-1 block text-xs font-medium text-slate-500">服务商 *</label>
            <n-input v-model:value="aiProvider" placeholder="deepseek" />
          </div>
          <div>
            <label class="mb-1 block text-xs font-medium text-slate-500">模型 ID *</label>
            <n-input v-model:value="aiModelId" placeholder="如：deepseek-v4-flash" />
          </div>
        </div>
        <div>
          <label class="mb-1 block text-xs font-medium text-slate-500">API Key</label>
          <n-input v-model:value="aiApiKey" type="password" show-password-on="click" placeholder="sk-..." />
        </div>
        <div>
          <label class="mb-1 block text-xs font-medium text-slate-500">Base URL</label>
          <n-input v-model:value="aiBaseURL" placeholder="https://api.deepseek.com/v1" />
        </div>
        <n-checkbox v-model:checked="aiEnabled">启用（启用后才能在 AI 分析页被选用）</n-checkbox>
        <div class="flex justify-end gap-2">
          <n-button @click="showAIEditor = false">取消</n-button>
          <n-button type="primary" :loading="saving" :disabled="!aiName.trim() || !aiModelId.trim()" @click="saveAIModel">保存</n-button>
        </div>
      </div>
    </n-modal>

    <ConfirmDialog
      v-model:show="confirmDeleteAI.show"
      title="删除 AI 模型"
      :content="`确定删除模型「${confirmDeleteAI.target?.name}」吗？已被报告引用的模型不可删除。`"
      positive-text="删除"
      :danger="true"
      :loading="confirmDeleteAI.loading"
      @confirm="doDeleteAI"
    />
  </div>
</template>
