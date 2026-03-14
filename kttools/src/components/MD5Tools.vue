<template>
  <div class="page-view page-view--centered">
    <div class="tool-grid tool-grid--two tool-fill-area">
      <n-card class="tool-surface hover-lift" size="small" :bordered="false">
        <div class="tool-stack">
          <div class="tool-panel__title">
            <strong>输入内容</strong>
            <span class="tool-panel__meta">适合快速生成文本指纹，用于签名校验、比对和临时调试。</span>
          </div>

          <n-input
            v-model:value="args.input"
            class="tool-code-input tool-fill-area md5-input"
            type="textarea"
            placeholder="输入要计算 MD5 的内容..."
          />

          <div class="tool-inline-actions">
            <n-button type="primary" @click="calculate" :disabled="!args.input">计算 MD5</n-button>
            <span class="tool-note">计算结果会在右侧即时展示。</span>
          </div>
        </div>
      </n-card>

      <n-card class="tool-surface hover-lift" size="small" :bordered="false">
        <div class="tool-stack tool-fill-area">
          <div class="tool-panel__header">
            <div class="tool-panel__title">
              <strong>MD5 结果</strong>
              <span class="tool-panel__meta">适合直接复制到请求签名、接口文档或比对脚本中。</span>
            </div>
            <n-button v-if="args.output" text type="primary" @click="copyResult">复制</n-button>
          </div>

          <div class="tool-result-stage tool-fill-area">
            <n-text v-if="args.output" code class="mono-text md5-output">{{ args.output }}</n-text>
            <n-empty v-else description="点击左侧按钮计算 MD5" />
          </div>
        </div>
      </n-card>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { invoke } from '@tauri-apps/api/core'
import { useMessage } from 'naive-ui'

const message = useMessage()

const args = ref({ input: '', output: '' })

const calculate = async () => {
  if (!args.value.input) {
    args.value.output = ''
    return
  }

  try {
    args.value.output = await invoke('md5_encode', { input: args.value.input })
  } catch {
    message.error('MD5 计算失败')
  }
}

const copyResult = async () => {
  await navigator.clipboard.writeText(args.value.output)
  message.success('已复制到剪贴板')
}
</script>

<style scoped>
.md5-input {
  --tool-textarea-min-height: clamp(10rem, 30vh, 14rem);
}

.md5-output {
  display: block;
  max-width: 100%;
  padding: 18px;
  border-radius: 20px;
  text-align: center;
  font-size: 1.08rem;
  word-break: break-all;
}
</style>
