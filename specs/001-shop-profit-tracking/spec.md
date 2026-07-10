# Feature Specification: Shop Profit Tracking

**Feature Branch**: `001-shop-profit-tracking`

**Created**: 2026-07-10

**Status**: Draft

**Input**: User description: "Functional Requirements

Users authenticate via Telegram and manage only their own shop data.
Users can create, update, archive, and view ingredients and menu items.
Users can define recipes by specifying ingredient quantities per item.
Users can record ingredient purchases (quantity, total cost, date, notes).
The system computes and optionally caches the current unit cost of each ingredient.
Users can record sales through Telegram or the web dashboard.
At the time of sale, the system stores the effective production cost per item sold for historical profit reporting.
The dashboard displays revenue, cost of goods sold, gross profit, and trends over selectable date ranges.

Non-Functional Requirements

Telegram commands should complete within a few seconds under normal load.
All monetary values should use fixed-precision decimals.
Every change must be scoped to the authenticated user's shop.
The system should support backup and restore of the PostgreSQL database.

Core Data Model

users
items
ingredients
item_ingredients
ingredient_purchases
sales
sale_items

Acceptance Criteria

Updating an ingredient's current cost does not alter historical purchases or past profit calculations.
Editing a recipe affects only future cost calculations.
Profit reports for a given period remain unchanged after later price updates."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Set Up Shop Catalog (Priority: P1)

A shop owner sets up their shop by defining the ingredients they buy, the menu items they sell, and the recipe (ingredient quantities) that make up each item.

**Why this priority**: Every other capability (purchases, sales, cost/profit reporting) depends on a catalog of ingredients, items, and recipes existing first. Without this, there is nothing to track.

**Independent Test**: Can be fully tested by creating an ingredient, creating a menu item, attaching a recipe (ingredient quantities) to that item, and confirming they appear in the shop's catalog — independent of any purchase or sale activity.

**Acceptance Scenarios**:

1. **Given** an authenticated shop owner with no existing catalog, **When** they create a new ingredient with a name and unit of measure, **Then** the ingredient appears in their shop's ingredient list.
2. **Given** an authenticated shop owner, **When** they create a new menu item and define its recipe by specifying ingredients and quantities, **Then** the item is available for future sales with its recipe attached.
3. **Given** an existing ingredient or item, **When** the shop owner archives it, **Then** it no longer appears in lists used for new recipes, purchases, or sales, but remains visible in historical records.
4. **Given** an ingredient or item belonging to another shop owner, **When** this shop owner attempts to view, edit, or archive it, **Then** the system denies the action.

---

### User Story 2 - Record Ingredient Purchases and Track Cost (Priority: P1)

A shop owner records each ingredient purchase (quantity, total cost, date, notes) so the system can compute and track the current unit cost of that ingredient.

**Why this priority**: Accurate ingredient costs are the foundation of production cost and profit calculations; this must work before sales-time costing can be trusted.

**Independent Test**: Can be fully tested by recording one or more purchases for an ingredient and confirming the system computes an updated current unit cost, without needing any sales to exist.

**Acceptance Scenarios**:

1. **Given** an existing ingredient, **When** the shop owner records a purchase with quantity, total cost, date, and optional notes, **Then** the purchase is saved and the ingredient's current unit cost is recomputed.
2. **Given** an ingredient with prior purchases, **When** the shop owner records a new purchase, **Then** previously recorded purchases remain unchanged.
3. **Given** an ingredient's current unit cost has changed, **When** the shop owner views past purchase records, **Then** those records still show their original recorded quantity, cost, and date.

---

### User Story 3 - Record Sales with Locked-In Cost (Priority: P1)

A shop owner records a sale of one or more menu items, via Telegram or the web dashboard, and the system permanently stores the production cost of each item as of the moment of sale.

**Why this priority**: Capturing revenue and a point-in-time cost snapshot per sale is the core transaction the whole feature exists to support — it is what makes trustworthy profit reporting possible.

**Independent Test**: Can be fully tested by recording a sale for an item with a known recipe and ingredient costs, then confirming the sale stores a fixed production cost, and that this stored cost does not change after later editing the recipe or ingredient costs.

