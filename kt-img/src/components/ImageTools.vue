<template>
  <div class="page-view image-page">
    <section class="tool-surface image-toolbar-card image-card">
      <div class="image-toolbar">
        <div class="image-toolbar__intro">
          <span class="image-toolbar__eyebrow">KT IMAGE</span>
          <div class="tool-panel__title image-toolbar__copy">
            <strong>图片压缩转换</strong>
            <span class="tool-panel__meta">本地图片预览、压缩、转码与批量导出。</span>
          </div>
        </div>

        <div class="tool-inline-actions image-toolbar-actions">
          <button
            class="ui-button ui-button--secondary image-toolbar-button"
            type="button"
            :disabled="batchProcessing"
            @click="addImages"
          >
            + 添加图片
          </button>
          <button
            class="ui-button ui-button--danger image-toolbar-button"
            type="button"
            :disabled="images.length === 0 || batchProcessing"
            @click="clearAll"
          >
            清空
          </button>
          <div class="image-toolbar-divider"></div>
          <button
            class="ui-button ui-button--primary image-toolbar-button"
            type="button"
            :disabled="!selectedImage || processing || saving || batchProcessing"
            @click="saveImage"
          >
            <span v-if="saving" class="button-spinner"></span>
            <span>保存单张</span>
          </button>
          <button
            class="ui-button ui-button--primary image-toolbar-button"
            type="button"
            :disabled="images.length === 0 || batchProcessing"
            @click="batchProcess"
          >
            <span v-if="batchProcessing" class="button-spinner"></span>
            <span>批量保存 {{ batchProcessing ? `(${batchCompleted}/${batchTotal})` : '' }}</span>
          </button>
        </div>
      </div>

      <div class="image-toolbar-config">
        <div class="image-toolbar-config__layout">
          <div class="image-toolbar-config__group image-toolbar-config__group--format">
            <div class="config-grid config-grid--format">
              <div class="config-row">
                <span class="label">转换格式</span>
                <div class="image-button-group">
                  <button
                    class="ui-button ui-button--chip image-group-button"
                    :class="{ 'is-active': outputFormat === 'Jpg' }"
                    type="button"
                    @click="outputFormat = 'Jpg'"
                  >
                    JPG
                  </button>
                  <button
                    class="ui-button ui-button--chip image-group-button"
                    :class="{ 'is-active': outputFormat === 'Png' }"
                    type="button"
                    @click="outputFormat = 'Png'"
                  >
                    PNG
                  </button>
                  <button
                    class="ui-button ui-button--chip image-group-button"
                    :class="{ 'is-active': outputFormat === 'WebP' }"
                    type="button"
                    @click="outputFormat = 'WebP'"
                  >
                    WebP
                  </button>
                </div>
                <div v-if="outputFormat === 'WebP'" class="image-button-group image-button-group--mode">
                  <button
                    class="ui-button ui-button--chip image-group-button image-group-button--mode"
                    :class="{ 'is-active': !webpLossless }"
                    type="button"
                    @click="webpLossless = false"
                  >
                    有损
                  </button>
                  <button
                    class="ui-button ui-button--chip image-group-button image-group-button--mode"
                    :class="{ 'is-active': webpLossless }"
                    type="button"
                    @click="webpLossless = true"
                  >
                    无损
                  </button>
                </div>
              </div>
            </div>

            <p class="image-format-hint">{{ formatHint }}</p>
          </div>

          <div class="image-toolbar-config__group image-toolbar-config__group--adjustments">
            <div class="config-grid config-grid--adjustments">
              <div class="config-row config-row--quality image-control image-control--slider">
                <label class="label label--stack" for="quality-slider">
                  <span>{{ qualityLabel }} {{ quality }}%</span>
                </label>
                <input
                  id="quality-slider"
                  v-model.number="quality"
                  class="image-control__slider"
                  type="range"
                  min="1"
                  max="100"
                  step="1"
                  :disabled="qualityControlsDisabled"
                />
                <div class="image-button-group image-button-group--presets">
                  <button
                    v-for="preset in qualityPresets"
                    :key="preset.label"
                    class="ui-button ui-button--chip image-group-button"
                    :class="{ 'is-active': quality === preset.value }"
                    type="button"
                    :disabled="qualityControlsDisabled"
                    @click="applyQualityPreset(preset.value)"
                  >
                    {{ preset.label }}
                  </button>
                </div>
              </div>

              <div class="config-row config-row--stack image-control image-control--slider">
                <label class="label label--stack" for="scale-slider">
                  <span>缩放 {{ scale }}%</span>
                  <span class="label__meta">{{ getScaledSize(scale) }}</span>
                </label>
                <input
                  id="scale-slider"
                  v-model.number="scale"
                  class="image-control__slider"
                  type="range"
                  min="1"
                  max="100"
                  step="1"
                  :title="getScaledSize(scale)"
                />
              </div>
            </div>
          </div>
        </div>

      </div>
    </section>

    <div class="main-layout">
      <aside class="sidebar tool-surface image-card">
        <div v-if="images.length > 0" class="image-sidebar-scroll">
          <div class="image-list">
            <button
              v-for="(imageEntry, idx) in images"
              :key="idx"
              class="image-list-item"
              :class="{
                active: selectedIndex === idx,
                'image-list-item--error': !!imageEntry.error
              }"
              type="button"
              :title="getImageListTitle(imageEntry)"
              @click="selectImage(idx)"
            >
              <span class="image-list-item__name">{{ imageEntry.name }}</span>
              <span
                class="image-list-item__detail"
                :class="{ 'image-list-item__detail--error': !!imageEntry.metadataError }"
              >
                {{ getImageListDetail(imageEntry) }}
              </span>
              <span
                v-if="imageEntry.error"
                class="image-list-item__meta image-list-item__meta--error"
              >
                {{ imageEntry.error }}
              </span>
              <span v-else-if="imageEntry.outputPath" class="image-list-item__meta">
                已导出到 {{ getOutputFileName(imageEntry.outputPath) }}
              </span>
            </button>
          </div>
        </div>
        <div v-else class="empty-state image-empty-state">
          <strong>还没有图片</strong>
          <span>点击右上角按钮添加图片</span>
        </div>
      </aside>

      <div class="main-content">
        <section v-if="selectedImage" class="preview-area tool-surface image-card">
          <div class="image-preview-head">
            <strong class="image-preview-title">{{ selectedImage.name }}</strong>
          </div>

          <div
            ref="splitContainer"
            class="image-compare-split"
            :class="{
              'image-compare-split--stacked': stackedPreview,
              'image-compare-split--dragging': splitDragging
            }"
            :style="splitLayoutStyle"
          >
            <section class="image-panel tool-surface image-card image-card--inner">
              <div class="panel-header">
                <span class="panel-title">原图</span>
                <div class="panel-header-meta">
                  <span class="image-preview-load-time">{{ originalPreviewLoadText }}</span>
                  <span class="panel-subtitle">{{ originalPreviewInfo || '--' }}</span>
                </div>
              </div>

              <div class="image-viewer">
                <img
                  v-if="originalImageUrl"
                  class="image-viewer__asset"
                  :src="originalImageUrl"
                  alt="原图"
                  @load="handleOriginalPreviewLoad"
                  @error="handleOriginalPreviewError"
                />
                <div v-else-if="previewLoading" class="status-stack">
                  <span class="spinner"></span>
                </div>
                <span v-else class="empty-copy">加载中...</span>
              </div>
            </section>

            <button
              class="split-handle"
              :class="{ 'split-handle--stacked': stackedPreview }"
              type="button"
              aria-label="调整预览分栏比例"
              @pointerdown="startSplitDrag"
              @keydown.left.prevent="nudgeSplit(-0.05)"
              @keydown.right.prevent="nudgeSplit(0.05)"
              @keydown.up.prevent="nudgeSplit(-0.05)"
              @keydown.down.prevent="nudgeSplit(0.05)"
            >
              <span class="split-handle__grip"></span>
            </button>

            <section class="image-panel tool-surface image-card image-card--inner">
              <div class="panel-header">
                <span class="panel-title">处理后</span>
                <div class="panel-header-meta">
                  <span class="image-preview-load-time">{{ processedPreviewLoadText }}</span>
                  <span class="panel-subtitle image-compression-meta">
                    {{ compressionInfo || '--' }}
                  </span>
                </div>
              </div>

              <div class="image-viewer">
                <img
                  v-if="processedImageUrl"
                  class="image-viewer__asset"
                  :src="processedImageUrl"
                  alt="处理后"
                  @load="handleProcessedPreviewLoad"
                  @error="handleProcessedPreviewError"
                />
                <div v-else-if="processing" class="status-stack">
                  <span class="spinner"></span>
                  <span class="empty-copy">处理中...</span>
                </div>
                <button
                  v-else
                  class="ui-button ui-button--primary"
                  type="button"
                  :disabled="!selectedImage || processing"
                  @click="processImage"
                >
                  处理图片
                </button>
              </div>
            </section>
          </div>
        </section>

        <section v-else class="empty-preview tool-surface image-card">
          <div class="empty-state">
            <strong>暂无预览</strong>
            <span>从左侧列表中选择一张图片</span>
          </div>
        </section>
      </div>
    </div>

    <TransitionGroup name="toast" tag="div" class="toast-stack">
      <div v-for="toast in toasts" :key="toast.id" class="toast" :class="`toast--${toast.type}`">
        {{ toast.text }}
      </div>
    </TransitionGroup>

    <div
      v-if="confirmDialogState.open"
      class="modal-backdrop"
      @click.self="closeConfirmDialog(false)"
    >
      <div class="modal-card">
        <div class="modal-card__header">
          <strong>{{ confirmDialogState.title }}</strong>
          <p>{{ confirmDialogState.content }}</p>
        </div>
        <div class="modal-card__actions">
          <button
            class="ui-button ui-button--ghost"
            type="button"
            @click="closeConfirmDialog(false)"
          >
            {{ confirmDialogState.negativeText }}
          </button>
          <button
            class="ui-button ui-button--danger"
            type="button"
            @click="closeConfirmDialog(true)"
          >
            {{ confirmDialogState.positiveText }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { invoke } from '@tauri-apps/api/core'
import { open, save } from '@tauri-apps/plugin-dialog'

const DEFAULT_QUALITY_BY_FORMAT = Object.freeze({
  Jpg: 80,
  Png: 65,
  WebP: 80
})
const DEFAULT_EXPORT_SETTINGS = Object.freeze({
  outputFormat: 'Jpg',
  qualityByFormat: DEFAULT_QUALITY_BY_FORMAT,
  scale: 100,
  webpLossless: false
})
const EXPORT_SETTINGS_STORAGE_KEY = 'kt-img:last-export-settings'
const EXPORT_FORMATS = ['Jpg', 'Png', 'WebP']

const images = ref([])
const selectedIndex = ref(null)
const outputFormat = ref(DEFAULT_EXPORT_SETTINGS.outputFormat)
const qualityByFormat = reactive({
  ...DEFAULT_QUALITY_BY_FORMAT
})
const scale = ref(DEFAULT_EXPORT_SETTINGS.scale)
const webpLossless = ref(DEFAULT_EXPORT_SETTINGS.webpLossless)
const processing = ref(false)
const saving = ref(false)
const batchProcessing = ref(false)
const batchCompleted = ref(0)
const batchTotal = ref(0)
const previewLoading = ref(false)
const originalImageUrl = ref('')
const originalPreviewLoadMs = ref(null)
const processedImageUrl = ref('')
const processedPreviewMeta = ref(null)
const processedPreviewLoadMs = ref(null)
const processedPreviewLoading = ref(false)
const viewportWidth = ref(0)
const splitContainer = ref(null)
const toasts = ref([])
const confirmDialogState = reactive({
  open: false,
  title: '',
  content: '',
  positiveText: '确定',
  negativeText: '取消',
  onPositiveClick: null
})

let previewRequestId = 0
let processRequestId = 0
let autoProcessTimer = null
let persistSettingsTimer = null
let originalPreviewLoadStartedAt = 0
let processedPreviewLoadStartedAt = 0
let nextToastId = 0
let componentUnmounted = false
const toastTimers = new Map()
const metadataQueue = []
let metadataWorkers = 0
const AUTO_PROCESS_DELAY = 180
const PERSIST_SETTINGS_DELAY = 180
const METADATA_CONCURRENCY = 6
const splitDragging = ref(false)
const splitRatio = ref(0.5)
let removeSplitListeners = null

const pushToast = (type, text) => {
  const id = ++nextToastId

  toasts.value = [...toasts.value, { id, type, text: String(text) }]

  const timer = window.setTimeout(() => {
    removeToast(id)
  }, 2800)

  toastTimers.set(id, timer)
}

const removeToast = (id) => {
  const timer = toastTimers.get(id)
  if (timer) {
    clearTimeout(timer)
    toastTimers.delete(id)
  }

  toasts.value = toasts.value.filter((toast) => toast.id !== id)
}

const clearToasts = () => {
  for (const timer of toastTimers.values()) {
    clearTimeout(timer)
  }

  toastTimers.clear()
  toasts.value = []
}

const showSuccessToast = (text) => {
  pushToast('success', text)
}

const showErrorToast = (text) => {
  pushToast('error', text)
}

const clampInteger = (value, min, max, fallback) => {
  const numericValue = Number(value)
  if (!Number.isFinite(numericValue)) return fallback

  return Math.min(max, Math.max(min, Math.round(numericValue)))
}

const normalizeOutputFormat = (value) => {
  return EXPORT_FORMATS.includes(value)
    ? value
    : DEFAULT_EXPORT_SETTINGS.outputFormat
}

const normalizeExportSettings = (settings) => {
  const source = settings && typeof settings === 'object' ? settings : {}
  const qualitySource = source.qualityByFormat && typeof source.qualityByFormat === 'object'
    ? source.qualityByFormat
    : {}

  return {
    outputFormat: normalizeOutputFormat(source.outputFormat),
    qualityByFormat: {
      Jpg: clampInteger(qualitySource.Jpg, 1, 100, DEFAULT_QUALITY_BY_FORMAT.Jpg),
      Png: clampInteger(qualitySource.Png, 1, 100, DEFAULT_QUALITY_BY_FORMAT.Png),
      WebP: clampInteger(qualitySource.WebP, 1, 100, DEFAULT_QUALITY_BY_FORMAT.WebP)
    },
    scale: clampInteger(source.scale, 1, 100, DEFAULT_EXPORT_SETTINGS.scale),
    webpLossless: source.webpLossless === true
  }
}

const createExportSettingsSnapshot = () => normalizeExportSettings({
  outputFormat: outputFormat.value,
  qualityByFormat: {
    Jpg: qualityByFormat.Jpg,
    Png: qualityByFormat.Png,
    WebP: qualityByFormat.WebP
  },
  scale: scale.value,
  webpLossless: webpLossless.value
})

const applyExportSettings = (settings) => {
  const normalized = normalizeExportSettings(settings)

  qualityByFormat.Jpg = normalized.qualityByFormat.Jpg
  qualityByFormat.Png = normalized.qualityByFormat.Png
  qualityByFormat.WebP = normalized.qualityByFormat.WebP
  outputFormat.value = normalized.outputFormat
  scale.value = normalized.scale
  webpLossless.value = normalized.webpLossless
}

const restoreLastExportSettings = () => {
  if (typeof window === 'undefined') return null

  try {
    const rawValue = window.localStorage.getItem(EXPORT_SETTINGS_STORAGE_KEY)
    if (!rawValue) return null

    return normalizeExportSettings(JSON.parse(rawValue))
  } catch {
    window.localStorage.removeItem(EXPORT_SETTINGS_STORAGE_KEY)
    return null
  }
}

const persistLastExportSettings = (settings) => {
  if (typeof window === 'undefined') return

  try {
    window.localStorage.setItem(
      EXPORT_SETTINGS_STORAGE_KEY,
      JSON.stringify(normalizeExportSettings(settings))
    )
  } catch {
    // localStorage 不可用时跳过，不影响主流程
  }
}

const clearPersistSettingsTimer = () => {
  if (!persistSettingsTimer) return

  clearTimeout(persistSettingsTimer)
  persistSettingsTimer = null
}

const flushPersistExportSettings = () => {
  clearPersistSettingsTimer()
  persistLastExportSettings(createExportSettingsSnapshot())
}

const queuePersistExportSettings = () => {
  clearPersistSettingsTimer()
  const settingsSnapshot = createExportSettingsSnapshot()

  persistSettingsTimer = window.setTimeout(() => {
    persistSettingsTimer = null
    persistLastExportSettings(settingsSnapshot)
  }, PERSIST_SETTINGS_DELAY)
}

const openWarningDialog = ({
  title,
  content,
  positiveText = '确定',
  negativeText = '取消',
  onPositiveClick
}) => {
  confirmDialogState.open = true
  confirmDialogState.title = title
  confirmDialogState.content = content
  confirmDialogState.positiveText = positiveText
  confirmDialogState.negativeText = negativeText
  confirmDialogState.onPositiveClick = onPositiveClick || null
}

const closeConfirmDialog = (confirmed) => {
  const onPositiveClick = confirmed ? confirmDialogState.onPositiveClick : null

  confirmDialogState.open = false
  confirmDialogState.title = ''
  confirmDialogState.content = ''
  confirmDialogState.positiveText = '确定'
  confirmDialogState.negativeText = '取消'
  confirmDialogState.onPositiveClick = null

  if (onPositiveClick) {
    onPositiveClick()
  }
}

const quality = computed({
  get: () => qualityByFormat[outputFormat.value],
  set: (value) => {
    qualityByFormat[outputFormat.value] = value
  }
})

const getScaledSize = (value) => {
  if (!selectedImage.value?.width) return value + '%'
  const w = Math.round(selectedImage.value.width * value / 100)
  const h = Math.round(selectedImage.value.height * value / 100)
  return `${w} x ${h}`
}

const normalizeErrorMessage = (error) => {
  if (typeof error === 'string') return error
  if (error instanceof Error) return error.message

  return String(error)
}

const formatFileSize = (bytes) => {
  if (!Number.isFinite(bytes) || bytes < 0) return '--'
  if (bytes < 1024) return `${bytes} B`

  const units = ['KB', 'MB', 'GB', 'TB']
  let value = bytes / 1024
  let unitIndex = 0

  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024
    unitIndex++
  }

  let digits = 2
  if (value >= 100) {
    digits = 0
  } else if (value >= 10) {
    digits = 1
  }

  return `${value.toFixed(digits)} ${units[unitIndex]}`
}

