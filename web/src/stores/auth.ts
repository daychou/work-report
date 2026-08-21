import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { api, type User } from '../api'

const IMPERSONATOR_TOKEN_KEY = 'impersonator_token'

export const useAuthStore = defineStore('auth', () => {
  // token 必须是响应式 ref：若 computed 直接读 localStorage（非响应式），
  // 未登录时首次求值会短路（false && ... 不读 user），导致 user 依赖未被订阅，
  // 登录写入后 isLoggedIn 永远返回缓存的 false（登录被弹回登录页的根因）
  const token = ref<string | null>(localStorage.getItem('token'))
  const user = ref<User | null>(null)
  const loaded = ref(false)

  const isLoggedIn = computed(() => !!token.value && !!user.value)
  // 当前是否处于管理员模拟身份状态
  const impersonating = computed(() => !!user.value?.impersonated_by)

  async function fetchMe() {
    if (!token.value) {
      loaded.value = true
      return
    }
    try {
      const { data } = await api.me()
      user.value = data
    } catch {
      // 401 由 axios 拦截器处理
    } finally {
      loaded.value = true
    }
  }

  function setSession(t: string, u: User) {
    localStorage.setItem('token', t)
    token.value = t
    user.value = u
    loaded.value = true
  }

  // 管理员模拟指定成员身份：保存当前（管理员）token，切换为模拟 token。
  // 身份切换后整页跳转，确保所有视图以新身份重新加载（同路由 push 不会触发组件重新挂载）
  async function impersonate(userId: number) {
    const current = token.value
    if (!current) throw new Error('未登录')
    const { data } = await api.impersonate(userId)
    localStorage.setItem(IMPERSONATOR_TOKEN_KEY, current)
    localStorage.setItem('token', data.token)
    token.value = data.token
    location.href = '/board'
  }

  // 退出模拟身份，恢复管理员会话（整页刷新，各视图数据随之重新加载）
  async function exitImpersonation() {
    const t = localStorage.getItem(IMPERSONATOR_TOKEN_KEY)
    if (!t) return
    localStorage.removeItem(IMPERSONATOR_TOKEN_KEY)
    localStorage.setItem('token', t)
    token.value = t
    location.href = '/board'
  }

  function logout() {
    localStorage.removeItem('token')
    localStorage.removeItem(IMPERSONATOR_TOKEN_KEY)
    token.value = null
    user.value = null
    location.href = '/login'
  }

  return { user, loaded, isLoggedIn, impersonating, fetchMe, setSession, impersonate, exitImpersonation, logout }
})
