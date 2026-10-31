# Contributing to Oracle Fisher

Thank you for your interest in contributing to Oracle Fisher! We welcome contributions, bug reports, and suggestions from the community.

## Architectural Invariants

When submitting code changes, the following invariants must be preserved:

1. **Go Native & Minimal Footprint:** The application is written in Go (Go 1.22+) using the official Oracle Cloud SDK (`github.com/oracle/oci-go-sdk/v65`). Do not introduce heavy third-party web frameworks, Python runtimes, or unneeded dependencies.
2. **Persistent Keep-Alive Connections:** HTTP connections to OCI APIs must reuse keep-alive sessions to maintain low latency (~50ms) and avoid socket exhaustion.
3. **Structured NDJSON Logging:** Logs are emitted to `stdout` in newline-delimited JSON using Go's standard `log/slog` library with canonical fields (`timestamp`, `level`, `service`, `app`, `env`, `message`).
4. **Platform Compatibility:** Configurations and JSON parsers must remain transparently compatible with both Linux (`\n`) and Windows (`\r\n`) line endings.
5. **Secret Isolation:** Never commit real credentials, OCIDs, or private keys. All sensitive files in `oci_keys/` (except `.gitkeep` and `*.example`) must remain ignored by git.

## Local Development Setup

### Prerequisites
- Go 1.22 or later
- Docker & Docker Compose (optional, for containerized testing)
- Git

### Running Tests
Run the automated test suite locally:
```bash
go test -v ./...
```

### Building the Binary
Build the standalone static binary:
```bash
CGO_ENABLED=0 go build -o oracle-fisher ./cmd/oracle-fisher
```

### Docker Build
Verify the multi-stage Docker build:
```bash
docker compose build
```

## Commit Message Guidelines

This repository strictly enforces **Conventional Commits**. Please format your commit messages accordingly:

```text
<type>(<optional scope>): <description>
```

Common types:
- `feat:` A new feature or capability
- `fix:` A bug fix
- `docs:` Documentation updates only
- `refactor:` Code restructuring without changing functional behavior
- `test:` Adding or improving tests
- `chore:` Dependency bumps, CI updates, or maintenance tasks

Example:
```text
feat(oci): add support for custom retry backoff on 429
fix(config): handle windows CRLF in accounts.json correctly
docs: update setup steps for multi-account profiles
```

## Pull Request Process

1. Fork the repository and create your branch from `main`.
2. Ensure `go test ./...` passes without errors.
3. Ensure `git diff --check` reports no trailing whitespaces or syntax issues.
4. Keep pull requests focused on a single concern.
5. Open a Pull Request referencing any relevant issues.
