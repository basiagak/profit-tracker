# Implementation Plan: Shop Profit Tracking

**Branch**: `001-shop-profit-tracking` | **Date**: 2026-07-10 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/001-shop-profit-tracking/spec.md`

**Note**: This template is filled in by the `/speckit-plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

A multi-tenant shop profit tracker for small food/beverage shop owners. Owners authenticate with their Telegram account (one account = one shop, auto-provisioned on first contact) and manage ingredients, menu items, and recipes; record ingredient purchases to derive a current unit cost per ingredient; record sales via Telegram commands or a web dashboard, with the production cost of each sold item permanently snapshotted at sale time so historical profit is immune to later price/recipe edits; and view a dashboard of revenue, cost of goods sold (COGS), and gross profit trends over a selected date range.

Technical approach: a single Go service exposes two front doors — a long-polling Telegram bot and a JSON HTTP API for the web dashboard (no server-rendered HTML; the dashboard's presentation layer is a separate, out-of-scope client that consumes this API) — sharing one domain/service layer backed by PostgreSQL via GORM. All monetary and quantity values use `NUMERIC` columns mapped to `shopspring/decimal` in Go to avoid floating-point drift, and are serialized on the wire as JSON strings (never bare JSON numbers) for the same reason. Every table scoped to a shop carries a `user_id` and every query is filtered by the authenticated user's ID at the repository layer.

## Technical Context

**Language/Version**: Go 1.25

**Primary Dependencies**: GORM (`gorm.io/gorm`, `gorm.io/driver/postgres`) for persistence; `go-telegram-bot-api/v5` (long polling) for the Telegram bot; Echo (`github.com/labstack/echo/v4`) as the HTTP router/middleware layer for a JSON-only REST API (`c.JSON`, no `html/template`, no server-rendered views); `shopspring/decimal` for fixed-precision money/quantity math; `golang-migrate/migrate` for versioned SQL schema migrations; `joho/godotenv` to load a local `.env` file in development (never required in production, where real environment variables are set directly).

**Storage**: PostgreSQL 15+, `NUMERIC` columns for all monetary/quantity values, `TIMESTAMPTZ` for all timestamps.

**Testing**: `go test` + `testify` for unit tests; `testcontainers-go` (Postgres module) for repository/integration tests against a real database, gated behind an `integration` build tag (`go test -tags=integration ./tests/integration/...`) so the default `go test ./...` needs no Docker; `net/http/httptest` driving Echo's `e.ServeHTTP` for web handler tests; an interface-mocked Telegram Bot API for bot command-handler unit tests.

**Target Platform**: Linux server, single self-contained Go binary (bot + dashboard + migrations run from one process/deployment artifact).

**Project Type**: Web service (single backend binary serving a Telegram bot client and a JSON HTTP API; no HTML/view layer in this codebase) — Option 1 (single project) structure. This is still not the "frontend/backend split" template option, because there is no frontend project *inside this repository/feature* — any UI that consumes the JSON API is out of scope for this backend feature.

**Performance Goals**: Telegram commands acknowledge within ~3-5s under normal load (SC-003, FR-019); dashboard summary/trend queries return within a few seconds even for shops with several years of sales history (SC-004).

**Constraints**: All monetary and quantity values fixed-precision decimal, never `float64` (FR-020), including on the wire — API responses encode these fields as JSON strings, not JSON numbers, so no client-side float coercion can reintroduce rounding error; every read/write scoped to `user_id` with no cross-shop leakage (FR-002, SC-007); production cost on a `sale_item` is computed once at sale time and never recomputed or mutated afterward (FR-014, FR-015, FR-008, FR-012); database must be backup/restorable via standard PostgreSQL tooling (FR-021).

**Scale/Scope**: Small-to-medium multi-tenant deployment — many independent single-owner shops, each with a modest catalog (tens of ingredients/items) and up to several years of purchase/sale history per shop. No cross-shop aggregation or admin-facing multi-shop views in this feature.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

`.specify/memory/constitution.md` in this repository is still the unpopulated template (all principle sections contain placeholder text like `[PRINCIPLE_1_NAME]`), so no project-specific principles or gates have been ratified yet. There is nothing to check this plan against, and therefore no violations to justify in Complexity Tracking.

**Sensible defaults applied in the absence of a ratified constitution** (documented here so they're visible for later review, not because a gate demanded them):
- Single-project structure preferred over splitting the dashboard into a separate frontend project (no stated need for independent frontend deployment/scaling).
- Integration tests against a real PostgreSQL instance (via testcontainers) are treated as required for the repository layer, given the correctness-critical nature of cost/profit snapshotting (FR-008, FR-012, FR-015, FR-018).
- No speculative abstractions (e.g., no plugin system, no multi-currency, no generic reporting engine) beyond what FR-001 through FR-023 require, per the Assumptions section of the spec.

## Project Structure

### Documentation (this feature)

```text
specs/001-shop-profit-tracking/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 output (/speckit-plan command)
├── data-model.md        # Phase 1 output (/speckit-plan command)
├── quickstart.md        # Phase 1 output (/speckit-plan command)
├── contracts/           # Phase 1 output (/speckit-plan command)
│   ├── telegram-commands.md
│   └── web-api.yaml     # OpenAPI 3.0 contract for the JSON dashboard API
├── checklists/
│   └── requirements.md
└── tasks.md              # Phase 2 output (/speckit-tasks command - NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
cmd/
└── server/
    └── main.go              # wires config, DB, GORM, telegram bot (long polling), and HTTP server; starts both concurrently

internal/
├── config/                  # env-based configuration (DB DSN, Telegram bot token, HTTP port, session secret)
├── domain/                  # entity structs + pure business rules: User, Ingredient, Item, ItemIngredient,
│                             #   IngredientPurchase, Sale, SaleItem; decimal-based cost/profit calculations
├── repository/               # GORM-backed repositories, one per aggregate; every method takes/filters by userID
│   ├── ingredient_repo.go
│   ├── item_repo.go
│   ├── purchase_repo.go
│   └── sale_repo.go
├── service/                  # use-case orchestration: CatalogService, PurchaseService, SalesService, ReportingService
├── telegram/                  # bot command router + handlers (/start, /ingredient, /item, /purchase, /sale, /report)
├── api/                       # Echo router setup, JSON handlers, Telegram Login Widget verification, session middleware
│   └── handlers/               # one file per resource: ingredients, items, purchases, sales, reports, auth
└── db/
    └── decimal.go             # shopspring/decimal <-> NUMERIC GORM scan/value glue

migrations/                    # golang-migrate SQL files: 0001_init.up.sql / .down.sql, ...

tests/
├── integration/                # testcontainers-backed repository + service tests
└── unit/                       # pure domain logic tests (cost/profit calculations, validation)
```

**Structure Decision**: Single Go project (repository root as the module root). The web dashboard's backend is a JSON API served from the same binary as the Telegram bot; there is no `html/template`/view layer in this codebase — any HTML/JS presentation is a separate, out-of-scope client of this API, per the chosen architecture (Echo JSON API, GORM, go-telegram-bot-api long polling). `internal/` holds all application code (Go convention preventing external import), split by layer (`domain` → `repository` → `service` → delivery via `telegram`/`api`), so the two entry points share one tested core instead of duplicating business rules.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

*No violations — constitution is unratified, no gates were evaluated.*
