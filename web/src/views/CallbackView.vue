<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NSpin } from 'naive-ui'
import { api } from '../api'
import { useAuthStore } from '../stores/auth'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const error = ref('')

onMounted(async () => {
  const code = route.query.code as string
  const state = (route.query.state as string) || ''
  if (!code) {
    error.value = '回调缺少 code 参数'
    return
  }
  try {
    const { data } = await api.callback(code, state)
    auth.setSession(data.token, data.user)
    router.replace('/board')
  } catch (e: any) {
    error.value = e.response?.data?.error || '登录失败，请重试'
  }
})
</script>

<template>
  <div class="grid min-h-screen place-items-center bg-slate-50 dark:bg-[#0b0d11]">
    <div class="text-center">
      <n-spin v-if="!error" size="large" />
      <p v-if="!error" class="mt-4 text-sm text-slate-500">正在完成登录…</p>
      <div v-else>
        <p class="text-red-500">{{ error }}</p>
        <router-link to="/login" class="mt-3 inline-block text-violet-500 underline">返回登录</router-link>
      </div>
    </div>
  </div>
</template>
