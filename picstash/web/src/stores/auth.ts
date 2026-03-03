import { defineStore } from "pinia";
import { ref, computed } from "vue";
import type { User } from "@/types";
import { authApi, configApi } from "@/api";

interface VerifyResponse {
  token: string;
  expires_at: string;
}

export const useAuthStore = defineStore("auth", () => {
  const user = ref<User | null>(null);
  const isAuthenticated = computed(() => !!user.value);
  const homeAuth = ref(false);

  const sendCode = async (email: string) => {
    const res = await authApi.sendCode(email);
    return res.data;
  };

  const verifyCode = async (email: string, code: string) => {
    const res = await authApi.verifyCode(email, code);
    const data = res.data as VerifyResponse | undefined;
    if (data?.token) {
      localStorage.setItem("token", data.token);
      localStorage.setItem("email", email);
      user.value = { email, token: data.token };
    }
    return res.data;
  };

  const logout = () => {
    localStorage.removeItem("token");
    localStorage.removeItem("email");
    user.value = null;
  };

  const initFromStorage = () => {
    const token = localStorage.getItem("token");
    const email = localStorage.getItem("email");
    if (token && email) {
      user.value = { email, token };
    }
  };

  const loadConfig = async () => {
    try {
      const res = await configApi.getConfig();
      homeAuth.value = res.data.home_auth;
    } catch (error) {
      console.error("Failed to load config:", error);
      homeAuth.value = false;
    }
  };

  return {
    user,
    isAuthenticated,
    homeAuth,
    sendCode,
    verifyCode,
    logout,
    initFromStorage,
    loadConfig,
  };
});