const stackedPreview = computed(() => viewportWidth.value <= 1180)
const splitLayoutStyle = computed(() => {
  const primary = Number(splitRatio.value.toFixed(3))
  const secondary = Number((1 - splitRatio.value).toFixed(3))

  return stackedPreview.value
    ? {
        gridTemplateColumns: 'minmax(0, 1fr)',
        gridTemplateRows: `${primary}fr 14px ${secondary}fr`
      }
    : {
        gridTemplateColumns: `${primary}fr 14px ${secondary}fr`,
        gridTemplateRows: 'minmax(0, 1fr)'
      }
})

const syncViewport = () => {
  viewportWidth.value = window.innerWidth
}

const clampSplitRatio = (value) => Math.min(0.8, Math.max(0.2, value))

const stopSplitDrag = () => {
  splitDragging.value = false

  if (removeSplitListeners) {
    removeSplitListeners()
    removeSplitListeners = null
  }
}

const updateSplitRatioFromPointer = (clientX, clientY) => {
  const rect = splitContainer.value?.getBoundingClientRect()
  if (!rect) return

  const axisSize = stackedPreview.value ? rect.height : rect.width
  if (axisSize <= 0) return

  const offset = stackedPreview.value ? clientY - rect.top : clientX - rect.left
  splitRatio.value = clampSplitRatio(offset / axisSize)
}

