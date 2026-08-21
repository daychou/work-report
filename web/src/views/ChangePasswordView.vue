<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { NButton, NInput, NCard } from 'naive-ui'
import { api } from '../api'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const router = useRouter()

const oldPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const loading = ref(false)
const error = ref('')

// 首次登录（初始密码）场景下展示提示文案
const isFirstLogin = computed(() => !!auth.user?.must_change_password)

async function submit() {
  error.value = ''
  if (newPassword.value.length < 6) {
    error.value = '新密码长度至少 6 位'
    return
  }
  if (newPassword.value !== confirmPassword.value) {
    error.value = '两次输入的新密码不一致'
    return
  }
  loading.value = true
  try {
    const { data } = await api.changePassword(oldPassword.value, newPassword.value)
    auth.user = data
    router.push('/board')
  } catch (e: any) {
    error.value = e.response?.data?.error || '修改失败'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="grid min-h-screen place-items-center bg-slate-50 dark:bg-[#0b0d11]">
    <n-card :bordered="false" style="width: 380px; max-width: calc(100vw - 32px); border-radius: 16px">
      <div class="mb-6 flex flex-col items-center">
        <span
          class="mb-3 grid size-12 place-items-center rounded-xl border border-violet-400/55 bg-gradient-to-br from-violet-500 to-violet-800 text-xl font-bold text-white italic shadow-[inset_0_1px_rgb(255_255_255/.25),0_0_22px_rgb(124_58_237/.28)]"
          >W</span
        >
        <h1 class="text-xl font-bold">修改密码</h1>
        <p v-if="isFirstLogin" class="mt-1 text-center text-sm text-amber-500">
          首次登录请先修改初始密码，修改后才能继续使用
        </p>
      </div>

      <div class="flex flex-col gap-3">
        <n-input
          v-model:value="oldPassword"
          type="password"
          show-password-on="click"
          placeholder="原密码（初始密码为 123456）"
          size="large"
          @keyup.enter="submit"
        />
        <n-input
          v-model:value="newPassword"
          type="password"
          show-password-on="click"
          placeholder="新密码（至少 6 位）"
          size="large"
          @keyup.enter="submit"
        />
        <n-input
          v-model:value="confirmPassword"
          type="password"
          show-password-on="click"
          placeholder="确认新密码"
          size="large"
          @keyup.enter="submit"
        />
        <n-button
          type="primary"
          block
          size="large"
          :loading="loading"
          :disabled="!oldPassword || !newPassword || !confirmPassword"
          @click="submit"
        >
          确认修改
        </n-button>
        <p v-if="error" class="text-sm text-red-500">{{ error }}</p>
        <button class="mt-1 text-center text-xs text-slate-400 hover:text-slate-600 dark:hover:text-slate-300" @click="auth.logout()">
          退出登录
        </button>
      </div>
    </n-card>
  </div>
</template>
