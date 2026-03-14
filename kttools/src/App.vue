<script setup>
import { computed, h, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { homeMeta, toolItems, toolMetaMap } from './constants/tool-meta'

const route = useRoute()
const router = useRouter()

const collapsed = ref(false)
const isCompact = ref(false)
const viewportWidth = ref(0)
const viewportHeight = ref(0)

const themeOverrides = {
  common: {
    fontFamily: 'Lato, Avenir Next, Segoe UI, sans-serif',
    fontFamilyMono: 'Fira Code, SFMono-Regular, Consolas, monospace',
    primaryColor: '#f582ae',
    primaryColorHover: '#f79abb',
    primaryColorPressed: '#ea6e9f',
    primaryColorSuppl: '#f6a8c2',
    infoColor: '#8bd3dd',
    infoColorHover: '#a0dde5',
    infoColorPressed: '#73c3ce',
    successColor: '#58b7a7',
    successColorHover: '#6cc4b6',
    successColorPressed: '#469f92',
    warningColor: '#f0b45a',
    warningColorHover: '#f4c06f',
    warningColorPressed: '#dc9f47',
    errorColor: '#df6f93',
    errorColorHover: '#e684a3',
    errorColorPressed: '#cb5b82',
    textColorBase: '#172c66',
    textColor1: '#001858',
    textColor2: '#172c66',
    textColor3: '#5b6b96',
    borderColor: 'rgba(0, 24, 88, 0.12)',
    dividerColor: 'rgba(0, 24, 88, 0.1)',
    placeholderColor: 'rgba(23, 44, 102, 0.48)',
    bodyColor: '#fffdf8',
    cardColor: '#fffffe',
    modalColor: '#fffffe',
    popoverColor: '#fffffe',
    tableColor: '#fffffe',
    tableHeaderColor: 'rgba(243, 210, 193, 0.42)',
    codeColor: 'rgba(139, 211, 221, 0.16)'
  },
  Layout: {
    color: 'transparent',
    siderColor: 'rgba(255, 255, 255, 0.56)',
    headerColor: 'transparent'
  },
  Card: {
    color: 'rgba(255, 255, 255, 0.88)',
    borderColor: 'rgba(0, 24, 88, 0.08)',
    borderRadius: '26px'
  },
  Button: {
    borderRadiusMedium: '14px',
    borderRadiusSmall: '12px',
    textColorPrimary: '#001858',
    textColorInfo: '#001858',
    colorInfo: '#8bd3dd',
    colorHoverInfo: '#9fdce5',
    colorPressedInfo: '#74c4cf'
  },
  Input: {
    borderRadius: '18px',
    color: 'rgba(255, 255, 255, 0.76)',
    colorFocus: '#fffffe'
  },
  Select: {
    peers: {
      InternalSelection: {
        borderRadius: '18px',
        color: 'rgba(255, 255, 255, 0.76)',
        colorActive: '#fffffe'
      }
    }
  },
  DataTable: {
    thColor: 'rgba(243, 210, 193, 0.42)',
    tdColor: 'rgba(255, 255, 255, 0.72)',
    borderColor: 'rgba(0, 24, 88, 0.08)',
    tdColorHover: 'rgba(139, 211, 221, 0.12)'
  },
  Tabs: {
    tabBorderRadius: '14px'
  },
  Tag: {
    borderRadius: '999px'
  }
}

const currentMeta = computed(() => route.meta?.path ? route.meta : toolMetaMap[route.path] || homeMeta)
const activeKey = computed(() => toolMetaMap[route.path] ? route.path : null)
const isHomeRoute = computed(() => route.path === '/')
const showPageHero = computed(() => !isHomeRoute.value)
const showHeader = computed(() => showPageHero.value || isCompact.value)
const siderWidth = computed(() => {
  if (!viewportWidth.value) {
    return 286
  }

  return Math.round(Math.min(286, Math.max(236, viewportWidth.value * 0.82)))
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
  viewportHeight.value = window.innerHeight

  document.documentElement.style.setProperty('--app-vw', `${viewportWidth.value}px`)
  document.documentElement.style.setProperty('--app-vh', `${viewportHeight.value}px`)

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
  document.documentElement.style.removeProperty('--app-vw')
  document.documentElement.style.removeProperty('--app-vh')
})
</script>

<template>
  <n-config-provider :theme-overrides="themeOverrides">
    <n-message-provider>
      <n-dialog-provider>
        <div class="app-shell">
          <div class="app-orb app-orb--rose"></div>
          <div class="app-orb app-orb--aqua"></div>
          <div class="app-orb app-orb--sand"></div>
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
                  <span class="brand-panel__icon">🧰</span>
                  <div class="brand-panel__copy">
                    <strong>Kt 工具箱</strong>
                    <p>提供编码转换、哈希、二维码、图片处理与端口管理等常用工具。</p>
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
              <n-layout-header
                v-if="showHeader"
                class="app-header"
                :class="{ 'app-header--compact-only': !showPageHero }"
              >
                <div v-if="showPageHero" class="page-hero-card">
                  <div class="page-hero-card__top">
                    <n-button v-if="isCompact" text class="menu-toggle" @click="toggleSider">
                      <span>{{ collapsed ? '☰' : '✕' }}</span>
                    </n-button>
                    <span class="page-kicker">{{ currentMeta.eyebrow }}</span>
                  </div>

                  <div class="page-hero-card__body">
                    <div class="page-icon-badge">{{ currentMeta.icon }}</div>

                    <div class="page-copy-block">
                      <h1>{{ currentMeta.label }}</h1>
                      <p>{{ currentMeta.description }}</p>

                      <div class="page-highlight-list">
                        <span v-for="highlight in currentMeta.highlights" :key="highlight">
                          {{ highlight }}
                        </span>
                      </div>
                    </div>
                  </div>
                </div>

                <div v-else class="page-header-compact-actions">
                  <n-button text class="menu-toggle" @click="toggleSider">
                    <span>{{ collapsed ? '☰' : '✕' }}</span>
                  </n-button>
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
  </n-config-provider>
</template>

<style>
.app-shell {
  position: relative;
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: max(100dvh, var(--app-vh, 0px));
  overflow: hidden;
}

.app-orb {
  position: absolute;
  border-radius: 999px;
  filter: blur(18px);
  opacity: 0.7;
  pointer-events: none;
}

.app-orb--rose {
  width: clamp(14rem, 34vw, 23.75rem);
  aspect-ratio: 1;
  top: clamp(-7.5rem, -10vw, -3rem);
  right: clamp(-5rem, -6vw, -1.5rem);
  background: rgba(245, 130, 174, 0.24);
}

.app-orb--aqua {
  width: clamp(12rem, 30vw, 21.25rem);
  aspect-ratio: 1;
  left: 18%;
  bottom: clamp(-8rem, -11vw, -3rem);
  background: rgba(139, 211, 221, 0.26);
}

.app-orb--sand {
  width: clamp(15rem, 36vw, 26.25rem);
  aspect-ratio: 1;
  top: 12%;
  left: clamp(-12rem, -15vw, -4rem);
  background: rgba(243, 210, 193, 0.44);
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
  z-index: 1;
  background: transparent;
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
}

.app-sider-backdrop {
  position: absolute;
  inset: 0;
  z-index: 15;
  background: rgba(0, 24, 88, 0.22);
  backdrop-filter: blur(4px);
}

.app-sider {
  min-height: 100%;
  backdrop-filter: blur(22px);
  background: linear-gradient(180deg, rgba(255, 253, 248, 0.94), rgba(255, 250, 245, 0.9));
  z-index: 20;
}

.app-sider-content {
  height: 100%;
}

.app-sider-inner {
  display: flex;
  flex-direction: column;
  gap: clamp(0.875rem, 1.8vw, 1.125rem);
  height: 100%;
  padding: clamp(1rem, 1.8vw, 1.375rem) clamp(0.875rem, 1.6vw, 1.125rem) clamp(1rem, 1.8vw, 1.25rem);
}

.brand-panel {
  display: flex;
  align-items: flex-start;
  gap: clamp(0.75rem, 1.4vw, 0.875rem);
  width: 100%;
  padding: clamp(0.875rem, 1.8vw, 1rem) clamp(1rem, 2vw, 1.125rem);
  border: 0;
  border-radius: clamp(1.125rem, 2.8vw, 1.5rem);
  background: linear-gradient(145deg, rgba(243, 210, 193, 0.95), rgba(255, 255, 255, 0.78));
  color: #001858;
  box-shadow: 0 16px 32px rgba(0, 24, 88, 0.08);
  text-align: left;
  cursor: pointer;
}

.brand-panel__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: clamp(2.75rem, 6vw, 2.875rem);
  aspect-ratio: 1;
  border-radius: clamp(0.875rem, 2vw, 1rem);
  background: rgba(255, 255, 255, 0.68);
  font-size: clamp(1.125rem, 2.8vw, 1.5rem);
  flex-shrink: 0;
}

