<template>
  <div class="page-view image-page">
    <n-card class="tool-surface image-toolbar-card" :bordered="false">
      <n-flex justify="space-between" align="center" class="image-toolbar" wrap>
        <div class="tool-panel__title">
          <strong>图片压缩转换</strong>
          <span class="tool-panel__meta">批量处理图片并对比压缩效果，适合静态资源整理和交付前清洗。</span>
        </div>

        <n-flex align="center" class="tool-inline-actions image-toolbar-actions">
          <n-button class="image-toolbar-button" type="info" @click="addImages">+ 添加图片</n-button>
          <n-button class="image-toolbar-button" :disabled="images.length === 0 || batchProcessing" type="error" @click="clearAll">
            清空
          </n-button>
          <n-divider class="image-toolbar-divider" vertical></n-divider>
          <n-button class="image-toolbar-button" :disabled="!processedData" type="primary" @click="saveImage" :loading="saving">
            保存单张
          </n-button>
          <n-button class="image-toolbar-button" :disabled="images.length === 0 || batchProcessing" type="primary" @click="batchProcess"
            :loading="batchProcessing">
            批量保存 {{ batchProcessing ? `(${batchCompleted}/${batchTotal})` : '' }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-card>

    <div class="main-layout">
      <!-- Sidebar -->
        <n-card
          class="sidebar tool-surface"
          :bordered="false"
          content-class="image-card-content image-card-content--tight image-card-content--column image-card-content--full"
        >
        <n-scrollbar v-if="images.length > 0" class="image-sidebar-scroll">
          <n-list hoverable clickable class="image-list">
            <n-list-item v-for="(img, idx) in images" :key="idx" size="small" class="image-list-item" :class="{ active: selectedIndex === idx }"
              @click="selectImage(idx)">
              {{ img.name }}
            </n-list-item>
          </n-list>
        </n-scrollbar>
        <n-empty v-else description="点击右上角按钮添加图片" class="image-empty-state" />
      </n-card>

      <!-- Main Content -->
      <div class="main-content">
        <!-- Config Panel -->
        <n-card class="config-panel tool-surface" :bordered="false" content-class="image-card-content image-card-content--config">
            <n-flex wrap :gap="24" align="center">
              <n-flex align="center" :gap="8">
                <n-text depth="3" class="label">格式</n-text>
              <n-button-group class="image-button-group">
                <n-button class="image-group-button" :type="outputFormat === 'Jpg' ? 'primary' : 'default'" @click="outputFormat = 'Jpg'"
                  size="small">JPG</n-button>
                <n-button class="image-group-button" :type="outputFormat === 'Png' ? 'primary' : 'default'" @click="outputFormat = 'Png'"
                  size="small">PNG</n-button>
                <n-button class="image-group-button" :type="outputFormat === 'WebP' ? 'primary' : 'default'" @click="outputFormat = 'WebP'"
                  size="small">WebP</n-button>
              </n-button-group>
            </n-flex>

            <n-flex v-if="outputFormat === 'WebP'" align="center" :gap="8">
              <n-text depth="3" class="label">模式</n-text>
              <n-button-group class="image-button-group">
                <n-button class="image-group-button" :type="webpLossless ? 'default' : 'primary'" @click="webpLossless = false" size="small">有损</n-button>
                <n-button class="image-group-button" :type="webpLossless ? 'primary' : 'default'" @click="webpLossless = true" size="small">无损</n-button>
              </n-button-group>
            </n-flex>

            <n-flex v-if="showQualitySlider" align="center" :gap="8" class="image-control image-control--slider">
              <n-text depth="3" class="label">{{ qualityLabel }} {{ quality }}%</n-text>
              <n-slider v-model:value="quality" :min="1" :max="100" :step="1" class="image-control__slider" />
            </n-flex>

            <n-flex v-if="showQualitySlider" align="center" :gap="8">
              <n-text depth="3" class="label">预设</n-text>
              <n-button-group class="image-button-group">
                <n-button
                  v-for="preset in qualityPresets"
                  :key="preset.label"
                  class="image-group-button"
                  :type="quality === preset.value ? 'primary' : 'default'"
                  @click="applyQualityPreset(preset.value)"
                  size="small"
                >
                  {{ preset.label }}
                </n-button>
              </n-button-group>
            </n-flex>

            <n-flex align="center" :gap="8" class="image-control image-control--slider">
              <n-text depth="3" class="label">缩放 {{ scale }}%</n-text>
              <n-slider v-model:value="scale" :min="1" :max="100" :step="1" :format-tooltip="getScaledSize"
                class="image-control__slider" />
            </n-flex>
          </n-flex>
          <n-text depth="3" class="image-format-hint">{{ formatHint }}</n-text>
        </n-card>

        <!-- Preview Area -->
        <n-card
          v-if="selectedImage"
          class="preview-area tool-surface"
          :bordered="false"
          content-class="image-card-content image-card-content--tight image-card-content--column"
        >
          <n-flex align="center">
            <n-text strong class="image-preview-title">{{ selectedImage.name }}</n-text>
          </n-flex>

          <n-split class="image-compare-split" :direction="stackedPreview ? 'vertical' : 'horizontal'" :default-size="0.5">
            <template #1>
              <n-card
                class="image-panel tool-surface"
                :bordered="false"
                content-class="image-card-content image-card-content--tight image-card-content--column image-card-content--full"
              >
                <n-flex justify="space-between" align="center" class="panel-header">
                  <n-text depth="3">原图</n-text>
                  <n-text depth="3">{{ selectedImage.width }} x {{ selectedImage.height }}</n-text>
                </n-flex>
                <div class="image-viewer">
                  <n-image
                    v-if="originalImageUrl"
                    class="image-viewer__media"
                    :img-props="{ class: 'image-viewer__asset' }"
                    :src="originalImageUrl"
                    alt="原图"
                    object-fit="contain"
                  />
                  <n-spin v-else-if="previewLoading" />
                  <n-text v-else depth="3">加载中...</n-text>
                </div>
              </n-card>
            </template>
            <template #2>
              <n-card
                class="image-panel tool-surface"
                :bordered="false"
                content-class="image-card-content image-card-content--tight image-card-content--column image-card-content--full"
              >
                <n-flex justify="space-between" align="center" class="panel-header">
                  <n-text depth="3">处理后</n-text>
                  <n-text depth="3" class="image-compression-meta">{{ compressionInfo }}</n-text>
                </n-flex>
                <div class="image-viewer">
                  <n-image
                    v-if="processedImageUrl"
                    class="image-viewer__media"
                    :img-props="{ class: 'image-viewer__asset' }"
                    :src="processedImageUrl"
                    alt="处理后"
                    object-fit="contain"
                  />
                  <n-flex v-else-if="processing" vertical align="center" :gap="8">
                    <n-spin />
                    <n-text depth="3">处理中...</n-text>
                  </n-flex>

                  <n-button v-else :disabled="!selectedImage || processing" @click="processImage" :loading="processing">
                    处理图片
                  </n-button>
                </div>
              </n-card>
            </template>
          </n-split>
        </n-card>

        <n-card v-else class="empty-preview tool-surface" :bordered="false">
          <n-empty description="从左侧列表中选择一张图片" />
        </n-card>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { NButton, NButtonGroup, NSlider, NSpin, NEmpty, NText, NFlex, NSpace, NScrollbar, NEllipsis, NImage, NCard, NSplit, useMessage, useDialog } from 'naive-ui'
import { invoke } from "@tauri-apps/api/core"
import { open, save } from '@tauri-apps/plugin-dialog'
import { writeFile } from '@tauri-apps/plugin-fs'

const message = useMessage()
const dialog = useDialog()

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
let previewRequestId = 0
let processRequestId = 0
let autoProcessTimer = null
const viewportWidth = ref(0)
const AUTO_PROCESS_DELAY = 180

const stackedPreview = computed(() => viewportWidth.value <= 1180)

const syncViewport = () => {
  viewportWidth.value = window.innerWidth
}

const selectedImage = computed(() => {
  if (selectedIndex.value !== null && selectedIndex.value < images.value.length) {
    return images.value[selectedIndex.value]
  }
  return null
})

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

const applyQualityPreset = (value) => {
  quality.value = value
}

const revokeImagePreview = (img) => {
  if (!img?.previewUrl) return
  revokeObjectUrl(img.previewUrl)
  img.previewUrl = ''
  img.previewLoaded = false
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
}

const resetProcessedState = () => {
  processedData.value = null
  processedDimensions.value = null
  revokeObjectUrl(processedImageUrl.value)
  processedImageUrl.value = ''
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
    previewUrl: ''
  }
}

