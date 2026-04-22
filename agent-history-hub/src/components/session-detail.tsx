import { useEffect, useMemo, useRef, useState } from "react";
import type { RefObject } from "react";
import "./session-detail.css";
import {
  getSessionEvents,
  getSessionMessages,
  importSession,
  previewImport
} from "../lib/api";
import type {
  SessionAgent,
  ImportResult,
  ImportPreview,
  SessionEvent,
  SessionDetail as SessionDetailValue,
  SessionMessage,
  SourceApp
} from "../lib/types";

type SessionDetailProps = {
  detail: SessionDetailValue | null;
  loading: boolean;
  scrollContainerRef: RefObject<HTMLElement | null>;
};

const DETAIL_PAGE_SIZE = 40;
const CONVERSATION_BLOCK_KINDS = new Set(["text", "input_text", "output_text"]);
const IMPORT_TARGET_APPS: SourceApp[] = ["codex", "claude_code", "opencode"];

const APP_LABELS: Record<SourceApp, string> = {
  codex: "Codex",
  claude_code: "Claude Code",
  opencode: "OpenCode"
};

type ImportTargetCopy = {
  optionLabel: string;
  methodLabel: string;
  methodDescription: string;
  successTitle: string;
  successNote: string | null;
  manualLabel: string;
};

const IMPORT_TARGET_COPY: Record<SourceApp, ImportTargetCopy> = {
  codex: {
    optionLabel: "Codex · 兼容写入",
    methodLabel: "兼容写入",
    methodDescription: "生成 Codex 兼容的 transcript JSONL。",
    successTitle: "已写入 Codex transcript",
    successNote: "已通过 transcript 读回校验。",
    manualLabel: "手动恢复命令"
  },
  claude_code: {
    optionLabel: "Claude Code · 兼容写入",
    methodLabel: "兼容写入",
    methodDescription: "生成 Claude Code 项目目录下的会话 JSONL。",
    successTitle: "已导入到 Claude Code",
    successNote: null,
    manualLabel: "手动打开命令"
  },
  opencode: {
    optionLabel: "OpenCode · 官方导入",
    methodLabel: "官方导入",
    methodDescription: "通过 OpenCode 官方 CLI 导入。",
    successTitle: "已导入到 OpenCode",
    successNote: null,
    manualLabel: "手动打开命令"
  }
};

function defaultImportTarget(sourceApp: SourceApp): SourceApp {
  return IMPORT_TARGET_APPS.find((app) => app !== sourceApp) ?? sourceApp;
}

function isNearBottom(element: HTMLElement, threshold = 240): boolean {
  return element.scrollHeight - element.scrollTop - element.clientHeight <= threshold;
}

function formatTimestamp(timestamp: number | null): string {
  if (!timestamp) {
    return "未知";
  }

  return new Intl.DateTimeFormat("zh-CN", {
    dateStyle: "medium",
    timeStyle: "short"
  }).format(timestamp);
}

function renderPayload(value: unknown): string | null {
  if (value === null || value === undefined) {
    return null;
  }

  return JSON.stringify(value, null, 2);
}

function formatAppName(app: SourceApp): string {
  return APP_LABELS[app] ?? app;
}

function normalizeFilterValue(value: string): string {
  return value.trim().toLocaleLowerCase("zh-CN");
}

function textMatchesFilter(value: string | null | undefined, filter: string): boolean {
  if (filter.length === 0 || !value) {
    return filter.length === 0;
  }

  return normalizeFilterValue(value).includes(filter);
}

function importTargetCopy(app: SourceApp): ImportTargetCopy {
  return IMPORT_TARGET_COPY[app];
}

function formatImportLevel(value: ImportPreview["importLevel"]): string {
  switch (value) {
    case "full":
      return "完整导入";
    case "partial":
      return "部分导入";
    case "unsupported":
      return "不支持导入";
    default:
      return value;
  }
}

function translateWarning(warning: string): string {
  switch (warning) {
    case "The source session has no importable messages.":
      return "源会话中没有可导入的消息。";
    case "OpenCode support is limited to source detection until real transcript samples are available.":
      return "在拿到真实转录样本前，OpenCode 目前只支持来源检测。";
    case "The current MVP only exposes cross-program import. Same-app cloning is intentionally disabled.":
      return "当前 MVP 仅支持跨程序导入，暂不支持同程序克隆。";
    case "Only the normalized message timeline is imported. Side-channel runtime events are skipped.":
      return "仅导入标准化后的消息时间线，旁路运行事件会被跳过。";
    case "The target program may render the imported conversation differently from the original UI.":
      return "目标程序对导入会话的显示效果，可能与原始界面不同。";
    case "This import path creates a brand-new session file, so backup paths are usually empty.":
      return "该导入方式会创建全新的会话文件，因此备份路径通常为空。";
    case "The source session does not expose a cwd, so the importer will fall back to the home directory.":
      return "源会话未提供工作目录，导入器将回退到用户主目录。";
    case "Non-message events are not recreated in the target program.":
      return "非消息类事件不会在目标程序中重建。";
    case "Codex function calls are mapped into Claude tool_use/tool_result records on a best-effort basis.":
      return "Codex 的函数调用会尽力映射为 Claude 的 tool_use/tool_result 记录。";
    case "Tool calls are mapped into Claude tool_use/tool_result records on a best-effort basis.":
      return "工具调用会尽力映射为 Claude 的 tool_use/tool_result 记录。";
    case "Codex import writes transcript JSONL only. SQLite state is expected to be populated lazily by Codex when the session is resumed.":
      return "导入到 Codex 时只会写入 transcript JSONL；SQLite 状态预计会在恢复会话时由 Codex 延迟补齐。";
    case "Codex import writes transcript JSONL only. Agent Session Hub validates the written transcript, but Codex still needs to resume the session once before its internal SQLite state is populated.":
      return "导入到 Codex 时只会写入 transcript JSONL；本工具只校验写入后的 transcript 可读，仍需要在 Codex 中至少恢复一次会话。恢复时建议显式带上工作目录，避免 Codex 询问使用当前目录还是程序目录。";
    case "OpenCode import target is not supported yet. Export from OpenCode is supported.":
      return "暂不支持导入到 OpenCode，但已支持从 OpenCode 导出到其他程序。";
    case "OpenCode import is delegated to the official CLI importer using generated session JSON.":
      return "导入到 OpenCode 将通过官方 CLI 的 import 命令完成，使用生成的会话 JSON。";
    default:
      return warning;
  }
}

