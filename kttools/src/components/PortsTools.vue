<template>
  <div class="page-view ports-page">
    <n-card class="tool-surface ports-toolbar-card" size="small">
      <div class="tool-panel__header ports-toolbar">
        <div class="tool-panel__title">
          <strong>实时连接视图</strong>
          <span class="tool-panel__meta">支持按协议、状态、进程、地址和用户快速筛选。</span>
        </div>

        <div class="tool-inline-actions ports-toolbar__filters">
          <n-input
            v-model:value="searchQuery"
            class="tool-field tool-field--grow"
            placeholder="搜索端口、PID、进程、地址或用户..."
            clearable
          />
          <n-select
            v-model:value="selectedProtocol"
            :options="protocolOptions"
            class="tool-field tool-field--compact"
            placeholder="协议"
            clearable
          />
          <n-select
            v-model:value="selectedStatus"
            :options="statusOptions"
            class="tool-field tool-field--medium"
            placeholder="状态"
            clearable
          />
          <n-button type="primary" @click="refreshPorts()" :loading="loading">刷新</n-button>
          <span class="tool-chip">{{ filteredPorts.length }} 条结果</span>
        </div>
      </div>
    </n-card>

    <n-card class="tool-surface tool-surface--fill ports-table-card" size="small">
      <div
        v-if="!loading && filteredPorts.length > 0"
        class="ports-table-wrap"
        :class="{ 'ports-table-wrap--table': !useCardView }"
      >
        <div v-if="useCardView" class="ports-card-list">
          <article v-for="row in filteredPorts" :key="getRowKey(row)" class="ports-card">
            <div class="ports-card__header">
              <div class="ports-card__title">
                <strong>端口 {{ row.port }}</strong>
                <div class="ports-card__chips">
                  <n-tag :bordered="false" size="small" :type="row.protocol === 'UDP' ? 'info' : 'default'">
                    {{ row.protocol }}
                  </n-tag>
                  <n-tag :bordered="false" size="small" :type="getStatusPresentation(row).type">
                    {{ getStatusPresentation(row).label }}
                  </n-tag>
                </div>
              </div>

              <n-button
                class="ports-card__action"
                size="small"
                type="error"
                ghost
                :disabled="getRowActionState(row).disabled"
                :loading="getRowActionState(row).isKilling"
                @click="killPort(row)"
              >
                {{ getRowActionState(row).label }}
              </n-button>
            </div>

            <div class="ports-card__grid">
              <div class="ports-card__item">
                <span class="tool-value-label">本地地址</span>
                <span class="mono-text ports-card__value">{{ row.local_addr || '-' }}</span>
              </div>
              <div class="ports-card__item">
                <span class="tool-value-label">远程地址</span>
                <span class="mono-text ports-card__value">{{ row.remote_addr || '-' }}</span>
              </div>
              <div class="ports-card__item">
                <span class="tool-value-label">进程名</span>
                <span class="ports-card__value" :class="{ 'ports-card__value--error': row.process_name_unknown }">
                  {{ row.process_name || '-' }}
                </span>
              </div>
              <div class="ports-card__item">
                <span class="tool-value-label">PID</span>
                <span class="mono-text ports-card__value">{{ row.pid }}</span>
              </div>
              <div class="ports-card__item">
                <span class="tool-value-label">用户</span>
                <span class="ports-card__value">{{ row.user || '-' }}</span>
              </div>
            </div>
          </article>
        </div>

        <n-data-table
          v-else
          class="ports-data-table"
          :columns="columns"
          :data="filteredPorts"
          :row-key="getRowKey"
          :bordered="false"
          :single-line="false"
          :max-height="tableMaxHeight"
          :scroll-x="tableScrollX"
        />
      </div>

      <div v-else-if="!loading && ports.length > 0" class="tool-result-stage tool-fill-area">
        <n-empty description="没有符合当前筛选条件的端口记录" />
      </div>

      <div v-else-if="!loading" class="tool-result-stage tool-fill-area">
        <n-empty description="未找到端口占用，点击刷新按钮获取数据" />
      </div>

      <n-flex v-else vertical align="center" justify="center" class="tool-fill-area ports-loading-state">
        <n-spin size="large" />
        <n-text>加载中...</n-text>
      </n-flex>
    </n-card>
  </div>
</template>

<script setup>
import { computed, h, onBeforeUnmount, onMounted, ref } from 'vue'
import { NButton, NInput, NDataTable, NEmpty, NSpin, NTag, NText, NFlex, NSelect, useMessage } from 'naive-ui'
import { invoke } from '@tauri-apps/api/core'

const message = useMessage()

const loading = ref(false)
const ports = ref([])
const searchQuery = ref('')
const killingRows = ref(new Set())
const selectedProtocol = ref(null)
const selectedStatus = ref(null)
const viewportWidth = ref(0)
const viewportHeight = ref(0)

