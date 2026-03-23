<template>
  <main class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
    <div class="mb-6">
      <h1 class="text-2xl font-bold text-gray-900">存储同步</h1>
      <p class="mt-1 text-sm text-gray-500">确认后会触发一次同步任务，下面可以查看同步历史和文件明细。</p>
    </div>

    <div class="mb-6 rounded-lg border border-gray-200 bg-white p-4">
      <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <div class="text-base font-medium text-gray-900">手动触发同步</div>
          <p class="mt-1 text-sm text-gray-500">
            会扫描当前存储中的图片文件，并更新数据库记录及同步日志。
          </p>
        </div>
        <div class="flex flex-col gap-3 sm:flex-row">
          <button
            @click="refreshLogs"
            :disabled="loading"
            class="rounded-lg border border-gray-300 px-4 py-2 text-gray-700 transition hover:border-gray-400 hover:text-gray-900 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {{ loading ? '刷新中...' : '刷新记录' }}
          </button>
          <button
            @click="handleSync"
            :disabled="syncing"
            class="rounded-lg bg-gray-900 px-4 py-2 text-white transition hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {{ syncing ? '提交中...' : '开始同步' }}
          </button>
        </div>
      </div>

      <div v-if="syncTask" class="mt-4 rounded-lg bg-blue-50 px-4 py-3 text-sm text-blue-900">
        <p class="font-medium">
          {{ syncTask.started ? '同步任务已创建' : '已有同步任务在进行中' }}
        </p>
        <p class="mt-1">
          日志 ID: {{ syncTask.log_id }}。可以点击“刷新记录”查看最新进度，并展开文件处理明细。
        </p>
      </div>
    </div>

    <div v-if="loading" class="text-center py-8">
      加载中...
    </div>

    <div v-else>
      <div class="mb-4 text-sm font-medium text-gray-700">
        同步历史
      </div>

      <div v-for="log in syncLogs" :key="log.id" class="border rounded-lg mb-4 p-4 hover:shadow-md transition-shadow">
        <div class="flex justify-between items-start mb-2">
          <div>
            <p class="text-sm font-medium text-gray-900">日志 ID: {{ log.id }}</p>
            <p class="text-sm text-gray-500">{{ log.triggered_by }}</p>
            <p class="text-xs text-gray-400">
              {{ formatTime(log.started_at) }} - {{ log.completed_at ? formatTime(log.completed_at) : '进行中...' }}
            </p>
          </div>
          <span
            :class="{
              'text-green-600': log.status === 'completed',
              'text-yellow-600': log.status === 'completed_with_errors',
              'text-red-600': log.status === 'failed',
              'text-blue-600': log.status === 'running',
            }"
            class="text-sm font-medium"
          >
            {{ getStatusText(log.status) }}
          </span>
        </div>

        <div class="text-sm text-gray-600 mb-3">
          处理 {{ log.processed_files }}/{{ log.total_files }} 个文件，
          <span v-if="log.error_count > 0" class="text-red-600">{{ log.error_count }} 个失败</span>
          <span v-else class="text-green-600">0 个失败</span>
        </div>

        <button
          @click="toggleFileDetails(log.id)"
          class="text-primary hover:underline text-sm"
        >
          {{ selectedLogId === log.id ? '收起文件详情' : '查看文件详情' }}
        </button>

        <div v-if="selectedLogId === log.id" class="mt-3 pl-4 border-l-2 border-gray-200">
          <div v-if="fileLogsLoading" class="text-center py-4 text-gray-500">
            加载文件详情...
          </div>
          <div v-else>
            <div v-if="fileLogs.length === 0" class="text-gray-500 text-sm py-2">
              暂无文件记录
            </div>
            <div v-else>
              <div
                v-for="file in fileLogs"
                :key="file.id"
                class="text-sm py-2 border-b border-gray-100 last:border-0"
              >
                <div class="flex items-start gap-2">
                  <span
                    :class="{
                      'text-green-600': file.status === 'success',
                      'text-red-600': file.status === 'failed',
                    }"
                    class="font-mono text-xs"
                  >
                    [{{ getActionText(file.action) }}]
                  </span>
                  <span class="flex-1 break-all">{{ file.path }}</span>
                </div>

                <div v-if="file.old_sha && file.sha && file.sha !== file.old_sha" class="ml-6 text-xs text-gray-500 mt-1">
                  SHA: {{ file.old_sha.slice(0, 7) }} → {{ file.sha.slice(0, 7) }}
                </div>

                <div v-if="file.old_size !== null && file.size !== null" class="ml-6 text-xs text-gray-500 mt-1">
                  大小: {{ formatSize(file.old_size) }} → {{ formatSize(file.size) }}
                </div>

                <div v-if="file.error_message" class="ml-6 text-red-600 text-xs mt-1">
                  错误: {{ file.error_message }}
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div v-if="!loading && syncLogs.length === 0" class="text-center py-16">
        <p class="text-gray-500 text-lg">暂无同步日志</p>
        <router-link to="/upload" class="text-primary hover:underline mt-4 inline-block">
          去上传页面
        </router-link>
      </div>

      <div v-if="!noMore && syncLogs.length > 0" class="text-center py-6">
        <button
          @click="loadMore"
          :disabled="loadingMore"
          class="bg-primary text-white px-6 py-2 rounded-lg hover:bg-blue-600 transition disabled:opacity-50"
        >
          {{ loadingMore ? '加载中...' : '加载更多' }}
        </button>
      </div>

      <div v-if="noMore" class="text-center py-8 text-gray-500">
        没有更多日志了
      </div>
    </div>
  </main>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { imagesApi } from '@/api'