function blockContentText(block: SessionMessage["blocks"][number]): string | null {
  if (block.text) {
    return block.text;
  }

  return renderPayload(block.payload);
}

function blockNeedsExpand(block: SessionMessage["blocks"][number]): boolean {
  const content = blockContentText(block);

  if (!content) {
    return false;
  }

  return content.split("\n").length > 10 || content.length > 800;
}

function messageNeedsExpand(message: SessionMessage): boolean {
  return message.blocks.some((block) => blockNeedsExpand(block));
}

function isSubagentMarkerMessage(message: SessionMessage): boolean {
  return message.blocks.some((block) => {
    if (typeof block.payload !== "object" || block.payload === null) {
      return false;
    }

    const payload = block.payload as Record<string, unknown>;
    return payload.type === "subagent_started";
  });
}

function sessionIdFromPayload(value: unknown): string | null {
  if (typeof value !== "object" || value === null) {
    return null;
  }

  const sessionId = Reflect.get(value, "session_id");
  return typeof sessionId === "string" ? sessionId : null;
}

function extractSubagentLabel(message: SessionMessage): string | null {
  for (const block of message.blocks) {
    if (typeof block.payload !== "object" || block.payload === null) {
      continue;
    }
    const payload = block.payload as Record<string, unknown>;
    if (payload.type === "subagent_started") {
      const title = typeof payload.title === "string" ? payload.title : null;
      const id = typeof payload.session_id === "string" ? payload.session_id : message.sessionId ?? null;
      if (title) {
        return title;
      }

      if (id) {
        return `Sub-agent ${id.slice(0, 8)}`;
      }
    }
  }
  return null;
}

function isSubagentLifecycleEvent(event: SessionEvent): boolean {
  return event.kind === "subagent_started" || event.kind === "subagent_spawned" || event.kind === "subagent_closed" || event.kind === "subagent_notification";
}

function eventSessionId(event: SessionEvent): string | null {
  return event.sessionId ?? sessionIdFromPayload(event.payload);
}

function messageMatchesAgent(
  message: SessionMessage,
  selectedAgentId: string | "all",
  rootSessionId: string
): boolean {
  if (selectedAgentId === "all") {
    return true;
  }

  if (message.sessionId) {
    return message.sessionId === selectedAgentId;
  }

  return selectedAgentId === rootSessionId;
}

function eventMatchesAgent(
  event: SessionEvent,
  selectedAgentId: string | "all",
  rootSessionId: string
): boolean {
  if (selectedAgentId === "all") {
    return true;
  }

  const sessionId = eventSessionId(event);

  if (sessionId) {
    return sessionId === selectedAgentId;
  }

  return selectedAgentId === rootSessionId;
}

function messageTags(message: SessionMessage): Array<{ key: string; label: string; kind: "block" | "tool" }> {
  const seen = new Set<string>();
  const tags: Array<{ key: string; label: string; kind: "block" | "tool" }> = [];

  for (const block of message.blocks) {
    const blockKey = `block:${block.kind}`;

    if (!seen.has(blockKey)) {
      seen.add(blockKey);
      tags.push({
        key: blockKey,
        label: formatBlockKind(block.kind),
        kind: "block"
      });
    }

    if (block.toolName) {
      const toolKey = `tool:${block.toolName}`;

      if (!seen.has(toolKey)) {
        seen.add(toolKey);
        tags.push({
          key: toolKey,
          label: block.toolName,
          kind: "tool"
        });
      }
    }
  }

  return tags;
}

function formatBlockKind(kind: string): string {
  switch (kind) {
    case "text":
      return "文本";
    case "reasoning":
      return "思考";
    case "tool":
      return "工具";
    case "patch":
      return "补丁";
    case "file":
      return "文件";
    case "input_text":
      return "输入文本";
    case "output_text":
      return "输出文本";
    default:
      return kind;
  }
}

function shellQuote(value: string): string {
  return `'${value.replace(/'/g, `'"'"'`)}'`;
}

function codexResumeCommand(sessionId: string, cwd: string | null): string {
  if (!cwd) {
    return `codex resume ${sessionId}`;
  }

  return `codex resume -C ${shellQuote(cwd)} ${sessionId}`;
}

function claudeResumeCommand(sessionId: string, cwd: string | null): string {
  if (!cwd) {
    return `claude --resume ${sessionId}`;
  }

  return `cd ${shellQuote(cwd)} && claude --resume ${sessionId}`;
}

function opencodeResumeCommand(sessionId: string, cwd: string | null): string {
  if (!cwd) {
    return `opencode --session ${sessionId}`;
  }

  return `cd ${shellQuote(cwd)} && opencode --session ${sessionId}`;
}

function manualOpenCommand(targetApp: SourceApp, sessionId: string, cwd: string | null): string {
  switch (targetApp) {
    case "codex":
      return codexResumeCommand(sessionId, cwd);
    case "claude_code":
      return claudeResumeCommand(sessionId, cwd);
    case "opencode":
      return opencodeResumeCommand(sessionId, cwd);
    default:
      return sessionId;
  }
}

function toConversationMessage(message: SessionMessage): SessionMessage | null {
  if (message.role !== "user" && message.role !== "assistant") {
    return null;
  }

  const blocks = message.blocks.filter((block) =>
    CONVERSATION_BLOCK_KINDS.has(block.kind)
  );

  if (blocks.length === 0) {
    return null;
  }

  return {
    ...message,
    blocks
  };
}

