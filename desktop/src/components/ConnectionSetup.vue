<script setup lang="ts">
import { ref } from 'vue'
import { ArrowRight, CheckCircle2, KeyRound, LoaderCircle, Server, ShieldCheck } from '@lucide/vue'

defineProps<{ loading: boolean; error?: string }>()
const emit = defineEmits<{ connect: [serverURL: string, apiKey: string] }>()

const serverURL = ref('')
const apiKey = ref('')

function submit() {
  if (!serverURL.value.trim() || !apiKey.value.trim()) return
  emit('connect', serverURL.value, apiKey.value)
}
</script>

<template>
  <main class="onboarding-shell">
    <section class="onboarding-card">
      <div class="onboarding-mark">
        <span class="mark-petal mark-petal-a"></span>
        <span class="mark-petal mark-petal-b"></span>
        <span class="mark-core">W</span>
      </div>

      <div class="eyebrow"><ShieldCheck :size="14" /> 安全连接</div>
      <h1>把工作台，放到指尖。</h1>
      <p class="onboarding-lead">绑定你的 work-report 平台后，可从菜单栏随时查看和新建任务。</p>

      <form class="connection-form" @submit.prevent="submit">
        <label>
          <span>平台地址</span>
          <div class="field-shell">
            <Server :size="17" />
            <input v-model="serverURL" type="url" placeholder="https://work.example.com" autocomplete="url" />
          </div>
        </label>
        <label>
          <span>个人 API Key</span>
          <div class="field-shell">
            <KeyRound :size="17" />
            <input v-model="apiKey" type="password" placeholder="wrk_••••••••••••••••" autocomplete="off" />
          </div>
          <small>在网页版「个人设置 → 桌面客户端与 API Key」中创建。</small>
        </label>

        <div v-if="error" class="connection-error">{{ error }}</div>

        <button class="primary-button connect-button" :disabled="loading || !serverURL.trim() || !apiKey.trim()" type="submit">
          <LoaderCircle v-if="loading" class="spin" :size="17" />
          <CheckCircle2 v-else :size="17" />
          {{ loading ? '正在验证连接…' : '验证并绑定' }}
          <ArrowRight v-if="!loading" :size="16" />
        </button>
      </form>

      <p class="security-note">
        API Key 会加密后存在本机应用目录（仅当前用户可读），并与本机绑定，复制到其他电脑无法解密。
      </p>
    </section>

    <aside class="onboarding-preview" aria-hidden="true">
      <div class="preview-window">
        <div class="preview-sidebar">
          <span class="preview-dot"></span>
          <span></span><span></span><span></span>
        </div>
        <div class="preview-list">
          <b>今日聚焦</b>
          <div class="preview-task selected"><i></i><span></span><em></em></div>
          <div class="preview-task"><i></i><span></span><em></em></div>
          <div class="preview-task"><i></i><span></span><em></em></div>
        </div>
        <div class="preview-detail">
          <small>进行中</small>
          <strong></strong>
          <p></p><p></p>
          <button></button>
        </div>
      </div>
    </aside>
  </main>
</template>