const batchConcurrency = () => Math.max(1, Math.min(images.value.length, globalThis.navigator?.hardwareConcurrency || 4, 6))

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
    message.error('选择文件失败: ' + e)
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
    previewLoading.value = false
    originalImageUrl.value = img.previewUrl
    return
  }

  previewLoading.value = true

  try {
    const result = await invoke('load_original_image', {
      path: img.path
    })

    images.value[idx].width = result.width
    images.value[idx].height = result.height
    images.value[idx].size = result.original_size

    revokeImagePreview(images.value[idx])
    const blob = new Blob([new Uint8Array(result.preview_data)], { type: result.preview_mime_type })
    images.value[idx].previewUrl = URL.createObjectURL(blob)
    images.value[idx].previewLoaded = true

    if (currentRequestId === previewRequestId && selectedIndex.value === idx) {
      originalImageUrl.value = images.value[idx].previewUrl
    }
  } catch (e) {
    if (currentRequestId === previewRequestId && selectedIndex.value === idx) {
      message.error('加载预览失败: ' + e)
    }
  } finally {
    if (currentRequestId === previewRequestId && selectedIndex.value === idx) {
      previewLoading.value = false
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

    const mimeType = targetFormat === 'Jpg' ? 'image/jpeg' :
      targetFormat === 'WebP' ? 'image/webp' : 'image/png'
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
    message.error('处理失败: ' + e)
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
    message.error('保存失败: ' + e)
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

    batchProcessing.value = true
    batchCompleted.value = 0
    batchTotal.value = images.value.length
    const concurrency = batchConcurrency()
    let nextIndex = 0

    const worker = async () => {
      while (true) {
        const currentIndex = nextIndex++
        if (currentIndex >= images.value.length) {
          return
        }

        const img = images.value[currentIndex]

        try {
          const result = await invoke('process_and_save_image', {
            inputPath: img.path,
            outputDir: outputDir,
            format: outputFormat.value,
            quality: quality.value,
            scale: scale.value,
            webpLossless: webpLossless.value
          })

          images.value[currentIndex].processed = true
          images.value[currentIndex].error = null
          images.value[currentIndex].outputPath = result.output_path
        } catch (e) {
          images.value[currentIndex].error = e
        } finally {
          batchCompleted.value++
        }
      }
    }

    await Promise.all(Array.from({ length: concurrency }, () => worker()))

    message.success('批量处理完成')
  } catch (e) {
    message.error('批量处理失败: ' + e)
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
:global(.image-card-content) {
  min-height: 0;
}

:global(.image-card-content--tight) {
  padding: 5px;
}

:global(.image-card-content--column) {
  display: flex;
  flex-direction: column;
}

:global(.image-card-content--full) {
  height: 100%;
}

:global(.image-card-content--config) {
  padding: 16px 18px;
}

/* ========== 容器布局 ========== */
.tool-container {
  height: 100%;
  display: flex;
  flex-direction: column;
}

.image-page {
  min-height: 0;
  overflow: auto;
}

.image-toolbar-card {
  flex-shrink: 0;
}

.image-toolbar {
  gap: 16px;
}

.image-toolbar-actions {
  align-items: center;
}

.image-control {
  flex: 1 1 min(100%, 12rem);
  min-width: 0;
  flex-wrap: wrap;
}

.image-control--slider {
  align-items: center;
}

.image-control__slider {
  flex: 1 1 min(100%, 7rem);
  min-width: 0;
}

.title {
  font-size: 18px;
  font-weight: 500;
}

.main-layout {
  flex: 1;
  display: flex;
  gap: 12px;
  min-height: 0;
  align-items: stretch;
  min-block-size: clamp(26rem, 62vh, 40rem);
}

/* ========== 侧边栏样式 ========== */
.sidebar {
  width: clamp(15rem, 24vw, 17.5rem);
  flex: 0 0 clamp(15rem, 24vw, 17.5rem);
  max-width: 100%;
  display: flex;
  flex-direction: column;
  min-height: 100%;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.76), rgba(243, 210, 193, 0.24));
}

