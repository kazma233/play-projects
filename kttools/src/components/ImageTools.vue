<template>
  <div class="tool-container">
    <n-flex justify="space-between" align="center" style="margin-bottom: 16px;">
      <n-text class="title">图片压缩转换</n-text>
      <n-flex align="center">
        <n-button type="info" @click="addImages">+ 添加图片</n-button>
        <n-button :disabled="images.length === 0 || batchProcessing" type="error" @click="clearAll">
          清空
        </n-button>
        <n-divider vertical></n-divider>
        <n-button :disabled="!processedData" type="primary" @click="saveImage" :loading="saving">
          保存单张
        </n-button>
        <n-button :disabled="images.length === 0 || batchProcessing" type="primary" @click="batchProcess"
          :loading="batchProcessing">
          批量保存 {{ batchProcessing ? `(${batchCompleted}/${batchTotal})` : '' }}
        </n-button>
      </n-flex>
    </n-flex>

    <div class="main-layout">
      <!-- Sidebar -->
      <n-card class="sidebar" :bordered="false"
        content-style="padding: 5px; display: flex; flex-direction: column; height: 100%;">
        <n-scrollbar v-if="images.length > 0" style="flex: 1; min-height: 0;">
          <n-list hoverable clickable>
            <n-list-item v-for="(img, idx) in images" :key="idx" size="small" :class="{ active: selectedIndex === idx }"
              @click="selectImage(idx)">
              {{ img.name }}
            </n-list-item>
          </n-list>
        </n-scrollbar>
        <n-empty v-else description="点击右上角按钮添加图片" style="flex: 1;" />
      </n-card>

      <!-- Main Content -->
      <div class="main-content">
        <!-- Config Panel -->
        <n-card class="config-panel" :bordered="false" content-style="padding: 12px 16px;">
          <n-flex wrap :gap="24" align="center">
            <n-flex align="center" :gap="8">
              <n-text depth="3" class="label">格式</n-text>
              <n-button-group>
                <n-button :type="outputFormat === 'Jpg' ? 'primary' : 'default'" @click="outputFormat = 'Jpg'"
                  size="small">JPG</n-button>
                <n-button :type="outputFormat === 'Png' ? 'primary' : 'default'" @click="outputFormat = 'Png'"
                  size="small">PNG</n-button>
                <n-button :type="outputFormat === 'WebP' ? 'primary' : 'default'" @click="outputFormat = 'WebP'"
                  size="small">WebP</n-button>
              </n-button-group>
            </n-flex>

            <n-flex v-if="outputFormat !== 'WebP'" align="center" :gap="8" style="min-width: 150px;">
              <n-text depth="3" class="label">质量 {{ quality }}%</n-text>
              <n-slider v-model:value="quality" :min="1" :max="100" :step="1" style="width: 100px;" />
            </n-flex>
            <n-text v-else depth="3" class="label">WebP 无损编码</n-text>

            <n-flex align="center" :gap="8" style="min-width: 150px;">
              <n-text depth="3" class="label">缩放 {{ scale }}%</n-text>
              <n-slider v-model:value="scale" :min="1" :max="100" :step="1" :format-tooltip="getScaledSize"
                style="width: 100px;" />
            </n-flex>
          </n-flex>
          <n-text depth="3" style="font-size: 12px; margin-top: 8px; display: block;">* 输出图片不含元数据</n-text>
        </n-card>

        <!-- Preview Area -->
        <n-card v-if="selectedImage" class="preview-area" :bordered="false"
          content-style="padding: 5px; display: flex; flex-direction: column;">
          <n-flex align="center">
            <n-text strong style="font-size: 15px;">{{ selectedImage.name }}</n-text>
          </n-flex>

          <n-split direction="horizontal" :default-size="0.5">
            <template #1>
              <n-card class="image-panel" :bordered="false"
                content-style="padding: 5px; display: flex; flex-direction: column; height: 100%;">
                <n-flex justify="space-between" align="center" class="panel-header">
                  <n-text depth="3">原图</n-text>
                  <n-text depth="3">{{ selectedImage.width }} x {{ selectedImage.height }}</n-text>
                </n-flex>
                <div class="image-viewer">
                  <n-image v-if="originalImageUrl" :src="originalImageUrl" alt="原图" object-fit="contain" width="100%"
                    height="100%" />
                  <n-spin v-else-if="previewLoading" />
                  <n-text v-else depth="3">加载中...</n-text>
                </div>
              </n-card>
            </template>
            <template #2>
              <n-card class="image-panel" :bordered="false"
                content-style="padding: 5px; display: flex; flex-direction: column; height: 100%;">
                <n-flex justify="space-between" align="center" class="panel-header">
                  <n-text depth="3">处理后</n-text>
                  <n-text depth="3" style="font-size: 13px;">{{ compressionInfo }}</n-text>
                </n-flex>
                <div class="image-viewer">
                  <n-image v-if="processedImageUrl" :src="processedImageUrl" alt="处理后" object-fit="contain" width="100%"
                    height="100%" />
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

        <n-card v-else class="empty-preview" :bordered="false">
          <n-empty description="从左侧列表中选择一张图片" />
        </n-card>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { NButton, NButtonGroup, NSlider, NSpin, NEmpty, NText, NFlex, NSpace, NScrollbar, NEllipsis, NImage, NCard, NSplit, useMessage, useDialog } from 'naive-ui'
