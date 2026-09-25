CREATE TABLE IF NOT EXISTS contact_threads (
    id BIGSERIAL PRIMARY KEY,
    sender_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    recipient_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    resume_id BIGINT NOT NULL REFERENCES resumes(id) ON DELETE CASCADE,
    subject VARCHAR(200) NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','accepted','declined','blocked')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK(sender_id <> recipient_id)
);

CREATE TABLE IF NOT EXISTS contact_messages (
    id BIGSERIAL PRIMARY KEY,
    thread_id BIGINT NOT NULL REFERENCES contact_threads(id) ON DELETE CASCADE,
    author_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS contact_reports (
    id BIGSERIAL PRIMARY KEY,
    thread_id BIGINT NOT NULL REFERENCES contact_threads(id) ON DELETE CASCADE,
    reporter_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reason TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(thread_id, reporter_id)
);

CREATE INDEX IF NOT EXISTS contact_threads_sender_idx ON contact_threads(sender_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS contact_threads_recipient_idx ON contact_threads(recipient_id, status, updated_at DESC);
CREATE INDEX IF NOT EXISTS contact_messages_thread_idx ON contact_messages(thread_id, created_at);
