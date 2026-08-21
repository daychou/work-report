<script setup lang="ts">
import { NModal, NButton } from 'naive-ui'

defineProps<{
  show: boolean
  title: string
  content: string
  loading?: boolean
  positiveText?: string
  // 危险操作（删除等）确认按钮为红色，否则为主色
  danger?: boolean
}>()

const emit = defineEmits<{
  'update:show': [boolean]
  confirm: []
}>()
</script>

<template>
  <n-modal
    :show="show"
    preset="card"
    :title="title"
    style="width: 380px"
    :mask-closable="!loading"
    @update:show="emit('update:show', $event)"
  >
    <p class="text-sm text-slate-600 dark:text-[#c7cad1]">{{ content }}</p>
    <div class="mt-6 flex justify-end gap-2">
      <n-button :disabled="loading" @click="emit('update:show', false)">取消</n-button>
      <n-button :type="danger === false ? 'primary' : 'error'" :loading="loading" @click="emit('confirm')">{{ positiveText || '删除' }}</n-button>
    </div>
  </n-modal>
</template>