function messageMatchesTextFilter(message: SessionMessage, filter: string): boolean {
  if (filter.length === 0) {
    return true;
  }

  if (
    textMatchesFilter(message.id, filter) ||
    textMatchesFilter(message.role, filter) ||
    textMatchesFilter(message.sessionId, filter)
  ) {
    return true;
  }

  return message.blocks.some((block) => {
    if (
      textMatchesFilter(block.kind, filter) ||
      textMatchesFilter(formatBlockKind(block.kind), filter) ||
      textMatchesFilter(block.toolName, filter) ||
      textMatchesFilter(block.toolCallId, filter) ||
      textMatchesFilter(block.text, filter)
    ) {
      return true;
    }

    return textMatchesFilter(renderPayload(block.payload), filter);
  });
}

function eventMatchesTextFilter(event: SessionEvent, filter: string): boolean {
  if (filter.length === 0) {
    return true;
  }

  if (
    textMatchesFilter(event.id, filter) ||
    textMatchesFilter(event.kind, filter) ||
    textMatchesFilter(event.summary, filter) ||
    textMatchesFilter(event.sessionId, filter) ||
    textMatchesFilter(eventSessionId(event), filter)
  ) {
    return true;
  }

  return textMatchesFilter(renderPayload(event.payload), filter);
}

function fallbackAgent(detail: SessionDetailValue): SessionAgent {
  return {
    sessionId: detail.summary.sourceSessionId,
    label: "主 Agent",
    isRoot: true
  };
}

function sessionRequestKey(
  sourceApp: SourceApp,
  sourceSessionId: string,
  transcriptPath: string
): string {
  return `${sourceApp}:${sourceSessionId}:${transcriptPath}`;
}

function messageRenderKey(message: SessionMessage, index: number): string {
  return `${message.id}:${index}`;
}

function eventRenderKey(event: SessionEvent, index: number): string {
  return `${event.id}:${index}`;
}

function agentLabel(sessionId: string, agents: SessionAgent[]): string {
  const match = agents.find((agent) => agent.sessionId === sessionId);

  if (match) {
    return match.label;
  }

  return `Agent ${sessionId.slice(0, 8)}`;
}

function extractErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message) {
    return error.message;
  }

  if (typeof error === "string" && error.trim().length > 0) {
    return error;
  }

  if (error && typeof error === "object") {
    const message = Reflect.get(error, "message");

    if (typeof message === "string" && message.trim().length > 0) {
      return message;
    }

    try {
      const serialized = JSON.stringify(error, null, 2);

      if (serialized && serialized !== "{}") {
        return serialized;
      }
    } catch {
      // Fall through to the generic formatter below.
    }
  }

  const nextMessage = String(error ?? "").trim();
  return nextMessage.length > 0 ? nextMessage : fallback;
}

function formatVisibleMessageLabel(
  selectedAgentLabel: string,
  conversationOnly: boolean,
  visibleMessageCount: number,
  totalMessageCount: number,
  timelineFilter: string
): string {
  const unit = conversationOnly ? "轮对话" : "条消息";

  if (timelineFilter.length > 0) {
    return `${selectedAgentLabel} · 匹配 ${visibleMessageCount} / ${totalMessageCount} ${unit}`;
  }

  if (conversationOnly) {
    return `${selectedAgentLabel} · 已显示 ${visibleMessageCount} ${unit}`;
  }

  return `${selectedAgentLabel} · 已显示 ${totalMessageCount} ${unit}`;
}

function formatVisibleEventLabel(
  selectedAgentLabel: string,
  filteredEventCount: number,
  totalEventCount: number,
  nextEventOffset: number | null,
  timelineFilter: string
): string {
  if (timelineFilter.length > 0) {
    return `${selectedAgentLabel} · 匹配 ${filteredEventCount} / ${totalEventCount} 条事件`;
  }

  if (totalEventCount === 0 && nextEventOffset === 0) {
    return `${selectedAgentLabel} · 未加载事件`;
  }

  return `${selectedAgentLabel} · 已显示 ${totalEventCount} 条事件`;
}

function emptyMessageText(
  selectedAgentLabel: string,
  conversationOnly: boolean,
  timelineFilter: string
): string {
  if (timelineFilter.length > 0) {
    return `${selectedAgentLabel} 中没有匹配“${timelineFilter}”的${
      conversationOnly ? "用户/助手对话" : "消息"
    }。`;
  }

  if (conversationOnly) {
    return `${selectedAgentLabel} 当前暂无用户/助手对话。`;
  }

  return `${selectedAgentLabel} 当前暂无消息。`;
}

function emptyEventText(selectedAgentLabel: string, timelineFilter: string): string {
  if (timelineFilter.length > 0) {
    return `${selectedAgentLabel} 中没有匹配“${timelineFilter}”的事件。`;
  }

  return `${selectedAgentLabel} 当前暂无事件。`;
}

type MessageTimelineProps = {
  activeDetail: SessionDetailValue;
  agentOptions: SessionAgent[];
  conversationOnly: boolean;
  emptyText: string;
  expandedMessageIds: Record<string, boolean>;
  messageError: string | null;
  messages: SessionMessage[];
  messagesLoading: boolean;
  nextMessageOffset: number | null;
  onToggleExpanded: (messageKey: string) => void;
  rootSessionId: string;
  visibleMessageLabel: string;
  visibleMessages: Array<{ key: string; message: SessionMessage }>;
};

function messageCardClassName(isMarker: boolean, isInSubagent: boolean): string {
  if (isMarker) {
    return "timeline-card subagent-marker-card";
  }

  if (isInSubagent) {
    return "timeline-card subagent-message-card";
  }

  return "timeline-card";
}

function blockClassName(isExpanded: boolean): string {
  return isExpanded ? "message-block-text expanded" : "message-block-text";
}