.brand-panel__copy {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.brand-panel__copy strong {
  font-family: 'Palatino Linotype', 'Book Antiqua', Georgia, serif;
  font-size: clamp(1.125rem, 2.8vw, 1.28rem);
  line-height: 1.1;
}

.brand-panel__copy p {
  margin: 0;
  color: rgba(23, 44, 102, 0.74);
  font-size: clamp(0.84rem, 1.8vw, 0.92rem);
  line-height: 1.6;
}

.page-kicker,
.sider-section-title {
  letter-spacing: 0.16em;
  text-transform: uppercase;
  font-size: 0.72rem;
  font-weight: 700;
  color: rgba(0, 24, 88, 0.56);
}

.sider-section-title {
  padding: 0 8px;
}

.sider-scroll-area {
  flex: 1;
  min-height: 0;
  width: 100%;
}

.app-menu .n-menu-item-content {
  margin: 4px 0;
  border-radius: 18px;
  font-weight: 600;
}

.app-menu .n-menu-item-content::before {
  border-radius: 18px;
}

.app-menu .n-menu-item-content--selected {
  background: rgba(245, 130, 174, 0.14);
}

.app-menu-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: clamp(1.625rem, 4vw, 1.75rem);
  aspect-ratio: 1;
  border-radius: clamp(0.625rem, 1.8vw, 0.75rem);
  background: rgba(255, 255, 255, 0.68);
}

