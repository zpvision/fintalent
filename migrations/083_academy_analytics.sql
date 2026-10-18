CREATE TABLE IF NOT EXISTS academy_analytics_settings (
 course_id BIGINT PRIMARY KEY REFERENCES courses(id), settings JSONB NOT NULL,
 revision BIGINT NOT NULL DEFAULT 1,updated_by BIGINT NOT NULL REFERENCES users(id),updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS academy_report_jobs (
 id BIGSERIAL PRIMARY KEY,course_id BIGINT NOT NULL REFERENCES courses(id),created_by BIGINT NOT NULL REFERENCES users(id),
 settings JSONB NOT NULL DEFAULT '{}',owner_scope BOOLEAN NOT NULL DEFAULT false,scope_cohorts JSONB NOT NULL DEFAULT '[]',
 kind TEXT NOT NULL,format TEXT NOT NULL CHECK(format IN('csv','xlsx')),filters JSONB NOT NULL,
 status TEXT NOT NULL DEFAULT 'queued' CHECK(status IN('queued','running','ready','failed','cancelled','expired')),
 request_key TEXT NOT NULL UNIQUE,scope_ids JSONB NOT NULL DEFAULT '[]',lease_key TEXT,storage_key TEXT,
 rows_count INTEGER NOT NULL DEFAULT 0,size_bytes BIGINT NOT NULL DEFAULT 0,error_code TEXT NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),started_at TIMESTAMPTZ,finished_at TIMESTAMPTZ,
 expires_at TIMESTAMPTZ NOT NULL DEFAULT now()+INTERVAL '7 days',deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS academy_report_queue ON academy_report_jobs(created_at,id) WHERE status='queued';
CREATE INDEX IF NOT EXISTS academy_report_owner ON academy_report_jobs(course_id,created_by,id DESC);
CREATE INDEX IF NOT EXISTS academy_report_expiry ON academy_report_jobs(expires_at) WHERE storage_key IS NOT NULL;
CREATE INDEX IF NOT EXISTS academy_learning_analytics_time ON academy_learning_events(enrollment_id,created_at,type);
CREATE INDEX IF NOT EXISTS academy_review_analytics_time ON academy_submission_reviews(submission_id,created_at);
CREATE INDEX IF NOT EXISTS academy_delivery_analytics_time ON academy_message_deliveries(enrollment_id,sent_at) WHERE NOT test;
