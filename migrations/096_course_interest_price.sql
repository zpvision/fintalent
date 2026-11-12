ALTER TABLE course_interests ADD COLUMN IF NOT EXISTS selected_price BIGINT CHECK (selected_price >= 0);
