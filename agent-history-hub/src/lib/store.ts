import { create } from "zustand";
import type {
  SessionDetail,
  SessionSummary,
  SourceApp,
  SourceStatus
} from "./types";

type AppState = {
  sources: SourceStatus[];
  selectedSource: SourceApp;
  sessions: SessionSummary[];
  selectedSessionKey: string | null;
  sessionDetail: SessionDetail | null;
  loadingSources: boolean;
  loadingSessions: boolean;
  loadingDetail: boolean;
  error: string | null;
  setSources: (sources: SourceStatus[]) => void;
  setSelectedSource: (source: SourceApp) => void;
  setSessions: (sessions: SessionSummary[]) => void;
  appendSessions: (sessions: SessionSummary[]) => void;
  setSelectedSessionKey: (sessionKey: string | null) => void;
  setSessionDetail: (detail: SessionDetail | null) => void;
  setLoadingSources: (value: boolean) => void;
  setLoadingSessions: (value: boolean) => void;
  setLoadingDetail: (value: boolean) => void;
  setError: (error: string | null) => void;
};

export const useAppStore = create<AppState>((set) => ({
  sources: [],
  selectedSource: "codex",
  sessions: [],
  selectedSessionKey: null,
  sessionDetail: null,
  loadingSources: false,
  loadingSessions: false,
  loadingDetail: false,
  error: null,
  setSources: (sources) => set({ sources }),
  setSelectedSource: (selectedSource) =>
    set({
      selectedSource,
      sessions: [],
      selectedSessionKey: null,
      sessionDetail: null
    }),
  setSessions: (sessions) => set({ sessions }),
  appendSessions: (incomingSessions) =>
    set((state) => {
      const sessionsById = new Map(
        state.sessions.map((session) => [session.transcriptPath, session])
      );

      for (const session of incomingSessions) {
        sessionsById.set(session.transcriptPath, session);
      }

      return { sessions: Array.from(sessionsById.values()) };
    }),
  setSelectedSessionKey: (selectedSessionKey) => set({ selectedSessionKey }),
  setSessionDetail: (sessionDetail) => set({ sessionDetail }),
  setLoadingSources: (loadingSources) => set({ loadingSources }),
  setLoadingSessions: (loadingSessions) => set({ loadingSessions }),
  setLoadingDetail: (loadingDetail) => set({ loadingDetail }),
  setError: (error) => set({ error })
}));
