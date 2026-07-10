# Telegram Command Contract

All commands require the sender's Telegram account to resolve to a `users` row; if none exists, one is auto-provisioned before the command executes (FR-022). Every command's data access is scoped to that `users.id` (FR-002). Replies are sent within a few seconds under normal load (FR-019, SC-003).

Argument syntax: `<required>`, `[optional]`. Free-text arguments (names, notes) containing spaces should be the last argument or the implementation should support quoting; exact tokenizer choice is an implementation detail, not a contract concern.

## /start

`/start`

First contact: auto-provisions the shop (if not already provisioned) and replies with a welcome message plus a summary of available commands.

## /help

`/help [command]`

Replies with the command list, or usage for one `command` if given.

## Ingredients

`/ingredient add <name> <unit>`
Creates an ingredient. **Reply**: confirmation with the new ingredient's name/unit. **Errors**: duplicate active name for this shop → rejected with existing-ingredient message.

`/ingredient list`
**Reply**: table of non-archived ingredients with name, unit, current unit cost (or "no cost yet" if `NULL`).

`/ingredient archive <name>`
**Reply**: confirmation; if any active item recipe references it, the reply includes a warning listing those items (does not block the archive — see data-model.md Edge Cases).

## Items

`/item add <name> <price>`
Creates a menu item with no recipe yet. **Errors**: `price` not a valid non-negative decimal → rejected.

`/item list`
**Reply**: table of non-archived items with name and sale price.

`/item recipe <item-name> <ingredient-name> <quantity> [<ingredient-name> <quantity> ...]`
Replaces/sets the quantity for each named ingredient in the item's recipe (upsert per `(item, ingredient)` pair — FR-006, FR-007). **Errors**: unknown item/ingredient name for this shop, ingredient belongs to another shop, or `quantity <= 0` → rejected, no partial write.

`/item archive <name>`
Same semantics as `/ingredient archive`.

## Purchases

`/purchase <ingredient-name> <quantity> <total-cost> [date] [notes...]`
`date` defaults to today if omitted (format `YYYY-MM-DD`). Records the purchase and — if it is the ingredient's most recent purchase — refreshes `ingredients.current_unit_cost` (FR-009, FR-010, FR-011). **Reply**: confirmation including the recomputed current unit cost. **Errors**: unknown ingredient, `quantity <= 0`, `total-cost < 0`, unparseable `date` → rejected, no write.

## Sales

`/sale <item-name> <quantity> [<item-name> <quantity> ...]`
Records one sale with one line item per `(item-name, quantity)` pair; computes and permanently stores `unit_price` and `unit_production_cost` per line at this moment (FR-013, FR-014). **Reply**: confirmation with total revenue and total production cost for the sale. **Errors**: unknown item, `quantity <= 0`, or any recipe ingredient with no recorded cost yet (`current_unit_cost IS NULL`) → entire sale rejected, no partial write (see data-model.md Edge Cases).

## Reports

`/report [today|week|month|YYYY-MM-DD YYYY-MM-DD]`
Defaults to `today` if omitted. **Reply**: revenue, COGS, gross profit for the resolved date range (FR-016), computed the same way as the dashboard's `GET /reports/summary` (see `web-api.yaml`) so the two surfaces never disagree.