**Acceptance Scenarios**:

1. **Given** a menu item with a defined recipe and ingredients with known current unit costs, **When** the shop owner records a sale of that item (via Telegram or dashboard), **Then** the system stores the item's production cost, computed from the recipe and ingredient costs at that moment, alongside the sale.
2. **Given** a previously recorded sale, **When** the shop owner later updates an ingredient's cost or edits the item's recipe, **Then** the previously recorded sale's stored production cost and profit remain unchanged.
3. **Given** a shop owner issuing a sale-recording command in Telegram, **When** the command is submitted, **Then** the system confirms the recorded sale within a few seconds under normal load.

---

### User Story 4 - View Profit Dashboard (Priority: P2)

A shop owner views a dashboard showing revenue, cost of goods sold, gross profit, and trends over a date range they select.

**Why this priority**: Reporting is the payoff of tracking purchases and sales, but it depends on User Stories 1–3 already producing data; it is valuable but not a blocker for the underlying data being correct.

**Independent Test**: Can be fully tested by seeding sales across multiple dates, selecting different date ranges on the dashboard, and confirming revenue, cost of goods sold, gross profit, and trend figures match the underlying recorded sales for each range.

**Acceptance Scenarios**:

1. **Given** recorded sales within a selected date range, **When** the shop owner views the dashboard, **Then** it displays total revenue, cost of goods sold, and gross profit for that range.
2. **Given** sales spread across multiple periods, **When** the shop owner changes the selected date range, **Then** the trend view updates to reflect only sales within the newly selected range.
3. **Given** a profit report was generated for a past date range, **When** ingredient costs or recipes are updated afterward and the same date range is viewed again, **Then** the reported revenue, cost of goods sold, and gross profit figures are identical to the original report.

---

### Edge Cases

- What happens when a shop owner records a sale for an item whose recipe includes an ingredient that has no recorded purchases yet (no cost data available)?
- What happens when a shop owner attempts to archive an ingredient or item that is still referenced by an active (non-archived) recipe?
- How does the system handle a purchase or sale submission with a zero, negative, or missing quantity/cost value?
- What happens when a shop owner sends a malformed or incomplete Telegram command (e.g., missing quantity)?
- What happens when an unrecognized Telegram account issues a command before completing authentication/setup?
- How does the system behave for a selected date range with no recorded sales (e.g., empty dashboard state)?
- What happens to in-flight or partially recorded sales if a database restore is performed?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Users MUST authenticate via their Telegram account before performing any catalog, purchase, sale, or reporting action, on Telegram or the web dashboard.
- **FR-002**: System MUST scope every ingredient, item, recipe, purchase, and sale record to the authenticated user's shop, and MUST prevent any user from viewing or modifying another user's shop data.
- **FR-003**: Users MUST be able to create, update, archive, and view ingredients, each with at minimum a name and unit of measure.
- **FR-004**: Users MUST be able to create, update, archive, and view menu items, each with at minimum a name and sale price.
- **FR-005**: Archiving an ingredient or item MUST remove it from selection lists used for new recipes, purchases, or sales, while preserving it and its history for existing records and reports.
- **FR-006**: Users MUST be able to define a recipe for a menu item by specifying which ingredients, and what quantity of each, are required to produce one unit of that item.
- **FR-007**: Users MUST be able to edit an existing recipe's ingredients or quantities at any time.
- **FR-008**: Editing a recipe MUST affect only sales recorded after the edit; it MUST NOT alter the stored production cost or profit of any sale recorded before the edit.
- **FR-009**: Users MUST be able to record an ingredient purchase with a quantity, total cost, purchase date, and optional notes.
- **FR-010**: System MUST compute the current unit cost of each ingredient as the unit price of that ingredient's most recently recorded purchase.
- **FR-011**: System MAY cache the computed current unit cost of an ingredient for faster lookups, and MUST refresh that cached value whenever a new purchase affecting that ingredient is recorded.
- **FR-012**: Recording a new ingredient purchase MUST NOT alter any previously recorded purchase, or any production cost already stored on a past sale.
- **FR-013**: Users MUST be able to record a sale of one or more menu items through either Telegram commands or the web dashboard.
- **FR-014**: At the moment a sale is recorded, System MUST compute and permanently store the production cost of each item sold, derived from that item's recipe and the ingredients' current unit costs at that moment.
- **FR-015**: Later changes to an ingredient's current unit cost or to an item's recipe MUST NOT alter the production cost or profit figures already stored on previously recorded sales.
- **FR-016**: Dashboard MUST display total revenue, cost of goods sold, and gross profit for a date range selected by the user.
- **FR-017**: Dashboard MUST display revenue, cost of goods sold, and gross profit trends over time within the selected date range.
- **FR-018**: A profit report generated for a given date range MUST return identical revenue, cost of goods sold, and gross profit figures if the same range is queried again later, regardless of subsequent ingredient cost or recipe updates.
- **FR-019**: Telegram commands MUST complete (respond to the user) within a few seconds under normal load.
- **FR-020**: All monetary values (costs, prices, revenue, profit) MUST be stored and displayed using fixed-precision decimal representation, avoiding floating-point rounding error.
- **FR-021**: System MUST support backing up and restoring the full database, including all catalog, purchase, and sale history.
- **FR-022**: System MUST automatically provision a new shop for a Telegram account the first time it issues a command, requiring no separate invite or approval step.
- **FR-023**: Each authenticated user MUST have their data scoped to exactly one shop; a user account owns and manages a single shop.

