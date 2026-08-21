<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { NButton, NModal, NInput, NSelect, NEmpty, NTag, useMessage } from 'naive-ui'
import { api, type Project, type User } from '../api'
import { useAuthStore } from '../stores/auth'
import UserAvatar from '../components/UserAvatar.vue'
import ConfirmDialog from '../components/ConfirmDialog.vue'

const auth = useAuthStore()
const message = useMessage()

const projects = ref<Project[]>([])
const users = ref<User[]>([])
const showArchived = ref(false)

const showCreate = ref(false)
const name = ref('')
const description = ref('')
const ownerId = ref<number | null>(null)
const saving = ref(false)

// 编辑弹窗（仅管理员）
const showEdit = ref(false)
const editTarget = ref<Project | null>(null)
const editName = ref('')
const editDescription = ref('')
const editOwnerId = ref<number | null>(null)
const editSaving = ref(false)

const isAdmin = computed(() => !!auth.user?.is_admin)
const userOptions = computed(() => users.value.map((u) => ({ label: u.name, value: u.id })))

async function load() {
  const [p, u] = await Promise.all([api.projects(showArchived.value), api.users()])
  projects.value = p.data
  users.value = u.data
}
onMounted(load)

async function create() {
  if (!name.value.trim() || !ownerId.value) return
  saving.value = true
  try {
    await api.createProject({ name: name.value.trim(), description: description.value, owner_id: ownerId.value })
    message.success('项目已创建')
    showCreate.value = false
    name.value = ''
    description.value = ''
    ownerId.value = null
    load()
  } catch (e: any) {
    message.error(e.response?.data?.error || '创建失败')
  } finally {
    saving.value = false
  }
}

function openEdit(p: Project) {
  editTarget.value = p
  editName.value = p.name
  editDescription.value = p.description
  editOwnerId.value = p.owner_id
  showEdit.value = true
}

async function saveEdit() {
  if (!editTarget.value || !editName.value.trim() || !editOwnerId.value) return
  editSaving.value = true
  try {
    await api.updateProject(editTarget.value.id, {
      name: editName.value.trim(),
      description: editDescription.value,
      owner_id: editOwnerId.value,
    })
    message.success('已保存')
    showEdit.value = false
    load()
  } catch (e: any) {
    message.error(e.response?.data?.error || '保存失败')
  } finally {
    editSaving.value = false
  }
}

// 通用确认弹窗状态（归档/恢复/删除共用）
const confirm = ref<{
  show: boolean
  title: string
  content: string
  positiveText: string
  loading: boolean
  action: (() => Promise<void>) | null
}>({ show: false, title: '', content: '', positiveText: '确定', loading: false, action: null })

function openConfirm(title: string, content: string, positiveText: string, action: () => Promise<void>) {
  confirm.value = { show: true, title, content, positiveText, loading: false, action }
}

async function doConfirm() {
  if (!confirm.value.action) return
  confirm.value.loading = true
  try {
    await confirm.value.action()
    confirm.value.show = false
  } catch (e: any) {
    message.error(e.response?.data?.error || '操作失败')
  } finally {
    confirm.value.loading = false
  }
  load()
}

function archive(p: Project) {
  const isActive = p.status === 'active'
  openConfirm(
    isActive ? '归档项目' : '恢复项目',
    `确定${isActive ? '归档' : '恢复'}「${p.name}」吗？`,
    '确定',
    async () => {
      await api.updateProject(p.id, { status: isActive ? 'archived' : 'active' })
      message.success('操作成功')
    },
  )
}

function remove(p: Project) {
  openConfirm('删除项目', `确定删除「${p.name}」吗？（项目下有工作内容时无法删除）`, '删除', async () => {
    await api.deleteProject(p.id)
    message.success('已删除')
  })
}
</script>

