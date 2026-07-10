---

description: "Task list for feature implementation"
---

# Tasks: Shop Profit Tracking

**Input**: Design documents from `/specs/001-shop-profit-tracking/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/telegram-commands.md, contracts/web-api.yaml, quickstart.md

**Tests**: Included. `plan.md`'s Technical Context and Constitution Check commit to `testcontainers-go` integration tests for the repository/service layer as required (not optional), given the correctness-critical nature of cost/profit snapshotting (FR-008, FR-012, FR-015, FR-018).

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3, US4)
- Every task includes an exact file path

## Path Conventions

Single Go project per `plan.md`'s Project Structure — `cmd/`, `internal/`, `migrations/`, `tests/` at repository root. No `frontend/`; the JSON API in `internal/api` is the only client-facing surface besides the Telegram bot in `internal/telegram`.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and basic structure

- [X] T001 Initialize the Go module (`go mod init github.com/fsetiawan29/profit-tracker`) and create the directory skeleton from `plan.md`: `cmd/server/`, `internal/{config,domain,repository,service,telegram,api/handlers,db}/`, `migrations/`, `tests/{integration,unit}/` (done manually by user)
- [X] T002 Add core dependencies to `go.mod`: `gorm.io/gorm`, `gorm.io/driver/postgres`, `github.com/go-telegram-bot-api/telegram-bot-api/v5`, `github.com/labstack/echo/v4`, `github.com/shopspring/decimal`, `github.com/golang-migrate/migrate/v4`, `github.com/stretchr/testify`, `github.com/testcontainers/testcontainers-go` + its `postgres` module (done manually by user)
- [X] T003 [P] Add `.golangci.yml` at repository root configuring `gofmt`/`go vet`/`golangci-lint` per Go conventions
- [X] T004 [P] internal/config/config.go: load and validate `DATABASE_URL`, `TELEGRAM_BOT_TOKEN`, `SESSION_SECRET`, `HTTP_ADDR` from environment variables per `quickstart.md`'s Setup section

**Checkpoint**: Module compiles, directory layout matches `plan.md`.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T005 internal/db/decimal.go: custom `Decimal` type wrapping `shopspring/decimal.Decimal` implementing `sql.Scanner`/`driver.Valuer`, for GORM `NUMERIC` columns (research.md §4)
- [X] T006 migrations/0001_init.up.sql and migrations/0001_init.down.sql: full schema for `users`, `ingredients`, `items`, `item_ingredients`, `ingredient_purchases`, `sales`, `sale_items` per `data-model.md` — `NUMERIC`/`TIMESTAMPTZ` column types, foreign keys, unique constraints (e.g. `(user_id, lower(name))` on ingredients/items), check constraints for positive quantities/costs
- [X] T007 [P] internal/domain/user.go: `User` struct (`ID`, `TelegramID`, `TelegramUsername`, `DisplayName`, timestamps) with `gorm` tags matching T006
- [X] T008 [P] internal/domain/ingredient.go: `Ingredient` struct (uses `db.Decimal` for `CurrentUnitCost`) + validation (`name`/`unit_of_measure` required)
- [X] T009 [P] internal/domain/item.go: `Item` struct (uses `db.Decimal` for `SalePrice`) + validation (`sale_price >= 0`)
- [X] T010 [P] internal/domain/recipe.go: `ItemIngredient` struct (uses `db.Decimal` for `Quantity`) + validation (`quantity > 0`)
- [X] T011 [P] internal/domain/purchase.go: `IngredientPurchase` struct (uses `db.Decimal` for `Quantity`/`TotalCost`/`UnitCost`) + validation (`quantity > 0`, `total_cost >= 0`)
- [X] T012 [P] internal/domain/sale.go: `Sale` and `SaleItem` structs (`SaleItem` uses `db.Decimal` for `Quantity`/`UnitPrice`/`UnitProductionCost`) + validation (`quantity > 0`)
- [X] T013 internal/repository/user_repo.go: `FindOrCreateByTelegramID` implementing shop auto-provisioning (FR-022), depends on T005–T007
- [X] T014 [P] internal/api/router.go: Echo app bootstrap — recovery/logger/CORS middleware, `/api/auth/*` (open) vs `/api/*` (session-guarded) route groups, JSON error handler producing the `{ "error", "code" }` shape from `contracts/web-api.yaml`
- [X] T015 [P] internal/api/session.go: signed, `HttpOnly`, `SameSite=Lax` session cookie issue/verify helpers keyed by `SESSION_SECRET` (research.md §5)
- [X] T016 internal/api/handlers/auth.go: Telegram Login Widget `hash` verification (HMAC-SHA256 per Telegram's algorithm) + `POST /api/auth/telegram`, `POST /api/auth/logout`, `GET /api/me` handlers per `contracts/web-api.yaml`, depends on T013–T015
- [X] T017 [P] internal/telegram/bot.go: bot command dispatcher skeleton + `/start` (auto-provision via T013) + `/help`, depends on T013
- [ ] T018 cmd/server/main.go: wire config (T004), connect GORM to Postgres, apply/verify migrations (T006), start the Echo server (T014/T016) and the Telegram long-polling loop (T017) concurrently with graceful shutdown

**Checkpoint**: Foundation ready — a Telegram account can `/start` (or `POST /api/auth/telegram`) and get an authenticated shop; user story implementation can now begin.

---

## Phase 3: User Story 1 - Set Up Shop Catalog (Priority: P1) 🎯 MVP

**Goal**: A shop owner can create ingredients, create menu items, attach a recipe (ingredient quantities) to an item, and archive any of these — all scoped strictly to their own shop.

**Independent Test**: Create an ingredient, create a menu item, attach a recipe, and confirm they appear in the shop's catalog — independent of any purchase or sale activity. Confirm a second shop cannot see or modify the first shop's catalog.

### Tests for User Story 1

- [ ] T019 [P] [US1] Integration test (testcontainers): ingredient create/list/update/archive + cross-shop access denied (404) in tests/integration/catalog_ingredient_test.go
- [ ] T020 [P] [US1] Integration test (testcontainers): item create/list/update/archive, recipe upsert/list/delete, and archiving an ingredient still referenced by an active recipe succeeds with a warning in tests/integration/catalog_item_test.go

### Implementation for User Story 1

- [ ] T021 [P] [US1] internal/repository/ingredient_repo.go: `Create`, `List` (with `includeArchived`), `Get`, `Update`, `SetArchived` — every method scoped by `userID`
- [ ] T022 [P] [US1] internal/repository/item_repo.go: `Create`, `List`, `Get`, `Update`, `SetArchived` for items, plus `UpsertRecipeLine`, `ListRecipe`, `DeleteRecipeLine` for `item_ingredients` — every method scoped by `userID`
- [ ] T023 [US1] internal/service/catalog_service.go: `CatalogService` orchestrating ingredient/item/recipe validation (duplicate active name → `409`, quantity/price rules) and the archive-while-referenced warning, depends on T021, T022
- [ ] T024 [US1] internal/telegram/handlers_catalog.go: `/ingredient add|list|archive`, `/item add|list|archive`, `/item recipe` commands per `contracts/telegram-commands.md`, depends on T023
- [ ] T025 [US1] internal/api/handlers/ingredients.go: `GET/POST /api/ingredients`, `PATCH /api/ingredients/{id}`, `POST /api/ingredients/{id}/archive` per `contracts/web-api.yaml`, depends on T023
- [ ] T026 [US1] internal/api/handlers/items.go: `GET/POST /api/items`, `PATCH /api/items/{id}`, `POST /api/items/{id}/archive`, `GET /api/items/{id}/recipe`, `PUT/DELETE /api/items/{id}/recipe/{ingredient_id}` per `contracts/web-api.yaml`, depends on T023
- [ ] T027 [US1] Register the ingredients and items route groups in internal/api/router.go, depends on T025, T026

**Checkpoint**: User Story 1 is fully functional and testable independently via both Telegram and the API.

---

## Phase 4: User Story 2 - Record Ingredient Purchases and Track Cost (Priority: P1)

**Goal**: A shop owner records ingredient purchases (quantity, total cost, date, notes); the system computes and refreshes the ingredient's current unit cost from the most recent purchase, without ever altering past purchase records.

**Independent Test**: Record one or more purchases for an ingredient and confirm the system computes an updated current unit cost, without needing any sales to exist.

### Tests for User Story 2

- [ ] T028 [P] [US2] Integration test (testcontainers): recording a purchase updates `current_unit_cost` only when it is the most recent purchase by date; earlier/out-of-order purchases never retroactively change it; past purchase rows are never mutated in tests/integration/purchase_test.go

### Implementation for User Story 2

- [ ] T029 [US2] internal/repository/purchase_repo.go: `Create` — inserts the purchase and, in the same DB transaction, recomputes `ingredients.current_unit_cost`/`current_unit_cost_updated_at` if this purchase is the ingredient's most recent (by `purchase_date`, tie-broken by `created_at`), depends on T021
- [ ] T030 [US2] internal/service/purchase_service.go: `PurchaseService.RecordPurchase` — validates `quantity > 0`, `total_cost >= 0`, defaults `purchase_date` to today, depends on T029
- [ ] T031 [US2] internal/telegram/handlers_purchase.go: `/purchase` command per `contracts/telegram-commands.md`, depends on T030
- [ ] T032 [US2] internal/api/handlers/purchases.go: `GET/POST /api/purchases` per `contracts/web-api.yaml`, depends on T030
- [ ] T033 [US2] Register the purchases route group in internal/api/router.go, depends on T032

**Checkpoint**: User Stories 1 AND 2 both work independently — the catalog exists and ingredient costs are tracked from purchase history.

---

## Phase 5: User Story 3 - Record Sales with Locked-In Cost (Priority: P1)

**Goal**: A shop owner records a sale of one or more items, via Telegram or the API, and the system permanently stores the production cost (and price) of each item as of the moment of sale.

**Independent Test**: Record a sale for an item with a known recipe and ingredient costs, then confirm the sale stores a fixed production cost, and that this stored cost does not change after later editing the recipe or ingredient costs.

### Tests for User Story 3

- [ ] T034 [P] [US3] Integration test (testcontainers): a sale's stored `unit_price`/`unit_production_cost` are unaffected by later ingredient cost changes or recipe edits (FR-008, FR-014, FR-015) in tests/integration/sale_test.go
- [ ] T035 [P] [US3] Integration test (testcontainers): a sale is rejected with no partial write when any line's recipe references an ingredient with `current_unit_cost = null` (data-model.md Edge Cases) in tests/integration/sale_test.go

### Implementation for User Story 3

- [ ] T036 [US3] internal/repository/sale_repo.go: `Create` — inserts a `sale` and its `sale_items` in a single DB transaction, `List` scoped by `userID` and date range, depends on T022 (item/recipe lookups)
- [ ] T037 [US3] internal/service/sales_service.go: `SalesService.RecordSale` — computes `unit_production_cost = Σ(recipe quantity × ingredient.current_unit_cost)` per line, snapshots `unit_price` from the item's current `sale_price`, rejects the whole sale if any line's ingredient cost is missing, validates `quantity > 0`, depends on T036, T021, T022
- [ ] T038 [US3] internal/telegram/handlers_sale.go: `/sale` command per `contracts/telegram-commands.md`, depends on T037
- [ ] T039 [US3] internal/api/handlers/sales.go: `GET/POST /api/sales` per `contracts/web-api.yaml`, depends on T037
- [ ] T040 [US3] Register the sales route group in internal/api/router.go, depends on T039

**Checkpoint**: User Stories 1, 2, AND 3 all work independently — the core transactional value (immutable cost/profit history) is in place.

---

## Phase 6: User Story 4 - View Profit Dashboard (Priority: P2)

**Goal**: A shop owner views revenue, cost of goods sold, and gross profit — plus trends — for a date range they select, with figures that never drift after later price/recipe/cost updates.

**Independent Test**: Seed sales across multiple dates, select different date ranges, and confirm revenue, cost of goods sold, gross profit, and trend figures match the underlying recorded sales for each range.

### Tests for User Story 4

- [ ] T041 [P] [US4] Integration test (testcontainers): summary/trend totals are correct for a populated range, empty for a zero-sale range, and byte-for-byte stable across repeated queries after later ingredient-cost/recipe edits (FR-018, SC-006) in tests/integration/reporting_test.go

### Implementation for User Story 4

- [ ] T042 [US4] internal/service/reporting_service.go: `ReportingService.Summary(userID, from, to)` — aggregates `sale_items` joined to `sales.sold_at` into total revenue/COGS/gross profit plus a per-day `trend` array, depends on T036
- [ ] T043 [US4] internal/telegram/handlers_report.go: `/report` command (`today|week|month|custom range`) per `contracts/telegram-commands.md`, reusing `ReportingService` so bot and API always agree, depends on T042
- [ ] T044 [US4] internal/api/handlers/reports.go: `GET /api/reports/summary` per `contracts/web-api.yaml`, depends on T042
- [ ] T045 [US4] Register the reports route group in internal/api/router.go, depends on T044

**Checkpoint**: All user stories are independently functional.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Operational readiness and final validation across all stories

- [ ] T046 [P] scripts/backup.sh and scripts/restore.sh: `pg_dump`/`pg_restore` (custom format) operational runbook per FR-021 and research.md §9
- [ ] T047 [P] docker-compose.yml at repository root: local PostgreSQL 15 service for development, matching `quickstart.md`'s Prerequisites
- [ ] T048 Validate contracts/web-api.yaml against the implemented `internal/api/handlers/*` routes (e.g. `swagger-cli validate` plus a manual route-by-route diff) so the OpenAPI contract has no drift from the running API
- [ ] T049 Execute every scenario in quickstart.md (all four user stories plus the Edge Cases section) end-to-end against a running instance and fix any discrepancies found

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Setup completion — BLOCKS all user stories
- **User Story 1 (Phase 3)**: Depends only on Foundational
- **User Story 2 (Phase 4)**: Depends on Foundational; its repository/service/handlers build on `ingredient_repo.go` from US1 (T021), so implement after US1 in practice even though the story is conceptually independent
- **User Story 3 (Phase 5)**: Depends on Foundational; builds on `ingredient_repo.go`/`item_repo.go` from US1 (T021, T022) and reads `current_unit_cost` populated by US2 — implement after US1 and US2
- **User Story 4 (Phase 6)**: Depends on Foundational; builds on `sale_repo.go` from US3 (T036) — implement after US3
- **Polish (Phase 7)**: Depends on all four user stories being complete

### Within Each User Story

- Tests written first, expected to fail until the corresponding repository/service/handlers exist
- Repositories before services
- Services before Telegram/API handlers
- Handlers before route registration

### Parallel Opportunities

- Setup: T003, T004 in parallel once T001–T002 are done
- Foundational: T007–T012 (domain structs) in parallel once T005/T006 are done; T014, T015, T017 in parallel once T013 is done
- US1: T019, T020 (tests) in parallel; T021, T022 (repositories) in parallel
- US2: only T028 is parallelizable (single test task); T029–T033 are a sequential chain
- US3: T034, T035 (tests) in parallel
- US4: only T041 is parallelizable (single test task)
- Polish: T046, T047 in parallel

---

## Parallel Example: User Story 1

```bash
# Launch both integration tests for User Story 1 together:
Task: "Integration test: ingredient create/list/update/archive + cross-shop denial in tests/integration/catalog_ingredient_test.go"
Task: "Integration test: item/recipe CRUD + archive-while-referenced warning in tests/integration/catalog_item_test.go"

# Launch both repositories for User Story 1 together:
Task: "internal/repository/ingredient_repo.go: Create/List/Get/Update/SetArchived"
Task: "internal/repository/item_repo.go: Create/List/Get/Update/SetArchived + recipe methods"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL — blocks all stories)
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: run the User Story 1 independent test (see quickstart.md scenario 1)
5. Deploy/demo if ready — a shop owner can already build and browse a catalog, even with no purchase/sale/reporting capability yet

### Incremental Delivery

1. Setup + Foundational → foundation ready (auth works end-to-end)
2. Add User Story 1 → catalog works → demo
3. Add User Story 2 → ingredient costs tracked → demo
4. Add User Story 3 → sales recorded with locked-in cost → demo (core value delivered)
5. Add User Story 4 → profit dashboard → demo (full feature complete)
6. Polish → operational readiness (backup/restore, contract validation, full quickstart pass)

### Notes

- [P] tasks = different files, no unmet dependencies
- [Story] label maps task to its user story for traceability
- Although US2–US4 build on repositories introduced in earlier stories (unavoidable given the domain: costs come from purchases, sales come from a costed catalog, reports come from sales), each story still adds an independently testable increment and no story's implementation is blocked by a *later* story
- Commit after each task or logical group
- Stop at any checkpoint to validate a story independently
