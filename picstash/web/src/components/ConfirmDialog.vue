<template>
  <Transition name="confirm-fade">
    <div
      v-if="confirmState.open"
      class="fixed inset-0 z-[110] flex items-center justify-center bg-slate-950/40 px-4"
      @click="cancelConfirm"
    >
      <div
        class="w-full max-w-md rounded-3xl bg-white p-6 shadow-2xl ring-1 ring-black/5"
        @click.stop
      >
        <div class="flex items-start gap-4">
          <div
            class="mt-0.5 flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl text-white"
            :class="iconToneClass"
          >
            <svg
              v-if="confirmState.tone === 'danger'"
              xmlns="http://www.w3.org/2000/svg"
              class="h-5 w-5"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v3m0 3h.01M5.07 19h13.86c1.54 0 2.5-1.67 1.73-3L13.73 4c-.77-1.33-2.69-1.33-3.46 0L3.34 16c-.77 1.33.19 3 1.73 3z" />
            </svg>
            <svg
              v-else
              xmlns="http://www.w3.org/2000/svg"
              class="h-5 w-5"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
          </div>

          <div class="min-w-0 flex-1">
            <h2 class="text-lg font-semibold text-gray-900">{{ confirmState.title }}</h2>
            <p v-if="confirmState.message" class="mt-2 text-sm leading-6 text-gray-500">
              {{ confirmState.message }}
            </p>
          </div>
        </div>

        <div class="mt-6 flex justify-end gap-3">
          <button
            type="button"
            class="rounded-full border border-gray-200 bg-white px-4 py-2 text-sm font-medium text-gray-600 transition hover:border-gray-300 hover:text-gray-900"
            @click="cancelConfirm"
          >
            {{ confirmState.cancelText }}
          </button>
          <button
            type="button"
            class="rounded-full px-4 py-2 text-sm font-medium text-white transition"
            :class="confirmToneClass"
            @click="acceptConfirm"
          >
            {{ confirmState.confirmText }}
          </button>
        </div>
      </div>
    </div>
  </Transition>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { useConfirm } from "@/utils/confirm";

const { confirmState, acceptConfirm, cancelConfirm } = useConfirm();

const confirmToneClass = computed(() =>
  confirmState.value.tone === "danger"
    ? "bg-red-600 hover:bg-red-700"
    : "bg-gray-900 hover:bg-black",
);

const iconToneClass = computed(() =>
  confirmState.value.tone === "danger" ? "bg-red-500" : "bg-gray-900",
);
</script>

<style scoped>
.confirm-fade-enter-active,
.confirm-fade-leave-active {
  transition: opacity 0.18s ease;
}

.confirm-fade-enter-active > div,
.confirm-fade-leave-active > div {
  transition: transform 0.18s ease, opacity 0.18s ease;
}

.confirm-fade-enter-from,
.confirm-fade-leave-to {
  opacity: 0;
}

.confirm-fade-enter-from > div,
.confirm-fade-leave-to > div {
  opacity: 0;
  transform: translateY(8px) scale(0.98);
}
</style>
