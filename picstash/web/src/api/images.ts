import { api } from "./client";
import type {
  ApiResponse,
  CursorPaginatedResponse,
  Image,
  SyncStartResult,
} from "@/types";

export const imagesApi = {
  getList: (
    params: { cursor?: string; limit?: number; tag_id?: number },
    signal?: AbortSignal,
  ) =>
    api.get<CursorPaginatedResponse<Image>>(
      "/images",
      { params, signal },
    ),

  getById: (id: number) => api.get<Image>(`/images/${id}`),

  upload: (formData: FormData, onProgress?: (percent: number) => void) =>
    api.post<ApiResponse<{ data: Image[]; count: number }>>(
      "/images/upload",
      formData,
      {
        headers: { "Content-Type": "multipart/form-data" },
        timeout: 120000,
        onUploadProgress: (e) => {
          const percent = Math.round((e.loaded * 100) / (e.total || 1));
          onProgress?.(percent);
        },
      },
    ),

  delete: (id: number) => api.delete<ApiResponse>(`/images/${id}`),

  updateTags: (id: number, tagIds: number[]) =>
    api.put<ApiResponse>(`/images/${id}/tags`, { tag_ids: tagIds }),

  sync: () => api.post<ApiResponse<SyncStartResult>>("/images/sync"),
};
