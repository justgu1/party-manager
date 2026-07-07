-- Unit of measure for shopping items (un, kg, g, L, ml, cx, pct…).
ALTER TABLE shopping_items ADD COLUMN unit TEXT NOT NULL DEFAULT 'un';
