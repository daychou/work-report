<script setup lang="ts">
import { computed } from 'vue'
import type { User } from '../api'

const props = withDefaults(
  defineProps<{
    user: User | null | undefined
    size?: number
  }>(),
  { size: 8 },
)

const palette = ['#7c3aed', '#0ea5e9', '#10b981', '#f59e0b', '#ef4444', '#ec4899', '#14b8a6', '#8b5cf6']

const initial = computed(() => props.user?.name?.slice(0, 1)?.toUpperCase() || '?')
const color = computed(() => {
  const name = props.user?.name || ''
  let h = 0
  for (const ch of name) h = (h * 31 + ch.charCodeAt(0)) % 997
  return palette[h % palette.length]
})
const sizeClass = computed(() => `size-${props.size}`)
</script>

<template>
  <img
    v-if="user?.avatar"
    :src="user.avatar"
    :alt="user.name"
    class="rounded-full border-2 border-white object-cover dark:border-[#0d0f13]"
    :class="sizeClass"
    :title="user.name"
  />
  <span
    v-else
    class="grid place-items-center rounded-full border-2 border-white text-[.7rem] font-extrabold text-white dark:border-[#0d0f13]"
    :class="sizeClass"
    :style="{ backgroundColor: color }"
    :title="user?.name"
    >{{ initial }}</span
  >
</template>
