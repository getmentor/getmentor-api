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

## Auth Realms & Gotchas
- Three cookie realms: `mentor_session`, `admin_session`, `mentee_session`. Mentor and admin tokens are signed with `JWT_SECRET` and the mentor realm accepts tokens without any realm claim, so a new realm must not sign with `JWT_SECRET` directly. The mentee realm (`pkg/jwt/mentee.go`) signs with a key derived from it and requires `aud`, `token_type`, `sub`, `sv` and `exp`.
- Mentee sessions carry a `session_version` (`sv`) that the middleware re-checks against `mentees.session_version` on every request; logout increments it, so copied cookies stop working at once.
- Per-IP rate limiters see the Next.js server's address for all proxied traffic, so a per-IP limit there is effectively global. Limit per person by key (`RateLimiter.AllowKey`), as mentee login does per email hash.
- A request id is a capability: the emailed review link `/reviews/new?request_id=...` accepts a review for it. Keep request ids out of URLs, logs and analytics in new code (the mentee dashboard sends them only in POST bodies).
- `client_requests` has many nullable columns (`email`, `name`, `description`, `mentor_id`, ...). Scan them into pointers; one NULL scanned into a `string` breaks the whole list query.
- DB-backed tests in `test/internal/repository` run only with `TEST_DATABASE_URL` set to a migrated database, for example a local Postgres: `TEST_DATABASE_URL=postgres://...?sslmode=disable go test -race ./...`.