const statusLabelMap = {
  LISTEN: '监听中',
  ESTABLISHED: '已连接',
  TIME_WAIT: '等待中',
  CLOSE_WAIT: '关闭中',
  SYN_SENT: '发起中',
  SYN_RECV: '连接中',
  FIN_WAIT1: '关闭阶段1',
  FIN_WAIT2: '关闭阶段2',
  LAST_ACK: '最后确认',
  CLOSING: '关闭中',
  CLOSE: '已关闭',
  NEW_SYN_RECV: '新连接中'
}

const protocolOptions = [
  { label: 'TCP', value: 'TCP' },
  { label: 'UDP', value: 'UDP' }
]

const statusTagTypeMap = {
  LISTEN: 'success',
  ESTABLISHED: 'info',
  TIME_WAIT: 'warning',
  CLOSE_WAIT: 'warning',
  SYN_SENT: 'warning',
  SYN_RECV: 'warning',
  FIN_WAIT1: 'warning',
  FIN_WAIT2: 'warning',
  LAST_ACK: 'warning',
  CLOSING: 'warning',
  CLOSE: 'default',
  NEW_SYN_RECV: 'warning'
}

const useCardView = computed(() => viewportWidth.value <= 980)
const tableScrollX = 1158
const tableMaxHeight = computed(() => {
  if (useCardView.value) {
    return undefined
  }

  if (!viewportHeight.value) {
    return 520
  }

  return Math.round(Math.min(960, Math.max(360, viewportHeight.value - 320)))
})

const statusOptions = computed(() => {
  const statuses = [...new Set(ports.value.map((port) => port.status).filter(Boolean))].sort((left, right) => left.localeCompare(right))

  return statuses.map((status) => ({
    label: statusLabelMap[status] || status,
    value: status
  }))
})

const normalizeText = (value) => (value || '').toString().toLowerCase()

const getRowKey = (row) => [row.protocol, row.local_addr, row.remote_addr, row.pid, row.port].join('::')

const getKillTarget = (row) => ({
  port: row.port,
  protocol: row.protocol,
  pid: row.pid,
  local_addr: row.local_addr,
  remote_addr: row.remote_addr
})

const getStatusPresentation = (row) => {
  if (row.protocol === 'UDP') {
    return { label: 'UDP', type: 'info' }
  }

  const status = row.status || 'UNKNOWN'
  return {
    label: statusLabelMap[status] || status,
    type: statusTagTypeMap[status] || 'default'
  }
}

const getRowActionState = (row) => {
  const isKilling = killingRows.value.has(getRowKey(row))
  const disabled = row.pid === 0 || isKilling
  const label = row.pid === 0 ? 'PID 缺失' : isKilling ? '终止中...' : '终止进程'

  return {
    isKilling,
    disabled,
    label
  }
}

const syncViewport = () => {
  viewportWidth.value = window.innerWidth
  viewportHeight.value = window.innerHeight
}

const setKillingState = (row, isKilling) => {
  const next = new Set(killingRows.value)
  const key = getRowKey(row)

  if (isKilling) {
    next.add(key)
  } else {
    next.delete(key)
  }

  killingRows.value = next
}

const filteredPorts = computed(() => {
  let result = [...ports.value]

  if (selectedProtocol.value) {
    result = result.filter((port) => port.protocol === selectedProtocol.value)
  }

  if (selectedStatus.value) {
    result = result.filter((port) => port.status === selectedStatus.value)
  }

  if (searchQuery.value.trim()) {
    const query = searchQuery.value.trim().toLowerCase()
    result = result.filter((port) =>
      port.port.toString().includes(query) ||
      port.pid.toString().includes(query) ||
      normalizeText(port.process_name).includes(query) ||
      normalizeText(port.local_addr).includes(query) ||
      normalizeText(port.remote_addr).includes(query) ||
      normalizeText(port.user).includes(query) ||
      normalizeText(port.protocol).includes(query) ||
      normalizeText(port.status).includes(query)
    )
  }

  return result.sort((left, right) => {
    if (left.port !== right.port) {
      return left.port - right.port
    }

    return getRowKey(left).localeCompare(getRowKey(right))
  })
})

