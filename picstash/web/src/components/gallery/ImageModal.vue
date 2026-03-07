<template>
  <div
    v-if="show"
    class="fixed inset-0 bg-black bg-opacity-90 flex flex-col items-center justify-center z-50 p-4"
    @click="$emit('close')"
  >
    <div class="relative max-w-5xl w-full flex flex-col items-center">
      <img
        :src="image.watermark_url || image.url"
        :alt="image.original_filename"
        class="max-w-full max-h-[70vh] object-contain"
        @click.stop
      />
      <button
        @click="$emit('close')"
        class="absolute top-0 right-0 bg-white text-gray-800 rounded-full w-10 h-10 flex items-center justify-center hover:bg-gray-100"
      >
        ×
      </button>
      <div class="mt-4 bg-black bg-opacity-70 text-white p-4 rounded w-full max-w-5xl" @click.stop>
        <div class="flex flex-wrap items-start justify-between gap-4">
          <div class="flex-1">
            <h3 class="text-lg font-bold">{{ image.original_filename }}</h3>
            <p class="text-sm mt-1 text-gray-300">
              {{ new Date(image.uploaded_at).toLocaleString() }}
            </p>
            <p class="text-sm mt-1">
              原图: {{ image.width }} × {{ image.height }} ({{ formatSize(image.size) }})
            </p>
            <p v-if="image.has_thumbnail" class="text-sm">
              缩略图: {{ image.thumbnail_width }} × {{ image.thumbnail_height }} ({{ formatSize(image.thumbnail_size) }})
            </p>
            <div v-if="!authStore.isAuthenticated && image.tags && image.tags.length > 0" class="mt-3">
              <p class="text-xs text-gray-300 mb-2">标签</p>
              <div class="flex flex-wrap gap-2">
                <span
                  v-for="tag in image.tags"
                  :key="tag.id"
                  class="px-3 py-1 rounded-full text-sm"
                  :style="{ backgroundColor: tag.color, color: 'white' }"
                >
                  {{ tag.name }}
                </span>
              </div>
            </div>
            <div v-if="authStore.isAuthenticated" class="mt-3">
              <TagPicker
                v-model="selectedTagIds"
                label="标签"
                empty-text="暂无标签，可直接新增"
                theme="dark"
              />
            </div>
          </div>
          <div class="flex flex-col gap-2">
            <button
              v-if="showDelete"
              @click="handleDelete"
              :disabled="deleting"
              class="bg-red-600 text-white px-4 py-2 rounded-lg hover:bg-red-700 transition disabled:opacity-50"
            >
              {{ deleting ? '删除中...' : '删除' }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import axios from 'axios'
import { nextTick, ref, watch } from 'vue'
import type { Image } from '@/types'
import { imagesApi } from '@/api'
import { useAuthStore } from '@/stores/auth'
import TagPicker from '@/components/tag/TagPicker.vue'
import { useConfirm } from '@/utils/confirm'
import { useNotifications } from '@/utils/notification'

const props = defineProps<{
  show: boolean
  image: Image
  showDelete?: boolean
  onDeleted?: (id: number) => void
  onTagsUpdated?: (image: Image) => void
}>()

const emit = defineEmits(['close'])

const authStore = useAuthStore()
const { confirmAction } = useConfirm()
const { notifyError } = useNotifications()
const deleting = ref(false)
const selectedTagIds = ref<string[]>([])
let previousImageId: number | null = null
let syncingSelectedTags = false

const formatSize = (bytes?: number) => {
  if (!bytes) return '-'
  const units = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return `${(bytes / Math.pow(1024, i)).toFixed(2)} ${units[i]}`
}

const updateSelectedTags = () => {
  syncingSelectedTags = true
  const currentIds = props.image.tags?.map(t => String(t.id)) || []
  const newIds = JSON.stringify(currentIds)
  const oldIds = JSON.stringify(selectedTagIds.value)
  if (newIds !== oldIds) {
    selectedTagIds.value = currentIds
  }
  nextTick(() => {
    syncingSelectedTags = false
  })
}

const handleSaveTags = async (tagIds: string[]) => {
  const numericIds = tagIds.map(id => parseInt(id))
  try {
    await imagesApi.updateTags(props.image.id, numericIds)

    try {
      const res = await imagesApi.getById(props.image.id)
      props.onTagsUpdated?.(res.data)
    } catch (detailError) {
      console.error('刷新图片详情失败:', detailError)
    }
  } catch (error) {
    console.error('更新标签失败:', error)
    updateSelectedTags()
    notifyError('更新标签失败')
  }
}

const handleDelete = async () => {
  const confirmed = await confirmAction({
    title: '删除这张图片？',
    message: '删除后将无法恢复，图片及相关衍生文件会一起删除。',
    confirmText: '确认删除',
    cancelText: '取消',
    tone: 'danger',
  })

  if (!confirmed) return

  deleting.value = true
  try {
    const res = await imagesApi.delete(props.image.id)

    if (res.data?.error) {
      throw new Error(res.data.error)
    }

    try {
      await imagesApi.getById(props.image.id)
      throw new Error('删除未生效，请稍后重试')
    } catch (verifyError) {
      if (!(axios.isAxiosError(verifyError) && verifyError.response?.status === 404)) {
        throw verifyError
      }
    }
  } catch (error) {
    console.error('删除失败:', error)
    if (axios.isAxiosError(error)) {
      notifyError(error.response?.data?.error || '删除失败')
    } else if (error instanceof Error) {
      notifyError(error.message)
    } else {
      notifyError('删除失败')
    }
    return
  } finally {
    deleting.value = false
  }

  props.onDeleted?.(props.image.id)
  emit('close')
}

watch(() => props.image.id, () => {
  if (props.image.id !== previousImageId) {
    previousImageId = props.image.id
    updateSelectedTags()
  }
})

watch(() => selectedTagIds.value, async (newIds, oldIds) => {
  if (!authStore.isAuthenticated || syncingSelectedTags) {
    return
  }

  if (JSON.stringify(newIds) === JSON.stringify(oldIds)) {
    return
  }

  await handleSaveTags(newIds)
}, { deep: true })

watch(() => props.show, (show) => {
  if (!show) {
    return
  }
  previousImageId = props.image.id
  updateSelectedTags()
}, { immediate: true })

watch(() => props.image.tags, () => {
  if (!props.show || props.image.id !== previousImageId) {
    return
  }
  updateSelectedTags()
}, { deep: true })
</script>