const startSplitDrag = (event) => {
  event.preventDefault()
  splitDragging.value = true
  updateSplitRatioFromPointer(event.clientX, event.clientY)

  const handlePointerMove = (moveEvent) => {
    updateSplitRatioFromPointer(moveEvent.clientX, moveEvent.clientY)
  }

  const handlePointerUp = () => {
    stopSplitDrag()
  }

  window.addEventListener('pointermove', handlePointerMove)
  window.addEventListener('pointerup', handlePointerUp, { once: true })
  window.addEventListener('pointercancel', handlePointerUp, { once: true })

  removeSplitListeners = () => {
    window.removeEventListener('pointermove', handlePointerMove)
    window.removeEventListener('pointerup', handlePointerUp)
    window.removeEventListener('pointercancel', handlePointerUp)
  }
}

const nudgeSplit = (delta) => {
  splitRatio.value = clampSplitRatio(splitRatio.value + delta)
}

const selectedImage = computed(() => {
  if (selectedIndex.value !== null && selectedIndex.value < images.value.length) {
    return images.value[selectedIndex.value]
  }
  return null
})

const originalPreviewLoadText = computed(() => {
  if (previewLoading.value) return '原图加载中...'

  const loadMs = originalPreviewLoadMs.value
  if (typeof loadMs !== 'number') return '原图加载耗时: --'

  return `原图加载耗时: ${loadMs.toFixed(2)} ms`
})

