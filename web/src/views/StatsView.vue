<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from 'vue'
import { NRadioGroup, NRadioButton } from 'naive-ui'
import * as echarts from 'echarts'
import { api, type StatsOverview } from '../api'

const days = ref(30)
const stats = ref<StatsOverview | null>(null)

const trendRef = ref<HTMLElement>()
const userRef = ref<HTMLElement>()
const projRef = ref<HTMLElement>()
let charts: echarts.ECharts[] = []

const isDark = () => document.documentElement.classList.contains('dark')

function renderCharts() {
  if (!stats.value) return
  charts.forEach((c) => c.dispose())
  charts = []

  const textColor = isDark() ? '#c7cad1' : '#475569'
  const axisLine = { lineStyle: { color: isDark() ? '#2b2e37' : '#e2e8f0' } }

  // 每日提交趋势
  if (trendRef.value) {
    const c = echarts.init(trendRef.value)
    c.setOption({
      grid: { left: 40, right: 16, top: 24, bottom: 28 },
      tooltip: { trigger: 'axis' },
      xAxis: {
        type: 'category',
        data: stats.value.daily_trend.map((d) => d.day.slice(5)),
        axisLabel: { color: textColor },
        axisLine,
      },
      yAxis: { type: 'value', minInterval: 1, axisLabel: { color: textColor }, splitLine: axisLine },
      series: [
        {
          type: 'line',
          smooth: true,
          data: stats.value.daily_trend.map((d) => d.cnt),
          areaStyle: { opacity: 0.15 },
          itemStyle: { color: '#7c3aed' },
        },
      ],
    })
    charts.push(c)
  }

  // 按人工作量（任务数 + 其中已完成数）
  if (userRef.value) {
    const c = echarts.init(userRef.value)
    const top = stats.value.by_user.slice(0, 10)
    c.setOption({
      grid: { left: 70, right: 24, top: 16, bottom: 28 },
      tooltip: {},
      xAxis: { type: 'value', minInterval: 1, axisLabel: { color: textColor }, splitLine: axisLine },
      yAxis: { type: 'category', data: top.map((u) => u.user_name).reverse(), axisLabel: { color: textColor }, axisLine },
      series: [
        { name: '任务', type: 'bar', data: top.map((u) => u.work_cnt).reverse(), itemStyle: { color: '#0ea5e9' } },
        { name: '已完成', type: 'bar', data: top.map((u) => u.done_cnt).reverse(), itemStyle: { color: '#10b981' } },
      ],
      legend: { textStyle: { color: textColor }, top: 0 },
    })
    charts.push(c)
  }

  // 按项目分布
  if (projRef.value) {
    const c = echarts.init(projRef.value)
    c.setOption({
      tooltip: { trigger: 'item' },
      legend: { textStyle: { color: textColor }, bottom: 0, type: 'scroll' },
      series: [
        {
          type: 'pie',
          radius: ['42%', '68%'],
          center: ['50%', '44%'],
          data: stats.value.by_project.map((p) => ({ name: p.project_name, value: p.cnt })),
          label: { color: textColor },
        },
      ],
    })
    charts.push(c)
  }

}

async function load() {
  const { data } = await api.stats(days.value)
  stats.value = data
  setTimeout(renderCharts)
}
watch(days, load)
onMounted(() => {
  load()
  window.addEventListener('resize', renderCharts)
})
onUnmounted(() => {
  window.removeEventListener('resize', renderCharts)
  charts.forEach((c) => c.dispose())
})
</script>

<template>
  <div>
    <div class="mb-4 flex items-center justify-between">
      <h2 class="text-lg font-bold">统计分析</h2>
      <n-radio-group v-model:value="days" size="small">
        <n-radio-button :value="7">近 7 天</n-radio-button>
        <n-radio-button :value="30">近 30 天</n-radio-button>
        <n-radio-button :value="90">近 90 天</n-radio-button>
      </n-radio-group>
    </div>

    <div v-if="stats" class="grid grid-cols-3 gap-3">
      <div class="rounded-xl border border-slate-200 bg-white p-3 text-center dark:border-[#242730] dark:bg-[#12151b]">
        <div class="text-2xl font-bold text-sky-500">{{ stats.total_work }}</div>
        <div class="text-xs text-slate-400">任务</div>
      </div>
      <div class="rounded-xl border border-slate-200 bg-white p-3 text-center dark:border-[#242730] dark:bg-[#12151b]">
        <div class="text-2xl font-bold text-emerald-500">{{ stats.total_projects }}</div>
        <div class="text-xs text-slate-400">进行中项目</div>
      </div>
      <div class="rounded-xl border border-slate-200 bg-white p-3 text-center dark:border-[#242730] dark:bg-[#12151b]">
        <div class="text-2xl font-bold text-amber-500">{{ stats.total_users }}</div>
        <div class="text-xs text-slate-400">团队成员</div>
      </div>
    </div>

    <div class="mt-4 grid grid-cols-1 gap-4 lg:grid-cols-2">
      <div class="rounded-xl border border-slate-200 bg-white p-4 dark:border-[#242730] dark:bg-[#12151b]">
        <h3 class="mb-2 text-sm font-bold">每日提交趋势</h3>
        <div ref="trendRef" class="h-64"></div>
      </div>
      <div class="rounded-xl border border-slate-200 bg-white p-4 dark:border-[#242730] dark:bg-[#12151b]">
        <h3 class="mb-2 text-sm font-bold">成员工作量</h3>
        <div ref="userRef" class="h-64"></div>
      </div>
      <div class="rounded-xl border border-slate-200 bg-white p-4 dark:border-[#242730] dark:bg-[#12151b]">
        <h3 class="mb-2 text-sm font-bold">项目工作量分布</h3>
        <div ref="projRef" class="h-64"></div>
      </div>
    </div>
  </div>
</template>
