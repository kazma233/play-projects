import { useEffect, useRef } from "react";
import type { UIEvent } from "react";
import "./session-list.css";
import type { SessionSummary } from "../lib/types";

type SessionListProps = {
  sourceKey: string;
  sessions: SessionSummary[];
  totalCount: number;
  selectedSessionKey: string | null;
  loading: boolean;
  loadingMore: boolean;
  hasMore: boolean;
  onLoadMore: () => void;
  onSelect: (sessionKey: string) => void;
};

function formatTimestamp(timestamp: number | null): string {
  if (!timestamp) {
    return "未知";
  }

  return new Intl.DateTimeFormat("zh-CN", {
    dateStyle: "medium",
    timeStyle: "short"
  }).format(timestamp);
}

function cardClassName(
  transcriptPath: string,
  selectedSessionKey: string | null
): string {
  if (transcriptPath === selectedSessionKey) {
    return "session-card active";
  }

  return "session-card";
}

function footerContent(
  hasMore: boolean,
  loadingMore: boolean,
  sessionCount: number,
  totalCount: number
) {
  if (loadingMore) {
    return (
      <div className="session-list-footer">
        <small>已加载 {sessionCount} / {totalCount}</small>
        <span className="session-list-hint">正在加载更多...</span>
      </div>
    );
  }

  if (hasMore) {
    return (
      <div className="session-list-footer">
        <small>已加载 {sessionCount} / {totalCount}</small>
        <span className="session-list-hint">向下滚动以加载更多</span>
      </div>
    );
  }

  if (totalCount > 0) {
    return (
      <div className="session-list-footer complete">
        <small>共加载 {sessionCount} 条会话</small>
      </div>
    );
  }

  return null;
}

export function SessionList(props: SessionListProps) {
  const {
    hasMore,
    loading,
    loadingMore,
    onLoadMore,
    onSelect,
    selectedSessionKey,
    sessions,
    sourceKey,
    totalCount
  } = props;
  const listRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (listRef.current) {
      listRef.current.scrollTop = 0;
    }
  }, [sourceKey]);

  useEffect(() => {
    const listElement = listRef.current;

    if (!listElement || loading || loadingMore || !hasMore) {
      return;
    }

    if (listElement.scrollHeight <= listElement.clientHeight + 32) {
      onLoadMore();
    }
  }, [hasMore, loading, loadingMore, onLoadMore, sessions.length, sourceKey]);

  function handleScroll(event: UIEvent<HTMLDivElement>) {
    if (loading || loadingMore || !hasMore) {
      return;
    }

    const listElement = event.currentTarget;
    const remainingDistance =
      listElement.scrollHeight - listElement.scrollTop - listElement.clientHeight;

    if (remainingDistance < 240) {
      onLoadMore();
    }
  }

  if (loading && sessions.length === 0) {
    return (
      <div className="panel-shell session-list-shell">
        <div className="session-list" aria-busy="true">
          {Array.from({ length: 6 }, (_, index) => (
            <div key={index} className="session-card skeleton-card">
              <span className="skeleton-line skeleton-title" />
              <span className="skeleton-line" />
              <span className="skeleton-line skeleton-short" />
              <div className="session-meta-row">
                <span className="skeleton-line skeleton-meta" />
                <span className="skeleton-line skeleton-meta" />
              </div>
            </div>
          ))}
        </div>
      </div>
    );
  }

  if (sessions.length === 0) {
    return (
      <div className="session-list-shell">
        <div className="empty-state">该来源下暂无会话。</div>
      </div>
    );
  }

  return (
    <div className="panel-shell session-list-shell">
      <div
        ref={listRef}
        className="session-list"
        aria-busy={loading || loadingMore}
        onScroll={handleScroll}
      >
        {sessions.map((session) => (
          <button
            key={session.transcriptPath}
            className={cardClassName(session.transcriptPath, selectedSessionKey)}
            onClick={() => onSelect(session.transcriptPath)}
            type="button"
          >
            <strong>{session.title}</strong>
            <span>{session.cwd ?? "无工作目录"}</span>
            <span>{session.gitBranch ?? "无分支信息"}</span>
            <div className="session-meta-row">
              <small>{formatTimestamp(session.updatedAt)}</small>
            </div>
          </button>
        ))}
        {footerContent(hasMore, loadingMore, sessions.length, totalCount)}
      </div>
      {loading ? (
        <div className="panel-loading-overlay">
          <div className="loading-pill">正在刷新会话...</div>
        </div>
      ) : null}
    </div>
  );
}
