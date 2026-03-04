import { api } from "./client";
import type { ApiResponse } from "@/types";

export const tagsApi = {
  getAll: () => api.get<ApiResponse<any[]>>("/tags"),

  create: (data: { name: string; color: string }) =>
    api.post<ApiResponse<any>>("/tags", data),

  update: (id: number, data: { name?: string; color?: string }) =>
    api.put<ApiResponse<any>>(`/tags/${id}`, data),

  delete: (id: number) => api.delete<ApiResponse>(`/tags/${id}`),
};
