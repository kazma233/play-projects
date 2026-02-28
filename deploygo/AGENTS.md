# Repository Guidelines

## Project Structure & Module Organization
DeployGo is a Go CLI application.
- `main.go`: program entrypoint (`cmd.RootCmd.Execute()`).
- `cmd/`: Cobra commands (`build`, `deploy`, `pipeline`, `write`, `list`, `clone`).
- `internal/`: non-exported implementation packages:
  - `config/` for `workspace/<project>/config.yaml` parsing.
  - `container/` for Docker/Podman runtime abstraction.
  - `stage/` for build/deploy/cleanup pipeline stages.
  - `deploy/`, `git/`, `retry/` for execution helpers.
- `workspace/` (runtime data, not source): per-project configs and files used by commands.

## Build, Test, and Development Commands
- `go build -o deploygo .`: build the CLI binary.
- `go run main.go list`: list projects in `workspace/`.
- `go run main.go -P myproject pipeline`: run full flow (clone/write/build/deploy/cleanup).
- `go run main.go -P myproject build -s build`: run one build stage.
- `go test ./...`: run all unit tests.
- `go test ./... -cover`: run tests with coverage.

## Coding Style & Naming Conventions
- Use standard Go formatting: `gofmt -w .` before commit.
- Keep packages small and cohesive; place reusable internal logic under `internal/`.
- Naming:
  - exported identifiers: `PascalCase`.
  - unexported/local identifiers: `camelCase`.
  - CLI flags: short + long form when useful (example: `-P, --project`).
- Prefer explicit error wrapping (`fmt.Errorf("...: %w", err)`) for actionable failures.

## Testing Guidelines
- Framework: Go `testing` package.
- Test files should be named `*_test.go` next to the code they verify.
- Prefer table-driven tests for config parsing, stage selection, and path handling.
- Add regression tests for bugs in build/deploy path resolution and workspace behavior.

## Commit & Pull Request Guidelines
- Current history follows lightweight Conventional Commit style: `feat: ...`, `fix: ...`, `revert: ...`.
- Write concise, scoped messages (example: `fix: handle missing config.yaml in pipeline`).
- PRs should include:
  - what changed and why,
  - impacted commands/flags,
  - test evidence (`go test ./...` output),
  - sample CLI usage/output for behavior changes.

## Security & Configuration Tips
- Never commit real SSH keys, passwords, or production host credentials.
- Prefer key-based auth in `config.yaml` and environment-variable substitution for sensitive values.
- Validate container runtime (`docker` or `podman`) locally before running pipeline commands.