.app-main-layout {
  background: transparent;
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
  padding: clamp(0.875rem, 1.8vw, 1rem) clamp(1rem, 2vw, 1.25rem) 0;
  background: transparent;
  height: auto;
}

.app-header--compact-only {
  padding-bottom: 0;
}

.page-header-compact-actions {
  display: flex;
  align-items: center;
}

.page-hero-card {
  padding: clamp(0.875rem, 1.8vw, 1rem) clamp(1rem, 2vw, 1.125rem);
  border-radius: clamp(1.125rem, 2.6vw, 1.5rem);
  background: linear-gradient(135deg, rgba(255, 255, 255, 0.78), rgba(243, 210, 193, 0.52));
  box-shadow: 0 14px 34px rgba(0, 24, 88, 0.08);
  backdrop-filter: blur(22px);
}

.page-hero-card__top,
.page-hero-card__body,
.page-highlight-list {
  display: flex;
}

.page-hero-card__top {
  align-items: center;
  gap: clamp(0.5rem, 1.4vw, 0.625rem);
  margin-bottom: clamp(0.5rem, 1.4vw, 0.625rem);
}

.menu-toggle {
  width: clamp(2rem, 5vw, 2.125rem);
  aspect-ratio: 1;
  border-radius: clamp(0.75rem, 1.8vw, 0.875rem);
  background: rgba(255, 255, 255, 0.7);
  color: #001858;
}

.page-hero-card__body {
  align-items: center;
  gap: clamp(0.875rem, 1.8vw, 1rem);
}

.page-icon-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: clamp(3.25rem, 9vw, 4rem);
  aspect-ratio: 1;
  border-radius: clamp(1rem, 2.5vw, 1.25rem);
  background: rgba(255, 255, 255, 0.78);
  font-size: clamp(1.25rem, 3.5vw, 1.6rem);
  flex-shrink: 0;
  box-shadow: 0 10px 20px rgba(0, 24, 88, 0.08);
}

.page-copy-block {
  flex: 1;
}

.page-copy-block h1 {
  margin: 0;
  color: #001858;
  font-family: 'Palatino Linotype', 'Book Antiqua', Georgia, serif;
  font-size: clamp(1.5rem, 2.3vw, 2.1rem);
  line-height: 1.08;
}

.page-copy-block p {
  margin: 6px 0 0;
  color: rgba(23, 44, 102, 0.76);
  line-height: 1.55;
  font-size: 0.94rem;
}

.page-highlight-list {
  flex-wrap: wrap;
  gap: clamp(0.375rem, 1vw, 0.5rem);
  margin-top: clamp(0.5rem, 1.4vw, 0.625rem);
}

.page-highlight-list span {
  padding: 0.375rem clamp(0.625rem, 1.8vw, 0.75rem);
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.66);
  color: #001858;
  font-size: clamp(0.75rem, 1.6vw, 0.8rem);
  font-weight: 600;
}

.app-content {
  background: transparent;
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
  padding: clamp(0.875rem, 2vw, 1.25rem) clamp(1rem, 2.4vw, 1.5rem) clamp(1rem, 2.4vw, 1.5rem);
  overflow: hidden;
}

@media (max-width: 980px) {
  .app-header {
    padding: var(--page-gutter) var(--page-gutter) 0;
  }

  .page-hero-card__body {
    flex-wrap: wrap;
    align-items: flex-start;
  }

  .page-frame {
    padding: var(--page-gutter);
    overflow: auto;
  }

  .app-content > .n-layout-scroll-container {
    overflow: auto !important;
  }

  .app-main-layout > .n-layout-scroll-container {
    overflow: auto !important;
  }
}

@media (max-width: 980px) {
  .app-sider {
    position: absolute;
    left: 0;
    top: 0;
    bottom: 0;
    max-width: calc(var(--app-vw, 100vw) - 1rem);
    box-shadow: 18px 0 40px rgba(0, 24, 88, 0.16);
  }
}

@media (max-width: 640px) {
  .page-hero-card {
    padding: var(--page-gutter);
    border-radius: clamp(1rem, 3vw, 1.25rem);
  }

  .page-hero-card__top {
    flex-wrap: wrap;
  }

  .brand-panel__copy p {
    display: none;
  }
}

@media (max-height: 820px) {
  .app-content > .n-layout-scroll-container,
  .app-main-layout > .n-layout-scroll-container,
  .page-frame {
    overflow: auto !important;
  }
}
</style>