function renderMessageBlock(
  block: SessionMessage["blocks"][number],
  messageKey: string,
  index: number,
  expandedMessageIds: Record<string, boolean>
) {
  const isExpanded = blockNeedsExpand(block) && Boolean(expandedMessageIds[messageKey]);
  const className = blockNeedsExpand(block) ? blockClassName(isExpanded) : undefined;

  return (
    <div key={`${messageKey}-${index}`} className="block-card">
      {block.text ? <pre className={className}>{block.text}</pre> : null}
      {!block.text && block.payload ? (
        <pre className={className}>{renderPayload(block.payload)}</pre>
      ) : null}
    </div>
  );
}

function renderMessageCard(
  activeDetail: SessionDetailValue,
  agentOptions: SessionAgent[],
  expandedMessageIds: Record<string, boolean>,
  messageKey: string,
  message: SessionMessage,
  onToggleExpanded: (messageKey: string) => void,
  rootSessionId: string
) {
  const isMarker = isSubagentMarkerMessage(message);
  const messageAgentId = message.sessionId ?? rootSessionId;
  const isInSubagent = messageAgentId !== rootSessionId;
  const agentName = agentLabel(messageAgentId, agentOptions);

  return (
    <article key={messageKey} className={messageCardClassName(isMarker, isInSubagent)}>
      {isMarker ? (
        <div className="subagent-marker-header">
          <div className="subagent-marker-title-row">
            <span className="subagent-badge strong">Sub-agent</span>
            <strong className="subagent-marker-title">
              {extractSubagentLabel(message) ?? "Sub-agent"}
            </strong>
          </div>
          <p className="subagent-marker-note">
            该 subagent 的消息和事件可通过上方 Agent 标签单独查看。
          </p>
        </div>
      ) : (
        <header className="message-card-header">
          <div className="message-card-meta">
            <strong>{message.role === "user" ? "用户" : "助手"}</strong>
            <small>{formatTimestamp(message.timestamp)}</small>
            {isInSubagent ? (
              <span className="message-source-chip">
                <span className="message-source-chip-label">Agent</span>
                <span className="message-source-chip-value">{agentName}</span>
              </span>
            ) : null}
            <div className="message-header-tags">
              {messageTags(message).map((tag) => (
                <span
                  key={tag.key}
                  className={tag.kind === "tool" ? "block-tag tool-tag" : "block-kind"}
                >
                  {tag.label}
                </span>
              ))}
            </div>
          </div>
          {messageNeedsExpand(message) ? (
            <button
              className="text-button"
              onClick={() => onToggleExpanded(messageKey)}
              type="button"
            >
              {expandedMessageIds[messageKey] ? "收起" : "展开"}
            </button>
          ) : null}
        </header>
      )}
      {message.blocks.map((block, index) =>
        renderMessageBlock(block, messageKey, index, expandedMessageIds)
      )}
    </article>
  );
}

function renderMessageTimeline(props: MessageTimelineProps) {
  const {
    activeDetail,
    agentOptions,
    conversationOnly,
    emptyText,
    expandedMessageIds,
    messageError,
    messages,
    messagesLoading,
    nextMessageOffset,
    onToggleExpanded,
    rootSessionId,
    visibleMessageLabel,
    visibleMessages
  } = props;

  return (
    <>
      {messageError ? <p className="error-text">{messageError}</p> : null}
      {messagesLoading && messages.length === 0 ? (
        <div className="timeline">
          {Array.from({ length: 3 }, (_, index) => (
            <article key={index} className="timeline-card skeleton-card">
              <span className="skeleton-line skeleton-title" />
              <span className="skeleton-line" />
              <span className="skeleton-line skeleton-short" />
            </article>
          ))}
        </div>
      ) : visibleMessages.length === 0 ? (
        <p className="muted-text">{emptyText}</p>
      ) : (
        <div className="timeline">
          {visibleMessages.map(({ key: messageKey, message }) =>
            renderMessageCard(
              activeDetail,
              agentOptions,
              expandedMessageIds,
              messageKey,
              message,
              onToggleExpanded,
              rootSessionId
            )
          )}
        </div>
      )}
      {activeDetail.messageCount !== null || visibleMessages.length > 0 || nextMessageOffset !== null ? (
        <div className="detail-pagination">
          <span className="muted-text">{visibleMessageLabel}</span>
        </div>
      ) : null}
    </>
  );
}

type EventTimelineProps = {
  emptyText: string;
  eventError: string | null;
  events: SessionEvent[];
  eventsLoading: boolean;
  filteredEvents: SessionEvent[];
  nextEventOffset: number | null;
  selectedAgentLabel: string;
  visibleEventLabel: string;
  agentOptions: SessionAgent[];
  rootSessionId: string;
};

function shouldShowEventAgentChip(
  agentSessionId: string | null,
  rootSessionId: string
): boolean {
  return agentSessionId !== null && agentSessionId !== rootSessionId;
}

function renderEventTimeline(props: EventTimelineProps) {
  const {
    agentOptions,
    emptyText,
    eventError,
    events,
    eventsLoading,
    filteredEvents,
    nextEventOffset,
    rootSessionId,
    selectedAgentLabel,
    visibleEventLabel
  } = props;

  return (
    <>
      {eventError ? <p className="error-text">{eventError}</p> : null}
      {eventsLoading && events.length === 0 ? (
        <div className="timeline">
          {Array.from({ length: 3 }, (_, index) => (
            <article key={index} className="timeline-card compact skeleton-card">
              <span className="skeleton-line skeleton-title" />
              <span className="skeleton-line" />
            </article>
          ))}
        </div>
      ) : filteredEvents.length === 0 ? (
        <p className="muted-text">{emptyText}</p>
      ) : (
        <div className="timeline">
          {filteredEvents.map((event, index) => {
            const agentSessionId = eventSessionId(event);
            const showAgentChip = shouldShowEventAgentChip(
              agentSessionId,
              rootSessionId
            );

            return (
              <article
                key={eventRenderKey(event, index)}
                className={
                  isSubagentLifecycleEvent(event)
                    ? "timeline-card compact event-card subagent-event-card"
                    : "timeline-card compact event-card"
                }
              >
                <header className="event-card-header">
                  <span className="event-kind">{event.kind}</span>
                  <small>{formatTimestamp(event.timestamp)}</small>
                  {showAgentChip ? (
                    <div className="message-source-chip">
                      <span className="message-source-chip-label">Agent</span>
                      <span className="message-source-chip-value">
                        {agentSessionId ? agentLabel(agentSessionId, agentOptions) : ""}
                      </span>
                    </div>
                  ) : null}
                </header>
                <p className="event-summary">{event.summary}</p>
                {event.payload ? <pre className="event-payload">{renderPayload(event.payload)}</pre> : null}
              </article>
            );
          })}
        </div>
      )}
      {filteredEvents.length > 0 || nextEventOffset !== null ? (
        <div className="detail-pagination">
          <span className="muted-text">{visibleEventLabel}</span>
        </div>
      ) : null}
    </>
  );
}

