import { api } from "./client";
import type { SyncLog, SyncFileLog, PaginatedResponse } from "@/types";

export const syncApi = {
  getLogs: (params: { page?: number; limit?: number }) =>
    api.get<PaginatedResponse<SyncLog>>("/sync/logs", { params }),

  getFileLogs: (id: number) =>
    api.get<{ data: SyncFileLog[] }>(`/sync/logs/${id}/files`),
};
