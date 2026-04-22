<template>
  <div class="page-view code-duplex-page">
    <n-card class="tool-surface tool-surface--fill" size="small">
      <n-split class="tool-split tool-split-frame" direction="vertical" :default-size="0.5">
        <template #1>
          <div class="tool-panel">
            <div class="tool-panel__header">
              <div class="tool-panel__title">
                <strong>原始值</strong>
                <span class="tool-panel__meta">输入后自动生成 Base64 结果，适合粘贴请求体、token 或签名原文。</span>
              </div>
              <n-checkbox v-model:checked="base64Args.urlMode">URL 安全模式</n-checkbox>
            </div>

            <n-input
              v-model:value="base64Args.encode"
              class="tool-code-input tool-fill-area"
              type="textarea"
              placeholder="输入要编码的内容..."
              @update:value="encode"
            />
          </div>
        </template>

        <template #2>
          <div class="tool-panel">
            <div class="tool-panel__header">
              <div class="tool-panel__title">
                <strong>Base64 编码值</strong>
                <span class="tool-panel__meta">支持直接反向编辑和双向校验。</span>
              </div>
              <n-button ghost type="primary" @click="swap">⇅ 交换内容</n-button>
            </div>

            <n-input
              v-model:value="base64Args.decode"
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
import { ref, watch } from 'vue'
import { invoke } from '@tauri-apps/api/core'
import { useMessage } from 'naive-ui'

const message = useMessage()

const base64Args = ref({
  encode: '',
  decode: '',
  urlMode: false
})

const encode = async (input) => {
  if (!input) {
    base64Args.value.decode = ''
    return
  }

  try {
    base64Args.value.decode = await invoke('base64_encode', {
      input,
      urlMode: base64Args.value.urlMode
    })
  } catch {
    message.error('编码失败')
  }
}

const decode = async (input) => {
  if (!input) {
    base64Args.value.encode = ''
    return
  }

  try {
    base64Args.value.encode = await invoke('base64_decode', { input })
  } catch {
    message.error('解码失败')
  }
}

const swap = () => {
  const temp = base64Args.value.encode
  base64Args.value.encode = base64Args.value.decode
  base64Args.value.decode = temp
}

watch(
  () => base64Args.value.urlMode,
  () => {
    encode(base64Args.value.encode)
  }
)
</script>

<style scoped>
.code-duplex-page {
  --tool-textarea-min-height: 9rem;
}
</style>