### Key Entities

- **User**: A shop owner identified by their Telegram account; the boundary to which all of that shop's ingredients, items, purchases, and sales are scoped.
- **Ingredient**: A raw material used to produce menu items; has a name, a unit of measure, and a computed (optionally cached) current unit cost derived from purchase history.
- **Item**: A menu item sold to customers; has a name and a sale price, and is composed of ingredients through a recipe.
- **Item Ingredient (Recipe)**: The association between an item and an ingredient, specifying the quantity of that ingredient required to produce one unit of the item; recipes can change over time without affecting past sales.
- **Ingredient Purchase**: A record of buying a quantity of an ingredient for a total cost on a given date, with optional notes; the historical basis from which current unit cost is computed.
- **Sale**: A transaction recording that one or more items were sold, including when it occurred.
- **Sale Item**: A line within a sale for one menu item, including the quantity sold and the production cost snapshot computed and stored at the moment of sale, used for permanent historical profit reporting.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A shop owner can complete initial catalog setup — one ingredient, one menu item, and its recipe — in under 10 minutes.
- **SC-002**: A shop owner recording an ingredient purchase sees the ingredient's updated current unit cost reflected within a few seconds of submission.
- **SC-003**: A shop owner recording a sale via Telegram receives confirmation within a few seconds under normal load.
- **SC-004**: The dashboard displays revenue, cost of goods sold, and gross profit for a selected date range within a few seconds, for shops with several years of sales history.
- **SC-005**: 100% of previously recorded sales retain their originally computed production cost and profit after any later ingredient cost or recipe change.
- **SC-006**: Profit figures returned for a previously reported date range are identical (0% drift) when the same range is queried again after subsequent price or recipe updates.
- **SC-007**: In testing, no user is ever able to view or modify another user's shop data.
- **SC-008**: A shop's full data set can be backed up and subsequently restored with no loss of catalog, purchase, or sales history.

## Assumptions

- "Archive" is a soft-delete: archived ingredients and items are excluded from new recipe, purchase, and sale entry, but remain visible in historical records and past reports; hard deletion is out of scope.
- The web dashboard is a read/report-and-manage surface tied to the same Telegram-authenticated identity as the bot (e.g., via a linked login/session), not a separate independent account system.
- Currency is single and consistent per shop; multi-currency support is out of scope for this feature.
- Backup and restore of the database is an administrative/operational capability rather than a feature exposed for self-service use by shop owners through Telegram or the dashboard.
- "A few seconds" for Telegram command completion is treated as a soft target of roughly 3-5 seconds under normal load, consistent with typical chat-bot responsiveness expectations.
