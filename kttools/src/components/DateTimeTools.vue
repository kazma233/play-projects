<template>
  <div class="page-view page-view--centered">
    <div class="tool-grid tool-grid--two tool-fill-area">
      <n-card class="tool-surface" size="small">
        <div class="tool-stack">
          <div class="tool-panel__title">
            <strong>时间解析</strong>
            <span class="tool-panel__meta">输入任意时间戳或时间文本，统一转换为更易读的格式。</span>
          </div>

          <div class="tool-inline-actions datetime-row">
            <n-input
              v-model:value="dateChangeArgs.input"
              class="tool-field tool-field--grow datetime-field"
              placeholder="输入任意格式的时间[戳]"
              clearable
            />
            <n-select v-model:value="dateChangeArgs.timezone" :options="timezoneOptions" class="tool-field tool-field--wide datetime-field" />
          </div>

          <div class="tool-value-grid">
            <div v-for="item in exchangeItems" :key="item.key" class="tool-value-card">
              <div class="tool-value-card__content">
                <span class="tool-value-label">{{ item.label }}</span>
                <span class="tool-value-card__text mono-text">{{ item.value || '-' }}</span>
              </div>
              <n-button v-if="item.value" text type="primary" @click="copy(item.value)">复制</n-button>
            </div>
          </div>
        </div>
      </n-card>

      <n-card class="tool-surface" size="small">
        <div class="tool-stack">
          <div class="tool-panel__title">
            <strong>时间计算</strong>
            <span class="tool-panel__meta">基于上方解析出的时间继续偏移，适合日志回放和任务调度排查。</span>
          </div>

          <div class="tool-inline-actions datetime-row">
            <n-input-number v-model:value="clacReqArgs.timeValue" class="tool-field tool-field--compact datetime-field" />
            <n-select v-model:value="clacReqArgs.timeUnit" :options="timeUnitOptions" class="tool-field tool-field--compact datetime-field" />
            <n-select v-model:value="clacReqArgs.timezone" :options="timezoneOptions" class="tool-field tool-field--wide datetime-field" />
          </div>

          <div class="tool-chip-row">
            <span class="tool-chip">基准时间：{{ exchangeMsg.rfc3339 || '等待输入' }}</span>
          </div>

          <div class="tool-value-grid">
            <div v-for="item in calcItems" :key="item.key" class="tool-value-card">
              <div class="tool-value-card__content">
                <span class="tool-value-label">{{ item.label }}</span>
                <span class="tool-value-card__text mono-text">{{ item.value || '-' }}</span>
              </div>
              <n-button v-if="item.value" text type="primary" @click="copy(item.value)">复制</n-button>
            </div>
          </div>
        </div>
      </n-card>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { invoke } from '@tauri-apps/api/core'
import { useMessage } from 'naive-ui'

const message = useMessage()

const exchangeMsg = ref({})
const dateChangeArgs = ref({
  input: new Date().getTime().toString(),
  timezone: 'Asia/Shanghai'
})

const timezoneOptions = [
  { label: 'Asia/Shanghai', value: 'Asia/Shanghai' },
  { label: 'America/New_York', value: 'America/New_York' }
]

const exchangeItems = computed(() => [
  { key: 'readable', label: '可读时间', value: exchangeMsg.value.readable },
  { key: 'rfc3339', label: 'RFC3339', value: exchangeMsg.value.rfc3339 },
  { key: 'timestamp', label: '时间戳', value: exchangeMsg.value.timestamp }
])

const exchange2date = async () => {
  if (!dateChangeArgs.value.input) {
    exchangeMsg.value = {}
    return
  }

  try {
    exchangeMsg.value = JSON.parse(await invoke('exchange_date', dateChangeArgs.value))
  } catch {
    exchangeMsg.value = {}
  }
}

watch(dateChangeArgs, exchange2date, { deep: true })

const clacReqArgs = ref({
  timeValue: 0,
  timeUnit: 'days',
  timezone: 'Asia/Shanghai'
})

const calcMsg = ref({})

const timeUnitOptions = [
  { label: '秒', value: 'seconds' },
  { label: '分钟', value: 'minutes' },
  { label: '小时', value: 'hours' },
  { label: '天', value: 'days' },
  { label: '周', value: 'weeks' },
  { label: '月', value: 'months' },
  { label: '年', value: 'years' }
]

const calcItems = computed(() => [
  { key: 'readable', label: '可读时间', value: calcMsg.value.readable },
  { key: 'rfc3339', label: 'RFC3339', value: calcMsg.value.rfc3339 },
  { key: 'timestamp', label: '时间戳', value: calcMsg.value.timestamp }
])

const calculate = async () => {
  if (!exchangeMsg.value.rfc3339) {
    calcMsg.value = {}
    return
  }

  try {
    const result = await invoke('calc_date', {
      ...clacReqArgs.value,
      rfc3339: exchangeMsg.value.rfc3339
    })
    calcMsg.value = JSON.parse(result)
  } catch {
    calcMsg.value = {}
  }
}

watch(clacReqArgs, calculate, { deep: true })
watch(() => exchangeMsg.value.rfc3339, calculate)

onMounted(exchange2date)

const copy = async (text) => {
  await navigator.clipboard.writeText(text.toString())
  message.success('已复制')
}
</script>

<style scoped>
.datetime-row {
  align-items: stretch;
}
</style>
