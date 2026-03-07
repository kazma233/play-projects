<template>
  <main class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
    <div v-if="authStore.homeAuth && !authStore.isAuthenticated" class="flex flex-col items-center justify-center py-20">
      <div class="text-center">
        <svg xmlns="http://www.w3.org/2000/svg" class="h-20 w-20 text-gray-300 mx-auto mb-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
        </svg>
        <h2 class="text-2xl font-semibold text-gray-700 mb-2">请登录后查看</h2>
        <p class="text-gray-500 mb-8">登录后即可浏览所有图片内容</p>
        <router-link
          to="/login"
          class="inline-flex items-center px-6 py-3 bg-primary text-white rounded-lg hover:bg-blue-600 transition font-medium"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 mr-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 16l-4-4m0 0l4-4m-4 4h14m-5 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h7a3 3 0 013 3v1" />
          </svg>
          前往登录
        </router-link>
      </div>
    </div>

    <template v-else>
      <div class="mb-6 flex flex-col gap-4 rounded-2xl bg-white p-4 shadow-sm ring-1 ring-black/5 md:flex-row md:items-end md:justify-between">
        <div class="flex-1">
          <div class="flex items-center gap-2 text-sm font-medium text-gray-700">
            <span>标签筛选</span>
            <span v-if="tagsLoading" class="text-xs font-normal text-gray-400">更新中...</span>
          </div>
          <div class="mt-3 flex flex-wrap gap-2">
            <button
              @click="clearTagFilter"
              class="rounded-full px-3 py-1.5 text-sm transition"
              :class="selectedTagId === null ? 'bg-gray-900 text-white shadow-sm' : 'bg-gray-100 text-gray-600 hover:bg-gray-200'"
            >
              全部
            </button>
            <button
              v-for="tag in tags"
              :key="tag.id"
              @click="toggleTagFilter(tag.id)"
              class="rounded-full px-3 py-1.5 text-sm text-white transition"
              :class="selectedTagId === tag.id ? 'scale-[1.02] shadow-sm ring-2 ring-offset-2 ring-gray-200' : 'opacity-70 hover:opacity-100'"
              :style="{ backgroundColor: tag.color }"
            >
              {{ tag.name }}
            </button>
          </div>
        </div>

        <div class="text-sm text-gray-500">
          <span v-if="initialLoading">正在整理图片...</span>
          <span v-else-if="images.length > 0">已加载 {{ images.length }} / {{ total }} 张</span>
          <span v-else>等待内容加载</span>
        </div>
      </div>

      <div v-if="initialLoading && images.length === 0" class="py-16 text-center text-gray-500">
        正在加载图片...
      </div>

      <div v-else-if="error && images.length === 0" class="rounded-2xl border border-red-100 bg-red-50 px-6 py-10 text-center">
        <p class="text-base font-medium text-red-600">{{ error }}</p>
        <button
          @click="refresh"
          class="mt-4 rounded-full bg-white px-5 py-2 text-sm font-medium text-red-600 shadow-sm ring-1 ring-red-200 transition hover:bg-red-100"
        >
          重试
        </button>
      </div>

      <div v-else-if="!initialLoading && initialized && images.length === 0" class="text-center py-16">
        <p class="text-gray-500 text-lg">暂无图片</p>
        <router-link v-if="authStore.isAuthenticated" to="/upload" class="text-primary hover:underline mt-4 inline-block">
          去上传第一张图片
        </router-link>
      </div>

      <template v-else>
        <div class="grid grid-cols-1 items-start gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
          <div
            v-for="(column, columnIndex) in imageColumns"
            :key="`column-${columnIndex}`"
            class="flex min-w-0 flex-col gap-4"
          >
            <article
              v-for="item in column"
              :key="item.id"
              class="group cursor-pointer overflow-hidden rounded-2xl bg-white shadow-sm ring-1 ring-black/5 transition duration-200 hover:-translate-y-1 hover:shadow-xl"
              @click="selectImage(item)"
            >
              <div class="relative bg-gray-100">
                <img
                  :src="item.thumbnail_url || item.url"
                  :alt="item.original_filename"
                  :width="getReservedDimensions(item).width"
                  :height="getReservedDimensions(item).height"
                  loading="lazy"
                  decoding="async"
                  class="block h-auto w-full"
                />
                <button
                  v-if="authStore.isAuthenticated"
                  @click.stop="copyImageUrl(item.url)"
                  class="absolute right-3 top-3 rounded-full bg-black/55 p-2 text-white opacity-0 shadow-sm backdrop-blur-sm transition group-hover:opacity-100 hover:bg-black/70"
                  title="复制图片链接"
                >
                  <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" />
                  </svg>
                </button>
              </div>

              <div class="p-3">
                <p class="truncate text-sm font-medium text-gray-800">{{ item.original_filename }}</p>
                <p class="mt-1 text-xs text-gray-400">{{ formatImageMeta(item) }}</p>

                <div v-if="item.tags && item.tags.length > 0" class="mt-3 flex flex-wrap gap-1.5">
                  <span
                    v-for="tag in item.tags.slice(0, 4)"
                    :key="tag.id"
                    class="rounded-full px-2 py-1 text-xs text-white"
                    :style="{ backgroundColor: tag.color }"
                  >
                    {{ tag.name }}
                  </span>
                </div>
              </div>
            </article>
          </div>
        </div>

        <div v-if="images.length > 0" class="pt-2">
          <div v-if="hasMore" ref="loadMoreTrigger" class="h-8 w-full" aria-hidden="true"></div>

          <div v-if="loadingNext" class="py-4 text-center text-sm text-gray-500">
            正在加载更多图片...
          </div>

          <div v-else-if="hasMore" class="py-4 text-center">
            <button
              @click="loadNextPage"
              class="rounded-full border border-gray-200 bg-white px-5 py-2 text-sm font-medium text-gray-700 shadow-sm transition hover:border-gray-300 hover:text-gray-900"
            >
              继续加载
            </button>
          </div>

          <div v-else class="py-6 text-center text-sm text-gray-400">
            已经到底了，共 {{ total }} 张图片
          </div>

          <div v-if="error && images.length > 0" class="pb-2 text-center text-sm text-red-500">
            {{ error }}，可继续重试。
          </div>
        </div>
      </template>
    </template>
  </main>

  <ImageModal
    v-if="selectedImage"
    :show="true"
    :image="selectedImage"
    :showDelete="authStore.isAuthenticated"
    :onDeleted="handleImageDeleted"
    :onTagsUpdated="handleImageUpdated"
    @close="selectedImage = null"
  />
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { imagesApi, tagsApi } from '@/api'
import type { Image, Tag } from '@/types'
import ImageModal from '@/components/gallery/ImageModal.vue'
import { useImageFeed } from '@/utils/imageFeed'
import { useNotifications } from '@/utils/notification'

