-- Photo of the purchased item (in addition to the receipt).
ALTER TABLE shopping_items ADD COLUMN item_path TEXT NOT NULL DEFAULT '';
