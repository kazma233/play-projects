<template>
  <n-flex vertical style="height: 100%; gap: 16px; overflow: hidden;">
    <n-flex justify="space-between" align="center">
      <n-text style="font-size: 18px; font-weight: 500;">端口占用管理</n-text>
      <n-button type="primary" @click="refreshPorts" :loading="loading">刷新</n-button>
    </n-flex>

    <n-flex align="center" :gap="16">
      <n-input v-model:value="searchQuery" placeholder="搜索端口、PID或进程名..." clearable style="max-width: 400px;" />
      <n-select v-model:value="selectedStatus" :options="statusOptions" placeholder="状态" clearable style="width: 120px;" />
      <n-text depth="3" style="font-size: 14px;">共 {{ filteredPorts.length }} 个结果</n-text>
    </n-flex>

    <div v-if="!loading && ports.length > 0" style="flex: 1; overflow: hidden;">
      <n-data-table
        :columns="columns"
        :data="filteredPorts"
        :row-key="row => `${row.port}-${row.protocol}-${row.pid}`"
        :bordered="true"
        :single-line="false"
        flex-height
        style="height: calc(100vh - 200px);"
      />
    </div>

    <n-empty v-else-if="!loading" description="未找到端口占用，点击刷新按钮获取数据" style="flex: 1; justify-content: center;" />

    <n-flex v-else vertical align="center" justify="center" style="flex: 1; gap: 16px;">
      <n-spin size="large" />
      <n-text>加载中...</n-text>
    </n-flex>
  </n-flex>
</template>

<script setup>
import { ref, computed, onMounted, h } from 'vue'
import { NButton, NInput, NDataTable, NEmpty, NSpin, NTag, NText, NFlex, NSelect, useMessage } from 'naive-ui'
import { invoke } from "@tauri-apps/api/core"

const message = useMessage()

const loading = ref(false)
const ports = ref([])
const searchQuery = ref('')
const killingPorts = ref(new Set())
const sortColumn = ref('port')
const sortOrder = ref('asc')
const selectedStatus = ref(null)

const statusOptions = [
  { label: '监听中', value: 'LISTEN' },
  { label: '已连接', value: 'ESTABLISHED' },
  { label: '等待中', value: 'TIME_WAIT' },
  { label: '关闭中', value: 'CLOSE_WAIT' },
  { label: 'UDP', value: '*' }
]

const filteredPorts = computed(() => {
  let result = [...ports.value]
  
  if (selectedStatus.value !== null) {
    result = result.filter(port => port.status === selectedStatus.value)
  }
  
  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    result = result.filter(port =>
      port.port.toString().includes(query) ||
      port.pid.toString().includes(query) ||
      port.process_name.toLowerCase().includes(query)
    )
  }
  
  result.sort((a, b) => {
    let valA, valB
    switch (sortColumn.value) {
      case 'port':
        valA = a.port
        valB = b.port
        break
      case 'pid':
        valA = a.pid
        valB = b.pid
        break
      case 'process_name':
        valA = a.process_name.toLowerCase()
        valB = b.process_name.toLowerCase()
        break
      default:
        return 0
    }
    
    if (valA < valB) return sortOrder.value === 'asc' ? -1 : 1
    if (valA > valB) return sortOrder.value === 'asc' ? 1 : -1
    return 0
  })
  
  return result
})

const columns = computed(() => [
  {
    title: '端口',
    key: 'port',
    width: 80,
    sorter: (a, b) => a.port - b.port,
    render(row) {
      return h('span', { style: 'font-weight: 500' }, row.port)
    }
  },
  {
    title: '协议',
    key: 'protocol',
    width: 80
  },
  {
    title: '状态',
    key: 'status',
    width: 100,
    render(row) {
      const statusMap = {
        'LISTEN': { type: 'success', text: '监听中' },
        'ESTABLISHED': { type: 'info', text: '已连接' },
        '*': { type: 'default', text: 'UDP' },
        'TIME_WAIT': { type: 'warning', text: '等待中' },
        'CLOSE_WAIT': { type: 'warning', text: '关闭中' }
      }
      const status = statusMap[row.status] || { type: 'default', text: row.status }
      return h(NTag, { type: status.type, size: 'small' }, { default: () => status.text })
    }
  },
  {
    title: '本地地址',
    key: 'local_addr',
    width: 180,
    ellipsis: { tooltip: true }
  },
  {
    title: '远程地址',
    key: 'remote_addr',
    width: 180,
    ellipsis: { tooltip: true }
  },
  {
    title: '用户',
    key: 'user',
    width: 80
  },
  {
    title: '进程名',
    key: 'process_name',
    width: 150,
    ellipsis: { tooltip: true },
    sorter: (a, b) => a.process_name.localeCompare(b.process_name),
    render(row) {
      if (row.process_name_unknown) {
        return h('span', { style: 'color: #d03050' }, row.process_name)
      }
      return h('span', {}, row.process_name)
    }
  },
  {
    title: 'PID',
    key: 'pid',
    width: 80,
    sorter: (a, b) => a.pid - b.pid
  },
  {
    title: '操作',
    key: 'action',
    width: 100,
    render(row) {
      const isKilling = killingPorts.value.has(row.port)
      return h(NButton, {
        size: 'small',
        type: 'error',
        disabled: isKilling,
        loading: isKilling,
        onClick: () => killPort(row.port)
      }, { default: () => isKilling ? '终止中...' : '终止进程' })
    }
  }
])

const refreshPorts = async () => {
  loading.value = true
  try {
    ports.value = await invoke('get_port_list')
    message.success('刷新成功')
  } catch (error) {
    message.error('获取端口列表失败: ' + error)
  } finally {
    loading.value = false
  }
}

const killPort = async (port) => {
  try {
    killingPorts.value.add(port)
    const result = await invoke('kill_process', { port })
    message.success(result)
    await refreshPorts()
  } catch (error) {
    message.error('终止进程失败: ' + error)
  } finally {
    killingPorts.value.delete(port)
  }
}

onMounted(() => {
  refreshPorts()
})
</script>