const authStore = useAuthStore()
const { notifyError, notifySuccess } = useNotifications()
const tags = ref<Tag[]>([])
const tagsLoading = ref(false)
const selectedTagId = ref<number | null>(null)
const selectedImage = ref<Image | null>(null)
const loadMoreTrigger = ref<HTMLElement | null>(null)
const viewportWidth = ref(typeof window !== 'undefined' ? window.innerWidth : 1440)
const canViewGallery = computed(() => !authStore.homeAuth || authStore.isAuthenticated)

const {
  items: images,
  total,
  hasMore,
  initialLoading,
  loadingNext,
  initialized,
  error,
  refresh,
  loadNextPage,
  removeImage,
  updateImage,
} = useImageFeed({
  enabled: canViewGallery,
  tagId: selectedTagId,
  onError: notifyError,
})

const extractTags = (payload: unknown): Tag[] => {
  if (Array.isArray(payload)) {
    return payload as Tag[]
  }

  if (
    payload &&
    typeof payload === 'object' &&
    Array.isArray((payload as { data?: unknown }).data)
  ) {
    return (payload as { data: Tag[] }).data
  }

  return []
}

const loadTags = async () => {
  if (!canViewGallery.value) {
    tags.value = []
    return
  }

  tagsLoading.value = true
  try {
    const res = await tagsApi.getAll()
    tags.value = extractTags(res.data)
  } catch (loadError) {
    console.error('加载标签失败:', loadError)
    notifyError('加载标签失败')
  } finally {
    tagsLoading.value = false
  }
}

const toggleTagFilter = (tagId: number) => {
  selectedImage.value = null
  selectedTagId.value = selectedTagId.value === tagId ? null : tagId
}