const originalPreviewInfo = computed(() => {
  if (!selectedImage.value) return ''

  const hasMetadata = selectedImage.value.metadataLoaded
  const sizeText = hasMetadata ? formatFileSize(selectedImage.value.size) : '--'
  const resolutionText = hasMetadata
    ? `${selectedImage.value.width} x ${selectedImage.value.height}`
    : '--'

  return `${sizeText} | ${resolutionText}`
})

const compressionRatioText = computed(() => {
  if (!processedPreviewMeta.value) {
    return '--'
  }

  const originalSize = processedPreviewMeta.value.originalSize || selectedImage.value?.size
  if (!Number.isFinite(originalSize) || originalSize <= 0) {
    return '--'
  }

  const ratio = Math.round((1 - processedPreviewMeta.value.processedSize / originalSize) * 100)

  if (ratio > 0) {
    return `↓${ratio}%`
  }

  if (ratio < 0) {
    return `↑${Math.abs(ratio)}%`
  }

  return '0%'
})

const finishOriginalPreviewLoad = ({ failed = false } = {}) => {
  if (!previewLoading.value) return

  if (!failed && originalPreviewLoadStartedAt > 0) {
    originalPreviewLoadMs.value = performance.now() - originalPreviewLoadStartedAt
  }

  previewLoading.value = false
  originalPreviewLoadStartedAt = 0
}

const handleOriginalPreviewLoad = () => {
  finishOriginalPreviewLoad()
}

const handleOriginalPreviewError = () => {
  finishOriginalPreviewLoad({ failed: true })
  showErrorToast('原图预览渲染失败')
}

const processedPreviewLoadText = computed(() => {
  if (processing.value || processedPreviewLoading.value) return '处理结果生成中...'

  if (typeof processedPreviewLoadMs.value !== 'number') return '处理结果耗时: --'

  return `处理结果耗时: ${processedPreviewLoadMs.value.toFixed(2)} ms`
})

const finishProcessedPreviewLoad = ({ failed = false } = {}) => {
  if (!processedPreviewLoading.value) return

  if (!failed && processedPreviewLoadStartedAt > 0) {
    processedPreviewLoadMs.value = performance.now() - processedPreviewLoadStartedAt
  }

  processedPreviewLoading.value = false
  processedPreviewLoadStartedAt = 0
}

const handleProcessedPreviewLoad = () => {
  finishProcessedPreviewLoad()
}

const handleProcessedPreviewError = () => {
  finishProcessedPreviewLoad({ failed: true })
  showErrorToast('处理后预览渲染失败')
}

const compressionInfo = computed(() => {
  if (!processedPreviewMeta.value) return ''

  const meta = processedPreviewMeta.value
  const sizeText = formatFileSize(meta.processedSize)
  const resolutionText = `${meta.outputWidth} x ${meta.outputHeight}`

  return `${sizeText} | ${resolutionText} | ${compressionRatioText.value}`
})

const qualityControlsDisabled = computed(() => outputFormat.value === 'WebP' && webpLossless.value)

const qualityPresets = computed(() => {
  if (outputFormat.value === 'Png') {
    return [
      { label: '快速', value: 25 },
      { label: '平衡', value: 65 },
      { label: '高压缩', value: 95 }
    ]
  }

  return [
    { label: '快速', value: 55 },
    { label: '平衡', value: 80 },
    { label: '高质量', value: 92 }
  ]
})

const qualityLabel = computed(() => outputFormat.value === 'Png' ? '压缩强度' : '质量')

const formatHint = computed(() => {
  if (outputFormat.value === 'Png') {
    return '* PNG 为无损压缩，数值越高通常越慢但体积更小，输出图片不含元数据'
  }

  if (outputFormat.value === 'WebP') {
    return webpLossless.value
      ? '* WebP 当前使用无损编码，适合保真导出，输出图片不含元数据'
      : '* WebP 当前使用有损编码，质量越低体积越小、速度越快，输出图片不含元数据'
  }

  return '* JPG 质量越低体积越小、速度越快，输出图片不含元数据'
})

const revokeObjectUrl = (url) => {
  if (url) {
    URL.revokeObjectURL(url)
  }
}

const getOutputFileName = (outputPath) => outputPath.split(/[/\\]/).pop() || outputPath

const getImageListTitle = (imageEntry) => {
  const metadata = getImageListDetail(imageEntry)
  if (imageEntry.error) {
    return `${imageEntry.name}\n${metadata}\n错误: ${imageEntry.error}`
  }

  if (imageEntry.outputPath) {
    return `${imageEntry.name}\n${metadata}\n输出: ${imageEntry.outputPath}`
  }

  return `${imageEntry.name}\n${metadata}`
}

