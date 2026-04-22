<template>
  <div class="page-view">
    <n-card class="tool-surface tool-surface--fill" size="small">
      <n-split class="tool-split tool-split-frame tool-split-frame--tall" direction="vertical" :default-size="0.38">
        <template #1>
          <div class="tool-panel">
            <div class="tool-panel__header">
              <div class="tool-panel__title">
                <strong>输入 JSON</strong>
                <span class="tool-panel__meta">支持粘贴原始 JSON，自动格式化后同步到树视图。</span>
              </div>

              <div class="tool-inline-actions">
                <n-input
                  v-if="args.output"
                  v-model:value="jsonpathFilter"
                  class="tool-field tool-field--grow"
                  placeholder="JsonPath: $.store.book[0].title"
                  clearable
                  @update:value="filter"
                />
                <n-button text type="primary" tag="a" href="https://goessner.net/articles/JsonPath" target="_blank">
                  JsonPath 文档
                </n-button>
              </div>
            </div>

            <n-input
              v-model:value="args.input"
              class="tool-code-input tool-fill-area json-input"
              type="textarea"
              placeholder="输入原始 JSON..."
              @update:value="format"
            />
          </div>
        </template>

        <template #2>
          <div class="tool-panel">
            <div class="tool-panel__header">
              <div class="tool-panel__title">
                <strong>格式化结果</strong>
                <span class="tool-panel__meta">树视图、代码视图和表单视图可快速切换。</span>
              </div>
            </div>

            <div ref="containerContainer" class="json-editor-stage tool-fill-area"></div>
          </div>
        </template>
      </n-split>
    </n-card>
  </div>
</template>

<script setup>
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { JSONPath } from 'jsonpath-plus'
import JSONEditor from 'jsoneditor'
import 'jsoneditor/dist/jsoneditor.min.css'

const args = ref({ input: '', output: '' })
const jsonpathFilter = ref('')
const containerContainer = ref(null)
const jsonEditor = ref(null)

const refreshEditorLayout = () => {
  jsonEditor.value?.refresh?.()
}

watch(containerContainer, (newVal) => {
  if (newVal) {
    initJsonEditor()
    if (args.value.input) {
      try {
        jsonEditor.value.set(JSON.parse(args.value.input))
      } catch {
        jsonEditor.value.setText(args.value.input)
      }
    }
  }
})

const initJsonEditor = () => {
  if (!containerContainer.value) {
    return
  }

  if (jsonEditor.value) {
    jsonEditor.value.destroy()
    jsonEditor.value = null
  }

  jsonEditor.value = new JSONEditor(containerContainer.value, {
    mode: 'tree',
    modes: ['code', 'tree', 'form'],
    mainMenuBar: true,
    navigationBar: true,
    statusBar: true
  })

  refreshEditorLayout()
}

const format = (value) => {
  if (!value) {
    args.value.output = ''
    return
  }

  try {
    const parsed = JSON.parse(value)
    args.value.output = JSON.stringify(parsed, null, 2)
    if (jsonEditor.value) {
      jsonEditor.value.set(parsed)
    }
  } catch (error) {
    args.value.output = error.toString()
    if (jsonEditor.value) {
      jsonEditor.value.setText(value)
    }
  }
}

const filter = (jsonpathStr) => {
  if (!jsonpathStr) {
    if (args.value.input) {
      try {
        jsonEditor.value.set(JSON.parse(args.value.input))
      } catch {
        jsonEditor.value.setText(args.value.input)
      }
    }
    return
  }

  try {
    const result = JSONPath({ path: jsonpathStr, json: JSON.parse(args.value.output || args.value.input) })
    if (jsonEditor.value) {
      jsonEditor.value.set(result)
    }
  } catch (error) {
    console.error('JsonPath error:', error)
  }
}

onMounted(() => {
  window.addEventListener('resize', refreshEditorLayout)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', refreshEditorLayout)
  jsonEditor.value?.destroy()
  jsonEditor.value = null
})
</script>

<style>
.json-editor-stage {
  min-height: clamp(14rem, 36vh, 20rem);
  overflow: hidden;
  border: 1px solid #e5e7eb;
  background: #fff;
}

.json-input {
  --tool-textarea-min-height: 12rem;
}

.json-editor-stage .jsoneditor {
  height: 100%;
  display: flex;
  flex-direction: column;
  border: 0;
}

.json-editor-stage .jsoneditor-menu {
  background: #111827;
  border-bottom: 0;
  display: flex;
  align-items: center;
  gap: 0.375rem;
  flex-wrap: wrap;
  padding: 0.5rem;
}

.json-editor-stage .jsoneditor-menu > button,
.json-editor-stage .jsoneditor-modes {
  flex: 0 0 auto;
}

.json-editor-stage .jsoneditor-search {
  flex: 1 1 min(100%, 12rem);
  min-width: min(100%, 10rem);
  margin-left: 0;
}

.json-editor-stage .jsoneditor-search input {
  width: 100%;
}

.json-editor-stage .jsoneditor-navigation-bar,
.json-editor-stage .jsoneditor-statusbar {
  background: #f9fafb;
  border-color: #e5e7eb;
}

.json-editor-stage .jsoneditor-outer {
  flex: 1;
  min-height: 0;
  height: auto;
}
</style>
