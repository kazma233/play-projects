<script setup>
import { ref, watch, h } from "vue";
import { useRouter, useRoute } from 'vue-router'

const router = useRouter()
const route = useRoute()

const collapsed = ref(false)
const activeKey = ref('')

const menuOptions = [
  { label: '时间转换', key: '/datetime', path: '/datetime', icon: '⏰' },
  { label: 'Base64', key: '/base64', path: '/base64', icon: '📝' },
  { label: 'URL 编码', key: '/url', path: '/url', icon: '🔗' },
  { label: 'JSON 格式化', key: '/json', path: '/json', icon: '📋' },
  { label: 'MD5 计算', key: '/md5', path: '/md5', icon: '🔐' },
  { label: 'SHA1 计算', key: '/sha1', path: '/sha1', icon: '🔒' },
  { label: '二维码', key: '/qrcode', path: '/qrcode', icon: '📱' },
  { label: '端口管理', key: '/ports', path: '/ports', icon: '🔌' },
  { label: '图片处理', key: '/image', path: '/image', icon: '🖼️' },
]

const handleMenuSelect = (key) => {
  const menu = menuOptions.find(m => m.key === key)
  if (menu) {
    activeKey.value = key
    router.push(menu.path)
  }
}

watch(() => route.path, (path) => {
  const menu = menuOptions.find(m => m.path === path)
  if (menu) {
    activeKey.value = menu.key
  } else if (path === '/') {
    activeKey.value = ''
  }
}, { immediate: true })

const getPageTitle = () => {
  return menuOptions.find(m => m.key === activeKey.value)?.label || '首页'
}

const getPageIcon = () => {
  return menuOptions.find(m => m.key === activeKey.value)?.icon || '🧰'
}
</script>

<template>
  <n-config-provider>
    <n-message-provider>
      <n-dialog-provider>
        <n-layout has-sider style="height: 100vh;">
          <n-layout-sider
            v-model:collapsed="collapsed"
            :width="200"
            :collapsed-width="0"
            show-trigger="bar"
            collapse-mode="transform"
            bordered
          >
            <n-flex align="center" justify="center" style="height: 64px; border-bottom: 1px solid #f0f0f0; cursor: pointer;" @click="$router.push('/')">
              <span style="font-size: 24px;">🧰</span>
              <span v-if="!collapsed" style="font-size: 18px; font-weight: 600; color: #333;">Kt工具箱</span>
            </n-flex>
            <n-menu
              :value="activeKey"
              :options="menuOptions.map(m => ({ 
                label: m.label, 
                key: m.key,
                icon: () => h('span', m.icon)
              }))"
              @update:value="handleMenuSelect"
            />
          </n-layout-sider>

          <n-layout>
            <n-layout-header bordered style="height: 64px; padding: 0 20px;">
              <n-flex align="center" style="height: 100%;">
                <n-button text @click="collapsed = !collapsed" style="font-size: 18px; margin-right: 16px;">
                  <template #icon>
                    <span>☰</span>
                  </template>
                </n-button>
                <n-flex align="center" :gap="8" style="font-size: 18px; font-weight: 500; color: #333;">
                  <span style="font-size: 20px;">{{ getPageIcon() }}</span>
                  <span>{{ getPageTitle() }}</span>
                </n-flex>
              </n-flex>
            </n-layout-header>

            <n-layout-content style="position: absolute; top: 64px; left: 0; right: 0; bottom: 0; background: #f5f7fa;">
              <div style="height: 100%; overflow: auto; padding: 20px;">
                <RouterView />
              </div>
            </n-layout-content>
          </n-layout>
        </n-layout>
      </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>

<style>
* {
  box-sizing: border-box;
}

html, body {
  margin: 0;
  padding: 0;
  height: 100%;
  overflow: hidden;
}

#app {
  height: 100vh;
  overflow: hidden;
}
</style>