const getImageListDetail = (imageEntry) => {
  if (imageEntry.metadataLoaded) {
    return `${imageEntry.width} x ${imageEntry.height} · ${formatFileSize(imageEntry.size)}`
  }

  if (imageEntry.metadataError) {
    return imageEntry.metadataError
  }

  if (imageEntry.metadataLoading || imageEntry.metadataQueued) {
    return '读取大小和分辨率中...'
  }

  return '等待元数据'
}

const applyQualityPreset = (value) => {
  quality.value = value
}

const clearAutoProcessTimer = () => {
  if (!autoProcessTimer) return
  clearTimeout(autoProcessTimer)
  autoProcessTimer = null
}

const resetOriginalPreviewState = () => {
  previewLoading.value = false
  originalPreviewLoadStartedAt = 0
  originalPreviewLoadMs.value = null
  revokeObjectUrl(originalImageUrl.value)
  originalImageUrl.value = ''
}

const invalidateProcessedPreview = () => {
  processRequestId++
  clearAutoProcessTimer()
  processing.value = false
  processedPreviewLoading.value = false
  processedPreviewLoadStartedAt = 0
}

const resetProcessedState = () => {
  processedPreviewMeta.value = null
  revokeObjectUrl(processedImageUrl.value)
  processedImageUrl.value = ''
  processedPreviewLoadMs.value = null
}

const createImageEntry = (file) => {
  const fileName = file.split(/[/\\]/).pop() || file

  return reactive({
    path: file,
    name: fileName,
    size: 0,
    width: 0,
    height: 0,
    error: null,
    outputPath: '',
    metadataLoaded: false,
    metadataLoading: false,
    metadataQueued: false,
    metadataError: ''
  })
}

const syncImageMetadata = (imageEntry, metadata) => {
  if (!imageEntry || !metadata) return

  imageEntry.size = metadata.original_size
  imageEntry.width = metadata.width
  imageEntry.height = metadata.height
  imageEntry.metadataLoaded = true
  imageEntry.metadataError = ''
}

const startMetadataWorkers = () => {
  while (
    !componentUnmounted &&
    metadataWorkers < METADATA_CONCURRENCY &&
    metadataQueue.length > 0
  ) {
    void runMetadataWorker()
  }
}

const runMetadataWorker = async () => {
  metadataWorkers++

  try {
    while (!componentUnmounted && metadataQueue.length > 0) {
      const imageEntry = metadataQueue.shift()
      if (!imageEntry || !imageEntry.path || imageEntry.metadataLoaded || imageEntry.metadataLoading) {
        continue
      }

      imageEntry.metadataQueued = false
      imageEntry.metadataLoading = true

      try {
        const metadata = await invoke('load_image_metadata', { path: imageEntry.path })
        if (componentUnmounted) {
          return
        }

        syncImageMetadata(imageEntry, metadata)
      } catch (e) {
        if (componentUnmounted) {
          return
        }

        imageEntry.metadataError = '元数据读取失败'
      } finally {
        imageEntry.metadataLoading = false
      }
    }
  } finally {
    metadataWorkers--
    if (!componentUnmounted) {
      startMetadataWorkers()
    }
  }
}

const enqueueMetadataLoad = (entries) => {
  for (const imageEntry of entries) {
    if (!imageEntry?.path || imageEntry.metadataLoaded || imageEntry.metadataLoading || imageEntry.metadataQueued) {
      continue
    }

    imageEntry.metadataQueued = true
    imageEntry.metadataError = ''
    metadataQueue.push(imageEntry)
  }

  startMetadataWorkers()
}

const batchConcurrency = (count = images.value.length) => {
  const availableWorkers = globalThis.navigator?.hardwareConcurrency || 4
  return Math.max(1, Math.min(count, availableWorkers, 6))
}

onBeforeUnmount(() => {
  metadataQueue.length = 0
  componentUnmounted = true
  window.removeEventListener('resize', syncViewport)
  invalidateProcessedPreview()
  resetOriginalPreviewState()
  resetProcessedState()
  flushPersistExportSettings()
  clearToasts()
  stopSplitDrag()
})

onMounted(() => {
  componentUnmounted = false
  const restoredSettings = restoreLastExportSettings()
  if (restoredSettings) {
    applyExportSettings(restoredSettings)
  }
  syncViewport()
  window.addEventListener('resize', syncViewport)
})

const addImages = async () => {
  try {
    const files = await open({
      multiple: true,
      filters: [{ name: 'Images', extensions: ['jpg', 'jpeg', 'png', 'webp', 'bmp'] }]
    })

    if (files && files.length > 0) {
      const newEntries = files.map((file) => createImageEntry(file))

      images.value.push(...newEntries)
      enqueueMetadataLoad(newEntries)
    }
  } catch (e) {
    showErrorToast('选择文件失败: ' + normalizeErrorMessage(e))
  }
}

const selectImage = async (idx) => {
  invalidateProcessedPreview()
  selectedIndex.value = idx
  const currentRequestId = ++previewRequestId
  resetProcessedState()
  resetOriginalPreviewState()

  const imageEntry = images.value[idx]
  if (!imageEntry || !imageEntry.path) return
  if (!imageEntry.metadataLoaded && !imageEntry.metadataLoading) {
    enqueueMetadataLoad([imageEntry])
  }
  queueProcessedPreviewRefresh()

  previewLoading.value = true
  originalPreviewLoadStartedAt = performance.now()

  try {
    const result = await invoke('load_original_preview', {
      path: imageEntry.path
    })

    if (componentUnmounted || currentRequestId !== previewRequestId || selectedIndex.value !== idx) {
      return
    }

    syncImageMetadata(imageEntry, {
      original_size: result.original_size,
      width: result.width,
      height: result.height
    })

    const blob = new Blob([new Uint8Array(result.image_data)], { type: result.mime_type })
    originalImageUrl.value = URL.createObjectURL(blob)
  } catch (e) {
    if (!componentUnmounted && currentRequestId === previewRequestId && selectedIndex.value === idx) {
      previewLoading.value = false
      originalPreviewLoadStartedAt = 0
      showErrorToast('加载预览失败: ' + normalizeErrorMessage(e))
    }
  }
}

