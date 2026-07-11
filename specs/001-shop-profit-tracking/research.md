# Phase 0 Research: Shop Profit Tracking

All items originally marked NEEDS CLARIFICATION in Technical Context are resolved below. Frontend architecture, DB layer, and Telegram bot library/mode were confirmed directly with the user during planning; the remaining items are implementation-detail research needed to execute that architecture correctly.

## 1. Web dashboard architecture

- **Decision**: Echo (`github.com/labstack/echo/v4`) as the HTTP router/middleware layer, serving a **JSON-only REST API** — no `html/template`, no server-rendered views, no htmx. Every handler responds via `c.JSON(...)`. Presentation (whatever renders the dashboard UI) is a separate, out-of-scope client of this API.
- **Rationale**: User-directed: "the handler is only API, no html/template, response as JSON format." This also decouples the API's release cadence from any frontend's, and lets the same endpoints eventually serve a web SPA, a mobile app, or further bot integrations without change. Superseded the earlier server-rendered-Go decision below.
- **Alternatives considered (superseded)**: Server-rendered Go (`html/template` + htmx) — the original decision for this feature, single-binary and no frontend build pipeline; explicitly replaced by the JSON-API direction above. SPA (React/Vue) + JSON API — not chosen either, since the SPA/frontend project itself remains out of scope for this backend feature; only the JSON API side is being built here.

**Consequence for later phases**: `contracts/web-api.yaml` (OpenAPI 3.0) documents the JSON request/response contract (paths, JSON bodies, status codes, schemas) rather than HTML fragment responses; `internal/web/templates` and `internal/web/static` are removed from the project structure in favor of `internal/api/handlers`.

## 2. Database access layer

- **Decision**: GORM (`gorm.io/gorm`, `gorm.io/driver/postgres`) as the ORM, with explicit `NUMERIC` column types and a custom GORM `Scan`/`Value` wrapper type around `shopspring/decimal.Decimal` for every monetary/quantity field (never `float64`, per FR-020). Schema managed by hand-written SQL migrations (`golang-migrate`), not `AutoMigrate`, so column types (`NUMERIC(14,4)`, `TIMESTAMPTZ`, constraints, indexes) are explicit and reviewable.
- **Rationale**: User-confirmed GORM. Pairing it with `shopspring/decimal` + a custom `sql.Scanner`/`driver.Valuer` type closes GORM's main risk for this domain (implicit float handling) while keeping GORM's ergonomic query/association API for the catalog/recipe/report reads.
- **Alternatives considered**: `database/sql` + `sqlc` (more explicit SQL, more boilerplate, rejected in favor of GORM per user preference); `ent` (strong schema-as-code but steeper learning curve, rejected — not needed at this scale).

## 3. Telegram bot library and update delivery

- **Decision**: `go-telegram-bot-api/v5`, long polling (`GetUpdatesChan`) run in its own goroutine from `cmd/server/main.go`, alongside the HTTP server.
- **Rationale**: User-confirmed. Long polling needs no public HTTPS endpoint/TLS/domain, which keeps local development and small self-hosted deployments simple; update volume for a small number of shop owners is far below where polling latency would matter.
- **Alternatives considered**: Webhook mode — rejected for now (requires a public HTTPS endpoint and cert management); can be revisited later without changing the domain/service layer, since only `internal/telegram` would change.

## 4. Fixed-precision monetary/quantity representation

- **Decision**: PostgreSQL `NUMERIC(14,4)` for ingredient costs/quantities and `NUMERIC(12,2)` for currency amounts (sale price, revenue, total cost, production cost), mapped in Go to `github.com/shopspring/decimal`. A small `internal/db` package implements `sql.Scanner`/`driver.Valuer` so GORM reads/writes `decimal.Decimal` directly without passing through `float64`.
- **Rationale**: Directly required by FR-020 ("avoiding floating-point rounding error"). `NUMERIC` in Postgres is exact; `shopspring/decimal` is the standard Go library for exact decimal arithmetic and is GORM-compatible via the scanner/valuer interfaces.
- **Alternatives considered**: Store amounts as integer cents/minor-units (`int64`) — viable and simpler in some stacks, but rejected here because ingredient unit costs need more than 2 decimal places (e.g., cost per gram) where `shopspring/decimal` is a more natural fit than an integer minor-unit scheme.

## 5. Web dashboard authentication tied to Telegram identity

