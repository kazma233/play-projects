<template>
  <n-flex vertical align="center" style="max-width: 900px; height: 100%; margin: 0 auto;">
    <n-card size="small" style="width: 100%;">
      <n-flex vertical :gap="16" align="center">
        <n-flex align="center" wrap justify="center">
          <n-input 
            v-model:value="dateChangeArgs.input" 
            placeholder="输入任意格式的时间[戳]" 
            clearable 
            style="width: 300px;" 
          />
          <n-select v-model:value="dateChangeArgs.timezone" :options="timezoneOptions" style="width: 180px;" />
        </n-flex>
        
        <n-flex vertical :gap="12" style="width: 100%; max-width: 500px;">
          <n-flex justify="space-between" align="center">
            <n-text strong style="width: 80px;">可读时间</n-text>
            <n-flex align="center" :gap="8" style="flex: 1;">
              <n-text code style="font-size: 16px; flex: 1; text-align: center;">{{ exchangeMsg.readable || '-' }}</n-text>
              <n-button v-if="exchangeMsg.readable" text type="primary" @click="copy(exchangeMsg.readable)">
                <template #icon>
                  <n-icon><copy-outline /></n-icon>
                </template>
              </n-button>
            </n-flex>
          </n-flex>
          
          <n-flex justify="space-between" align="center">
            <n-text strong style="width: 80px;">RFC3339</n-text>
            <n-flex align="center" :gap="8" style="flex: 1;">
              <n-text code style="font-size: 16px; flex: 1; text-align: center;">{{ exchangeMsg.rfc3339 || '-' }}</n-text>
              <n-button v-if="exchangeMsg.rfc3339" text type="primary" @click="copy(exchangeMsg.rfc3339)">
                <template #icon>
                  <n-icon><copy-outline /></n-icon>
                </template>
              </n-button>
            </n-flex>
          </n-flex>
          
          <n-flex justify="space-between" align="center">
            <n-text strong style="width: 80px;">时间戳</n-text>
            <n-flex align="center" :gap="8" style="flex: 1;">
              <n-text code style="font-size: 16px; flex: 1; text-align: center;">{{ exchangeMsg.timestamp || '-' }}</n-text>
              <n-button v-if="exchangeMsg.timestamp" text type="primary" @click="copy(exchangeMsg.timestamp)">
                <template #icon>
                  <n-icon><copy-outline /></n-icon>
                </template>
              </n-button>
            </n-flex>
          </n-flex>
        </n-flex>
      </n-flex>
    </n-card>

    <n-card title="🧮 时间计算" size="small" style="width: 100%;">
      <n-flex vertical :gap="16" align="center">
        <n-flex align="center" wrap justify="center">
          <n-input-number v-model:value="clacReqArgs.timeValue" style="width: 120px;" />
          <n-select v-model:value="clacReqArgs.timeUnit" :options="timeUnitOptions" style="width: 100px;" />
          <n-select v-model:value="clacReqArgs.timezone" :options="timezoneOptions" style="width: 180px;" />
        </n-flex>
        
        <n-flex vertical :gap="12" style="width: 100%; max-width: 500px;">
          <n-flex justify="space-between" align="center">
            <n-text strong style="width: 80px;">可读时间</n-text>
            <n-flex align="center" :gap="8" style="flex: 1;">
              <n-text code style="font-size: 16px; flex: 1; text-align: center;">{{ calcMsg.readable || '-' }}</n-text>
              <n-button v-if="calcMsg.readable" text type="primary" @click="copy(calcMsg.readable)">
                <template #icon>
                  <n-icon><copy-outline /></n-icon>
                </template>
              </n-button>
            </n-flex>
          </n-flex>
          
          <n-flex justify="space-between" align="center">
            <n-text strong style="width: 80px;">RFC3339</n-text>
            <n-flex align="center" :gap="8" style="flex: 1;">
              <n-text code style="font-size: 16px; flex: 1; text-align: center;">{{ calcMsg.rfc3339 || '-' }}</n-text>
              <n-button v-if="calcMsg.rfc3339" text type="primary" @click="copy(calcMsg.rfc3339)">
                <template #icon>
                  <n-icon><copy-outline /></n-icon>
                </template>
              </n-button>
            </n-flex>
          </n-flex>
          
          <n-flex justify="space-between" align="center">
            <n-text strong style="width: 80px;">时间戳</n-text>
            <n-flex align="center" :gap="8" style="flex: 1;">
              <n-text code style="font-size: 16px; flex: 1; text-align: center;">{{ calcMsg.timestamp || '-' }}</n-text>
              <n-button v-if="calcMsg.timestamp" text type="primary" @click="copy(calcMsg.timestamp)">
                <template #icon>
                  <n-icon><copy-outline /></n-icon>
                </template>
              </n-button>
            </n-flex>
          </n-flex>
        </n-flex>
      </n-flex>
    </n-card>
  </n-flex>
</template>

<script setup>
import { watch, ref, onMounted } from "vue";
import { invoke } from "@tauri-apps/api/core";
import { useMessage } from 'naive-ui'
import { CopyOutline } from '@vicons/ionicons5'

const message = useMessage()

const exchangeMsg = ref({});
const dateChangeArgs = ref({
  input: new Date().getTime().toString(),
  timezone: "Asia/Shanghai"
});

const timezoneOptions = [
  { label: 'Asia/Shanghai', value: 'Asia/Shanghai' },
  { label: 'America/New_York', value: 'America/New_York' },
];

const exchange2date = async () => {
  if (!dateChangeArgs.value.input) {
    exchangeMsg.value = {};
    return;
  }
  try {
    exchangeMsg.value = JSON.parse(await invoke("exchange_date", dateChangeArgs.value))
  } catch (error) {
    exchangeMsg.value = {}
  }
}

watch(dateChangeArgs, exchange2date, { deep: true });

const clacReqArgs = ref({
  timeValue: 0,
  timeUnit: "days",
  timezone: "Asia/Shanghai"
});
const calcMsg = ref({});

const timeUnitOptions = [
  { label: '秒', value: 'seconds' },
  { label: '分钟', value: 'minutes' },
  { label: '小时', value: 'hours' },
  { label: '天', value: 'days' },
  { label: '周', value: 'weeks' },
  { label: '月', value: 'months' },
  { label: '年', value: 'years' },
];

const calculate = async () => {
  if (!exchangeMsg.value.rfc3339) {
    calcMsg.value = {};
    return;
  }
  try {
    const result = await invoke("calc_date", { ...clacReqArgs.value, rfc3339: exchangeMsg.value.rfc3339 })
    calcMsg.value = JSON.parse(result)
  } catch (error) {
    calcMsg.value = {}
  }
}

watch(clacReqArgs, calculate, { deep: true });

onMounted(exchange2date);

const copy = (text) => {
  navigator.clipboard.writeText(text.toString())
  message.success('已复制')
}
</script>