.image-sidebar-scroll,
.image-empty-state {
  flex: 1;
  min-height: 0;
}

.image-list-item {
  border-radius: 16px;
  margin: 4px 6px;
  transition: background 180ms ease, transform 180ms ease;
}

.image-list-item:hover {
  background: rgba(139, 211, 221, 0.16);
  transform: translateX(2px);
}

.image-list-item.active {
  background: rgba(245, 130, 174, 0.14);
  color: var(--text-strong);
  font-weight: 700;
}

/* ========== 主内容区域 ========== */
.main-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-width: 0;
  min-height: 100%;
}

.config-panel {
  flex-shrink: 0;
}

.image-button-group {
  display: inline-flex;
  flex-wrap: wrap;
  max-width: 100%;
}

.image-group-button {
  min-width: clamp(3rem, 7vw, 3.625rem);
}

.label {
  font-size: 13px;
  white-space: nowrap;
}

.image-format-hint {
  display: block;
  margin-top: 0.5rem;
  font-size: 0.8rem;
}

/* ========== 图片预览区域 ========== */
.preview-area {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  height: 100%;
  min-block-size: clamp(16rem, 38vh, 26rem);
}

.image-compare-split {
  flex: 1;
  min-height: clamp(20rem, 52vh, 34rem);
}

