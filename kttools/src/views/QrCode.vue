<template>
  <div class="page-view page-view--centered">
    <n-card class="tool-surface hover-lift" size="small" :bordered="false">
      <div class="tool-grid tool-grid--two">
        <div class="tool-stack">
          <div class="tool-panel__title">
            <strong>二维码设置</strong>
            <span class="tool-panel__meta">自定义深浅色与尺寸，适合快速生成分享链接、登录页码和临时文本二维码。</span>
          </div>

          <div class="tool-inline-actions qr-controls-row">
            <div class="qr-control-item">
              <span class="tool-subtitle">深色</span>
              <n-color-picker v-model:value="darkColor" :modes="['hex']" class="tool-field" />
            </div>
            <div class="qr-control-item">
              <span class="tool-subtitle">浅色</span>
              <n-color-picker v-model:value="lightColor" :modes="['hex']" class="tool-field" />
            </div>
            <div class="qr-control-item">
              <span class="tool-subtitle">尺寸</span>
              <n-input-number
                v-model:value="size"
                :min="qrSizeBounds.min"
                :max="qrSizeBounds.max"
                :step="10"
                class="tool-field"
              />
            </div>
          </div>

          <n-input
            v-model:value="inputText"
            type="textarea"
            class="tool-code-input"
            placeholder="输入要生成二维码的文本..."
            clearable
            :rows="6"
          />

          <div class="tool-inline-actions">
            <n-button type="primary" size="large" :loading="loading" :disabled="!inputText.trim()" @click="generate">
              生成二维码
            </n-button>
            <span class="tool-note">当前生成结果会同步更新到右侧预览舞台。</span>
          </div>
        </div>

        <div class="tool-stack">
          <div class="tool-panel__header">
            <div class="tool-panel__title">
              <strong>预览与导出</strong>
              <span class="tool-panel__meta">支持直接保存为 PNG 或复制到剪贴板。</span>
            </div>

            <div v-if="qrCodeImage" class="tool-inline-actions qr-action-row">
              <n-button class="qr-action-button" type="primary" ghost @click="saveQRCode">
                <template #icon>
                  <n-icon><download-outline /></n-icon>
                </template>
                保存图片
              </n-button>
              <n-button class="qr-action-button" ghost @click="copyQRCode">
                <template #icon>
                  <n-icon><copy-outline /></n-icon>
                </template>
                复制
              </n-button>
            </div>
          </div>

          <div class="qr-preview-stage tool-fill-area">
            <div v-if="qrCodeImage" ref="qrImageRef" v-html="qrCodeImage" class="qr-preview-frame"></div>
            <n-empty v-else description="输入文本后点击生成二维码">
              <template #icon>
                <span class="qr-empty-icon">📱</span>
              </template>
            </n-empty>
          </div>
        </div>
      </div>
    </n-card>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { invoke } from '@tauri-apps/api/core'
import { save } from '@tauri-apps/plugin-dialog'
import { writeFile } from '@tauri-apps/plugin-fs'
import { useMessage } from 'naive-ui'
import { CopyOutline, DownloadOutline } from '@vicons/ionicons5'

const message = useMessage()

const inputText = ref('')
const qrCodeImage = ref('')
const loading = ref(false)
const size = ref(256)
const darkColor = ref('#001858')
const lightColor = ref('#fef6e4')
const qrImageRef = ref(null)
const viewportWidth = ref(0)

const qrSizeBounds = computed(() => {
  const width = viewportWidth.value || 520

  return {
    min: Math.max(120, Math.round(width * 0.24)),
    max: Math.max(220, Math.min(480, Math.round(width * 0.74)))
  }
})

const syncViewport = () => {
  viewportWidth.value = window.innerWidth
}

watch(qrSizeBounds, ({ min, max }) => {
  size.value = Math.min(max, Math.max(min, size.value))
}, { immediate: true })

onMounted(() => {
  syncViewport()
  window.addEventListener('resize', syncViewport)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', syncViewport)
})

const generate = async () => {
  if (!inputText.value.trim()) {
    return
  }

  loading.value = true
  try {
    qrCodeImage.value = await invoke('generate_qr_code', {
      content: inputText.value.trim(),
      size: size.value,
      darkColor: darkColor.value,
      lightColor: lightColor.value
    })
  } catch (err) {
    message.error('生成失败: ' + err)
  } finally {
    loading.value = false
  }
}

const saveQRCode = async () => {
  if (!inputText.value.trim()) {
    message.error('请输入内容')
    return
  }

  try {
    const filePath = await save({
      filters: [{ name: 'PNG Image', extensions: ['png'] }],
      defaultPath: `qrcode-${Date.now()}.png`
    })

    if (!filePath) {
      return
    }

    const pngData = await invoke('generate_qr_code_png', {
      content: inputText.value.trim(),
      size: size.value,
      darkColor: darkColor.value,
      lightColor: lightColor.value
    })

    await writeFile(filePath, new Uint8Array(pngData))
    message.success('二维码已保存')
  } catch (err) {
    console.error('Save error:', err)
    message.error('保存失败: ' + (err.message || err))
  }
}

const copyQRCode = async () => {
  if (!inputText.value.trim()) {
    message.error('请输入内容')
    return
  }

  message.loading('正在复制...', { duration: 5000 })

  try {
    await invoke('copy_qr_code_to_clipboard', {
      content: inputText.value.trim(),
      size: size.value,
      darkColor: darkColor.value,
      lightColor: lightColor.value
    })
    message.destroyAll()
    message.success('已复制到剪贴板')
  } catch (err) {
    console.error('复制错误:', err)
    message.destroyAll()
    message.error('复制失败: ' + (err.message || err))
  }
}
</script>

<style scoped>
.qr-controls-row {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 6.75rem), 1fr));
  gap: clamp(0.75rem, 1.5vw, 1rem);
  padding: clamp(0.875rem, 1.8vw, 1rem);
  border-radius: clamp(1rem, 2.6vw, 1.375rem);
  background: rgba(255, 255, 255, 0.54);
}

.qr-control-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}

.qr-preview-stage {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: clamp(14rem, 40vh, 20rem);
  border-radius: clamp(1.25rem, 3vw, 1.75rem);
  background: linear-gradient(145deg, rgba(243, 210, 193, 0.28), rgba(255, 255, 255, 0.78));
}

.qr-action-row {
  justify-content: flex-end;
}

.qr-preview-frame {
  width: min(100%, 100%);
  max-width: 100%;
  padding: clamp(0.875rem, 2vw, 1.125rem);
  border-radius: clamp(1rem, 2.6vw, 1.5rem);
  background: rgba(255, 255, 255, 0.94);
  box-shadow: 0 18px 38px rgba(0, 24, 88, 0.08);
}

.qr-empty-icon {
  font-size: clamp(2.25rem, 8vw, 3rem);
}

@media (max-width: 640px) {
  .qr-action-row {
    width: 100%;
    justify-content: stretch;
  }

  .qr-action-button {
    flex: 1 1 calc(50% - 0.5rem);
  }
}
</style>