const processImage = async ({ notifySuccess = true } = {}) => {
  const targetImage = selectedImage.value
  if (!targetImage) return

  clearAutoProcessTimer()
  const currentRequestId = ++processRequestId
  const targetFormat = outputFormat.value
  const targetQuality = quality.value
  const targetScale = scale.value
  const targetWebpLossless = webpLossless.value

  processing.value = true
  resetProcessedState()
  processedPreviewLoading.value = true
  processedPreviewLoadStartedAt = performance.now()

  try {
    const result = await invoke('process_image_preview', {
      path: targetImage.path,
      format: targetFormat,
      quality: targetQuality,
      scale: targetScale,
      webpLossless: targetWebpLossless
    })

    if (currentRequestId !== processRequestId || selectedImage.value?.path !== targetImage.path) {
      return
    }

    processedPreviewMeta.value = {
      originalSize: result.original_size,
      outputWidth: result.output_width,
      outputHeight: result.output_height,
      processedSize: result.processed_size
    }

    const blob = new Blob([new Uint8Array(result.data)], { type: result.mime_type })
    processedImageUrl.value = URL.createObjectURL(blob)

    images.value[selectedIndex.value].error = null

    if (notifySuccess) {
      showSuccessToast('处理完成')
    }
  } catch (e) {
    if (currentRequestId !== processRequestId || selectedImage.value?.path !== targetImage.path) {
      return
    }
    processedPreviewLoading.value = false
    processedPreviewLoadStartedAt = 0
    showErrorToast('处理失败: ' + normalizeErrorMessage(e))
  } finally {
    if (currentRequestId === processRequestId && selectedImage.value?.path === targetImage.path) {
      processing.value = false
    }
  }
}

const queueProcessedPreviewRefresh = () => {
  clearAutoProcessTimer()

  if (!selectedImage.value?.path) return

  processRequestId++
  resetProcessedState()
  processing.value = true

  autoProcessTimer = window.setTimeout(() => {
    autoProcessTimer = null
    processImage({ notifySuccess: false })
  }, AUTO_PROCESS_DELAY)
}

const saveImage = async () => {
  if (!selectedImage.value) return

  saving.value = true
  try {
    const ext = outputFormat.value.toLowerCase()
    const suggestedName = selectedImage.value.name.replace(/\.[^.]+$/, '') + '_compress.' + ext

    const filePath = await save({
      defaultPath: suggestedName,
      filters: [{ name: ext.toUpperCase(), extensions: [ext] }]
    })

    if (filePath) {
      const result = await invoke('process_and_write_image', {
        inputPath: selectedImage.value.path,
        outputPath: filePath,
        format: outputFormat.value,
        quality: quality.value,
        scale: scale.value,
        webpLossless: webpLossless.value
      })

      selectedImage.value.outputPath = result.output_path
      selectedImage.value.error = null
      showSuccessToast('保存成功')
    }
  } catch (e) {
    showErrorToast('保存失败: ' + normalizeErrorMessage(e))
  } finally {
    saving.value = false
  }
}

const batchProcess = async () => {
  if (images.value.length === 0) return

  try {
    const outputDir = await open({
      directory: true,
      title: '选择输出目录'
    })

    if (!outputDir) return

    const batchImages = images.value.slice()
    const batchSettings = {
      format: outputFormat.value,
      quality: quality.value,
      scale: scale.value,
      webpLossless: webpLossless.value
    }

    batchProcessing.value = true
    batchCompleted.value = 0
    batchTotal.value = batchImages.length
    const concurrency = batchConcurrency(batchImages.length)
    let nextIndex = 0
    const failures = []

    for (const imageEntry of batchImages) {
      imageEntry.error = null
      imageEntry.outputPath = ''
    }

    const worker = async () => {
      while (true) {
        const currentIndex = nextIndex++
        if (currentIndex >= batchImages.length) {
          return
        }

        const imageEntry = batchImages[currentIndex]

        try {
          const result = await invoke('process_and_save_image', {
            inputPath: imageEntry.path,
            outputDir,
            format: batchSettings.format,
            quality: batchSettings.quality,
            scale: batchSettings.scale,
            webpLossless: batchSettings.webpLossless
          })

          imageEntry.error = null
          imageEntry.outputPath = result.output_path
        } catch (e) {
          const errorMessage = normalizeErrorMessage(e)

          imageEntry.error = errorMessage
          imageEntry.outputPath = ''
          failures.push({ name: imageEntry.name, error: errorMessage })
        } finally {
          batchCompleted.value++
        }
      }
    }

    await Promise.all(Array.from({ length: concurrency }, () => worker()))

    if (failures.length > 0) {
      showErrorToast(`批量处理完成，但有 ${failures.length} 个文件失败，请查看左侧列表。`)
    } else {
      showSuccessToast('批量处理完成')
    }
  } catch (e) {
    showErrorToast('批量处理失败: ' + normalizeErrorMessage(e))
  } finally {
    batchProcessing.value = false
  }
}

const clearAll = () => {
  openWarningDialog({
    title: '确认清空',
    content: '确定要清空所有图片吗？',
    positiveText: '确定',
    negativeText: '取消',
    onPositiveClick: () => {
      invalidateProcessedPreview()
      metadataQueue.length = 0
      previewRequestId++
      images.value = []
      selectedIndex.value = null
      resetOriginalPreviewState()
      resetProcessedState()
    }
  })
}

watch(
  [
    outputFormat,
    scale,
    webpLossless,
    () => qualityByFormat.Jpg,
    () => qualityByFormat.Png,
    () => qualityByFormat.WebP
  ],
  () => {
    queuePersistExportSettings()

    if (selectedImage.value?.path) {
      queueProcessedPreviewRefresh()
    }
  }
)
</script>

<style scoped>
.image-page {
  position: relative;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.image-card {
  padding: 18px;
}

.image-card--inner {
  padding: 8px;
  border-radius: 20px;
  background: rgba(255, 255, 255, 0.92);
}

.image-toolbar-card {
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.image-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
}

.image-toolbar__intro {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-width: 0;
}

.image-toolbar__eyebrow {
  display: inline-flex;
  color: #9a3412;
  font-size: 0.76rem;
  font-weight: 700;
  letter-spacing: 0.16em;
  text-transform: uppercase;
}

.image-toolbar__copy {
  gap: 6px;
}

.image-toolbar__copy strong {
  font-size: clamp(1.1rem, 2vw, 1.25rem);
}

.image-toolbar-actions {
  align-items: center;
}

.image-toolbar-divider {
  width: 1px;
  height: 2rem;
  background: rgba(148, 163, 184, 0.28);
}

.image-toolbar-config {
  padding-top: 16px;
  border-top: 1px solid rgba(148, 163, 184, 0.18);
}

.image-toolbar-config__layout {
  display: grid;
  grid-template-columns: minmax(18rem, 1.15fr) minmax(18rem, 1fr);
  gap: 20px 24px;
  align-items: start;
}

.image-toolbar-config__group {
  min-width: 0;
}

.image-toolbar-config__group--adjustments {
  display: flex;
  flex-direction: column;
}

.ui-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-height: 2.5rem;
  padding: 0.68rem 1rem;
  border: 1px solid transparent;
  border-radius: 14px;
  background: #fff;
  color: #0f172a;
  cursor: pointer;
  font-size: 0.92rem;
  font-weight: 600;
  line-height: 1;
  transition: transform 120ms ease, background 120ms ease, border-color 120ms ease, box-shadow 120ms ease, opacity 120ms ease;
}

