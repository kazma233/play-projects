<template>
  <n-card size="small" style="height: 100%;">
    <n-split direction="vertical" :default-size="0.6" style="height: calc(100vh - 280px); min-height: 400px;">
      <template #1>
        <n-flex vertical style="height: 100%; padding: 8px;">
          <n-flex justify="space-between" align="center">
            <n-text strong>输入内容</n-text>
            <n-button type="primary" size="small" @click="calculate" :disabled="!args.input">计算 MD5</n-button>
          </n-flex>
          <n-input
            v-model:value="args.input"
            type="textarea"
            placeholder="输入要计算 MD5 的内容..."
            style="flex: 1;"
          />
        </n-flex>
      </template>
      
      <template #2>
        <n-flex vertical style="height: 100%; padding: 8px; background: #f9f9f9; border-radius: 4px;">
          <n-flex justify="space-between" align="center">
            <n-text strong>MD5 结果</n-text>
            <n-button v-if="args.output" text type="primary" @click="copyResult">复制</n-button>
          </n-flex>
          <n-flex align="center" justify="center" style="flex: 1; padding: 16px;">
            <n-text v-if="args.output" code style="font-size: 18px; word-break: break-all;">{{ args.output }}</n-text>
            <n-empty v-else description="点击上方按钮计算 MD5" />
          </n-flex>
        </n-flex>
      </template>
    </n-split>
  </n-card>
</template>

<script setup>
import { ref } from "vue";
import { invoke } from "@tauri-apps/api/core";
import { useMessage } from 'naive-ui'

const message = useMessage()

const args = ref({ input: "", output: "" });

const calculate = async () => {
  if (!args.value.input) { args.value.output = ""; return; }
  try {
    args.value.output = await invoke("md5_encode", { input: args.value.input });
  } catch (error) {
    message.error("MD5计算失败")
  }
}

const copyResult = () => {
  navigator.clipboard.writeText(args.value.output);
  message.success("已复制到剪贴板");
}
</script>

<style scoped>
:deep(.n-input textarea) {
  font-family: 'Fira Code', 'Consolas', monospace;
  font-size: 14px;
  line-height: 1.6;
}
</style>
