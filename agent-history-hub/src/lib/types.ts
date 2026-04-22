export type SourceApp = "codex" | "claude_code" | "opencode";

export type SourceStatus = {
  app: SourceApp;
  available: boolean;
  rootPath: string | null;
  sessionCount: number;
  note: string | null;
};

export type SessionSummary = {
  sourceApp: SourceApp;
  sourceSessionId: string;
  title: string;
  cwd: string | null;
  gitBranch: string | null;
  transcriptPath: string;
  createdAt: number | null;
  updatedAt: number | null;

};

export type SessionPage = {
  sourceApp: SourceApp;
  sessions: SessionSummary[];
  offset: number;
  limit: number;
  nextOffset: number | null;
  totalCount: number;
  hasMore: boolean;
};

export type ContentBlock = {
  kind: string;
  text: string | null;
  toolName: string | null;
  toolCallId: string | null;
  payload: unknown | null;
};

export type SessionMessage = {
  id: string;
  role: string;
  timestamp: number | null;
  blocks: ContentBlock[];
  sessionId?: string;
};

export type SessionEvent = {
  id: string;
  kind: string;
  timestamp: number | null;
  summary: string;
  payload: unknown | null;
  sessionId?: string;
};

export type SessionAgent = {
  sessionId: string;
  label: string;
  isRoot: boolean;
};

export type SessionDetail = {
  summary: SessionSummary;
  sourcePaths: string[];
  messageCount: number | null;
  eventCount: number | null;
  agents: SessionAgent[];
};

export type SessionMessagePage = {
  messages: SessionMessage[];
  offset: number;
  limit: number;
  nextOffset: number | null;
  totalCount: number;
  hasMore: boolean;
};

export type SessionEventPage = {
  events: SessionEvent[];
  offset: number;
  limit: number;
  nextOffset: number | null;
  totalCount: number;
  hasMore: boolean;
};

export type ImportPreview = {
  sourceApp: SourceApp;
  sourceSessionId: string;
  targetApp: SourceApp;
  supported: boolean;
  importLevel: "full" | "partial" | "unsupported";
  warnings: string[];
  createdPaths: string[];
  backupPaths: string[];
};

export type ImportResult = {
  targetApp: SourceApp;
  createdSessionId: string;
  createdPaths: string[];
  backupPaths: string[];
  resumeCwd: string | null;
  warnings: string[];
};
