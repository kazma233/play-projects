<template>
  <n-card size="small" style="max-width: 600px; margin: 0 auto; min-height: 500px;">
    <n-flex vertical size="large" style="padding: 8px;">
      <n-card embedded size="small" style="background: #fafafa;">
        <n-flex justify="space-between" align="center" wrap :gap="12">
          <n-flex :gap="16" align="center">
            <n-flex vertical :gap="4">
              <n-text depth="3" style="font-size: 12px;">深色</n-text>
              <n-color-picker v-model:value="darkColor" :modes="['hex']" style="width: 80px;" />
            </n-flex>
            <n-flex vertical :gap="4">
              <n-text depth="3" style="font-size: 12px;">浅色</n-text>
              <n-color-picker v-model:value="lightColor" :modes="['hex']" style="width: 80px;" />
            </n-flex>
            <n-flex vertical :gap="4">
              <n-text depth="3" style="font-size: 12px;">尺寸</n-text>
              <n-input-number v-model:value="size" :min="100" :max="400" :step="50" style="width: 90px;" />
            </n-flex>
          </n-flex>
        </n-flex>
      </n-card>
      
      <n-flex :gap="12">
        <n-input 
          v-model:value="inputText" 
          placeholder="输入要生成二维码的文本..." 
          clearable 
          size="large"
          style="flex: 1;" 
        />
        <n-button 
          type="primary" 
          size="large"
          :loading="loading" 
          :disabled="!inputText.trim()" 
          @click="generate"
        >
          生成
        </n-button>
      </n-flex>
      
      <n-flex v-if="qrCodeImage" vertical align="center" style="padding: 24px; background: #f9f9f9; border-radius: 8px; margin-top: 8px;">
        <div ref="qrImageRef" v-html="qrCodeImage" style="max-width: 100%; padding: 16px; background: #fff; border-radius: 8px; box-shadow: 0 2px 8px rgba(0,0,0,0.1);"></div>
        <n-space style="margin-top: 16px;">
          <n-button type="primary" @click="saveQRCode">
            <template #icon>
              <n-icon><download-outline /></n-icon>
            </template>
            保存图片
          </n-button>
          <n-button @click="copyQRCode">
            <template #icon>
              <n-icon><copy-outline /></n-icon>
            </template>
            复制
          </n-button>
        </n-space>
      </n-flex>
      
      <n-empty v-else description="输入文本后点击生成二维码" style="margin-top: 60px;">
        <template #icon>
          <span style="font-size: 48px;">📱</span>
        </template>
      </n-empty>
    </n-flex>
  </n-card>
</template>

<script setup>
import { ref } from 'vue';
import { invoke } from "@tauri-apps/api/core";
import { save } from '@tauri-apps/plugin-dialog';
import { writeFile } from '@tauri-apps/plugin-fs';
import { useMessage } from 'naive-ui'
import { DownloadOutline, CopyOutline } from '@vicons/ionicons5'

const message = useMessage()

const inputText = ref('');
const qrCodeImage = ref('');
const loading = ref(false);
const size = ref(256);
const darkColor = ref('#000000');
const lightColor = ref('#ffffff');
const qrImageRef = ref(null);

const generate = async () => {
  if (!inputText.value.trim()) return;
  loading.value = true;
  try {
    qrCodeImage.value = await invoke('generate_qr_code', {
      content: inputText.value.trim(),
      size: size.value,
      darkColor: darkColor.value,
      lightColor: lightColor.value
    });
  } catch (err) {
    message.error('生成失败: ' + err)
  } finally {
    loading.value = false;
  }
};

const saveQRCode = async () => {
  if (!inputText.value.trim()) {
    message.error('请输入内容');
    return;
  }
  
  try {
    const filePath = await save({
      filters: [{
        name: 'PNG Image',
        extensions: ['png']
      }],
      defaultPath: `qrcode-${Date.now()}.png`
    });
    
    if (!filePath) return;

    const pngData = await invoke('generate_qr_code_png', {
      content: inputText.value.trim(),
      size: size.value,
      darkColor: darkColor.value,
      lightColor: lightColor.value
    });
    
    const uint8Array = new Uint8Array(pngData);
    await writeFile(filePath, uint8Array);
    message.success('二维码已保存');
  } catch (err) {
    console.error('Save error:', err);
    message.error('保存失败: ' + (err.message || err));
  }
};

const copyQRCode = async () => {
  if (!inputText.value.trim()) {
    message.error('请输入内容');
    return;
  }

  message.loading('正在复制...', { duration: 5000 });
  
  try {
    await invoke('copy_qr_code_to_clipboard', {
      content: inputText.value.trim(),
      size: size.value,
      darkColor: darkColor.value,
      lightColor: lightColor.value
    });
    
    message.destroyAll();
    message.success('已复制到剪贴板');
  } catch (err) {
    console.error('复制错误:', err);
    message.destroyAll();
    message.error('复制失败: ' + (err.message || err));
  }
};
</script>

<style scoped>
:deep(svg) {
  display: block;
  max-width: 100%;
  height: auto;
}
</style>
