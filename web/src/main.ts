import { createApp } from 'vue'
import { createPinia } from 'pinia'
import dayjs from 'dayjs'
import 'dayjs/locale/zh-cn'
import App from './App.vue'
import router from './router'
import './style.css'

// 全局中文 locale：startOf('week') 等按周一为一周开始
dayjs.locale('zh-cn')

const app = createApp(App)
app.use(createPinia())
app.use(router)
app.mount('#app')
