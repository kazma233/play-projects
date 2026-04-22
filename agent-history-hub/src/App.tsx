import { useEffect, useRef, useState } from "react";
import "./app-shell.css";
import { SessionDetail } from "./components/session-detail";
import { SessionList } from "./components/session-list";
import { SourceSwitcher } from "./components/source-switcher";
import {
  clearSessionCaches,
  detectSources,
  getSessionOverview,
  listSessions
} from "./lib/api";
import { useAppStore } from "./lib/store";
import type { SourceApp } from "./lib/types";

const SESSION_PAGE_SIZE = 20;

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

function pickInitialSource(
  requestedSource: SourceApp,
  availableSources: ReturnType<typeof useAppStore.getState>["sources"]
): SourceApp {
  const requested = availableSources.find(
    (source) => source.app === requestedSource && source.available
  );

  if (requested) {
    return requested.app;
  }

  return availableSources.find((source) => source.available)?.app ?? "codex";
}

export default function App() {
  const contentPanelRef = useRef<HTMLElement | null>(null);
  const detailRequestVersionRef = useRef(0);
  const {
    error,
    loadingDetail,
    loadingSessions,
    loadingSources,
    selectedSessionKey,
    selectedSource,
    sessionDetail,
    sessions,
    appendSessions,
    setError,
    setLoadingDetail,
    setLoadingSessions,
    setLoadingSources,
    setSelectedSessionKey,
    setSelectedSource,
    setSessionDetail,
    setSessions,
    setSources,
    sources
  } = useAppStore();
  const [sessionTotalCount, setSessionTotalCount] = useState(0);
  const [nextSessionOffset, setNextSessionOffset] = useState(0);
  const [hasMoreSessions, setHasMoreSessions] = useState(false);
  const [loadingMoreSessions, setLoadingMoreSessions] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [reloadToken, setReloadToken] = useState(0);
  const selectedSummary = selectedSessionKey
    ? sessions.find((session) => session.transcriptPath === selectedSessionKey) ?? null
    : null;

  useEffect(() => {
    async function bootstrap() {
      setLoadingSources(true);
      setError(null);

      try {
        const nextSources = await detectSources();
        setSources(nextSources);
        setSelectedSource(pickInitialSource(selectedSource, nextSources));
      } catch (bootstrapError) {
        setError(errorMessage(bootstrapError, "加载来源失败。"));
      } finally {
        setLoadingSources(false);
      }
    }

    void bootstrap();
  }, []);

  useEffect(() => {
    let cancelled = false;

    async function loadSessions() {
      setLoadingSessions(true);
      setLoadingMoreSessions(false);
      setError(null);
      setSessionTotalCount(0);
      setNextSessionOffset(0);
      setHasMoreSessions(false);

      try {
        const nextPage = await listSessions(selectedSource, {
          offset: 0,
          limit: SESSION_PAGE_SIZE,
          refresh: true
        });

        if (cancelled) {
          return;
        }

        setSessions(nextPage.sessions);
        setSessionTotalCount(nextPage.totalCount);
        setNextSessionOffset(nextPage.nextOffset ?? nextPage.sessions.length);
        setHasMoreSessions(nextPage.hasMore);

        const currentSelectedSessionKey = useAppStore.getState().selectedSessionKey;
        const hasCurrent = nextPage.sessions.some(
          (session) => session.transcriptPath === currentSelectedSessionKey
        );

        if (hasCurrent) {
          setSelectedSessionKey(currentSelectedSessionKey);
        } else {
          setSelectedSessionKey(null);
          setSessionDetail(null);
        }
      } catch (loadError) {
        if (cancelled) {
          return;
        }

        setError(errorMessage(loadError, "加载会话列表失败。"));
      } finally {
        if (!cancelled) {
          setLoadingSessions(false);
        }
      }
    }

    if (sources.length > 0) {
      void loadSessions();
    }

    return () => {
      cancelled = true;
    };
  }, [reloadToken, selectedSource, sources.length]);

  async function handleLoadMoreSessions() {
    if (loadingSessions || loadingMoreSessions || !hasMoreSessions) {
      return;
    }

    const sourceAtRequest = selectedSource;
    const offsetAtRequest = nextSessionOffset;
    setLoadingMoreSessions(true);

    try {
      const nextPage = await listSessions(sourceAtRequest, {
        offset: offsetAtRequest,
        limit: SESSION_PAGE_SIZE,
        refresh: false
      });

      if (useAppStore.getState().selectedSource !== sourceAtRequest) {
        return;
      }

      appendSessions(nextPage.sessions);
      setSessionTotalCount(nextPage.totalCount);
      setNextSessionOffset(nextPage.nextOffset ?? offsetAtRequest + nextPage.sessions.length);
      setHasMoreSessions(nextPage.hasMore);
    } catch (loadError) {
      if (useAppStore.getState().selectedSource !== sourceAtRequest) {
        return;
      }

      setError(errorMessage(loadError, "加载更多会话失败。"));
    } finally {
      if (useAppStore.getState().selectedSource === sourceAtRequest) {
        setLoadingMoreSessions(false);
      }
    }
  }

  async function handleRefresh() {
    if (loadingSources || loadingSessions || loadingMoreSessions || loadingDetail || refreshing) {
      return;
    }

    setRefreshing(true);
    setError(null);
    detailRequestVersionRef.current += 1;

    try {
      await clearSessionCaches();
      const nextSources = await detectSources();
      const nextSelectedSource = pickInitialSource(selectedSource, nextSources);
      setSources(nextSources);

      if (nextSelectedSource !== selectedSource) {
        setSelectedSource(nextSelectedSource);
      }

      setReloadToken((current) => current + 1);
    } catch (refreshError) {
      setError(errorMessage(refreshError, "刷新会话失败。"));
    } finally {
      setRefreshing(false);
    }
  }

  useEffect(() => {
    let cancelled = false;
    const requestVersion = ++detailRequestVersionRef.current;

    async function loadDetail() {
      if (!selectedSessionKey) {
        setSessionDetail(null);
        return;
      }

      if (!selectedSummary) {
        setSessionDetail(null);
        return;
      }

      setLoadingDetail(true);
      setError(null);

      try {
        const nextDetail = await getSessionOverview(
          selectedSource,
          selectedSummary.sourceSessionId,
          selectedSummary.transcriptPath
        );

        if (cancelled || detailRequestVersionRef.current !== requestVersion) {
          return;
        }

        setSessionDetail(nextDetail);
      } catch (loadError) {
        if (cancelled || detailRequestVersionRef.current !== requestVersion) {
          return;
        }

        setError(errorMessage(loadError, "加载会话详情失败。"));
      } finally {
        if (!cancelled && detailRequestVersionRef.current === requestVersion) {
          setLoadingDetail(false);
        }
      }
    }

    void loadDetail();

    return () => {
      cancelled = true;
    };
  }, [
    selectedSessionKey,
    selectedSource,
    selectedSummary?.sourceSessionId,
    selectedSummary?.transcriptPath
  ]);

  return (
    <main className="app-shell">
      <aside className="sidebar">
        <div className="sidebar-fixed">
          <header className="sidebar-header">
            <div>
              <h1>Agent Session Hub</h1>
            </div>
            <button
              className="secondary-button sidebar-refresh-button"
              disabled={refreshing || loadingSources || loadingSessions || loadingMoreSessions || loadingDetail}
              onClick={() => void handleRefresh()}
              type="button"
            >
              {refreshing ? "刷新中..." : "刷新"}
            </button>
          </header>
          <SourceSwitcher
            onChange={setSelectedSource}
            selectedSource={selectedSource}
            sources={sources}
          />
          {loadingSources || error ? (
            <div className="sidebar-status">
              {loadingSources ? <p className="muted-text">正在检测来源...</p> : null}
              {error ? <p className="error-text">{error}</p> : null}
            </div>
          ) : null}
        </div>
        <SessionList
          hasMore={hasMoreSessions}
          loading={loadingSessions}
          loadingMore={loadingMoreSessions}
          onLoadMore={handleLoadMoreSessions}
          onSelect={setSelectedSessionKey}
          selectedSessionKey={selectedSessionKey}
          sourceKey={selectedSource}
          sessions={sessions}
          totalCount={sessionTotalCount}
        />
      </aside>
      <section className="content-panel" ref={contentPanelRef}>
        <SessionDetail
          detail={sessionDetail}
          loading={loadingDetail}
          scrollContainerRef={contentPanelRef}
        />
      </section>
    </main>
  );
}
