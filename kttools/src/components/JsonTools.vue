<template>
  <n-card size="small" style="height: 100%;">
    <n-split direction="vertical" :default-size="0.35" style="height: calc(100vh - 280px); min-height: 400px;">
      <template #1>
        <n-flex vertical style="height: 100%; padding: 8px;">
          <n-flex justify="space-between" align="center">
            <n-text strong>输入 JSON</n-text>
            <n-space>
              <n-input
                v-if="args.output"
                v-model:value="jsonpathFilter"
                placeholder="JsonPath: $.store.book[0].title"
                clearable
                @update:value="filter"
                style="width: 260px;"
              />
              <n-button text type="primary" tag="a" href="https://goessner.net/articles/JsonPath" target="_blank">文档</n-button>
            </n-space>
          </n-flex>
          <n-input
            v-model:value="args.input"
            type="textarea"
            placeholder="输入原始 JSON..."
            @update:value="format"
            style="flex: 1;"
          />
        </n-flex>
      </template>
      
      <template #2>
        <n-flex vertical style="height: 100%; padding: 8px; background: #f9f9f9; border-radius: 4px;">
          <n-text strong>格式化结果</n-text>
          <div ref="containerContainer" style="flex: 1; overflow: hidden;"></div>
        </n-flex>
      </template>
    </n-split>
  </n-card>
</template>

<script setup>
import { ref, watch } from "vue";
import { JSONPath } from 'jsonpath-plus';
import JSONEditor from 'jsoneditor';
import 'jsoneditor/dist/jsoneditor.min.css';

const args = ref({ input: "", output: "" });
const jsonpathFilter = ref('');
const containerContainer = ref(null);
const jsonEditor = ref(null);

watch(containerContainer, (newVal) => {
  if (newVal) {
    initJsonEditor();
    if (args.value.input) {
      try { jsonEditor.value.set(JSON.parse(args.value.input)); } 
      catch { jsonEditor.value.setText(args.value.input); }
    }
  }
});

const initJsonEditor = () => {
  if (!containerContainer.value) return;
  if (jsonEditor.value) { jsonEditor.value.destroy(); jsonEditor.value = null; }
  jsonEditor.value = new JSONEditor(containerContainer.value, { 
    mode: 'tree', 
    modes: ['code', 'tree', 'form'],
    mainMenuBar: true
  });
};

const format = (value) => {
  if (!value) { args.value.output = ""; return; }
  try {
    const parsed = JSON.parse(value);
    args.value.output = JSON.stringify(parsed, null, 2);
    if (jsonEditor.value) jsonEditor.value.set(parsed);
  } catch (error) {
    args.value.output = error.toString();
    if (jsonEditor.value) jsonEditor.value.setText(value);
  }
}

const filter = (jsonpathStr) => {
  if (!jsonpathStr) {
    if (args.value.input) {
      try { jsonEditor.value.set(JSON.parse(args.value.input)); }
      catch { jsonEditor.value.setText(args.value.input); }
    }
    return;
  }
  try {
    const obj = JSONPath({ path: jsonpathStr, json: JSON.parse(args.value.output || args.value.input) });
    if (jsonEditor.value) jsonEditor.value.set(obj);
  } catch (error) { 
    console.error('JsonPath error:', error); 
  }
}
</script>

<style scoped>
:deep(.n-input textarea) {
  font-family: 'Fira Code', 'Consolas', monospace;
  font-size: 14px;
  line-height: 1.6;
}

:deep(.jsoneditor) {
  height: 100%;
  border: none;
}

:deep(.jsoneditor-outer) {
  height: 100%;
}
</style>
