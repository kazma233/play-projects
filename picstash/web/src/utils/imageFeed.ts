import { onBeforeUnmount, ref, watch } from "vue";
import type { Ref } from "vue";
import { imagesApi } from "@/api";
import type { CursorPaginatedResponse, Image } from "@/types";

interface UseImageFeedOptions {
  enabled: Ref<boolean>;
  tagId: Ref<number | null>;
  pageSize?: number;
  onError?: (message: string) => void;
}

const DEFAULT_PAGE_SIZE = 20;

const isCanceledRequest = (error: unknown) => {
  if (!error || typeof error !== "object") {
    return false;
  }

  const requestError = error as { code?: string; name?: string };
  return (
    requestError.code === "ERR_CANCELED" ||
    requestError.name === "CanceledError" ||
    requestError.name === "AbortError"
  );
};

const mergeImages = (current: Image[], incoming: Image[]) => {
  const merged = [...current];
  const seen = new Set(current.map((image) => image.id));

  for (const image of incoming) {
    if (seen.has(image.id)) {
      continue;
    }

    merged.push(image);
    seen.add(image.id);
  }

  return merged;
};

export const useImageFeed = (options: UseImageFeedOptions) => {
  const items = ref<Image[]>([]);
  const total = ref(0);
  const nextCursor = ref<string | null>(null);
  const hasMore = ref(true);
  const initialLoading = ref(false);
  const loadingNext = ref(false);
  const initialized = ref(false);
  const error = ref("");

  let activeController: AbortController | null = null;
  let activeRequestId = 0;

  const clearState = () => {
    items.value = [];
    total.value = 0;
    nextCursor.value = null;
    hasMore.value = true;
    initialLoading.value = false;
    loadingNext.value = false;
    initialized.value = false;
    error.value = "";
  };

  const cancelActiveRequest = () => {
    activeRequestId += 1;
    activeController?.abort();
    activeController = null;
  };

  const fetchPage = async (reset: boolean) => {
    if (!options.enabled.value) {
      return;
    }

    if (!reset && (initialLoading.value || loadingNext.value || !hasMore.value)) {
      return;
    }

    if (reset) {
      cancelActiveRequest();
      items.value = [];
      total.value = 0;
      nextCursor.value = null;
      hasMore.value = true;
      initialized.value = false;
      error.value = "";
      initialLoading.value = true;
      loadingNext.value = false;
    } else {
      loadingNext.value = true;
      error.value = "";
    }

    const requestId = activeRequestId + 1;
    activeRequestId = requestId;

    const controller = new AbortController();
    activeController = controller;

    try {
      const res = await imagesApi.getList(
        {
          cursor: reset ? undefined : nextCursor.value ?? undefined,
          limit: options.pageSize ?? DEFAULT_PAGE_SIZE,
          tag_id: options.tagId.value ?? undefined,
        },
        controller.signal,
      );

      if (requestId !== activeRequestId) {
        return;
      }

      const payload = res.data as CursorPaginatedResponse<Image>;
      const incomingItems = payload.data || [];

      items.value = reset ? incomingItems : mergeImages(items.value, incomingItems);
      total.value = typeof payload.total === "number" ? payload.total : items.value.length;
      nextCursor.value = payload.next_cursor ?? null;
      hasMore.value = Boolean(payload.has_more && payload.next_cursor);
      initialized.value = true;
    } catch (requestError) {
      if (requestId !== activeRequestId || isCanceledRequest(requestError)) {
        return;
      }

      error.value = "加载图片失败";
      options.onError?.(error.value);

      if (reset) {
        items.value = [];
        total.value = 0;
        nextCursor.value = null;
        hasMore.value = false;
        initialized.value = true;
      }
    } finally {
      if (requestId !== activeRequestId) {
        return;
      }

      if (activeController === controller) {
        activeController = null;
      }

      initialLoading.value = false;
      loadingNext.value = false;
    }
  };

  const refresh = async () => {
    if (!options.enabled.value) {
      cancelActiveRequest();
      clearState();
      return;
    }

    await fetchPage(true);
  };

  const loadNextPage = async () => {
    await fetchPage(false);
  };

  const removeImage = (imageId: number) => {
    const nextItems = items.value.filter((image) => image.id !== imageId);
    if (nextItems.length === items.value.length) {
      return;
    }

    items.value = nextItems;
    total.value = Math.max(0, total.value - 1);
  };

  const updateImage = (updatedImage: Image) => {
    const index = items.value.findIndex((image) => image.id === updatedImage.id);
    if (index === -1) {
      return;
    }

    const updatedTags = updatedImage.tags || [];
    const activeTagId = options.tagId.value;

    if (activeTagId !== null && !updatedTags.some((tag) => tag.id === activeTagId)) {
      removeImage(updatedImage.id);
      return;
    }

    const nextItems = [...items.value];
    nextItems[index] = {
      ...nextItems[index],
      ...updatedImage,
      tags: updatedTags,
    };
    items.value = nextItems;
  };

  watch(
    [options.enabled, options.tagId],
    ([enabled]) => {
      if (!enabled) {
        cancelActiveRequest();
        clearState();
        return;
      }

      void fetchPage(true);
    },
    { immediate: true },
  );

  onBeforeUnmount(() => {
    cancelActiveRequest();
  });

  return {
    items,
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
  };
};