const columns = computed(() => [
  {
    title: '端口',
    key: 'port',
    width: 90,
    sorter: (a, b) => a.port - b.port,
    render(row) {
      return h('span', { style: 'font-weight: 700;' }, row.port)
    }
  },
  {
    title: '协议',
    key: 'protocol',
    width: 90,
    render(row) {
      return h(NTag, { bordered: false, size: 'small', type: row.protocol === 'UDP' ? 'info' : 'default' }, { default: () => row.protocol })
    }
  },
  {
    title: '状态',
    key: 'status',
    width: 120,
    render(row) {
      if (row.protocol === 'UDP') {
        return h(NTag, { type: 'info', size: 'small', bordered: false }, { default: () => 'UDP' })
      }

      const status = row.status || 'UNKNOWN'

      return h(
        NTag,
        { type: statusTagTypeMap[status] || 'default', size: 'small', bordered: false },
        { default: () => statusLabelMap[status] || status }
      )
    }
  },
  {
    title: '本地地址',
    key: 'local_addr',
    width: 190,
    ellipsis: { tooltip: true }
  },
  {
    title: '远程地址',
    key: 'remote_addr',
    width: 190,
    ellipsis: { tooltip: true },
    render(row) {
      return row.remote_addr || '-'
    }
  },
  {
    title: '用户',
    key: 'user',
    width: 100,
    ellipsis: { tooltip: true }
  },
  {
    title: '进程名',
    key: 'process_name',
    width: 170,
    ellipsis: { tooltip: true },
    sorter: (a, b) => normalizeText(a.process_name).localeCompare(normalizeText(b.process_name)),
    render(row) {
      if (row.process_name_unknown) {
        return h('span', { style: 'color: #cb5b82;' }, row.process_name)
      }
      return h('span', {}, row.process_name)
    }
  },
  {
    title: 'PID',
    key: 'pid',
    width: 90,
    sorter: (a, b) => a.pid - b.pid
  },
  {
    title: '操作',
    key: 'action',
    width: 118,
    render(row) {
      const { isKilling, disabled, label } = getRowActionState(row)

      return h(
        NButton,
        {
          size: 'small',
          type: 'error',
          ghost: true,
          disabled,
          loading: isKilling,
          onClick: () => killPort(row)
        },
        { default: () => label }
      )
    }
  }
])

const formatError = (error) => {
  if (typeof error === 'string') {
    return error
  }

  if (error && typeof error.message === 'string') {
    return error.message
  }

  return String(error)
}

const refreshPorts = async ({ silent = false } = {}) => {
  loading.value = true
  try {
    const result = await invoke('get_port_list')
    ports.value = Array.isArray(result) ? result : []
    if (!silent) {
      message.success('刷新成功')
    }
  } catch (error) {
    message.error('获取端口列表失败: ' + formatError(error))
  } finally {
    loading.value = false
  }
}

const killPort = async (row) => {
  if (row.pid === 0) {
    message.warning('当前记录缺少 PID，无法终止进程')
    return
  }

  try {
    setKillingState(row, true)
    const result = await invoke('kill_process', { target: getKillTarget(row) })
    message.success(result)
    await refreshPorts({ silent: true })
  } catch (error) {
    message.error('终止进程失败: ' + formatError(error))
  } finally {
    setKillingState(row, false)
  }
}

onMounted(() => {
  syncViewport()
  window.addEventListener('resize', syncViewport)
  refreshPorts({ silent: true })
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', syncViewport)
})
</script>

<style>
.ports-page {
  min-height: 0;
  overflow: auto;
}

.ports-toolbar-card {
  flex-shrink: 0;
}

.ports-toolbar {
  align-items: flex-start;
}

.ports-toolbar__filters {
  flex: 1;
  width: 100%;
}

.ports-table-card {
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.ports-table-wrap {
  flex: 1;
  display: flex;
  flex-direction: column;
  width: 100%;
  min-height: 0;
  overflow: auto;
}

.ports-table-wrap--table {
  overflow: hidden;
}

.ports-table-card .n-card__content {
  display: flex;
  flex: 1;
  min-height: 0;
}

.ports-data-table,
.ports-table-card .n-data-table {
  width: 100%;
  flex: 1;
  min-height: 0;
}

.ports-table-card .n-data-table-base-table-body {
  min-height: 12rem;
}

.ports-loading-state {
  gap: 16px;
}

.ports-card-list {
  display: grid;
  gap: 1rem;
  align-content: start;
}

.ports-card {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  padding: clamp(0.875rem, 1.8vw, 1rem);
  border: 1px solid #e5e7eb;
  background: #fff;
}

.ports-card__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.ports-card__title {
  display: flex;
  flex-direction: column;
  gap: 0.625rem;
}

.ports-card__title strong {
  color: #111827;
  font-size: clamp(1.05rem, 2.8vw, 1.2rem);
}

.ports-card__chips {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.ports-card__grid {
  display: grid;
  gap: 0.875rem;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 12rem), 1fr));
}

.ports-card__item {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
  padding: 0.875rem 1rem;
  background: #f9fafb;
}

.ports-card__value {
  color: #111827;
  overflow-wrap: anywhere;
}

.ports-card__value--error {
  color: #cb5b82;
}

@media (max-width: 980px) {
  .ports-toolbar {
    flex-direction: column;
    align-items: stretch;
  }
}

@media (max-width: 640px) {
  .ports-card__action {
    width: 100%;
  }

  .ports-card__item {
    padding: 0.75rem 0.875rem;
  }
}
</style>
