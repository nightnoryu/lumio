# Repository Guidelines

## Project Structure & Module Organization

Lumio builds photographer portfolios with a Go backend and React/TypeScript dashboard.

- `backend/cmd/lumio/`: configuration, startup, and dependency wiring.
- `backend/internal/`: domain rules, application services, infrastructure adapters, HTTP transport, and portfolio templates/assets.
- `backend/data/migrations/`: timestamped SQL up/down migrations.
- `web/src/`: dashboard components, styles, API client, and colocated tests.
- `api/publicapi.yml`: shared OpenAPI contract.
- `backend/test/e2e/`: integration tests; `docs/`: development and operations guides; `k8s/` and `ops/`: deployment and backup tooling.

Keep dependencies pointing inward: domain code must not import HTTP, SQL, SDKs, or infrastructure packages.

## Build, Test, and Development Commands

Install mise and Docker Compose, then run commands from the repository root:

Always run development tools through mise: prefer `mise run <task>` for existing tasks, otherwise `mise exec -- <command>`. For example, install dependencies in `web/` with `mise exec -- pnpm install`.

- `mise install`: install pinned tool versions.
- `mise run dev`: build and start local services at `http://localhost:3000`, with frontend hot reload.
- `mise run dev:reload`: rebuild Go and restart backend services.
- `mise run build`: build `backend/bin/lumio` with embedded frontend assets.
- `mise run`: run the default build, tests, and lint checks.
- `mise run backend:test:e2e`: run Docker-backed integration tests.
- `mise run backend:test:media`: verify real libvips processing inside the worker image.
- `mise run k8s:lint`: render and validate base Kubernetes manifests.

## Coding Style & Naming Conventions

Use Go tabs and the configured gofmt/goimports/gci formatters; group imports as standard library, external, then `lumio/`. Follow existing four-space indentation, double quotes, and semicolons in React source. Use PascalCase component filenames and strict TypeScript. Run `mise run lint` for golangci-lint and ESLint checks.

Edit the OpenAPI contract and `.templ` sources, then regenerate with `mise run backend:generate`, `mise run web:generate`, or `mise run backend:templates`. Do not hand-edit generated output. Add new migrations rather than changing applied ones.

## Testing Guidelines

Use Go's testing package with `*_test.go`; use Vitest, Testing Library, and jsdom with `*.test.ts`/`*.test.tsx`. Run `mise run test` for unit/frontend tests. E2E tests use the `e2e` build tag and Testcontainers PostgreSQL. Cover changed behavior and failure paths; no numeric coverage threshold is configured.

## Definition of Done

- For code changes, both `mise run` and `mise run backend:test:e2e` must pass.
- For changes under `k8s/base/`, `mise run k8s:lint` must pass.
- When both conditions apply, all three commands must pass.

## Commit & Pull Request Guidelines

History uses short, imperative subjects such as “Make draft buttons panel sticky.” Follow that style. PRs should explain the problem and resulting behavior, link relevant issues, report validation, and include screenshots for UI changes. Note migration or configuration changes explicitly.
