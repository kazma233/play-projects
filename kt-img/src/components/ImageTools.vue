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
          <div class="image-toolbar__badge">JPG / PNG / WebP</div>
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
            :disabled="!processedData || saving"
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
    </section>

    <div class="main-layout">
      <aside class="sidebar tool-surface image-card">
        <div v-if="images.length > 0" class="image-sidebar-scroll">
          <div class="image-list">
            <button
              v-for="(img, idx) in images"
              :key="idx"
              class="image-list-item"
              :class="{
                active: selectedIndex === idx,
                'image-list-item--error': !!img.error
              }"
              type="button"
              :title="getImageListTitle(img)"
              @click="selectImage(idx)"
            >
              <span class="image-list-item__name">{{ img.name }}</span>
              <span v-if="img.error" class="image-list-item__meta image-list-item__meta--error">
                {{ img.error }}
              </span>
              <span v-else-if="img.outputPath" class="image-list-item__meta">
                已导出到 {{ getOutputFileName(img.outputPath) }}
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
        <section class="config-panel tool-surface image-card image-card--config">
          <div class="config-grid">
            <div class="config-row">
              <span class="label">格式</span>
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
            </div>

            <div v-if="outputFormat === 'WebP'" class="config-row">
              <span class="label">模式</span>
              <div class="image-button-group">
                <button
                  class="ui-button ui-button--chip image-group-button"
                  :class="{ 'is-active': !webpLossless }"
                  type="button"
                  @click="webpLossless = false"
                >
                  有损
                </button>
                <button
                  class="ui-button ui-button--chip image-group-button"
                  :class="{ 'is-active': webpLossless }"
                  type="button"
                  @click="webpLossless = true"
                >
                  无损
                </button>
              </div>
            </div>

            <div v-if="showQualitySlider" class="config-row image-control image-control--slider">
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
              />
            </div>

            <div v-if="showQualitySlider" class="config-row">
              <span class="label">预设</span>
              <div class="image-button-group">
                <button
                  v-for="preset in qualityPresets"
                  :key="preset.label"
                  class="ui-button ui-button--chip image-group-button"
                  :class="{ 'is-active': quality === preset.value }"
                  type="button"
                  @click="applyQualityPreset(preset.value)"
                >
                  {{ preset.label }}
                </button>
              </div>
            </div>

            <div class="config-row image-control image-control--slider">
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

          <p class="image-format-hint">{{ formatHint }}</p>
        </section>

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
                  <span class="panel-subtitle">{{ selectedImage.width }} x {{ selectedImage.height }}</span>
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
                  <span class="panel-subtitle image-compression-meta">{{ compressionInfo || '--' }}</span>
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

    <div v-if="confirmState.open" class="modal-backdrop" @click.self="closeConfirm(false)">
      <div class="modal-card">
        <div class="modal-card__header">
          <strong>{{ confirmState.title }}</strong>
          <p>{{ confirmState.content }}</p>
        </div>
        <div class="modal-card__actions">
          <button class="ui-button ui-button--ghost" type="button" @click="closeConfirm(false)">
            {{ confirmState.negativeText }}
          </button>
          <button class="ui-button ui-button--danger" type="button" @click="closeConfirm(true)">
            {{ confirmState.positiveText }}
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
import { writeFile } from '@tauri-apps/plugin-fs'