import { invoke } from "@tauri-apps/api/core"
import { open, save } from '@tauri-apps/plugin-dialog'
import { writeFile } from '@tauri-apps/plugin-fs'

const message = useMessage()
const dialog = useDialog()

const images = ref([])
const selectedIndex = ref(null)
const outputFormat = ref('Jpg')
const quality = ref(85)
const scale = ref(100)

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

const addImages = async () => {
  try {
    const files = await open({
      multiple: true,
      filters: [{ name: 'Images', extensions: ['jpg', 'jpeg', 'png', 'webp'] }]
    })

    if (files && files.length > 0) {
      for (const file of files) {
        const fileName = file.split(/[/\\]/).pop() || file
        images.value.push({
          path: file,
          name: fileName,
          size: 0,
          width: 0,
          height: 0,
          processed: false,
          error: null
        })
      }
    }
  } catch (e) {
    message.error('选择文件失败: ' + e)
  }
}

const selectImage = async (idx) => {
  selectedIndex.value = idx
  processedData.value = null
  processedDimensions.value = null
  processedImageUrl.value = ''
  originalImageUrl.value = ''

  const img = images.value[idx]
  if (img && img.path) {
    previewLoading.value = true
    try {
      const result = await invoke('load_original_image', {
        path: img.path
      })

      images.value[idx].width = result.width
      images.value[idx].height = result.height
      images.value[idx].size = result.original_size

      // 直接显示原图
      const ext = img.name.split('.').pop()?.toLowerCase()
      const mimeType = ext === 'png' ? 'image/png' :
        ext === 'webp' ? 'image/webp' :
          ext === 'bmp' ? 'image/bmp' : 'image/jpeg'
      const blob = new Blob([new Uint8Array(result.image_data)], { type: mimeType })
      originalImageUrl.value = URL.createObjectURL(blob)
    } catch (e) {
      message.error('加载预览失败: ' + e)
    } finally {
      previewLoading.value = false
    }
  }
}

const processImage = async () => {
  if (!selectedImage.value) return

  processing.value = true
  processedData.value = null

  try {
    const result = await invoke('process_image', {
      path: selectedImage.value.path,
      format: outputFormat.value,
      quality: quality.value,
      scale: scale.value
    })

    processedData.value = new Uint8Array(result.data)
    processedDimensions.value = { width: result.width, height: result.height }

    const mimeType = outputFormat.value === 'Jpg' ? 'image/jpeg' :
      outputFormat.value === 'WebP' ? 'image/webp' : 'image/png'
    const blob = new Blob([processedData.value], { type: mimeType })
    processedImageUrl.value = URL.createObjectURL(blob)

    images.value[selectedIndex.value].processed = true
    message.success('处理完成')
  } catch (e) {
    message.error('处理失败: ' + e)
  } finally {
    processing.value = false
  }
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

    for (let i = 0; i < images.value.length; i++) {
      const img = images.value[i]
      try {
        const result = await invoke('process_and_save_image', {
          inputPath: img.path,
          outputDir: outputDir,
          format: outputFormat.value,
          quality: quality.value,
          scale: scale.value
        })

        images.value[i].processed = true
        images.value[i].outputPath = result[0]
      } catch (e) {
        images.value[i].error = e
      }
      batchCompleted.value++
    }

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
      images.value = []
      selectedIndex.value = null
      originalImageUrl.value = ''
      processedImageUrl.value = ''
      processedData.value = null
    }
  })
}
</script>

<style scoped>
/* ========== 容器布局 ========== */
.tool-container {
  height: 100%;
  display: flex;
  flex-direction: column;
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
}

/* ========== 侧边栏样式 ========== */
.sidebar {
  width: 280px;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

/* ========== 主内容区域 ========== */
.main-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-width: 0;
}

.config-panel {
  flex-shrink: 0;
}

.label {
  font-size: 13px;
  white-space: nowrap;
}

/* ========== 图片预览区域 ========== */
.preview-area {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  height: 100%;
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
}

/* ========== 图片查看器 ========== */
.image-viewer {
  flex: 1;
  min-height: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #fafafa;
  border-radius: 4px;
}

/* ========== 空状态 ========== */
.empty-preview {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
}
</style>
