import { createApp } from "vue";
import App from "./App.vue";
import router from './router'

// Naive UI - 全局导入（确保样式正确加载）
import naive from 'naive-ui'
import 'vfonts/Lato.css'

import './assets/main.css'

const app = createApp(App);
app.use(router)
app.use(naive)

app.mount("#app");
