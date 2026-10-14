ALTER TABLE course_tariffs ADD COLUMN IF NOT EXISTS old_price BIGINT CHECK (old_price > price AND old_price <= 10000000000);
ALTER TABLE course_tariffs ADD COLUMN IF NOT EXISTS hours DOUBLE PRECISION;
UPDATE course_tariffs t SET hours=c.hours FROM courses c WHERE t.course_id=c.id AND t.hours IS NULL;
ALTER TABLE course_tariffs ALTER COLUMN hours SET DEFAULT 0;
ALTER TABLE course_tariffs ALTER COLUMN hours SET NOT NULL;
