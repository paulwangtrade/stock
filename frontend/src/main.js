import {createApp} from 'vue'
import naive from 'naive-ui'
import App from './App.vue'
import router from './router/router'
import { markAppBootStart } from './services/performanceMetrics'
// Phase16.26-A design tokens (brand / market up-down / opportunity)
import './styles/designTokens.css'
// 引入组件库的少量全局样式变量
import 'tdesign-vue-next/es/style/index.css';

markAppBootStart()

const app = createApp(App)

app.config.errorHandler = (err) => {
  if (err.message && err.message.includes('ResizeObserver')) {
    return
  }
  console.error(err)
}

window.addEventListener('error', (event) => {
  if (event.message && event.message.includes('ResizeObserver')) {
    event.preventDefault()
    return true
  }
})

window.addEventListener('unhandledrejection', (event) => {
  if (event.reason && event.reason.message && event.reason.message.includes('ResizeObserver')) {
    event.preventDefault()
    return true
  }
})

app.use(router)
app.use(naive)
app.mount('#app')
