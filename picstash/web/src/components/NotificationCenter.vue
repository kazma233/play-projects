<template>
  <div class="pointer-events-none fixed right-4 top-4 z-[100] flex w-[min(24rem,calc(100vw-2rem))] flex-col gap-3">
    <TransitionGroup name="notification">
      <div
        v-for="notification in notifications"
        :key="notification.id"
        class="pointer-events-auto overflow-hidden rounded-2xl border bg-white shadow-lg ring-1 ring-black/5"
        :class="notificationClassMap[notification.type]"
      >
        <div class="flex items-start gap-3 px-4 py-3">
          <div class="mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded-full text-white" :class="iconClassMap[notification.type]">
            <svg v-if="notification.type === 'success'" xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
            </svg>
            <svg v-else-if="notification.type === 'error'" xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M5.07 19h13.86c1.54 0 2.5-1.67 1.73-3L13.73 4c-.77-1.33-2.69-1.33-3.46 0L3.34 16c-.77 1.33.19 3 1.73 3z" />
            </svg>
            <svg v-else xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
          </div>

          <div class="min-w-0 flex-1">
            <p class="text-sm font-medium text-gray-800">{{ notification.message }}</p>
          </div>

          <button
            type="button"
            class="rounded-full p-1 text-gray-400 transition hover:bg-black/5 hover:text-gray-600"
            @click="removeNotification(notification.id)"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
      </div>
    </TransitionGroup>
  </div>
</template>

<script setup lang="ts">
import { useNotifications } from "@/utils/notification";

const { notifications, removeNotification } = useNotifications();

const notificationClassMap = {
  success: "border-emerald-100",
  error: "border-red-100",
  info: "border-sky-100",
};

const iconClassMap = {
  success: "bg-emerald-500",
  error: "bg-red-500",
  info: "bg-sky-500",
};
</script>

<style scoped>
.notification-enter-active,
.notification-leave-active {
  transition: all 0.2s ease;
}

.notification-enter-from,
.notification-leave-to {
  opacity: 0;
  transform: translateY(-8px) translateX(8px);
}
</style>
