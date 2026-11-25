import { createApp } from 'vue'
import ElementPlus from 'element-plus'
import ru from 'element-plus/es/locale/lang/ru'
import * as Icons from '@element-plus/icons-vue'
import 'element-plus/dist/index.css'
import dayjs from 'dayjs'
import 'dayjs/locale/ru'
import App from './App.vue'
import router from './router'
import './style.css'

dayjs.locale('ru')

const app = createApp(App)
for (const [name, comp] of Object.entries(Icons)) app.component(name, comp)
app.use(ElementPlus, { locale: ru })
app.use(router)
app.mount('#app')
