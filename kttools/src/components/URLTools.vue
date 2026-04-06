<template>
  <div class="page-view code-duplex-page">
    <n-card class="tool-surface tool-surface--fill" size="small" :bordered="false">
      <n-split class="tool-split tool-split-frame" direction="vertical" :default-size="0.5">
        <template #1>
          <div class="tool-panel">
            <div class="tool-panel__header">
              <div class="tool-panel__title">
                <strong>原始链接 / 文本</strong>
                <span class="tool-panel__meta">适合快速清洗查询参数、路径片段和接口中被转义的字段。</span>
              </div>
            </div>

            <n-input
              v-model:value="urlArgs.encode"
              class="tool-code-input tool-fill-area"
              type="textarea"
              placeholder="输入要编码的内容..."
              @update:value="encode"
            />
          </div>
        </template>

        <template #2>
          <div class="tool-panel tool-panel--accent">
            <div class="tool-panel__header">
              <div class="tool-panel__title">
                <strong>URL 编码结果</strong>
                <span class="tool-panel__meta">支持直接在结果面板中反向解码。</span>
              </div>
              <n-button ghost type="primary" @click="swap">⇅ 交换内容</n-button>
            </div>

            <n-input
              v-model:value="urlArgs.decode"
              class="tool-code-input tool-fill-area"
              type="textarea"
              placeholder="输入要解码的内容..."
              @update:value="decode"
            />
          </div>
        </template>
      </n-split>
    </n-card>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { invoke } from '@tauri-apps/api/core'
import { useMessage } from 'naive-ui'

const message = useMessage()

const urlArgs = ref({ encode: '', decode: '' })

const encode = async (input) => {
  if (!input) {
    urlArgs.value.decode = ''
    return
  }

  try {
    urlArgs.value.decode = await invoke('url_encode', { input })
  } catch {
    message.error('编码失败')
  }
}

const decode = async (input) => {
  if (!input) {
    urlArgs.value.encode = ''
    return
  }

  try {
    urlArgs.value.encode = await invoke('url_decode', { input })
  } catch {
    message.error('解码失败')
  }
}

const swap = () => {
  const temp = urlArgs.value.encode
  urlArgs.value.encode = urlArgs.value.decode
  urlArgs.value.decode = temp
}
</script>

<style scoped>
.code-duplex-page {
  --tool-textarea-min-height: clamp(7.5rem, 20vh, 10rem);
}
</style>
