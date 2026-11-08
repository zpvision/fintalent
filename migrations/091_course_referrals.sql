CREATE TABLE IF NOT EXISTS course_referral_links (
 id BIGSERIAL PRIMARY KEY,
 course_id BIGINT NOT NULL REFERENCES courses(id),
 name TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
 code TEXT NOT NULL CHECK (code ~ '^[a-z0-9_-]{3,64}$'),
 created_by BIGINT REFERENCES users(id),
 created_by_admin BOOLEAN NOT NULL DEFAULT false,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 deleted_at TIMESTAMPTZ,
 UNIQUE(course_id,code), UNIQUE(id,course_id)
);
CREATE TABLE IF NOT EXISTS course_referral_guests (
 token_hash TEXT PRIMARY KEY CHECK (length(token_hash)=64),
 claimed_by BIGINT REFERENCES users(id),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS course_referral_guest_claims (
 token_hash TEXT NOT NULL REFERENCES course_referral_guests(token_hash),
 course_id BIGINT NOT NULL,
 referral_link_id BIGINT NOT NULL,
 first_clicked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY(token_hash,course_id),
 FOREIGN KEY(referral_link_id,course_id) REFERENCES course_referral_links(id,course_id)
);
CREATE TABLE IF NOT EXISTS course_referral_attributions (
 user_id BIGINT NOT NULL REFERENCES users(id),
 course_id BIGINT NOT NULL,
 referral_link_id BIGINT NOT NULL,
 first_clicked_at TIMESTAMPTZ NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY(user_id,course_id),
 FOREIGN KEY(referral_link_id,course_id) REFERENCES course_referral_links(id,course_id)
);
CREATE INDEX IF NOT EXISTS course_referral_attributions_source ON course_referral_attributions(course_id,referral_link_id);
-- One row per link and pseudonymous browser/account, not a heavy click event log.
CREATE TABLE IF NOT EXISTS course_referral_visitors (
 referral_link_id BIGINT NOT NULL REFERENCES course_referral_links(id),
 visitor TEXT NOT NULL,
 visits BIGINT NOT NULL DEFAULT 1 CHECK(visits>0),
 first_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 last_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY(referral_link_id,visitor)
);
-- An expression of interest is not an enrollment, order, or payment.
CREATE TABLE IF NOT EXISTS course_interests (
 id BIGSERIAL PRIMARY KEY,
 course_id BIGINT NOT NULL REFERENCES courses(id),
 user_id BIGINT REFERENCES users(id),
 guest_hash TEXT REFERENCES course_referral_guests(token_hash),
 referral_link_id BIGINT,
 tariff_id BIGINT,
 first_clicked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 CHECK (user_id IS NOT NULL OR guest_hash IS NOT NULL),
 FOREIGN KEY(referral_link_id,course_id) REFERENCES course_referral_links(id,course_id)
);
CREATE UNIQUE INDEX IF NOT EXISTS course_interests_user ON course_interests(course_id,user_id) WHERE user_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS course_interests_guest ON course_interests(course_id,guest_hash) WHERE user_id IS NULL;
CREATE INDEX IF NOT EXISTS course_interests_source ON course_interests(course_id,referral_link_id,first_clicked_at);
