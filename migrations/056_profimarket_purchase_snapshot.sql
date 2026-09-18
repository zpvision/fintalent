ALTER TABLE profimarket_purchases
    ADD COLUMN IF NOT EXISTS product_title_snapshot TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS product_slug_snapshot VARCHAR(180) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS product_type_snapshot VARCHAR(30) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS product_cover_snapshot TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS product_description_snapshot TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS seller_name_snapshot VARCHAR(200) NOT NULL DEFAULT '';

UPDATE profimarket_purchases p
SET product_title_snapshot = s.title,
    product_slug_snapshot = s.slug,
    product_type_snapshot = s.type,
    product_cover_snapshot = COALESCE(s.cover_image, ''),
    product_description_snapshot = COALESCE(s.short_description, ''),
    seller_name_snapshot = COALESCE(u.full_name, '')
FROM profimarket_solutions s, users u
WHERE s.id = p.solution_id
  AND u.id = p.seller_user_id
  AND p.product_title_snapshot = '';
