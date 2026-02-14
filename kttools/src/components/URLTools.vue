<template>
  <n-card size="small" style="height: 100%;">
    <n-split direction="vertical" :default-size="0.5" style="height: calc(100vh - 280px); min-height: 400px;">
      <template #1>
        <n-flex vertical style="height: 100%; padding: 8px;">
          <n-text strong>原始值</n-text>
          <n-input
            v-model:value="urlArgs.encode"
            type="textarea"
            placeholder="输入要编码的内容..."
            @update:value="encode"
            style="flex: 1;"
          />
        </n-flex>
      </template>
      
      <template #2>
        <n-flex vertical style="height: 100%; padding: 8px;">
          <n-flex justify="space-between" align="center">
            <n-text strong>URL 编码值</n-text>
            <n-button text type="primary" @click="swap">⇅ 交换</n-button>
          </n-flex>
          <n-input
            v-model:value="urlArgs.decode"
            type="textarea"
            placeholder="输入要解码的内容..."
            @update:value="decode"
            style="flex: 1;"
          />
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

const urlArgs = ref({ encode: "", decode: "" });

const encode = async (input) => {
  if (!input) { urlArgs.value.decode = ""; return; }
  try { urlArgs.value.decode = await invoke("url_encode", { input }); } 
  catch { message.error("编码失败") }
}

const decode = async (input) => {
  if (!input) { urlArgs.value.encode = ""; return; }
  try { urlArgs.value.encode = await invoke("url_decode", { input }); } 
  catch { message.error("解码失败") }
}

const swap = () => {
  const temp = urlArgs.value.encode;
  urlArgs.value.encode = urlArgs.value.decode;
  urlArgs.value.decode = temp;
}
</script>

<style scoped>
:deep(.n-input textarea) {
  font-family: 'Fira Code', 'Consolas', monospace;
  font-size: 14px;
  line-height: 1.6;
}
</style>
