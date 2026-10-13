CREATE TABLE IF NOT EXISTS course_custom_features (
 id BIGSERIAL PRIMARY KEY,
 course_id BIGINT NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
 title VARCHAR(180) NOT NULL CHECK(length(btrim(title)) > 0),
 sort_order INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS course_custom_features_order_idx ON course_custom_features(course_id,sort_order,id);
