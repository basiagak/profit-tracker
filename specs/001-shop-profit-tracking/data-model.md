# Phase 1 Data Model: Shop Profit Tracking

Entities match the spec's Core Data Model list exactly (`users, items, ingredients, item_ingredients, ingredient_purchases, sales, sale_items`). There is no separate `shops` table: each `user` row *is* a shop (FR-023 — one user owns exactly one shop), so `user_id` is the tenant-scoping key on every other table.

All monetary/quantity columns are `NUMERIC` and map to `shopspring/decimal.Decimal` in Go (never `float64` — FR-020, see [research.md §4](./research.md)). All timestamps are `TIMESTAMPTZ`.

## users

The shop owner, identified by Telegram account. Auto-provisioned on first Telegram contact (FR-022) or first successful Telegram Login Widget callback on the dashboard.

| Column | Type | Notes |
|---|---|---|
| `id` | `BIGSERIAL PK` | internal ID, referenced by all other tables |
| `telegram_id` | `BIGINT NOT NULL UNIQUE` | Telegram numeric user ID; the authentication key (FR-001) |
| `telegram_username` | `TEXT NULL` | denormalized for display; Telegram usernames are optional/mutable |
| `display_name` | `TEXT NULL` | Telegram first/last name at last login, for greeting/display only |
| `created_at` | `TIMESTAMPTZ NOT NULL DEFAULT now()` | |
| `updated_at` | `TIMESTAMPTZ NOT NULL DEFAULT now()` | |

**Validation**: `telegram_id` required and unique — this is the sole identity/authorization key (FR-001, FR-023).

**Relationships**: one `user` → many `ingredients`, `items`, `sales` (and transitively `ingredient_purchases`, `item_ingredients`, `sale_items`).

## ingredients

A raw material purchased and consumed via recipes.

| Column | Type | Notes |
|---|---|---|
| `id` | `BIGSERIAL PK` | |
| `user_id` | `BIGINT NOT NULL REFERENCES users(id)` | tenant scope (FR-002) |
| `name` | `TEXT NOT NULL` | |
| `unit_of_measure` | `TEXT NOT NULL` | e.g. `kg`, `g`, `l`, `pcs` — free text, not a fixed enum (spec does not constrain units) |
| `current_unit_cost` | `NUMERIC(14,4) NULL` | cached derived value; `NULL` until the first purchase is recorded (see Edge Cases) |
| `current_unit_cost_updated_at` | `TIMESTAMPTZ NULL` | set alongside `current_unit_cost` |
| `is_archived` | `BOOLEAN NOT NULL DEFAULT false` | soft-delete flag (FR-005) |
| `created_at` / `updated_at` | `TIMESTAMPTZ NOT NULL DEFAULT now()` | |

**Validation**: `name` and `unit_of_measure` required (FR-003). Unique `(user_id, lower(name))` among non-archived rows, to avoid duplicate active ingredients per shop while still allowing an archived name to be reused.

**State transitions**: `is_archived` toggles `false → true` (archive) and may toggle back (`true → false`) to support correcting an accidental archive; archived ingredients are excluded from selection lists for new recipes/purchases (FR-005) but remain readable in history.

## items

A menu item sold to customers.

| Column | Type | Notes |
|---|---|---|
| `id` | `BIGSERIAL PK` | |
| `user_id` | `BIGINT NOT NULL REFERENCES users(id)` | tenant scope |
| `name` | `TEXT NOT NULL` | |
| `sale_price` | `NUMERIC(12,2) NOT NULL` | current price; used for *future* sales only, see `sale_items.unit_price` |
| `is_archived` | `BOOLEAN NOT NULL DEFAULT false` | |
| `created_at` / `updated_at` | `TIMESTAMPTZ NOT NULL DEFAULT now()` | |

**Validation**: `name` and `sale_price` required (FR-004); `sale_price >= 0`. Unique `(user_id, lower(name))` among non-archived rows.

**State transitions**: same archive/unarchive semantics as `ingredients` (FR-005).

## item_ingredients (recipe)

The current recipe: which ingredients, and how much of each, make one unit of an item. This table always reflects the *current* recipe — editing it never touches past sales, because `sale_items` stores its own independent snapshot (see below).

| Column | Type | Notes |
|---|---|---|
| `id` | `BIGSERIAL PK` | |
| `item_id` | `BIGINT NOT NULL REFERENCES items(id)` | |
| `ingredient_id` | `BIGINT NOT NULL REFERENCES ingredients(id)` | |
| `quantity` | `NUMERIC(14,4) NOT NULL` | amount of the ingredient (in its `unit_of_measure`) needed per one unit of the item |
| `created_at` / `updated_at` | `TIMESTAMPTZ NOT NULL DEFAULT now()` | |

**Validation**: `quantity > 0` (FR-009-adjacent validation rule, see Edge Cases: zero/negative quantities rejected). `item_id` and `ingredient_id` must belong to the same `user_id` (cross-tenant recipe references rejected — enforced in the service layer, not just FK). Unique `(item_id, ingredient_id)` — one row per ingredient per item; editing a quantity updates the existing row in place (FR-007, FR-008).

**Relationships**: many-to-many between `items` and `ingredients`, with `quantity` as the association attribute.

## ingredient_purchases

