import { invoke } from "@tauri-apps/api/core";
import type {
  ImportResult,
  ImportPreview,
  SessionPage,
  SessionEventPage,
  SessionDetail,
  SessionMessagePage,
  SourceApp,
  SourceStatus
} from "./types";

export function detectSources(): Promise<SourceStatus[]> {
  return invoke("detect_sources");
}

export function clearSessionCaches(): Promise<void> {
  return invoke("clear_session_caches");
}

type ListSessionsInput = {
  offset?: number;
  limit?: number;
  refresh?: boolean;
};

export function listSessions(
  sourceApp: SourceApp,
  input: ListSessionsInput = {}
): Promise<SessionPage> {
  return invoke("list_sessions", {
    sourceApp,
    offset: input.offset,
    limit: input.limit,
    refresh: input.refresh
  });
}

export function getSessionOverview(
  sourceApp: SourceApp,
  sourceSessionId: string,
  transcriptPath?: string
): Promise<SessionDetail> {
  return invoke("get_session_overview", { sourceApp, sourceSessionId, transcriptPath });
}

type GetSessionItemsInput = {
  offset?: number;
  limit?: number;
  transcriptPath?: string;
};

export function getSessionMessages(
  sourceApp: SourceApp,
  sourceSessionId: string,
  input: GetSessionItemsInput = {}
): Promise<SessionMessagePage> {
  return invoke("get_session_messages", {
    sourceApp,
    sourceSessionId,
    transcriptPath: input.transcriptPath,
    offset: input.offset,
    limit: input.limit
  });
}

export function getSessionEvents(
  sourceApp: SourceApp,
  sourceSessionId: string,
  input: GetSessionItemsInput = {}
): Promise<SessionEventPage> {
  return invoke("get_session_events", {
    sourceApp,
    sourceSessionId,
    transcriptPath: input.transcriptPath,
    offset: input.offset,
    limit: input.limit
  });
}

export function previewImport(
  sourceApp: SourceApp,
  sourceSessionId: string,
  targetApp: SourceApp,
  transcriptPath?: string
): Promise<ImportPreview> {
  return invoke("preview_import", {
    sourceApp,
    sourceSessionId,
    targetApp,
    transcriptPath
  });
}

export function importSession(
  sourceApp: SourceApp,
  sourceSessionId: string,
  targetApp: SourceApp,
  transcriptPath?: string
): Promise<ImportResult> {
  return invoke("import_session", {
    sourceApp,
    sourceSessionId,
    targetApp,
    transcriptPath
  });
}