.ui-button:hover:not(:disabled) {
  transform: translateY(-1px);
}

.ui-button:disabled {
  opacity: 0.56;
  cursor: not-allowed;
}

.ui-button--primary {
  background: linear-gradient(135deg, #f97316, #ea580c);
  color: #fff;
  box-shadow: 0 10px 24px rgba(234, 88, 12, 0.22);
}

.ui-button--secondary {
  border-color: rgba(37, 99, 235, 0.16);
  background: linear-gradient(135deg, rgba(239, 246, 255, 0.95), rgba(255, 255, 255, 0.95));
  color: #1d4ed8;
}

.ui-button--danger {
  border-color: rgba(220, 38, 38, 0.16);
  background: linear-gradient(135deg, rgba(254, 242, 242, 0.95), rgba(255, 255, 255, 0.95));
  color: #b91c1c;
}

.ui-button--ghost {
  border-color: rgba(148, 163, 184, 0.2);
  background: rgba(255, 255, 255, 0.88);
  color: #334155;
}

.ui-button--chip {
  min-height: 2.25rem;
  padding: 0.55rem 0.9rem;
  border-color: rgba(148, 163, 184, 0.22);
  background: rgba(255, 255, 255, 0.72);
  color: #475569;
  box-shadow: none;
}

.ui-button--chip.is-active {
  border-color: rgba(249, 115, 22, 0.26);
  background: linear-gradient(135deg, rgba(255, 237, 213, 0.96), rgba(255, 247, 237, 0.92));
  color: #c2410c;
}

.button-spinner,
.spinner {
  width: 1rem;
  height: 1rem;
  border-radius: 999px;
  border: 2px solid currentColor;
  border-right-color: transparent;
  animation: spin 0.8s linear infinite;
}

.button-spinner {
  width: 0.9rem;
  height: 0.9rem;
}

.main-layout {
  flex: 1;
  display: flex;
  gap: 12px;
  min-height: 0;
  align-items: stretch;
  min-block-size: clamp(26rem, 62vh, 40rem);
  overflow: hidden;
}

.sidebar {
  width: clamp(15rem, 24vw, 17.5rem);
  flex: 0 0 clamp(15rem, 24vw, 17.5rem);
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  overflow: hidden;
}

.image-sidebar-scroll,
.image-empty-state {
  flex: 1;
  min-height: 0;
}

.image-sidebar-scroll {
  overflow: auto;
  padding-right: 4px;
  overscroll-behavior: contain;
}

.image-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.image-list-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
  width: 100%;
  padding: 0.8rem 0.9rem;
  border: 1px solid transparent;
  border-radius: 14px;
  background: transparent;
  color: #334155;
  text-align: left;
  cursor: pointer;
  transition: background 120ms ease, border-color 120ms ease, transform 120ms ease;
}

.image-list-item__name,
.image-list-item__detail,
.image-list-item__meta {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.image-list-item__name {
  width: 100%;
}

.image-list-item__detail {
  color: #64748b;
  font-size: 0.74rem;
}

.image-list-item__detail--error {
  color: #b45309;
}

.image-list-item__meta {
  color: #94a3b8;
  font-size: 0.72rem;
}

.image-list-item__meta--error {
  color: #b91c1c;
}

.image-list-item:hover {
  background: rgba(241, 245, 249, 0.92);
  transform: translateX(2px);
}

.image-list-item--error {
  border-color: rgba(220, 38, 38, 0.14);
  background: linear-gradient(135deg, rgba(254, 242, 242, 0.95), rgba(255, 255, 255, 0.92));
}

.image-list-item.active {
  border-color: rgba(37, 99, 235, 0.16);
  background: linear-gradient(135deg, rgba(239, 246, 255, 0.96), rgba(224, 231, 255, 0.92));
  color: #0f172a;
  font-weight: 700;
}

.main-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-width: 0;
  height: 100%;
  min-height: 0;
  overflow: hidden;
}

.config-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 18px 24px;
  align-items: center;
}

.config-grid--format {
  gap: 14px;
}

.config-grid--adjustments {
  flex-direction: column;
  align-items: stretch;
  gap: 14px;
}

.config-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  flex: 1 1 14rem;
  min-width: 0;
}

.config-row--stack {
  align-items: stretch;
  flex: 1 1 auto;
}

.config-row--quality {
  flex-wrap: nowrap;
  align-items: center;
}

.config-row--quality .label {
  flex: 0 0 auto;
}

.image-control {
  flex: 1 1 min(100%, 12rem);
}

.image-control--slider {
  align-items: center;
}

.config-row--stack.image-control--slider {
  gap: 8px;
}

.image-control__slider {
  flex: 1 1 min(100%, 7rem);
  width: 100%;
  min-width: 0;
  accent-color: #f97316;
}

.config-row--quality .image-control__slider {
  flex: 1 1 8rem;
  min-width: 5rem;
}

.image-button-group {
  display: inline-flex;
  flex-wrap: wrap;
  gap: 8px;
  max-width: 100%;
}

.image-group-button {
  min-width: clamp(3rem, 7vw, 3.625rem);
}

.image-group-button--mode {
  min-width: 0;
  min-height: 2rem;
  padding: 0.45rem 0.75rem;
  font-size: 0.84rem;
}

.image-button-group--presets {
  flex: 0 0 auto;
  width: auto;
  flex-wrap: nowrap;
}

.image-button-group--mode {
  margin-left: 6px;
}

.label {
  color: #475569;
  font-size: 0.82rem;
  font-weight: 600;
  white-space: nowrap;
}

.label--stack {
  display: flex;
  flex-direction: column;
  gap: 4px;
  white-space: normal;
}

.label__meta {
  color: #94a3b8;
  font-size: 0.76rem;
  font-weight: 500;
}

.image-format-hint {
  margin: 0.85rem 0 0;
  color: #64748b;
  font-size: 0.8rem;
  line-height: 1.6;
}

.preview-area {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  min-block-size: clamp(16rem, 38vh, 26rem);
  overflow: hidden;
}

.image-preview-head {
  margin-bottom: 10px;
}

.image-preview-title {
  display: block;
  color: #0f172a;
  font-size: clamp(0.95rem, 2.2vw, 1rem);
  word-break: break-word;
}

