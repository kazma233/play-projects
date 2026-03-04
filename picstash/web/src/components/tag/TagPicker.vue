<template>
  <div>
    <div class="flex items-center justify-between mb-2">
      <label :class="theme === 'dark' ? 'text-gray-200' : 'text-gray-700'" class="block">
        {{ label }}
      </label>
      <div class="flex gap-2" v-if="!disabled">
        <button
          @click="loadTags(true)"
          type="button"
          class="p-1.5 rounded-lg flex items-center justify-center cursor-pointer transition"
          :class="theme === 'dark' ? 'bg-white/10 hover:bg-white/20' : 'bg-gray-100 hover:bg-gray-200'"
          title="刷新标签"
          :disabled="refreshing"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" :class="theme === 'dark' ? 'text-gray-200' : 'text-gray-600'" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
        </button>
        <button
          @click="toggleCreateForm"
          type="button"
          class="p-1.5 rounded-lg flex items-center justify-center cursor-pointer transition"
          :class="theme === 'dark' ? 'bg-white/10 hover:bg-white/20' : 'bg-gray-100 hover:bg-gray-200'"
          title="新增标签"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" :class="theme === 'dark' ? 'text-gray-200' : 'text-gray-600'" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
          </svg>
        </button>
      </div>
    </div>

    <div
      v-if="showCreateForm"
      class="mb-3 p-3 border rounded-lg"
      :class="theme === 'dark' ? 'bg-white/5 border-white/10' : 'bg-gray-50 border-gray-200'"
    >
      <form @submit.prevent="handleCreateTag" class="flex flex-col sm:flex-row gap-2">
        <input
          v-model.trim="newTagName"
          type="text"
          placeholder="输入标签名称"
          class="flex-1 px-3 py-2 border rounded-lg"
          :class="theme === 'dark' ? 'bg-white/10 border-white/20 text-white placeholder:text-gray-300' : 'bg-white border-gray-300 text-gray-900'"
          :disabled="creating"
        />
        <div class="flex gap-2">
          <input
            v-model="newTagColor"
            type="color"
            class="w-12 h-10 border rounded-lg cursor-pointer"
            :class="theme === 'dark' ? 'border-white/20 bg-white/10' : 'border-gray-300 bg-white'"
            :disabled="creating"
          />
          <button
            type="submit"
            :disabled="creating"
            class="bg-primary text-white px-3 py-2 rounded-lg hover:bg-blue-600 transition disabled:opacity-50 cursor-pointer"
          >
            {{ creating ? '创建中...' : '创建并选中' }}
          </button>
        </div>
      </form>
      <p v-if="createError" class="mt-2 text-xs" :class="theme === 'dark' ? 'text-red-300' : 'text-red-500'">
        {{ createError }}
      </p>
    </div>

    <div v-if="loading" class="text-sm py-2" :class="theme === 'dark' ? 'text-gray-300' : 'text-gray-500'">
      加载标签中...
    </div>

    <div v-else-if="tags.length === 0" class="text-sm py-2" :class="theme === 'dark' ? 'text-gray-300' : 'text-gray-400'">
      {{ emptyText }}
    </div>

    <div v-else class="flex flex-wrap gap-2">
      <button
        v-for="tag in tags"
        :key="tag.id"
        type="button"
        @click="toggleTag(tag.id)"
        class="px-3 py-1.5 rounded-full text-sm transition border-2 cursor-pointer"
        :disabled="disabled"
        :class="isSelected(tag.id)
          ? 'border-transparent text-white'
          : (theme === 'dark'
            ? 'border-gray-500 text-gray-200 hover:border-gray-300'
            : 'border-gray-300 text-gray-700 hover:border-gray-400')"
        :style="isSelected(tag.id) ? { backgroundColor: tag.color } : {}"
      >
        {{ tag.name }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { tagsApi } from '@/api'
import type { Tag } from '@/types'

const props = withDefaults(defineProps<{
  modelValue?: string[]
  label?: string
  emptyText?: string
  theme?: 'light' | 'dark'
  disabled?: boolean
}>(), {
  modelValue: () => [],
  label: '标签',
  emptyText: '暂无标签，可直接创建',
  theme: 'light',
  disabled: false,
})

const emit = defineEmits<{
  (e: 'update:modelValue', value: string[]): void
}>()

const tags = ref<Tag[]>([])
const loading = ref(false)
const refreshing = ref(false)
const creating = ref(false)
const showCreateForm = ref(false)
const newTagName = ref('')
const newTagColor = ref('#3B82F6')
const createError = ref('')

const selectedIds = computed({
  get: () => props.modelValue || [],
  set: (value: string[]) => emit('update:modelValue', value),
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

const extractTag = (payload: unknown): Tag | null => {
  if (!payload || typeof payload !== 'object') {
    return null
  }

  const directTag = payload as Partial<Tag> & { data?: unknown }
  if (typeof directTag.id === 'number' && typeof directTag.name === 'string') {
    return directTag as Tag
  }

  if (directTag.data && typeof directTag.data === 'object') {
    const nestedTag = directTag.data as Partial<Tag>
    if (typeof nestedTag.id === 'number' && typeof nestedTag.name === 'string') {
      return nestedTag as Tag
    }
  }

  return null
}

const loadTags = async (silent = false) => {
  if (silent) {
    refreshing.value = true
  } else {
    loading.value = true
  }

  try {
    const res = await tagsApi.getAll()
    tags.value = extractTags(res.data)
  } catch (error) {
    console.error('加载标签失败:', error)
  } finally {
    refreshing.value = false
    loading.value = false
  }
}

const isSelected = (tagId: number) => selectedIds.value.includes(String(tagId))

const selectTag = (tagId: number) => {
  const idStr = String(tagId)
  if (selectedIds.value.includes(idStr)) {
    return
  }
  selectedIds.value = [...selectedIds.value, idStr]
}

const toggleTag = (tagId: number) => {
  if (props.disabled) {
    return
  }

  const idStr = String(tagId)
  const index = selectedIds.value.indexOf(idStr)
  if (index > -1) {
    selectedIds.value = selectedIds.value.filter((id) => id !== idStr)
  } else {
    selectedIds.value = [...selectedIds.value, idStr]
  }
}

const toggleCreateForm = () => {
  showCreateForm.value = !showCreateForm.value
  createError.value = ''
  if (!showCreateForm.value) {
    newTagName.value = ''
  }
}

const handleCreateTag = async () => {
  const name = newTagName.value.trim()
  if (!name) {
    createError.value = '请输入标签名称'
    return
  }

  const normalizedName = name.toLowerCase()
  const existingTag = tags.value.find((tag) => tag.name.trim().toLowerCase() === normalizedName)
  if (existingTag) {
    selectTag(existingTag.id)
    createError.value = ''
    showCreateForm.value = false
    newTagName.value = ''
    return
  }

  creating.value = true
  createError.value = ''
  try {
    const res = await tagsApi.create({
      name,
      color: newTagColor.value,
    })

    const createdTag = extractTag(res.data)
    await loadTags(true)

    const targetTag = createdTag || tags.value.find((tag) => tag.name.trim().toLowerCase() === normalizedName)
    if (targetTag) {
      selectTag(targetTag.id)
    }

    showCreateForm.value = false
    newTagName.value = ''
  } catch (error: any) {
    if (error?.response?.status === 409) {
      await loadTags(true)
      const existing = tags.value.find((tag) => tag.name.trim().toLowerCase() === normalizedName)
      if (existing) {
        selectTag(existing.id)
        showCreateForm.value = false
        newTagName.value = ''
        createError.value = ''
        return
      }
    }

    createError.value = error.response?.data?.error || '创建标签失败'
  } finally {
    creating.value = false
  }
}

onMounted(async () => {
  await loadTags()
})
</script>
