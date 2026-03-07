import { createApp } from "vue";
import { createPinia } from "pinia";
import router from "./router";
import App from "./App.vue";
import { useAuthStore } from "./stores/auth";
import "./assets/styles/main.css";

async function bootstrap() {
  const app = createApp(App);

  const pinia = createPinia();
  app.use(pinia);
  app.use(router);

  const authStore = useAuthStore();
  authStore.initFromStorage();
  await authStore.loadConfig();

  app.mount("#app");
}

void bootstrap();
