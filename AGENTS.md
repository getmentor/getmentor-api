# Repository Guidelines

## Project Structure & Module Organization
- `cmd/api/main.go` is the service entry point.
- `internal/` holds core application code (handlers, middleware, services, repository, models, cache).
- `pkg/` contains shared clients/utilities (airtable, azure, logger, metrics).
- `config/` defines environment-backed configuration.
- `test/` mirrors package structure for tests (for example `test/internal/handlers/..._test.go`).
- `bin/` is the default build output location.

## Build, Test, and Development Commands
- `make build`: compile to `bin/getmentor-api`.
- `make run`: run locally with `go run`.
- `make test`: run all tests.
- `make test-race`: run tests with the race detector (CI requirement).
- `make test-coverage`: produce `coverage.out` and print totals.
- `make lint`: run `golangci-lint` using `.golangci.yml`.
- `make fmt` / `make fmt-check`: format or verify formatting.
- `make vet` / `make staticcheck` / `make security`: run static analysis.
- `make ci`: run the full local CI suite.
- Optional: `air` for hot reload (`go install github.com/cosmtrek/air@latest`).

## Coding Style & Naming Conventions
- Go formatting via `gofmt` (tabs, standard Go layout) and `goimports`.
- Package names are short, lower-case; exported identifiers use `PascalCase`.
- Test files end with `_test.go`; test names use `TestXxx`.
- Follow existing lint rules in `.golangci.yml` (gocyclo, revive, gosec, etc.).

## Testing Guidelines
- Run `go test ./...` for the full suite; tests live under `test/`.
- CI runs race tests and enforces a minimum coverage threshold (10%).
- Add or extend tests alongside new handlers/services to keep coverage moving upward.

## Commit & Pull Request Guidelines
- Commit messages are short and imperative; history often uses prefixes like `feat:`, `fix:`, `refactor:`, `test:`, `bugfix:`.
- Before opening a PR, ensure `go mod tidy` is clean and `make ci` passes.
- PRs should include a clear description and link relevant issues or tickets.

## Security & Configuration Tips
- Copy `.env.example` to `.env` and keep secrets out of Git.
- Public endpoints require auth tokens; verify local config before testing.
