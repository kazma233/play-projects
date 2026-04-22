<script setup>
import { computed, h, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { homeMeta, toolItems, toolMetaMap } from './constants/tool-meta'

const route = useRoute()
const router = useRouter()

const collapsed = ref(false)
const isCompact = ref(false)
const viewportWidth = ref(0)

const currentMeta = computed(() => route.meta?.path ? route.meta : toolMetaMap[route.path] || homeMeta)
const activeKey = computed(() => toolMetaMap[route.path] ? route.path : null)
const isHomeRoute = computed(() => route.path === '/')
const showPageHero = computed(() => !isHomeRoute.value)
const showHeader = computed(() => showPageHero.value || isCompact.value)
const siderWidth = computed(() => {
  if (!viewportWidth.value) {
    return 260
  }

  return Math.round(Math.min(280, Math.max(220, viewportWidth.value * 0.24)))
})

const menuOptions = computed(() =>
  toolItems.map((tool) => ({
    label: tool.label,
    key: tool.path,
    icon: () => h('span', { class: 'app-menu-icon' }, tool.icon)
  }))
)

const syncViewport = () => {
  viewportWidth.value = window.innerWidth

  const nextCompact = viewportWidth.value <= 980
  if (nextCompact !== isCompact.value) {
    isCompact.value = nextCompact
    collapsed.value = nextCompact
    return
  }

  if (!nextCompact) {
    collapsed.value = false
  }
}

const toggleSider = () => {
  if (!isCompact.value) {
    return
  }

  collapsed.value = !collapsed.value
}

const openPath = (path) => {
  router.push(path)
  if (isCompact.value) {
    collapsed.value = true
  }
}

onMounted(() => {
  syncViewport()
  window.addEventListener('resize', syncViewport)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', syncViewport)
})
</script>

<template>
  <n-message-provider>
    <n-dialog-provider>
      <div class="app-shell">
        <div v-if="isCompact && !collapsed" class="app-sider-backdrop" @click="collapsed = true"></div>

        <n-layout has-sider class="app-layout">
          <n-layout-sider
            v-model:collapsed="collapsed"
            :width="siderWidth"
            :collapsed-width="0"
            collapse-mode="transform"
            class="app-sider"
            content-class="app-sider-content"
            bordered
          >
            <div class="app-sider-inner">
              <button class="brand-panel" type="button" @click="openPath('/')">
                <span class="brand-panel__icon">{{ homeMeta.icon }}</span>
                <div class="brand-panel__copy">
                  <strong>Kt 工具箱</strong>
                  <p>常用开发工具集合</p>
                </div>
              </button>

              <div class="sider-section-title">工具列表</div>

              <n-scrollbar class="sider-scroll-area">
                <n-menu
                  class="app-menu"
                  :value="activeKey"
                  :options="menuOptions"
                  @update:value="openPath"
                />
              </n-scrollbar>
            </div>
          </n-layout-sider>

          <n-layout class="app-main-layout">
            <n-layout-header v-if="showHeader" class="app-header">
              <div class="page-header">
                <n-button v-if="isCompact" text class="menu-toggle" @click="toggleSider">
                  <span>{{ collapsed ? '☰' : '✕' }}</span>
                </n-button>

                <div v-if="showPageHero" class="page-copy-block">
                  <span class="page-kicker">{{ currentMeta.eyebrow }}</span>
                  <h1>{{ currentMeta.label }}</h1>
                  <p>{{ currentMeta.description }}</p>
                </div>
              </div>
            </n-layout-header>

            <n-layout-content class="app-content">
              <div class="page-frame">
                <RouterView />
              </div>
            </n-layout-content>
          </n-layout>
        </n-layout>
      </div>
    </n-dialog-provider>
  </n-message-provider>
</template>

<style>
.app-shell {
  position: relative;
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 100dvh;
  overflow: hidden;
  background: #f5f6f8;
}

.app-layout,
.app-main-layout,
.app-content {
  height: 100%;
  min-height: 0;
}

.app-layout {
  display: flex;
  flex: 1;
  min-height: 100%;
  position: relative;
}

.app-layout > .n-layout-scroll-container {
  position: absolute;
  inset: 0;
  display: flex;
  flex-flow: row;
}

.app-main-layout > .n-layout-scroll-container,
.app-content > .n-layout-scroll-container {
  height: 100%;
  min-height: 0;
}

.app-sider-backdrop {
  position: absolute;
  inset: 0;
  z-index: 15;
  background: rgba(15, 23, 42, 0.24);
}

.app-sider {
  min-height: 100%;
  z-index: 20;
  background: #fff;
}

.app-sider-content {
  height: 100%;
}

.app-sider-inner {
  display: flex;
  flex-direction: column;
  gap: 16px;
  height: 100%;
  padding: 16px;
}

.brand-panel {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  width: 100%;
  padding: 12px;
  border: 1px solid #e5e7eb;
  background: #fff;
  color: inherit;
  text-align: left;
  cursor: pointer;
}

.brand-panel__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  font-size: 20px;
  flex-shrink: 0;
}

.brand-panel__copy {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.brand-panel__copy strong {
  font-size: 1rem;
  line-height: 1.2;
}

.brand-panel__copy p,
.page-kicker,
.sider-section-title,
.page-copy-block p {
  margin: 0;
  color: #6b7280;
}

.sider-section-title,
.page-kicker {
  font-size: 0.75rem;
  font-weight: 600;
}

.sider-scroll-area {
  flex: 1;
  min-height: 0;
}

.app-menu-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
}

.app-main-layout {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-height: 0;
}

.app-main-layout > .n-layout-scroll-container {
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden !important;
}

.app-header {
  padding: 16px 16px 0;
  background: transparent;
  height: auto;
}

.page-header {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 12px 16px;
  border: 1px solid #e5e7eb;
  background: #fff;
}

.page-copy-block {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.page-copy-block h1 {
  margin: 0;
  font-size: 1.5rem;
  line-height: 1.2;
}

.page-copy-block p {
  line-height: 1.5;
}

.app-content {
  flex: 1;
  overflow: hidden;
}

.app-content > .n-layout-scroll-container {
  height: 100%;
  overflow: hidden !important;
}

.page-frame {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  padding: 16px;
  overflow: hidden;
}

@media (max-width: 980px) {
  .app-header,
  .page-frame {
    padding: 12px;
  }

  .page-header {
    padding: 12px;
  }

  .page-frame,
  .app-content > .n-layout-scroll-container,
  .app-main-layout > .n-layout-scroll-container {
    overflow: auto !important;
  }

  .app-sider {
    position: absolute;
    left: 0;
    top: 0;
    bottom: 0;
    max-width: calc(100vw - 16px);
    box-shadow: 0 8px 24px rgba(15, 23, 42, 0.12);
  }
}

@media (max-width: 640px) {
  .brand-panel__copy p {
    display: none;
  }
}
</style>