An immutable record of buying a quantity of an ingredient for a total cost on a date.

| Column | Type | Notes |
|---|---|---|
| `id` | `BIGSERIAL PK` | |
| `user_id` | `BIGINT NOT NULL REFERENCES users(id)` | denormalized tenant scope for direct scoped queries (matches `ingredients.user_id`) |
| `ingredient_id` | `BIGINT NOT NULL REFERENCES ingredients(id)` | |
| `quantity` | `NUMERIC(14,4) NOT NULL` | |
| `total_cost` | `NUMERIC(14,4) NOT NULL` | |
| `unit_cost` | `NUMERIC(14,4) NOT NULL` | stored, = `total_cost / quantity` at insert time, for historical display without recomputation |
| `purchase_date` | `DATE NOT NULL` | |
| `notes` | `TEXT NULL` | |
| `created_at` | `TIMESTAMPTZ NOT NULL DEFAULT now()` | |

**Validation**: `quantity > 0`, `total_cost >= 0` (FR-009; Edge Cases: reject zero/negative quantity, negative cost). No `updated_at` — rows are never updated after insert (FR-012: "MUST NOT alter any previously recorded purchase").

**Side effect on insert** (same transaction): recompute `ingredients.current_unit_cost` from this purchase's `unit_cost` if it is the most recent purchase (by `purchase_date`, tie-broken by `created_at`) for that ingredient (FR-010, FR-011; see [research.md §6](./research.md)).

## sales

A single sale transaction, one or more line items.

| Column | Type | Notes |
|---|---|---|
| `id` | `BIGSERIAL PK` | |
| `user_id` | `BIGINT NOT NULL REFERENCES users(id)` | tenant scope |
| `sold_at` | `TIMESTAMPTZ NOT NULL DEFAULT now()` | when the sale occurred |
| `source` | `TEXT NOT NULL CHECK (source IN ('telegram','dashboard'))` | which surface recorded it (FR-013) |
| `created_at` | `TIMESTAMPTZ NOT NULL DEFAULT now()` | |

**Validation**: must have at least one associated `sale_items` row (enforced in the service layer at creation — a sale is created together with its line items in one transaction).

## sale_items

One line within a sale: quantity of one item sold, with its production cost **and** price permanently snapshotted at the moment of sale.

| Column | Type | Notes |
|---|---|---|
| `id` | `BIGSERIAL PK` | |
| `sale_id` | `BIGINT NOT NULL REFERENCES sales(id)` | |
| `item_id` | `BIGINT NOT NULL REFERENCES items(id)` | items are only archived, never hard-deleted, so this FK always resolves |
| `quantity` | `NUMERIC(14,4) NOT NULL` | units of the item sold |
| `unit_price` | `NUMERIC(12,2) NOT NULL` | snapshot of `items.sale_price` at sale time (see [research.md §7](./research.md)) |
| `unit_production_cost` | `NUMERIC(14,4) NOT NULL` | snapshot: Σ(`item_ingredients.quantity` × `ingredients.current_unit_cost`) at sale time (FR-014) |
| `created_at` | `TIMESTAMPTZ NOT NULL DEFAULT now()` | |

**Validation**: `quantity > 0`. `unit_price` and `unit_production_cost` are set once at insert and never updated (FR-015) — no application code path performs an `UPDATE` on this table after creation.

**Derived (not stored, computed at query time from the snapshots above)**: `line_revenue = quantity × unit_price`; `line_cost = quantity × unit_production_cost`; `line_gross_profit = line_revenue − line_cost`. Dashboard aggregates (FR-016, FR-017) sum these per period directly from `sale_items` joined to `sales.sold_at`, so a report for a past range is always reproducible byte-for-byte from immutable snapshot data (FR-018, SC-006).

## Edge case handling (from spec)

- **Sale of an item whose recipe includes an ingredient with no purchases yet** (`current_unit_cost IS NULL`): the sale-recording service rejects the sale for that item with a clear error ("ingredient X has no recorded cost yet") rather than silently treating the cost as zero, since a silent zero would understate COGS and overstate profit.
- **Archiving an ingredient/item still referenced by an active recipe**: allowed — archiving only affects *selection lists* for new recipes/purchases/sales (FR-005); it does not delete or block the existing `item_ingredients` row, so existing recipes keep working for costing until the shop owner explicitly edits them. The UI surfaces a warning listing which active items reference the ingredient being archived.
- **Zero/negative/missing quantity or cost on purchase/sale**: rejected at the service layer with a validation error before any DB write (backed by the `CHECK`-equivalent validation rules above).
- **Malformed/incomplete Telegram command**: bot replies with a usage hint for that command; no partial DB write occurs (validation happens before any repository call).
- **Unrecognized Telegram account before "authentication"**: per FR-022 there is no separate approval step — any Telegram account issuing any command is auto-provisioned a `users` row on first contact, so this case reduces to "first command creates the account and proceeds."
- **Date range with no sales**: dashboard renders zero-valued revenue/COGS/profit and an empty trend series rather than erroring.
- **Database restore mid-sale**: out of scope for application logic — `pg_restore` runs against a stopped/quiesced database per the operational runbook ([research.md §9](./research.md)); no application-level "partial sale" state exists since a sale and its `sale_items` are written in one DB transaction.
