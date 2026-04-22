<template>
  <div class="page-view page-view--centered">
    <n-card class="tool-surface" size="small">
      <div class="tool-grid tool-grid--two">
        <div class="tool-stack">
          <div class="tool-panel__title">
            <strong>文件选择</strong>
            <span class="tool-panel__meta">选择本地文件后生成 SHA1，适合发布前做产物校验或与远端哈希值对比。</span>
          </div>

          <div class="tool-inline-actions sha1-actions">
            <n-input v-model:value="filePath" readonly placeholder="选择文件..." class="tool-field tool-field--grow sha1-path-input" />
            <n-button type="primary" ghost @click="selectFile">浏览文件</n-button>
          </div>

          <div class="tool-chip-row">
            <span class="tool-chip">单文件校验</span>
            <span class="tool-chip">结果可复制</span>
          </div>

          <n-button type="primary" size="large" @click="calculate" :loading="loading" :disabled="!filePath">
            计算 SHA1
          </n-button>
        </div>

        <div class="tool-stack">
          <div class="tool-panel__header">
            <div class="tool-panel__title">
              <strong>SHA1 结果</strong>
              <span class="tool-panel__meta">当前结果会以多行输入框展示，方便全选或复制。</span>
            </div>
            <n-button v-if="sha1Hash" text type="primary" @click="copyResult">复制结果</n-button>
          </div>

          <n-input
            v-if="sha1Hash"
            v-model:value="sha1Hash"
            class="tool-code-output sha1-result-output"
            readonly
            type="textarea"
            :rows="4"
          />
          <div v-else class="tool-result-stage">
            <n-empty description="选择文件后点击计算按钮" />
          </div>
        </div>
      </div>
    </n-card>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { invoke } from '@tauri-apps/api/core'
import { open } from '@tauri-apps/plugin-dialog'
import { useMessage } from 'naive-ui'

const message = useMessage()

const filePath = ref('')
const sha1Hash = ref('')
const loading = ref(false)

const selectFile = async () => {
  try {
    const selected = await open({ multiple: false })
    if (selected) {
      filePath.value = selected
      sha1Hash.value = ''
    }
  } catch {
    message.error('文件选择失败')
  }
}

const calculate = async () => {
  if (!filePath.value) {
    message.warning('请先选择文件')
    return
  }

  loading.value = true
  try {
    sha1Hash.value = await invoke('sha1_encode', { filePath: filePath.value })
  } catch {
    message.error('SHA1 计算失败')
  } finally {
    loading.value = false
  }
}

const copyResult = async () => {
  await navigator.clipboard.writeText(sha1Hash.value)
  message.success('已复制到剪贴板')
}
</script>

<style scoped>
.sha1-actions {
  align-items: stretch;
}

.sha1-path-input {
  width: 100%;
}

.sha1-result-output {
  --tool-code-font-size: 0.95rem;
  --tool-code-text-align: left;
}
</style>