<template>
  <div>
    <div class="mb-4 flex items-center justify-end gap-3">
      <label class="flex cursor-pointer items-center gap-1.5 text-sm text-slate-500">
        <input type="checkbox" v-model="showArchived" @change="load" class="accent-violet-500" />
        显示已归档
      </label>
      <n-button type="primary" size="small" @click="showCreate = true">+ 创建项目</n-button>
    </div>

    <div v-if="projects.length" class="grid grid-cols-1 gap-3 sm:grid-cols-2">
      <div
        v-for="p in projects"
        :key="p.id"
        class="rounded-xl border border-slate-200 bg-white p-4 dark:border-[#242730] dark:bg-[#12151b]"
      >
        <div class="flex items-start justify-between">
          <div>
            <div class="flex items-center gap-2">
              <h3 class="font-bold">{{ p.name }}</h3>
              <n-tag v-if="p.status === 'archived'" size="small" type="default" :bordered="false">已归档</n-tag>
            </div>
            <p v-if="p.description" class="mt-1 text-xs text-slate-500">{{ p.description }}</p>
          </div>
        </div>
        <div class="mt-3 flex items-center justify-between">
          <div class="flex items-center gap-1.5 text-xs text-slate-500">
            <UserAvatar :user="p.owner" :size="5" />
            <span>负责人：{{ p.owner?.name }}</span>
          </div>
          <div v-if="isAdmin" class="flex gap-1">
            <button class="rounded px-2 py-1 text-xs text-violet-500 hover:bg-violet-500/10" @click="openEdit(p)">编辑</button>
            <button class="rounded px-2 py-1 text-xs text-slate-400 hover:bg-slate-100 dark:hover:bg-[#1d212b]" @click="archive(p)">
              {{ p.status === 'active' ? '归档' : '恢复' }}
            </button>
            <button class="rounded px-2 py-1 text-xs text-red-400 hover:bg-red-50 dark:hover:bg-red-500/10" @click="remove(p)">删除</button>
          </div>
        </div>
      </div>
    </div>
    <n-empty v-else description="还没有项目，点击右上角创建" class="py-16" />

    <n-modal :show="showCreate" preset="card" title="创建项目" style="width: 440px" @update:show="showCreate = false">
      <div class="flex flex-col gap-4">
        <div>
          <label class="mb-1 block text-xs font-medium text-slate-500">项目名称 *</label>
          <n-input v-model:value="name" placeholder="如：Iris V0.9" />
        </div>
        <div>
          <label class="mb-1 block text-xs font-medium text-slate-500">负责人 *</label>
          <n-select v-model:value="ownerId" :options="userOptions" placeholder="选择负责人" />
        </div>
        <div>
          <label class="mb-1 block text-xs font-medium text-slate-500">项目描述</label>
          <n-input v-model:value="description" type="textarea" :rows="2" placeholder="可选" />
        </div>
        <div class="flex justify-end gap-2">
          <n-button @click="showCreate = false">取消</n-button>
          <n-button type="primary" :loading="saving" :disabled="!name.trim() || !ownerId" @click="create">创建</n-button>
        </div>
      </div>
    </n-modal>

    <n-modal :show="showEdit" preset="card" title="编辑项目" style="width: 440px" @update:show="showEdit = false">
      <div class="flex flex-col gap-4">
        <div>
          <label class="mb-1 block text-xs font-medium text-slate-500">项目名称 *</label>
          <n-input v-model:value="editName" />
        </div>
        <div>
          <label class="mb-1 block text-xs font-medium text-slate-500">负责人 *</label>
          <n-select v-model:value="editOwnerId" :options="userOptions" />
        </div>
        <div>
          <label class="mb-1 block text-xs font-medium text-slate-500">项目描述</label>
          <n-input v-model:value="editDescription" type="textarea" :rows="2" placeholder="可选" />
        </div>
        <div class="flex justify-end gap-2">
          <n-button @click="showEdit = false">取消</n-button>
          <n-button type="primary" :loading="editSaving" :disabled="!editName.trim() || !editOwnerId" @click="saveEdit">保存</n-button>
        </div>
      </div>
    </n-modal>

    <ConfirmDialog
      v-model:show="confirm.show"
      :title="confirm.title"
      :content="confirm.content"
      :positive-text="confirm.positiveText"
      :loading="confirm.loading"
      @confirm="doConfirm"
    />
  </div>
</template>