.image-compare-split {
  display: grid;
  gap: 12px;
  flex: 1;
  min-height: clamp(20rem, 52vh, 34rem);
}

.image-compare-split--stacked {
  gap: 10px;
}

.image-compare-split--dragging {
  user-select: none;
}

.image-panel {
  height: 100%;
  display: flex;
  flex-direction: column;
  min-height: 0;
  box-sizing: border-box;
  overflow: hidden;
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
  flex-wrap: wrap;
}

.panel-title {
  color: #475569;
  font-size: 0.86rem;
  font-weight: 700;
}

.panel-header-meta {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 4px;
  min-width: 0;
}

.panel-subtitle {
  color: #64748b;
  font-size: 0.8rem;
}

.image-compression-meta {
  font-size: clamp(0.75rem, 1.8vw, 0.8125rem);
}

.image-preview-load-time {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: 999px;
  background: #eff6ff;
  color: #1d4ed8;
  font-size: 12px;
  font-weight: 600;
  line-height: 1.5;
  font-variant-numeric: tabular-nums;
}

.image-viewer {
  flex: 1;
  min-height: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: clamp(0.75rem, 1.6vw, 1rem);
  border-radius: 16px;
  background:
    linear-gradient(135deg, rgba(248, 250, 252, 0.94), rgba(241, 245, 249, 0.94)),
    linear-gradient(45deg, rgba(255, 255, 255, 0.3) 25%, transparent 25%, transparent 75%, rgba(255, 255, 255, 0.3) 75%);
  overflow: hidden;
}

.image-viewer__asset {
  display: block;
  width: 100%;
  height: 100%;
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
}

.split-handle {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 14px;
  margin: 0;
  padding: 0;
  border: 0;
  border-radius: 999px;
  background: transparent;
  color: #94a3b8;
  cursor: col-resize;
}

.split-handle::before {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: inherit;
  background: linear-gradient(180deg, rgba(148, 163, 184, 0.2), rgba(148, 163, 184, 0.08));
}

.split-handle:hover::before,
.split-handle:focus-visible::before {
  background: linear-gradient(180deg, rgba(249, 115, 22, 0.3), rgba(249, 115, 22, 0.14));
}

.split-handle:focus-visible {
  outline: 2px solid rgba(249, 115, 22, 0.35);
  outline-offset: 2px;
}

.split-handle--stacked {
  width: 100%;
  height: 14px;
  cursor: row-resize;
}

.split-handle__grip {
  position: relative;
  z-index: 1;
  width: 4px;
  height: 3.5rem;
  border-radius: 999px;
  background: currentColor;
  opacity: 0.8;
}

.split-handle--stacked .split-handle__grip {
  width: 3.5rem;
  height: 4px;
}

.status-stack,
.empty-state,
.empty-preview {
  display: flex;
  align-items: center;
  justify-content: center;
}

.status-stack,
.empty-state {
  flex-direction: column;
  gap: 10px;
}

.empty-state {
  color: #64748b;
  text-align: center;
  line-height: 1.6;
}

.empty-state strong {
  color: #0f172a;
  font-size: 0.96rem;
}

.empty-copy {
  color: #64748b;
  font-size: 0.9rem;
}

.empty-preview {
  flex: 1;
  min-block-size: clamp(16rem, 38vh, 26rem);
}

.toast-stack {
  position: fixed;
  top: 24px;
  right: 24px;
  z-index: 40;
  display: flex;
  flex-direction: column;
  gap: 10px;
  pointer-events: none;
}

.toast {
  min-width: 14rem;
  max-width: min(24rem, calc(100vw - 2rem));
  padding: 0.9rem 1rem;
  border-radius: 16px;
  border: 1px solid rgba(148, 163, 184, 0.18);
  background: rgba(255, 255, 255, 0.95);
  box-shadow: 0 18px 38px rgba(15, 23, 42, 0.14);
  color: #0f172a;
  line-height: 1.5;
}

.toast--success {
  border-color: rgba(34, 197, 94, 0.18);
  color: #166534;
}

.toast--error {
  border-color: rgba(239, 68, 68, 0.18);
  color: #b91c1c;
}

.toast-enter-active,
.toast-leave-active {
  transition: opacity 160ms ease, transform 160ms ease;
}

.toast-enter-from,
.toast-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}

.modal-backdrop {
  position: fixed;
  inset: 0;
  z-index: 50;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
  background: rgba(15, 23, 42, 0.38);
  backdrop-filter: blur(10px);
}

.modal-card {
  width: min(100%, 28rem);
  padding: 20px;
  border: 1px solid rgba(148, 163, 184, 0.18);
  border-radius: 22px;
  background: rgba(255, 255, 255, 0.96);
  box-shadow: 0 30px 80px rgba(15, 23, 42, 0.22);
}

.modal-card__header {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.modal-card__header strong {
  color: #0f172a;
  font-size: 1.05rem;
}

.modal-card__header p {
  margin: 0;
  color: #64748b;
  line-height: 1.6;
}

.modal-card__actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 18px;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 1080px) {
  .main-layout {
    flex-direction: column;
    min-block-size: 0;
  }

  .sidebar {
    width: 100%;
    flex-basis: auto;
    min-height: 0;
    max-height: min(32vh, 16rem);
  }

  .main-content {
    min-height: 0;
  }
}

@media (max-width: 640px) {
  .image-card {
    padding: 16px;
  }

  .image-toolbar {
    gap: 14px;
  }

  .image-toolbar-actions {
    width: 100%;
  }

  .image-toolbar-button {
    flex: 1 1 calc(50% - 0.5rem);
  }

  .image-toolbar-divider {
    display: none;
  }

  .image-toolbar-config {
    padding-top: 14px;
  }

  .image-toolbar-config__layout {
    grid-template-columns: minmax(0, 1fr);
    gap: 16px;
  }

  .config-row--quality {
    flex-wrap: wrap;
    align-items: stretch;
  }

  .config-row--quality .image-control__slider,
  .image-button-group--presets {
    width: 100%;
  }

  .image-button-group--presets {
    flex-wrap: wrap;
  }

  .image-button-group--mode {
    margin-left: 0;
  }

  .image-button-group {
    width: 100%;
  }

  .config-row,
  .image-control--slider {
    align-items: stretch;
  }

  .panel-header-meta {
    align-items: flex-start;
  }

  .toast-stack {
    top: 16px;
    right: 16px;
    left: 16px;
  }

  .toast {
    max-width: none;
  }

  .modal-card__actions {
    flex-direction: column-reverse;
  }
}
</style>