- **Decision**: [Telegram Login Widget](https://core.telegram.org/widgets/login) data is verified by a JSON endpoint, `POST /api/auth/telegram`. The API does not render the widget itself (no HTML in this codebase — see §1); whatever out-of-scope client embeds the widget POSTs the widget's returned payload (`id`, `first_name`, `username`, `auth_date`, `hash`) as a JSON body to this endpoint. The server verifies `hash` using HMAC-SHA256 over the bot token (per Telegram's documented verification algorithm), looks up/creates the `users` row by `telegram_id` (same auto-provisioning rule as FR-022), sets a signed, `HttpOnly`, `SameSite=Lax` session cookie, and responds `200 application/json` with the user record. Subsequent `/api/*` requests are authenticated via that cookie; Echo middleware rejects unauthenticated requests to any non-`/api/auth/*` route with `401 application/json`.
- **Rationale**: Satisfies the spec's assumption that the dashboard is "tied to the same Telegram-authenticated identity as the bot," without building a separate password/email system (out of scope per Assumptions), while keeping the API itself free of any HTML/view responsibility per the JSON-only direction in §1. A cookie (rather than a bearer token returned in the JSON body) avoids requiring the out-of-scope client to manage token storage/refresh, and works whether that client is same-origin or a separate origin with CORS configured to allow credentials.
- **Alternatives considered**: Bearer/JWT token returned in the `POST /api/auth/telegram` response body, sent by the client as `Authorization: Bearer <token>` on subsequent calls — a reasonable alternative for a fully decoupled mobile/SPA client that can't rely on cookies (e.g., a native app); not chosen as the default here since a cookie is simpler for a browser-based dashboard client, but this can be added alongside the cookie later without changing the verification logic. Telegram Bot-issued one-time login code (bot sends a short-lived code, user submits it to the API) — viable fallback if the Login Widget's domain-binding requirement (widget must be configured against a fixed public domain in BotFather) is impractical for early self-hosted/dev deployments.

## 6. Current unit cost computation and caching

- **Decision**: `ingredients.current_unit_cost` is a plain cached column (`NUMERIC(14,4)`, nullable until a first purchase exists), recomputed and written in the same DB transaction as each new `ingredient_purchases` insert, as `total_cost / quantity` of that latest purchase (per FR-010: "unit price of that ingredient's most recently recorded purchase"). No separate cache store (e.g., Redis) is introduced.
- **Rationale**: FR-011 only requires the cached value to "refresh whenever a new purchase ... is recorded" — a column updated transactionally alongside the purchase insert satisfies this with no additional infrastructure, and trivially meets the "within a few seconds" target in SC-002 since it's synchronous.
- **Alternatives considered**: Compute unit cost on-the-fly from the latest purchase row on every read (no cache column) — rejected only because it adds a subquery to every recipe-costing read path; caching is a pure performance optimization here, not a correctness requirement, since FR-010's rule ("most recent purchase") is deterministic either way.

## 7. Sale-time cost/price snapshotting

- **Decision**: When a sale is recorded, for each line item the service layer computes `unit_production_cost = Σ(recipe_quantity_i × ingredient_i.current_unit_cost)` over the item's current `item_ingredients` rows, and also snapshots `unit_price` from the item's current `sale_price`. Both values are written onto the `sale_items` row and never recomputed afterward.
- **Rationale**: FR-014/FR-015/FR-008/FR-012 require the production cost to be immune to later recipe/cost edits. Snapshotting `unit_price` as well (not explicitly required by the FRs, which only discuss production cost) is a deliberate extension so that revenue in past reports is equally immune to a later menu price change — consistent with SC-006's "0% drift ... after subsequent price or recipe updates" and with treating a recorded sale as an immutable historical transaction.
- **Alternatives considered**: Recompute revenue from the item's live `sale_price` at report time — rejected because it would make historical revenue (and therefore gross profit) drift whenever a menu price changes, contradicting the immutable-history theme of User Story 3/4 and SC-006.

## 8. Multi-tenant scoping enforcement

- **Decision**: Every scoped table (`ingredients`, `items`, `item_ingredients` via its item, `ingredient_purchases`, `sales`, `sale_items` via its sale) carries or resolves to a `user_id`. All repository methods require an authenticated `userID` argument and add `WHERE user_id = ?` (or the equivalent join) to every query; there is no repository method that reads/writes without a `userID`. Enforced in application code, not Postgres Row-Level Security.
- **Rationale**: Satisfies FR-002/SC-007 with a single, easily testable enforcement point (repository layer) that both the Telegram and web delivery paths funnel through, avoiding duplicated authorization logic in two UIs.
- **Alternatives considered**: PostgreSQL Row-Level Security (RLS) — a stronger defense-in-depth option, rejected for the initial implementation to keep the connection-pooling/session-role setup simple, but noted here as a reasonable future hardening step if a bug ever bypasses the repository layer.

## 9. Backup and restore

- **Decision**: Standard `pg_dump`/`pg_restore` (custom format, `-Fc`) against the PostgreSQL database, run as an operational/admin task (cron + `pg_dump` to storage; documented `pg_restore` runbook), not exposed as a Telegram command or dashboard feature.
- **Rationale**: Matches the spec's Assumptions section explicitly: "Backup and restore ... is an administrative/operational capability rather than a feature exposed for self-service use." `pg_dump`/`pg_restore` natively handle the full schema plus data (FR-021) with no custom export/import code needed.
- **Alternatives considered**: Continuous WAL archiving / point-in-time recovery — more robust but operationally heavier than this feature's scope calls for; can be layered on later purely as infrastructure, with no application changes.

## 10. Testing strategy

- **Decision**: `testify` for assertions; `testcontainers-go` (Postgres module) to run real-Postgres integration tests for repositories and cost/profit-snapshot behavior (these are the correctness-critical paths: FR-008, FR-012, FR-015, FR-018); `net/http/httptest` driving the Echo instance's `ServeHTTP` for API handler tests, asserting on JSON response bodies (status code + decoded/compared JSON, including that monetary fields are emitted as strings, not numbers) since Echo handlers are testable the same way as plain `net/http` ones; a small hand-written interface around the parts of the Telegram Bot API the handlers use, so bot command handlers are unit-testable without hitting Telegram's servers.
- **Rationale**: The feature's core value proposition is that historical figures never drift — this is exactly the kind of behavior that's easy to get subtly wrong with an ORM and hard to trust without tests against a real database engine's `NUMERIC` arithmetic and transaction semantics.
- **Alternatives considered**: `dockertest` instead of `testcontainers-go` — comparable capability; `testcontainers-go` chosen for more active maintenance and a first-class Postgres module.

## 11. Dashboard trend visualization

- **Decision**: This API has no rendering responsibility at all (see §1). `GET /api/reports/summary` returns a `trend` array of `{ date, revenue, cogs, gross_profit }` buckets (all monetary fields as decimal strings) for whatever out-of-scope client renders it, satisfying FR-017 as a data contract rather than a UI implementation.
- **Rationale**: Follows directly from the JSON-only decision in §1 — charting library choice (Chart.js, SVG, native mobile charts, etc.) is entirely the presentation client's concern and has no bearing on this backend's design.
- **Alternatives considered**: N/A — superseded by §1; the previous decision (Chart.js + htmx-rendered dashboard page) assumed a server-rendered view that no longer exists in this codebase.

## 12. API contract format

- **Decision**: `contracts/web-api.yaml` is a single OpenAPI 3.0.3 document covering every `/api/*` route, request/response schema, and error shape, using the `apiKey`-in-cookie `sessionCookie` security scheme and a shared `DecimalString` schema (pattern-constrained string) for every monetary/quantity field.
- **Rationale**: User-directed: "Change API Contract with OpenAPI Format YAML." A machine-readable OpenAPI document (vs. hand-written prose) can drive server stub/route generation, client SDKs, and request/response validation in CI, and gives implementers (and `/speckit-tasks`) an unambiguous, single source of truth for every endpoint's shape. Replaces the prior `contracts/web-api.md` prose contract, which is removed.
- **Alternatives considered**: Continue with the Markdown route-by-route prose contract — simpler to skim but not machine-checkable and prone to drifting from the implementation; superseded once an OpenAPI document was requested. The Telegram bot's command grammar (`contracts/telegram-commands.md`) remains Markdown, since there is no equivalent standard machine-readable format for a chat-command grammar the way OpenAPI covers HTTP.

## 13. Duplicate active-name detection

- **Decision**: `internal/service/catalog_service.go` does not pre-check for an existing active ingredient/item name before insert/update. It always attempts the write and translates a Postgres unique-violation (SQLSTATE `23505`, surfaced as `*pgconn.PgError` from the `gorm.io/driver/postgres`/`pgx` stack) on `data-model.md`'s partial unique index (`(user_id, lower(name))` where `is_archived = false`) into a single `service.ErrDuplicateName` sentinel, which the API layer maps to `409 duplicate_name` and the Telegram bot surfaces as a plain reply.
- **Rationale**: A check-then-insert (`SELECT` for an existing name, then `INSERT` if none found) has a race window between two concurrent requests for the same shop; letting Postgres's own unique index be the single source of truth avoids that race entirely and needs no advisory locking. Both ingredients and items share this path (`translateDuplicate` in `catalog_service.go`), even though `contracts/web-api.yaml` currently only documents the `409` response on `POST /ingredients` — items hit the same code path and constraint, so the contract has a minor, known drift here pending a documentation pass (see `tasks.md` T048).
- **Alternatives considered**: Pre-checking via `ListIngredients`/`ListItems` before writing — rejected for the race condition above. GORM's `TranslateError` config option (which would turn the same violation into `gorm.ErrDuplicatedKey`) — not enabled on the `gorm.Config{}` used by `cmd/server/main.go` and the integration test helper, so the raw driver error is inspected directly instead.
