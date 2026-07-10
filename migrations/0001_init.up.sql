CREATE TABLE users (
    id                 BIGSERIAL PRIMARY KEY,
    telegram_id        BIGINT NOT NULL UNIQUE,
    telegram_username  TEXT NULL,
    display_name       TEXT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE ingredients (
    id                             BIGSERIAL PRIMARY KEY,
    user_id                        BIGINT NOT NULL REFERENCES users(id),
    name                           TEXT NOT NULL,
    unit_of_measure                TEXT NOT NULL,
    current_unit_cost              NUMERIC(14,4) NULL,
    current_unit_cost_updated_at   TIMESTAMPTZ NULL,
    is_archived                    BOOLEAN NOT NULL DEFAULT false,
    created_at                     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ingredients_user_id_lower_name_active_key
    ON ingredients (user_id, lower(name))
    WHERE is_archived = false;

CREATE INDEX ingredients_user_id_idx ON ingredients (user_id);

CREATE TABLE items (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT NOT NULL REFERENCES users(id),
    name         TEXT NOT NULL,
    sale_price   NUMERIC(12,2) NOT NULL CHECK (sale_price >= 0),
    is_archived  BOOLEAN NOT NULL DEFAULT false,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX items_user_id_lower_name_active_key
    ON items (user_id, lower(name))
    WHERE is_archived = false;

CREATE INDEX items_user_id_idx ON items (user_id);

CREATE TABLE item_ingredients (
    id             BIGSERIAL PRIMARY KEY,
    item_id        BIGINT NOT NULL REFERENCES items(id),
    ingredient_id  BIGINT NOT NULL REFERENCES ingredients(id),
    quantity       NUMERIC(14,4) NOT NULL CHECK (quantity > 0),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (item_id, ingredient_id)
);

CREATE INDEX item_ingredients_item_id_idx ON item_ingredients (item_id);
CREATE INDEX item_ingredients_ingredient_id_idx ON item_ingredients (ingredient_id);

CREATE TABLE ingredient_purchases (
    id             BIGSERIAL PRIMARY KEY,
    user_id        BIGINT NOT NULL REFERENCES users(id),
    ingredient_id  BIGINT NOT NULL REFERENCES ingredients(id),
    quantity       NUMERIC(14,4) NOT NULL CHECK (quantity > 0),
    total_cost     NUMERIC(14,4) NOT NULL CHECK (total_cost >= 0),
    unit_cost      NUMERIC(14,4) NOT NULL,
    purchase_date  DATE NOT NULL,
    notes          TEXT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ingredient_purchases_user_id_idx ON ingredient_purchases (user_id);
CREATE INDEX ingredient_purchases_ingredient_id_date_idx
    ON ingredient_purchases (ingredient_id, purchase_date, created_at);

CREATE TABLE sales (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id),
    sold_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    source      TEXT NOT NULL CHECK (source IN ('telegram', 'dashboard')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX sales_user_id_sold_at_idx ON sales (user_id, sold_at);

CREATE TABLE sale_items (
    id                     BIGSERIAL PRIMARY KEY,
    sale_id                BIGINT NOT NULL REFERENCES sales(id),
    item_id                BIGINT NOT NULL REFERENCES items(id),
    quantity               NUMERIC(14,4) NOT NULL CHECK (quantity > 0),
    unit_price             NUMERIC(12,2) NOT NULL,
    unit_production_cost   NUMERIC(14,4) NOT NULL,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX sale_items_sale_id_idx ON sale_items (sale_id);
CREATE INDEX sale_items_item_id_idx ON sale_items (item_id);