.image-preview-title {
  font-size: clamp(0.95rem, 2.2vw, 1rem);
}

/* ========== 图片面板 ========== */
.image-panel {
  height: 100%;
  display: flex;
  flex-direction: column;
  box-sizing: border-box;
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
  flex-shrink: 0;
  flex-wrap: wrap;
}

.image-compression-meta {
  font-size: clamp(0.75rem, 1.8vw, 0.8125rem);
}

/* ========== 图片查看器 ========== */
.image-viewer {
  flex: 1;
  min-height: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: clamp(0.75rem, 1.6vw, 1rem);
  background: linear-gradient(145deg, rgba(243, 210, 193, 0.22), rgba(255, 255, 255, 0.82));
  border-radius: 18px;
  overflow: hidden;
}

.image-viewer :deep(.image-viewer__media) {
  display: flex;
  align-items: center;
  justify-content: center;
  inline-size: 100%;
  block-size: 100%;
  min-inline-size: 0;
  min-block-size: 0;
}

.image-viewer :deep(.image-viewer__asset) {
  display: block;
  inline-size: 100%;
  block-size: 100%;
  max-inline-size: 100%;
  max-block-size: 100%;
  object-fit: contain;
}

/* ========== 空状态 ========== */
.empty-preview {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  min-block-size: clamp(16rem, 38vh, 26rem);
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

  .image-control--slider {
    align-items: stretch;
  }

  .image-control__slider {
    flex-basis: 100%;
  }
}
</style>
