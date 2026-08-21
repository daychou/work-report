<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { NButton, NInput, NCard } from 'naive-ui'
import { api } from '../api'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const router = useRouter()

const casdoorEnabled = ref(false)
const authorizeUrl = ref('')

// 账号密码登录
const username = ref('')
const password = ref('')
const loading = ref(false)
const error = ref('')

// 开发模式登录（casdoor 未启用时可用）
const devName = ref('')
const devLoading = ref(false)

onMounted(async () => {
  const { data } = await api.authConfig()
  casdoorEnabled.value = data.casdoor_enabled
  authorizeUrl.value = data.authorize_url
})

function loginWithCasdoor() {
  location.href = authorizeUrl.value
}

// 登录成功后：初始密码账号强制先去改密
function afterLogin(token: string, user: any) {
  auth.setSession(token, user)
  router.push(user.must_change_password ? '/change-password' : '/board')
}

async function login() {
  if (!username.value.trim() || !password.value) return
  loading.value = true
  error.value = ''
  try {
    const { data } = await api.login(username.value.trim(), password.value)
    afterLogin(data.token, data.user)
  } catch (e: any) {
    error.value = e.response?.data?.error || '登录失败'
  } finally {
    loading.value = false
  }
}

async function devLogin() {
  if (!devName.value.trim()) return
  devLoading.value = true
  error.value = ''
  try {
    const { data } = await api.devLogin(devName.value.trim())
    afterLogin(data.token, data.user)
  } catch (e: any) {
    error.value = e.response?.data?.error || '登录失败'
  } finally {
    devLoading.value = false
  }
}
</script>

<template>
  <div class="grid min-h-screen place-items-center bg-slate-50 dark:bg-[#0b0d11]">
    <!-- 宽度用内联样式：naive-ui 运行时注入的 .n-card{width:100%} 为 unlayered 样式，
         优先级高于 Tailwind v4 的 @layer utilities，class 方式会被覆盖 -->
    <n-card :bordered="false" style="width: 380px; max-width: calc(100vw - 32px); border-radius: 16px">
      <div class="mb-6 flex flex-col items-center">
        <span
          class="mb-3 grid size-12 place-items-center rounded-xl border border-violet-400/55 bg-gradient-to-br from-violet-500 to-violet-800 text-xl font-bold text-white italic shadow-[inset_0_1px_rgb(255_255_255/.25),0_0_22px_rgb(124_58_237/.28)]"
          >W</span
        >
        <h1 class="text-xl font-bold">工作日志</h1>
        <p class="mt-1 text-sm text-slate-500">日报 · 周报 · 计划 · 统计</p>
      </div>

      <!-- 账号密码登录 -->
      <div class="flex flex-col gap-3">
        <n-input
          v-model:value="username"
          placeholder="用户名"
          size="large"
          @keyup.enter="login"
        />
        <n-input
          v-model:value="password"
          type="password"
          show-password-on="click"
          placeholder="密码"
          size="large"
          @keyup.enter="login"
        />
        <n-button
          type="primary"
          block
          size="large"
          :loading="loading"
          :disabled="!username.trim() || !password"
          @click="login"
        >
          登录
        </n-button>
        <p v-if="error" class="text-sm text-red-500">{{ error }}</p>
      </div>

      <!-- 统一认证（Casdoor） -->
      <template v-if="casdoorEnabled">
        <div class="my-5 flex items-center gap-3 text-xs text-slate-400">
          <div class="h-px flex-1 bg-slate-200 dark:bg-[#242730]"></div>
          其他登录方式
          <div class="h-px flex-1 bg-slate-200 dark:bg-[#242730]"></div>
        </div>
        <div class="flex justify-center">
          <n-button size="large" class="px-8" block @click="loginWithCasdoor">
            使用统一认证
          </n-button>
        </div>
      </template>

      <!-- 开发模式：Casdoor 未启用时可用 -->
      <template v-else>
        <div class="my-5 flex items-center gap-3 text-xs text-slate-400">
          <div class="h-px flex-1 bg-slate-200 dark:bg-[#242730]"></div>
          开发模式
          <div class="h-px flex-1 bg-slate-200 dark:bg-[#242730]"></div>
        </div>
        <div class="rounded-lg bg-amber-50 px-3 py-2 text-xs text-amber-600 dark:bg-amber-500/10">
          Casdoor 未启用，输入名字即可登录
        </div>
        <n-input
          v-model:value="devName"
          placeholder="输入你的名字"
          size="large"
          class="mt-3"
          @keyup.enter="devLogin"
        />
        <n-button
          block
          size="large"
          class="mt-3"
          :loading="devLoading"
          :disabled="!devName.trim()"
          @click="devLogin"
        >
          进入平台
        </n-button>
      </template>
    </n-card>
  </div>
</template>
