# Repository Guidelines

## Project Structure & Module Organization
This repository is a small Go CLI application with most logic at the root.
- `main.go`: CLI flags, input validation, and extraction flow.
- `payload.go` and `reader.go`: payload parsing and extraction logic.
- `chromeos_update_engine/update_metadata.pb.go`: generated protobuf types used by the parser.
- `update_metadata.proto`: source protobuf schema.
- `.github/workflows/`: CI and release workflows (`build.yml`, `goreleaser.yml`).

## Build, Test, and Development Commands
- `go build .`: build the CLI binary for the current platform (matches CI).
- `go run . -l /path/to/payload.bin`: list partitions without extracting.
- `go run . -o out -p boot,vendor /path/to/payload.bin`: extract selected partitions.
- `go test ./...`: run package tests (currently minimal/no tests, but keep this green).
- `go fmt ./...`: format all Go packages before opening a PR.

Note: `xz`/`liblzma` must be installed on the host for decompression support.

## Coding Style & Naming Conventions
- Follow idiomatic Go style and keep code `go fmt`-clean.
- Use tabs for indentation and UTF-8 with LF endings (see `.editorconfig`).
- Keep exported names in `CamelCase` and internal helpers in `camelCase`.
- Prefer clear, descriptive flag names and keep shorthand flags aligned with long forms.

## Testing Guidelines
- Add table-driven tests for new parsing/extraction behavior where feasible.
- Keep tests next to implementation files with `_test.go` suffix.
- Use descriptive test names like `TestExtractSelected_SkipsMissingPartition`.
- For bug fixes, add a regression test when practical before or with the fix.

## Commit & Pull Request Guidelines
- Follow Conventional Commits used in history: `feat:`, `fix:`, `chore:`, `fix(ci):`, `feat(cli):`.
- Keep each commit scoped to one change and write imperative summaries.
- PRs should include:
1. a short problem/solution description,
2. linked issue(s) when applicable,
3. local verification output (`go build .`, `go test ./...`),
4. CLI examples or screenshots only when behavior/output changes materially.