import { syncApi } from '@/api/sync'
import type { SyncLog, SyncFileLog, PaginatedResponse, SyncStartResult } from '@/types'
import { useConfirm } from '@/utils/confirm'
import { useNotifications } from '@/utils/notification'

const { confirmAction } = useConfirm()
const { notifyError, notifyInfo, notifySuccess } = useNotifications()
const syncLogs = ref<SyncLog[]>([])
const fileLogs = ref<SyncFileLog[]>([])
const selectedLogId = ref<number | null>(null)
const syncTask = ref<SyncStartResult | null>(null)
const syncing = ref(false)
const loading = ref(false)
const loadingMore = ref(false)
const fileLogsLoading = ref(false)
const noMore = ref(false)
const page = ref(1)
const limit = 20

const loadLogs = async (loadMore = false) => {
  if (loading.value || loadingMore.value || (loadMore && noMore.value)) return

  if (loadMore) {
    loadingMore.value = true
    page.value++
  } else {
    loading.value = true
    page.value = 1
  }

  try {
    const res = await syncApi.getLogs({ page: page.value, limit })
    const result = res.data as PaginatedResponse<SyncLog>

    if (result?.data && result.data.length > 0) {
      if (loadMore) {
        syncLogs.value.push(...result.data)
      } else {
        syncLogs.value = result.data
      }

      if (result.data.length < limit) {
        noMore.value = true
      } else {
        noMore.value = false
      }
    } else {
      if (!loadMore) {
        syncLogs.value = []
      }
      noMore.value = true
    }
  } catch (error) {
    console.error('加载同步日志失败:', error)
    notifyError('加载失败')
  } finally {
    loading.value = false
    loadingMore.value = false
  }
}

const loadMore = () => loadLogs(true)
const refreshLogs = () => loadLogs()

const handleSync = async () => {
  const confirmed = await confirmAction({
    title: '开始从存储同步？',
    message: '同步会扫描当前存储中的文件，并写入新的同步日志。',
    confirmText: '确认同步',
    cancelText: '取消',
  })

  if (!confirmed) return

  syncing.value = true
  try {
    const res = await imagesApi.sync()
    syncTask.value = res.data.data as SyncStartResult
    if (syncTask.value.started) {
      notifySuccess('同步任务已开始')
    } else {
      notifyInfo('已有同步任务在进行中')
    }
    selectedLogId.value = null
    fileLogs.value = []
    await loadLogs()
  } catch (error) {
    console.error('同步失败:', error)
    notifyError('同步失败')
  } finally {
    syncing.value = false
  }
}

const toggleFileDetails = async (logId: number) => {
  if (selectedLogId.value === logId) {
    selectedLogId.value = null
    fileLogs.value = []
    return
  }

  selectedLogId.value = logId
  fileLogsLoading.value = true

  try {
    const res = await syncApi.getFileLogs(logId)
    fileLogs.value = res.data.data || []
  } catch (error) {
    console.error('加载文件详情失败:', error)
    notifyError('加载文件详情失败')
  } finally {
    fileLogsLoading.value = false
  }
}

const formatTime = (isoString: string) => {
  const date = new Date(isoString)
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

const formatSize = (bytes: number | null) => {
  if (!bytes) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return Math.round(bytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i]
}

const getStatusText = (status: string) => {
  const statusMap: Record<string, string> = {
    running: '进行中',
    completed: '完成',
    completed_with_errors: '完成（有错误）',
    failed: '失败',
  }
  return statusMap[status] || status
}

const getActionText = (action: string) => {
  const actionMap: Record<string, string> = {
    created: '新增',
    updated: '更新',
    deleted: '删除',
    skipped: '跳过',
  }
  return actionMap[action] || action
}

onMounted(() => {
  loadLogs()
})
</script>