const clearTagFilter = () => {
  selectedImage.value = null
  selectedTagId.value = null
}

const getReservedDimensions = (image: Image) => {
  const width = image.thumbnail_width ?? image.width ?? 4
  const height = image.thumbnail_height ?? image.height ?? 3

  return {
    width: width > 0 ? width : 4,
    height: height > 0 ? height : 3,
  }
}

const estimateCardHeight = (image: Image) => {
  const { width, height } = getReservedDimensions(image)
  const ratio = height / width
  const tagRows = image.tags && image.tags.length > 0 ? 1 : 0

  return ratio + 0.42 + tagRows * 0.12
}

const columnCount = computed(() => {
  if (viewportWidth.value >= 1280) {
    return 4
  }
  if (viewportWidth.value >= 1024) {
    return 3
  }
  if (viewportWidth.value >= 640) {
    return 2
  }
  return 1
})

const imageColumns = computed(() => {
  const totalColumns = columnCount.value
  const columns = Array.from({ length: totalColumns }, () => [] as Image[])
  const columnHeights = Array.from({ length: totalColumns }, () => 0)

  for (const image of images.value) {
    let targetColumn = 0
    for (let index = 1; index < totalColumns; index += 1) {
      if (columnHeights[index] < columnHeights[targetColumn]) {
        targetColumn = index
      }
    }

    columns[targetColumn].push(image)
    columnHeights[targetColumn] += estimateCardHeight(image)
  }

  return columns
})

const formatImageMeta = (image: Image) => {
  const width = image.width ?? image.thumbnail_width
  const height = image.height ?? image.thumbnail_height
  const uploadedAt = new Date(image.uploaded_at).toLocaleDateString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
  })

  if (width && height && width > 0 && height > 0) {
    return `${width} × ${height} · ${uploadedAt}`
  }

  return uploadedAt
}

const selectImage = async (image: Image) => {
  try {
    const res = await imagesApi.getById(image.id)
    selectedImage.value = res.data
  } catch (loadError) {
    console.error('获取图片详情失败:', loadError)
    notifyError('获取图片详情失败')
  }
}

const handleImageDeleted = (id: number) => {
  removeImage(id)
  if (selectedImage.value?.id === id) {
    selectedImage.value = null
  }
}

const handleImageUpdated = (image: Image) => {
  updateImage(image)
  if (selectedImage.value?.id === image.id) {
    selectedImage.value = image
  }
  void loadTags()
}

const copyImageUrl = async (url: string) => {
  try {
    await navigator.clipboard.writeText(url)
    notifySuccess('已复制链接')
  } catch (copyError) {
    console.error('复制失败:', copyError)
    notifyError('复制失败')
  }
}

let loadMoreObserver: IntersectionObserver | null = null

const syncViewportWidth = () => {
  viewportWidth.value = window.innerWidth
}

const disconnectLoadMoreObserver = () => {
  if (!loadMoreObserver) {
    return
  }

  loadMoreObserver.disconnect()
  loadMoreObserver = null
}

const connectLoadMoreObserver = (element: HTMLElement | null) => {
  disconnectLoadMoreObserver()

  if (!element || typeof IntersectionObserver === 'undefined') {
    return
  }

  loadMoreObserver = new IntersectionObserver(
    (entries) => {
      if (entries.some((entry) => entry.isIntersecting)) {
        void loadNextPage()
      }
    },
    { rootMargin: '360px 0px' },
  )

  loadMoreObserver.observe(element)
}

watch(
  canViewGallery,
  (enabled) => {
    if (!enabled) {
      tags.value = []
      tagsLoading.value = false
      selectedTagId.value = null
      selectedImage.value = null
      disconnectLoadMoreObserver()
      return
    }

    void loadTags()
  },
  { immediate: true },
)

watch(
  loadMoreTrigger,
  (element) => {
    if (!canViewGallery.value) {
      disconnectLoadMoreObserver()
      return
    }

    connectLoadMoreObserver(element)
  },
  { flush: 'post' },
)

onBeforeUnmount(() => {
  disconnectLoadMoreObserver()
  window.removeEventListener('resize', syncViewportWidth)
})

onMounted(() => {
  syncViewportWidth()
  window.addEventListener('resize', syncViewportWidth, { passive: true })
})
</script>
