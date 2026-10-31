CREATE TABLE IF NOT EXISTS test_question_materials (
 id BIGSERIAL PRIMARY KEY,
 test_id BIGINT NOT NULL REFERENCES tests(id),
 owner_id BIGINT NOT NULL REFERENCES users(id),
 name VARCHAR(180) NOT NULL,
 mime TEXT NOT NULL CHECK (mime IN ('image/jpeg','image/png','application/pdf')),
 size INTEGER NOT NULL CHECK (size BETWEEN 1 AND 6291456),
 content BYTEA NOT NULL CHECK (octet_length(content)=size),
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS test_question_materials_test ON test_question_materials(test_id,id);
