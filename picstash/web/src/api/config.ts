import { api } from "./client";

export interface ConfigResponse {
  home_auth: boolean;
}

export const configApi = {
  getConfig: () => api.get<ConfigResponse>("/config"),
};