export function SessionDetail(props: SessionDetailProps) {
  const { detail, loading, scrollContainerRef } = props;
  const [targetApp, setTargetApp] = useState<SourceApp>("claude_code");
  const [preview, setPreview] = useState<ImportPreview | null>(null);
  const [importResult, setImportResult] = useState<ImportResult | null>(null);
  const [previewError, setPreviewError] = useState<string | null>(null);
  const [importError, setImportError] = useState<string | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [importLoading, setImportLoading] = useState(false);
  const [messages, setMessages] = useState<SessionMessage[]>([]);
  const [events, setEvents] = useState<SessionEvent[]>([]);
  const [messagesLoading, setMessagesLoading] = useState(false);
  const [eventsLoading, setEventsLoading] = useState(false);
  const [messagesLoadingMore, setMessagesLoadingMore] = useState(false);
  const [eventsLoadingMore, setEventsLoadingMore] = useState(false);
  const [messageError, setMessageError] = useState<string | null>(null);
  const [eventError, setEventError] = useState<string | null>(null);
  const [nextMessageOffset, setNextMessageOffset] = useState<number | null>(null);
  const [nextEventOffset, setNextEventOffset] = useState<number | null>(null);
  const [expandedMessageIds, setExpandedMessageIds] = useState<Record<string, boolean>>({});
  const [conversationOnly, setConversationOnly] = useState(true);
  const [timelineTab, setTimelineTab] = useState<"messages" | "events">("messages");
  const [selectedAgentId, setSelectedAgentId] = useState<string | "all">("all");
  const [timelineFilter, setTimelineFilter] = useState("");
  const detailKeyRef = useRef<string | null>(null);

  useEffect(() => {
    if (detail) {
      setTargetApp(defaultImportTarget(detail.summary.sourceApp));
    }
  }, [detail]);

  useEffect(() => {
    setPreview(null);
    setImportResult(null);
    setPreviewError(null);
    setImportError(null);
  }, [detail, targetApp]);

  useEffect(() => {
    setExpandedMessageIds({});
    setTimelineFilter("");
  }, [detail]);

  useEffect(() => {
    if (!detail) {
      setSelectedAgentId("all");
      return;
    }

    setSelectedAgentId(detail.summary.sourceSessionId);
  }, [detail]);

  useEffect(() => {
    detailKeyRef.current = detail
      ? sessionRequestKey(
          detail.summary.sourceApp,
          detail.summary.sourceSessionId,
          detail.summary.transcriptPath
        )
      : null;
  }, [detail]);

  useEffect(() => {
    if (!detail) {
      setMessages([]);
      setEvents([]);
      setMessageError(null);
      setEventError(null);
      setMessagesLoading(false);
      setEventsLoading(false);
      setMessagesLoadingMore(false);
      setEventsLoadingMore(false);
      setNextMessageOffset(null);
      setNextEventOffset(null);
      return;
    }

    let cancelled = false;
    const activeSourceApp = detail.summary.sourceApp;
    const activeSessionId = detail.summary.sourceSessionId;
    const transcriptPath = detail.summary.transcriptPath;
    const requestKey = sessionRequestKey(
      activeSourceApp,
      activeSessionId,
      transcriptPath
    );

    setMessages([]);
    setEvents([]);
    setMessageError(null);
    setEventError(null);
    setMessagesLoading(true);
    setEventsLoading(false);
    setMessagesLoadingMore(false);
    setEventsLoadingMore(false);
    setNextMessageOffset(null);
    setNextEventOffset(null);

    async function loadInitialPages() {
      try {
        const messageBundle = await getSessionMessages(activeSourceApp, activeSessionId, {
          transcriptPath,
          offset: 0,
          limit: DETAIL_PAGE_SIZE
        });

        if (cancelled || detailKeyRef.current !== requestKey) {
          return;
        }

        setMessages(messageBundle.messages);
        setNextMessageOffset(messageBundle.nextOffset);
        setEvents([]);
        setNextEventOffset(0);
      } catch (error) {
        if (cancelled || detailKeyRef.current !== requestKey) {
          return;
        }

        const message = extractErrorMessage(error, "加载会话时间线失败。");
        setMessageError(message);
        setEventError(message);
      } finally {
        if (!cancelled && detailKeyRef.current === requestKey) {
          setMessagesLoading(false);
        }
      }
    }

    void loadInitialPages();

    return () => {
      cancelled = true;
    };
  }, [detail]);

  const activeDetail = detail;
  const normalizedTimelineFilter = normalizeFilterValue(timelineFilter);
  const agentOptions = useMemo(() => {
    if (!activeDetail) {
      return [];
    }

    if (activeDetail.agents.length > 0) {
      return activeDetail.agents;
    }

    return [fallbackAgent(activeDetail)];
  }, [activeDetail]);
  const filteredMessages = useMemo(() => {
    if (!activeDetail) {
      return [];
    }

    return messages.filter((message) =>
      messageMatchesAgent(message, selectedAgentId, activeDetail.summary.sourceSessionId)
    );
  }, [activeDetail, messages, selectedAgentId]);
  const timelineMessages = useMemo(() => {
    if (conversationOnly) {
      return filteredMessages.flatMap((message) => {
        const conversationMessage = toConversationMessage(message);
        return conversationMessage ? [conversationMessage] : [];
      });
    }

    return filteredMessages;
  }, [conversationOnly, filteredMessages]);
  const visibleMessages = useMemo(
    () =>
      timelineMessages
        .filter((message) => messageMatchesTextFilter(message, normalizedTimelineFilter))
        .map((message, index) => ({
          key: messageRenderKey(message, index),
          message
        })),
    [normalizedTimelineFilter, timelineMessages]
  );
  const agentFilteredEvents = useMemo(() => {
    if (!activeDetail) {
      return [];
    }

    return events.filter((event) =>
      eventMatchesAgent(event, selectedAgentId, activeDetail.summary.sourceSessionId)
    );
  }, [activeDetail, events, selectedAgentId]);
  const filteredEvents = useMemo(
    () =>
      agentFilteredEvents.filter((event) =>
        eventMatchesTextFilter(event, normalizedTimelineFilter)
      ),
    [agentFilteredEvents, normalizedTimelineFilter]
  );
  const selectedAgentLabel = useMemo(() => {
    if (!activeDetail) {
      return "";
    }

    if (selectedAgentId === "all") {
      return "全部 Agent";
    }

    return agentLabel(selectedAgentId, agentOptions);
  }, [activeDetail, agentOptions, selectedAgentId]);
  const visibleMessageLabel = useMemo(() => {
    if (!activeDetail) {
      return "";
    }

    return formatVisibleMessageLabel(
      selectedAgentLabel,
      conversationOnly,
      visibleMessages.length,
      timelineMessages.length,
      timelineFilter.trim()
    );
  }, [
    activeDetail,
    conversationOnly,
    selectedAgentLabel,
    timelineFilter,
    timelineMessages.length,
    visibleMessages.length
  ]);
  const visibleEventLabel = useMemo(() => {
    if (!activeDetail) {
      return "";
    }

    return formatVisibleEventLabel(
      selectedAgentLabel,
      filteredEvents.length,
      agentFilteredEvents.length,
      nextEventOffset,
      timelineFilter.trim()
    );
  }, [
    activeDetail,
    agentFilteredEvents.length,
    filteredEvents.length,
    nextEventOffset,
    selectedAgentLabel,
    timelineFilter
  ]);
  const emptyVisibleMessageText = useMemo(
    () => emptyMessageText(selectedAgentLabel, conversationOnly, timelineFilter.trim()),
    [conversationOnly, selectedAgentLabel, timelineFilter]
  );
  const emptyVisibleEventText = useMemo(
    () => emptyEventText(selectedAgentLabel, timelineFilter.trim()),
    [selectedAgentLabel, timelineFilter]
  );
  const rootSessionId = activeDetail?.summary.sourceSessionId ?? "";
  const timelineFilterPlaceholder =
    timelineTab === "messages"
      ? "按消息内容、工具名、块类型筛选"
      : "按事件类型、摘要、载荷筛选";

  const targetAppOptions = activeDetail
    ? IMPORT_TARGET_APPS.filter((app) => app !== activeDetail.summary.sourceApp)
    : IMPORT_TARGET_APPS;
  const selectedTargetCopy = importTargetCopy(targetApp);

  useEffect(() => {
    if (
      !activeDetail ||
      timelineTab !== "messages" ||
      messagesLoading ||
      messagesLoadingMore ||
      nextMessageOffset === null
    ) {
      return;
    }

    if (selectedAgentId === "all" && normalizedTimelineFilter.length === 0) {
      return;
    }

    if (
      timelineMessages.some((message) =>
        messageMatchesTextFilter(message, normalizedTimelineFilter)
      )
    ) {
      return;
    }

    void handleLoadMoreMessages();
  }, [
    activeDetail,
    messagesLoading,
    messagesLoadingMore,
    nextMessageOffset,
    normalizedTimelineFilter,
    selectedAgentId,
    timelineMessages,
    timelineTab
  ]);

  useEffect(() => {
    if (
      !activeDetail ||
      timelineTab !== "events" ||
      eventsLoading ||
      eventsLoadingMore ||
      nextEventOffset === null
    ) {
      return;
    }

    if (selectedAgentId === "all" && normalizedTimelineFilter.length === 0) {
      return;
    }

    if (
      agentFilteredEvents.some((event) =>
        eventMatchesTextFilter(event, normalizedTimelineFilter)
      )
    ) {
      return;
    }

    void handleLoadMoreEvents();
  }, [
    activeDetail,
    agentFilteredEvents,
    eventsLoading,
    eventsLoadingMore,
    nextEventOffset,
    normalizedTimelineFilter,
    selectedAgentId,
    timelineTab
  ]);

  useEffect(() => {
    const element = scrollContainerRef.current;

    if (!element) {
      return;
    }

    const onScroll = () => {
      if (!isNearBottom(element)) {
        return;
      }

      if (timelineTab === "messages") {
        if (
          nextMessageOffset !== null &&
          !messagesLoading &&
          !messagesLoadingMore
        ) {
          void handleLoadMoreMessages();
        }

        return;
      }

      if (nextEventOffset !== null && !eventsLoading && !eventsLoadingMore) {
        void handleLoadMoreEvents();
      }
    };

    element.addEventListener("scroll", onScroll, { passive: true });

    return () => {
      element.removeEventListener("scroll", onScroll);
    };
  }, [
    activeDetail,
    conversationOnly,
    eventsLoading,
    eventsLoadingMore,
    messagesLoading,
    messagesLoadingMore,
    nextEventOffset,
    nextMessageOffset,
    scrollContainerRef,
    timelineTab,
    visibleMessages.length,
    events.length
  ]);

  if (!detail && loading) {
    return (
      <div className="detail-panel">
        <section className="detail-section skeleton-card">
          <span className="skeleton-line skeleton-title" />
          <span className="skeleton-line" />
          <div className="summary-grid">
            {Array.from({ length: 6 }, (_, index) => (
              <div key={index}>
                <span className="skeleton-line skeleton-meta" />
                <span className="skeleton-line" />
              </div>
            ))}
          </div>
        </section>
        <section className="detail-section skeleton-card">
          <span className="skeleton-line skeleton-title" />
          <span className="skeleton-line" />
          <span className="skeleton-line skeleton-short" />
        </section>
        <section className="detail-section skeleton-card">
          <span className="skeleton-line skeleton-title" />
          <span className="skeleton-line" />
          <span className="skeleton-line" />
          <span className="skeleton-line skeleton-short" />
        </section>
      </div>
    );
  }

  if (!detail) {
    return (
      <div className="detail-empty">
        请选择一个会话以查看其转录内容和元数据。
      </div>
    );
  }

  const resolvedDetail = detail;

  async function handleLoadMoreMessages() {
    if (!activeDetail || messagesLoading || messagesLoadingMore || nextMessageOffset === null) {
      return;
    }

    const requestKey = sessionRequestKey(
      activeDetail.summary.sourceApp,
      activeDetail.summary.sourceSessionId,
      activeDetail.summary.transcriptPath
    );
    setMessagesLoadingMore(true);
    setMessageError(null);

    try {
      const bundle = await getSessionMessages(
        activeDetail.summary.sourceApp,
        activeDetail.summary.sourceSessionId,
        {
          transcriptPath: activeDetail.summary.transcriptPath,
          offset: nextMessageOffset,
          limit: DETAIL_PAGE_SIZE
        }
      );

      if (detailKeyRef.current !== requestKey) {
        return;
      }

      setMessages((current) => [...current, ...bundle.messages]);
      setNextMessageOffset(bundle.nextOffset);
    } catch (error) {
      if (detailKeyRef.current !== requestKey) {
        return;
      }

      const message = extractErrorMessage(error, "加载更多消息失败。");
      setMessageError(message);
    } finally {
      if (detailKeyRef.current === requestKey) {
        setMessagesLoadingMore(false);
      }
    }
  }

  async function handleLoadMoreEvents() {
    if (!activeDetail || eventsLoading || eventsLoadingMore || nextEventOffset === null) {
      return;
    }

    const requestKey = sessionRequestKey(
      activeDetail.summary.sourceApp,
      activeDetail.summary.sourceSessionId,
      activeDetail.summary.transcriptPath
    );
    const initialLoad = events.length === 0 && nextEventOffset === 0;

    if (initialLoad) {
      setEventsLoading(true);
    } else {
      setEventsLoadingMore(true);
    }
    setEventError(null);

    try {
      const page = await getSessionEvents(
        activeDetail.summary.sourceApp,
        activeDetail.summary.sourceSessionId,
        {
          transcriptPath: activeDetail.summary.transcriptPath,
          offset: nextEventOffset,
          limit: DETAIL_PAGE_SIZE
        }
      );

      if (detailKeyRef.current !== requestKey) {
        return;
      }

      setEvents((current) => [...current, ...page.events]);
      setNextEventOffset(page.nextOffset);
    } catch (error) {
      if (detailKeyRef.current !== requestKey) {
        return;
      }

      const message = extractErrorMessage(error, "加载更多事件失败。");
      setEventError(message);
    } finally {
      if (detailKeyRef.current === requestKey) {
        setEventsLoading(false);
        setEventsLoadingMore(false);
      }
    }
  }

  async function handlePreviewImport() {
    if (!activeDetail) {
      return;
    }

    setPreviewLoading(true);
    setPreviewError(null);
    setImportResult(null);
    setImportError(null);

    try {
      const nextPreview = await previewImport(
        activeDetail.summary.sourceApp,
        activeDetail.summary.sourceSessionId,
        targetApp,
        activeDetail.summary.transcriptPath
      );
      setPreview(nextPreview);
    } catch (error) {
      const message = extractErrorMessage(error, "导入预览失败。");
      setPreviewError(message);
    } finally {
      setPreviewLoading(false);
    }
  }

  async function handleImport() {
    if (!activeDetail) {
      return;
    }

    setImportLoading(true);
    setImportError(null);

    try {
      const nextImportResult = await importSession(
        activeDetail.summary.sourceApp,
        activeDetail.summary.sourceSessionId,
        targetApp,
        activeDetail.summary.transcriptPath
      );
      setImportResult(nextImportResult);
    } catch (error) {
      const message = extractErrorMessage(error, "导入会话失败。");
      setImportError(message);
    } finally {
      setImportLoading(false);
    }
  }

  function toggleMessageExpanded(messageKey: string) {
    setExpandedMessageIds((current) => ({
      ...current,
      [messageKey]: !current[messageKey]
    }));
  }

  return (
    <div className="detail-panel panel-shell" aria-busy={loading}>
      <section className="detail-section">
        <div className="detail-header">
          <div>
            <h2>{detail.summary.title}</h2>
            <p>{detail.summary.sourceSessionId}</p>
          </div>
        </div>
        <dl className="summary-grid">
          <div>
            <dt>来源</dt>
            <dd>{formatAppName(detail.summary.sourceApp)}</dd>
          </div>
          <div>
            <dt>工作目录</dt>
            <dd>{detail.summary.cwd ?? "未知"}</dd>
          </div>
          <div>
            <dt>分支</dt>
            <dd>{detail.summary.gitBranch ?? "未知"}</dd>
          </div>
          <div>
            <dt>创建时间</dt>
            <dd>{formatTimestamp(detail.summary.createdAt)}</dd>
          </div>
          <div>
            <dt>更新时间</dt>
            <dd>{formatTimestamp(detail.summary.updatedAt)}</dd>
          </div>
          <div>
            <dt>转录文件</dt>
            <dd>{detail.summary.transcriptPath}</dd>
          </div>
        </dl>
      </section>

      <section className="detail-section">
        <div className="section-row">
          <h3>导入预览</h3>
          <div className="import-row">
            <select onChange={(event) => setTargetApp(event.target.value as SourceApp)} value={targetApp}>
              {targetAppOptions.map((app) => (
                <option key={app} value={app}>
                  {importTargetCopy(app).optionLabel}
                </option>
              ))}
            </select>
            <button
              className="secondary-button"
              onClick={() => void handlePreviewImport()}
              type="button"
            >
              {previewLoading ? "检查中..." : "预览"}
            </button>
            <button
              className="primary-button"
              disabled={!preview?.supported || importLoading}
              onClick={() => void handleImport()}
              type="button"
            >
              {importLoading ? "导入中..." : "导入"}
            </button>
          </div>
        </div>
        <div className="import-method-box">
          <span className="pill">{selectedTargetCopy.methodLabel}</span>
          <p className="muted-text">{selectedTargetCopy.methodDescription}</p>
        </div>
        {previewError ? (
          <div className="error-box">
            <strong>预览失败</strong>
            <pre className="error-text">{previewError}</pre>
          </div>
        ) : null}
        {importError ? (
          <div className="error-box">
            <strong>导入失败</strong>
            <pre className="error-text">{importError}</pre>
          </div>
        ) : null}
        {preview ? (
          <div className="preview-box">
            <strong>{preview.supported ? `导入级别：${formatImportLevel(preview.importLevel)}` : "暂不支持导入"}</strong>
            {preview.createdPaths.length > 0 ? (
              <div className="path-list">
                {preview.createdPaths.map((path, index) => (
                  <code key={`${path}-${index}`}>{path}</code>
                ))}
              </div>
            ) : null}
            {preview.warnings.length > 0 ? (
              <ul>
                {preview.warnings.map((warning, index) => (
                  <li key={`${warning}-${index}`}>{translateWarning(warning)}</li>
                ))}
              </ul>
            ) : null}
          </div>
        ) : (
          <p className="muted-text">
            先运行预览以查看兼容性警告和目标文件路径。
          </p>
        )}
        {importResult ? (
          <div className="preview-box success-box">
            <strong>{selectedTargetCopy.successTitle}</strong>
            <p>会话 ID：{importResult.createdSessionId}</p>
            {selectedTargetCopy.successNote ? <p className="muted-text">{selectedTargetCopy.successNote}</p> : null}
            <p className="muted-text">
              推荐工作目录：{importResult.resumeCwd ?? "未知"}
            </p>
            <p className="muted-text">{selectedTargetCopy.manualLabel}：</p>
            <div className="path-list">
              <code>
                {manualOpenCommand(
                  importResult.targetApp,
                  importResult.createdSessionId,
                  importResult.resumeCwd
                )}
              </code>
            </div>
            <div className="path-list">
              {importResult.createdPaths.map((path, index) => (
                <code key={`${path}-${index}`}>{path}</code>
              ))}
            </div>
          </div>
        ) : null}
      </section>

      <section className="detail-section">
        <div className="timeline-controls">
          <div className="timeline-tab-bar">
            <button
              className={`timeline-tab${timelineTab === "messages" ? " active" : ""}`}
              onClick={() => setTimelineTab("messages")}
              type="button"
            >
              消息
            </button>
            <button
              className={`timeline-tab${timelineTab === "events" ? " active" : ""}`}
              onClick={() => {
                setTimelineTab("events");

                if (events.length === 0 && !eventsLoading && nextEventOffset === 0) {
                  void handleLoadMoreEvents();
                }
              }}
              type="button"
            >
              事件
            </button>
            {timelineTab === "messages" ? (
              <label className="toggle-control timeline-toggle-control">
                <input
                  checked={conversationOnly}
                  onChange={(event) => setConversationOnly(event.target.checked)}
                  type="checkbox"
                />
                <span>仅看对话</span>
              </label>
            ) : null}
          </div>
          {agentOptions.length > 1 ? (
            <div className="agent-tab-group">
              {agentOptions.map((agent) => (
                <button
                  key={agent.sessionId}
                  className={`timeline-tab${selectedAgentId === agent.sessionId ? " active" : ""}`}
                  onClick={() => setSelectedAgentId(agent.sessionId)}
                  type="button"
                >
                  {agent.label}
                </button>
              ))}
              <button
                className={`timeline-tab${selectedAgentId === "all" ? " active" : ""}`}
                onClick={() => setSelectedAgentId("all")}
                type="button"
              >
                全部
              </button>
            </div>
          ) : null}
          <div className="timeline-filter-row">
            <label className="timeline-filter-field">
              <span className="muted-text">筛选当前会话详情</span>
              <input
                onChange={(event) => setTimelineFilter(event.target.value)}
                placeholder={timelineFilterPlaceholder}
                type="search"
                value={timelineFilter}
              />
            </label>
          </div>
        </div>

        {timelineTab === "messages" ? (
          renderMessageTimeline({
            activeDetail: resolvedDetail,
            agentOptions,
            conversationOnly,
            emptyText: emptyVisibleMessageText,
            expandedMessageIds,
            messageError,
            messages,
            messagesLoading,
            nextMessageOffset,
            onToggleExpanded: toggleMessageExpanded,
            rootSessionId,
            visibleMessageLabel,
            visibleMessages
          })
        ) : (
          renderEventTimeline({
            agentOptions,
            emptyText: emptyVisibleEventText,
            eventError,
            events,
            eventsLoading,
            filteredEvents,
            nextEventOffset,
            rootSessionId,
            selectedAgentLabel,
            visibleEventLabel
          })
        )}
      </section>
      {loading ? (
        <div className="panel-loading-overlay">
          <div className="loading-pill">正在加载会话详情...</div>
        </div>
      ) : null}
    </div>
  );
}
