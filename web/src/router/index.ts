import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: () => import('../views/LoginView.vue'), meta: { public: true } },
    { path: '/callback', component: () => import('../views/CallbackView.vue'), meta: { public: true } },
    // 首次登录强制修改初始密码（需登录，守卫中特殊放行）
    { path: '/change-password', component: () => import('../views/ChangePasswordView.vue') },
    { path: '/', redirect: '/board' },
    { path: '/board', component: () => import('../views/BoardView.vue') },
    { path: '/tasks', component: () => import('../views/TasksView.vue') },
    // 「今日工作」「计划」已合并为「任务」，保留旧链接重定向
    { path: '/today', redirect: '/tasks' },
    { path: '/plans', redirect: '/tasks' },
    { path: '/ai-analysis', component: () => import('../views/AIAnalysisView.vue') },
    // 原「报表」页已重写为「AI 分析」，保留旧链接重定向
    { path: '/reports', redirect: '/ai-analysis' },
    { path: '/stats', component: () => import('../views/StatsView.vue') },
    // 项目管理已并入系统设置，保留 /projects 深链接重定向
    { path: '/projects', redirect: '/system?tab=projects' },
    // 系统设置（成员/项目管理）仅管理员可见
    { path: '/system', component: () => import('../views/UsersView.vue'), meta: { admin: true } },
    { path: '/users', redirect: '/system' },
    { path: '/settings', component: () => import('../views/SettingsView.vue') },
  ],
})

router.beforeEach(async (to) => {
  if (to.meta.public) return true
  const auth = useAuthStore()
  if (!auth.loaded) await auth.fetchMe()
  if (!auth.isLoggedIn) return { path: '/login' }
  // 初始密码未修改：所有页面重定向到强制改密页
  if (auth.user?.must_change_password && to.path !== '/change-password') {
    return { path: '/change-password' }
  }
  if (to.meta.admin && !auth.user?.is_admin) return { path: '/board' }
  return true
})

export default router
