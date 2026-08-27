<script setup lang="ts">
import { computed, ref } from 'vue'
import { NButton, NInput, useMessage } from 'naive-ui'
import { api, invalidateUsersCache } from '../api'
import { useAuthStore } from '../stores/auth'
import ApiKeyManager from '../components/ApiKeyManager.vue'
import UserAvatar from '../components/UserAvatar.vue'

const auth = useAuthStore()
const message = useMessage()

const name = ref(auth.user?.name || '')
const feishuOpenId = ref(auth.user?.feishu_open_id || '')
const saving = ref(false)

// 仅本地账号（账号密码登录）可修改密码；统一认证用户在 Casdoor 侧管理
const isLocalAccount = computed(() => auth.user?.casdoor_id.startsWith('local:'))
const oldPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const savingPwd = ref(false)

async function save() {
  saving.value = true
  try {
    if (name.value.trim() && name.value.trim() !== auth.user?.name) {
      const { data } = await api.updateUser(auth.user!.id, { name: name.value.trim() })
      invalidateUsersCache()
      auth.user = data
    }
    const { data } = await api.updateMe({ feishu_open_id: feishuOpenId.value })
    auth.user = data
    message.success('已保存')
  } catch (e: any) {
    message.error(e.response?.data?.error || '保存失败')
  } finally {
    saving.value = false
  }
}

async function savePassword() {
  if (newPassword.value.length < 6) {
    message.warning('新密码长度至少 6 位')
    return
  }
  if (newPassword.value !== confirmPassword.value) {
    message.warning('两次输入的新密码不一致')
    return
  }
  savingPwd.value = true
  try {
    await api.changePassword(oldPassword.value, newPassword.value)
    message.success('密码已修改')
    oldPassword.value = ''
    newPassword.value = ''
    confirmPassword.value = ''
  } catch (e: any) {
    message.error(e.response?.data?.error || '修改失败')
  } finally {
    savingPwd.value = false
  }
}
</script>

<template>
  <div class="mx-auto max-w-3xl">
    <h2 class="mb-5 text-lg font-bold">个人设置</h2>

    <div class="rounded-2xl border border-slate-200 bg-white p-5 dark:border-[#242730] dark:bg-[#12151b]">
      <div class="mb-5 flex items-center gap-3">
        <UserAvatar :user="auth.user" :size="12" />
        <div>
          <div class="font-bold">{{ auth.user?.name }}</div>
          <div class="text-xs text-slate-400">{{ auth.user?.email || auth.user?.casdoor_id }}</div>
        </div>
      </div>

      <label class="mb-1 block text-xs font-medium text-slate-500">我的名称</label>
      <n-input v-model:value="name" placeholder="显示名称" />
      <p class="mt-1.5 mb-4 text-xs text-slate-400">修改后全平台显示生效；管理员也可在「成员」页修改任何人的名称。</p>

      <label class="mb-1 block text-xs font-medium text-slate-500">飞书 Open ID（用于接收计划到期提醒与 @ 提及）</label>
      <n-input v-model:value="feishuOpenId" placeholder="ou_xxxxxxxxxxxxxxxx" />
      <p class="mt-1.5 text-xs text-slate-400">
        在飞书开放平台的管理后台或通过 API 查询自己的 open_id 后填入。留空则只接收平台内通知。
      </p>

      <div class="mt-4 flex justify-end">
        <n-button type="primary" :loading="saving" @click="save">保存</n-button>
      </div>
    </div>

    <!-- 修改密码：仅本地账号可见 -->
    <div v-if="isLocalAccount" class="mt-4 rounded-2xl border border-slate-200 bg-white p-5 dark:border-[#242730] dark:bg-[#12151b]">
      <h3 class="mb-4 text-sm font-bold">修改密码</h3>
      <div class="flex flex-col gap-3">
        <div>
          <label class="mb-1 block text-xs font-medium text-slate-500">原密码</label>
          <n-input v-model:value="oldPassword" type="password" show-password-on="click" placeholder="输入当前密码" />
        </div>
        <div>
          <label class="mb-1 block text-xs font-medium text-slate-500">新密码</label>
          <n-input v-model:value="newPassword" type="password" show-password-on="click" placeholder="至少 6 位" />
        </div>
        <div>
          <label class="mb-1 block text-xs font-medium text-slate-500">确认新密码</label>
          <n-input v-model:value="confirmPassword" type="password" show-password-on="click" placeholder="再次输入新密码" />
        </div>
        <div class="flex justify-end">
          <n-button
            type="primary"
            :loading="savingPwd"
            :disabled="!oldPassword || !newPassword || !confirmPassword"
            @click="savePassword"
          >
            修改密码
          </n-button>
        </div>
      </div>
    </div>

    <div class="mt-4 rounded-2xl border border-slate-200 bg-white p-5 dark:border-[#242730] dark:bg-[#12151b]">
      <h3 class="mb-1 text-sm font-bold">桌面客户端与 API Key</h3>
      <p class="mb-4 text-xs leading-5 text-slate-400">
        管理供桌面客户端和外部 API 集成使用的个人访问密钥。
      </p>
      <ApiKeyManager />
    </div>
  </div>
</template>
