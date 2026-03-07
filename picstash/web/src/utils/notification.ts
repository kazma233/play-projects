import { readonly, ref } from "vue";

export type NotificationType = "success" | "error" | "info";

export interface NotificationItem {
  id: number;
  type: NotificationType;
  message: string;
}

const notifications = ref<NotificationItem[]>([]);

let nextNotificationId = 1;
const timers = new Map<number, ReturnType<typeof setTimeout>>();

const defaultDurations: Record<NotificationType, number> = {
  success: 2600,
  error: 4200,
  info: 3200,
};

const removeNotification = (id: number) => {
  const timer = timers.get(id);
  if (timer) {
    clearTimeout(timer);
    timers.delete(id);
  }

  notifications.value = notifications.value.filter(
    (notification) => notification.id !== id,
  );
};

const notify = (
  message: string,
  type: NotificationType = "info",
  duration = defaultDurations[type],
) => {
  const id = nextNotificationId++;

  notifications.value = [
    ...notifications.value,
    {
      id,
      type,
      message,
    },
  ];

  if (duration > 0) {
    const timer = setTimeout(() => {
      removeNotification(id);
    }, duration);

    timers.set(id, timer);
  }

  return id;
};

export const useNotifications = () => ({
  notifications: readonly(notifications),
  notify,
  notifySuccess: (message: string, duration?: number) =>
    notify(message, "success", duration),
  notifyError: (message: string, duration?: number) =>
    notify(message, "error", duration),
  notifyInfo: (message: string, duration?: number) =>
    notify(message, "info", duration),
  removeNotification,
});
