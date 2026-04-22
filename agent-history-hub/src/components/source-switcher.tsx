import "./source-switcher.css";
import type { SourceApp, SourceStatus } from "../lib/types";

type SourceSwitcherProps = {
  sources: SourceStatus[];
  selectedSource: SourceApp;
  onChange: (source: SourceApp) => void;
};

const SOURCE_LABELS: Record<SourceApp, string> = {
  codex: "Codex",
  claude_code: "Claude Code",
  opencode: "OpenCode"
};

export function SourceSwitcher(props: SourceSwitcherProps) {
  const { onChange, selectedSource, sources } = props;

  return (
    <div className="source-switcher">
      {sources.map((source) => (
        <button
          key={source.app}
          className={source.app === selectedSource ? "active" : undefined}
          disabled={!source.available}
          onClick={() => onChange(source.app)}
          type="button"
        >
          {SOURCE_LABELS[source.app]}
          <small>{source.sessionCount}</small>
        </button>
      ))}
    </div>
  );
}
