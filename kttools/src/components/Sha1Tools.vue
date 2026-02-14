<template>
  <n-card size="small" style="max-width: 600px; margin: 0 auto; min-height: 400px;">
    <n-flex vertical size="large" style="padding: 8px;">
      <n-space>
        <n-input v-model:value="filePath" readonly placeholder="选择文件..." style="width: 400px;" />
        <n-button type="primary" ghost @click="selectFile">浏览...</n-button>
      </n-space>

      <n-button 
        type="primary" 
        size="large" 
        @click="calculate" 
        :loading="loading" 
        :disabled="!filePath"
        style="width: 100%;"
      >
        计算 SHA1
      </n-button>

      <template v-if="sha1Hash">
        <n-divider title-placement="left">SHA1 结果</n-divider>
        <n-input 
          v-model:value="sha1Hash" 
          readonly 
          type="textarea" 
          :rows="3"
        />
        <n-button text type="primary" @click="copyResult" style="margin-top: 8px;">复制结果</n-button>
      </template>

      <n-empty v-else description="选择文件后点击计算按钮" style="margin-top: 40px;" />
    </n-flex>
  </n-card>
</template>

<script setup>
import { ref } from "vue";
import { invoke } from "@tauri-apps/api/core";
import { open } from '@tauri-apps/plugin-dialog';
import { useMessage } from 'naive-ui'

const message = useMessage()

const filePath = ref("");
const sha1Hash = ref("");
const loading = ref(false);

const selectFile = async () => {
  try {
    const selected = await open({ multiple: false });
    if (selected) { filePath.value = selected; sha1Hash.value = ""; }
  } catch (error) {
    message.error("文件选择失败")
  }
};

const calculate = async () => {
  if (!filePath.value) { message.warning("请先选择文件"); return; }
  loading.value = true;
  try {
    sha1Hash.value = await invoke("sha1_encode", { filePath: filePath.value });
  } catch (error) {
    message.error("SHA1计算失败")
  } finally {
    loading.value = false;
  }
}

const copyResult = () => {
  navigator.clipboard.writeText(sha1Hash.value);
  message.success("已复制到剪贴板");
}
</script>

<style scoped>
:deep(.n-input textarea) {
  font-family: 'Fira Code', 'Consolas', monospace;
  font-size: 14px;
  line-height: 1.6;
  text-align: center;
}
</style>