const images = ref([])
const selectedIndex = ref(null)
const outputFormat = ref('Jpg')
const qualityByFormat = reactive({
  Jpg: 80,
  Png: 65,
  WebP: 80
})
const scale = ref(100)
const webpLossless = ref(false)
const processing = ref(false)
const saving = ref(false)
const batchProcessing = ref(false)
const batchCompleted = ref(0)
const batchTotal = ref(0)
const previewLoading = ref(false)
const originalImageUrl = ref('')
const processedData = ref(null)
const processedDimensions = ref(null)
const processedImageUrl = ref('')
const processedPreviewLoadMs = ref(null)
const processedPreviewLoading = ref(false)
const viewportWidth = ref(0)
const splitContainer = ref(null)
const toasts = ref([])
const confirmState = reactive({
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
let originalPreviewLoadStartedAt = 0
let processedPreviewLoadStartedAt = 0
let nextToastId = 0
const toastTimers = new Map()
const AUTO_PROCESS_DELAY = 180
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

const message = {
  success(text) {
    pushToast('success', text)
  },
  error(text) {
    pushToast('error', text)
  }
}

const dialog = {
  warning({ title, content, positiveText = '确定', negativeText = '取消', onPositiveClick }) {
    confirmState.open = true
    confirmState.title = title
    confirmState.content = content
    confirmState.positiveText = positiveText
    confirmState.negativeText = negativeText
    confirmState.onPositiveClick = onPositiveClick || null
  }
}

const closeConfirm = (confirmed) => {
  const onPositiveClick = confirmed ? confirmState.onPositiveClick : null

  confirmState.open = false
  confirmState.title = ''
  confirmState.content = ''
  confirmState.positiveText = '确定'
  confirmState.negativeText = '取消'
  confirmState.onPositiveClick = null

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

  const loadMs = selectedImage.value?.previewLoadMs
  if (typeof loadMs !== 'number') return '原图加载耗时: --'

  return `原图加载耗时: ${loadMs.toFixed(2)} ms`
})

const finishOriginalPreviewLoad = ({ failed = false } = {}) => {
  if (!previewLoading.value) return

  if (!failed && selectedImage.value && originalPreviewLoadStartedAt > 0) {
    selectedImage.value.previewLoadMs = performance.now() - originalPreviewLoadStartedAt
  }

  previewLoading.value = false
  originalPreviewLoadStartedAt = 0
}

const handleOriginalPreviewLoad = () => {
  finishOriginalPreviewLoad()
}

const handleOriginalPreviewError = () => {
  finishOriginalPreviewLoad({ failed: true })
  message.error('原图预览渲染失败')
}

const processedPreviewLoadText = computed(() => {
  if (processing.value || processedPreviewLoading.value) return '处理预览生成中...'

  if (typeof processedPreviewLoadMs.value !== 'number') return '处理预览耗时: --'

  return `处理预览耗时: ${processedPreviewLoadMs.value.toFixed(2)} ms`
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
  message.error('处理后预览渲染失败')
}

const compressionInfo = computed(() => {
  if (!processedData.value) return ''
  const dims = processedDimensions.value ? `${processedDimensions.value.width} x ${processedDimensions.value.height} | ` : ''
  const originalKb = selectedImage.value.size / 1024
  const processedKb = processedData.value.length / 1024
  if (originalKb === 0) return dims + Math.round(processedKb) + ' KB'
  const ratio = Math.round((originalKb - processedKb) / originalKb * 100)
  return dims + Math.round(originalKb) + ' KB → ' + Math.round(processedKb) + ' KB (↓' + ratio + '%)'
})

const showQualitySlider = computed(() => outputFormat.value !== 'WebP' || !webpLossless.value)

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

const getImageListTitle = (img) => {
  if (img.error) {
    return `${img.name}\n错误: ${img.error}`
  }

  if (img.outputPath) {
    return `${img.name}\n输出: ${img.outputPath}`
  }

  return img.name
}

const applyQualityPreset = (value) => {
  quality.value = value
}

const revokeImagePreview = (img) => {
  if (!img?.previewUrl) return
  revokeObjectUrl(img.previewUrl)
  img.previewUrl = ''
  img.previewLoaded = false
  img.previewLoadMs = null
}

const clearAutoProcessTimer = () => {
  if (!autoProcessTimer) return
  clearTimeout(autoProcessTimer)
  autoProcessTimer = null
}

const invalidateProcessedPreview = () => {
  processRequestId++
  clearAutoProcessTimer()
  processing.value = false
  processedPreviewLoading.value = false
  processedPreviewLoadStartedAt = 0
}

const resetProcessedState = () => {
  processedData.value = null
  processedDimensions.value = null
  revokeObjectUrl(processedImageUrl.value)
  processedImageUrl.value = ''
  processedPreviewLoadMs.value = null
}

const createImageEntry = (file) => {
  const fileName = file.split(/[/\\]/).pop() || file

  return {
    path: file,
    name: fileName,
    size: 0,
    width: 0,
    height: 0,
    processed: false,
    error: null,
    outputPath: '',
    previewLoaded: false,
    previewUrl: '',
    previewLoadMs: null
  }
}

const batchConcurrency = (count = images.value.length) => Math.max(1, Math.min(count, globalThis.navigator?.hardwareConcurrency || 4, 6))

const clearCachedImages = () => {
  for (const img of images.value) {
    revokeImagePreview(img)
  }
}

onBeforeUnmount(() => {
  window.removeEventListener('resize', syncViewport)
  invalidateProcessedPreview()
  clearCachedImages()
  resetProcessedState()
  clearToasts()
  stopSplitDrag()
})

onMounted(() => {
  syncViewport()
  window.addEventListener('resize', syncViewport)
})

const addImages = async () => {
  try {
    const files = await open({
      multiple: true,
      filters: [{ name: 'Images', extensions: ['jpg', 'jpeg', 'png', 'webp'] }]
    })

    if (files && files.length > 0) {
      for (const file of files) {
        images.value.push(createImageEntry(file))
      }
    }
  } catch (e) {
    message.error('选择文件失败: ' + normalizeErrorMessage(e))
  }
}

const selectImage = async (idx) => {
  invalidateProcessedPreview()
  selectedIndex.value = idx
  const currentRequestId = ++previewRequestId
  resetProcessedState()
  originalImageUrl.value = ''

  const img = images.value[idx]
  if (!img || !img.path) return
  queueProcessedPreviewRefresh()

  if (img.previewLoaded && img.previewUrl) {
    if (typeof img.previewLoadMs === 'number') {
      previewLoading.value = false
      originalPreviewLoadStartedAt = 0
    } else {
      previewLoading.value = true
      originalPreviewLoadStartedAt = performance.now()
    }
    originalImageUrl.value = img.previewUrl
    return
  }

  previewLoading.value = true
  originalPreviewLoadStartedAt = performance.now()
  img.previewLoadMs = null

  try {
    const result = await invoke('load_original_image', {
      path: img.path
    })

    images.value[idx].width = result.width
    images.value[idx].height = result.height
    images.value[idx].size = result.original_size

    const ext = img.name.split('.').pop()?.toLowerCase()
    const mimeType = ext === 'png'
      ? 'image/png'
      : ext === 'webp'
        ? 'image/webp'
        : ext === 'bmp'
          ? 'image/bmp'
          : 'image/jpeg'

    revokeImagePreview(images.value[idx])
    const blob = new Blob([new Uint8Array(result.image_data)], { type: mimeType })
    images.value[idx].previewUrl = URL.createObjectURL(blob)
    images.value[idx].previewLoaded = true

    if (currentRequestId === previewRequestId && selectedIndex.value === idx) {
      originalImageUrl.value = images.value[idx].previewUrl
    }
  } catch (e) {
    if (currentRequestId === previewRequestId && selectedIndex.value === idx) {
      previewLoading.value = false
      originalPreviewLoadStartedAt = 0
      message.error('加载预览失败: ' + normalizeErrorMessage(e))
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
    const result = await invoke('process_image', {
      path: targetImage.path,
      format: targetFormat,
      quality: targetQuality,
      scale: targetScale,
      webpLossless: targetWebpLossless
    })

    if (currentRequestId !== processRequestId || selectedImage.value?.path !== targetImage.path) {
      return
    }

    processedData.value = new Uint8Array(result.data)
    processedDimensions.value = { width: result.width, height: result.height }

    const mimeType = targetFormat === 'Jpg'
      ? 'image/jpeg'
      : targetFormat === 'WebP'
        ? 'image/webp'
        : 'image/png'
    const blob = new Blob([processedData.value], { type: mimeType })
    processedImageUrl.value = URL.createObjectURL(blob)

    images.value[selectedIndex.value].processed = true
    images.value[selectedIndex.value].error = null

    if (notifySuccess) {
      message.success('处理完成')
    }
  } catch (e) {
    if (currentRequestId !== processRequestId || selectedImage.value?.path !== targetImage.path) {
      return
    }
    processedPreviewLoading.value = false
    processedPreviewLoadStartedAt = 0
    message.error('处理失败: ' + normalizeErrorMessage(e))
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
  if (!processedData.value || !selectedImage.value) return

  saving.value = true
  try {
    const ext = outputFormat.value.toLowerCase()
    const suggestedName = selectedImage.value.name.replace(/\.[^.]+$/, '') + '_compress.' + ext

    const filePath = await save({
      defaultPath: suggestedName,
      filters: [{ name: ext.toUpperCase(), extensions: [ext] }]
    })

    if (filePath) {
      await writeFile(filePath, processedData.value)
      message.success('保存成功')
    }
  } catch (e) {
    message.error('保存失败: ' + normalizeErrorMessage(e))
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

    for (const img of batchImages) {
      img.processed = false
      img.error = null
      img.outputPath = ''
    }

    const worker = async () => {
      while (true) {
        const currentIndex = nextIndex++
        if (currentIndex >= batchImages.length) {
          return
        }

        const img = batchImages[currentIndex]

        try {
          const result = await invoke('process_and_save_image', {
            inputPath: img.path,
            outputDir,
            format: batchSettings.format,
            quality: batchSettings.quality,
            scale: batchSettings.scale,
            webpLossless: batchSettings.webpLossless
          })

          img.processed = true
          img.error = null
          img.outputPath = result.output_path
        } catch (e) {
          const errorMessage = normalizeErrorMessage(e)

          img.processed = false
          img.error = errorMessage
          img.outputPath = ''
          failures.push({ name: img.name, error: errorMessage })
        } finally {
          batchCompleted.value++
        }
      }
    }

    await Promise.all(Array.from({ length: concurrency }, () => worker()))

    if (failures.length > 0) {
      message.error(`批量处理完成，但有 ${failures.length} 个文件失败，请查看左侧列表。`)
    } else {
      message.success('批量处理完成')
    }
  } catch (e) {
    message.error('批量处理失败: ' + normalizeErrorMessage(e))
  } finally {
    batchProcessing.value = false
  }
}

const clearAll = () => {
  dialog.warning({
    title: '确认清空',
    content: '确定要清空所有图片吗？',
    positiveText: '确定',
    negativeText: '取消',
    onPositiveClick: () => {
      invalidateProcessedPreview()
      clearCachedImages()
      previewRequestId++
      previewLoading.value = false
      images.value = []
      selectedIndex.value = null
      originalImageUrl.value = ''
      resetProcessedState()
    }
  })
}

watch(
  [
    outputFormat,
    quality,
    scale,
    webpLossless
  ],
  () => {
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

.image-card--config {
  padding: 16px 18px;
}

.image-card--inner {
  padding: 8px;
  border-radius: 20px;
  background: rgba(255, 255, 255, 0.92);
}

.image-toolbar-card {
  flex-shrink: 0;
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

.image-toolbar__badge {
  display: inline-flex;
  align-items: center;
  width: fit-content;
  padding: 10px 14px;
  border: 1px solid rgba(249, 115, 22, 0.18);
  border-radius: 999px;
  background: linear-gradient(135deg, rgba(255, 247, 237, 0.95), rgba(255, 255, 255, 0.92));
  color: #c2410c;
  font-size: 0.82rem;
  font-weight: 700;
  letter-spacing: 0.08em;
}

.image-toolbar-actions {
  align-items: center;
}

.image-toolbar-divider {
  width: 1px;
  height: 2rem;
  background: rgba(148, 163, 184, 0.28);
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
.image-list-item__meta {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.image-list-item__name {
  width: 100%;
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

.config-panel {
  flex-shrink: 0;
}

.config-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 18px 24px;
  align-items: center;
}

.config-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  flex: 1 1 14rem;
  min-width: 0;
}

.image-control {
  flex: 1 1 min(100%, 12rem);
}

.image-control--slider {
  align-items: center;
}

.image-control__slider {
  flex: 1 1 min(100%, 7rem);
  width: 100%;
  min-width: 0;
  accent-color: #f97316;
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
  margin: 0.7rem 0 0;
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
  .image-card,
  .image-card--config {
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
