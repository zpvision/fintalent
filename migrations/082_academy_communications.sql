ALTER TABLE academy_cohort_staff ADD COLUMN IF NOT EXISTS can_communicate BOOLEAN NOT NULL DEFAULT false;
CREATE TABLE IF NOT EXISTS academy_email_templates (
 id BIGSERIAL PRIMARY KEY,course_id BIGINT NOT NULL REFERENCES courses(id),created_by BIGINT NOT NULL REFERENCES users(id),
 name TEXT NOT NULL, message JSONB NOT NULL,revision BIGINT NOT NULL DEFAULT 1,created_at TIMESTAMPTZ NOT NULL DEFAULT now(),updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),deleted_at TIMESTAMPTZ
);
CREATE TABLE IF NOT EXISTS academy_campaigns (
 id BIGSERIAL PRIMARY KEY,course_id BIGINT NOT NULL REFERENCES courses(id),created_by BIGINT NOT NULL REFERENCES users(id),
 name TEXT NOT NULL,message JSONB NOT NULL,audience JSONB NOT NULL,channel TEXT NOT NULL CHECK(channel IN('email','in_app')),
 category TEXT NOT NULL CHECK(category IN('service','marketing')),timezone TEXT NOT NULL DEFAULT 'Europe/Moscow',send_at TIMESTAMPTZ,
 status TEXT NOT NULL DEFAULT 'draft' CHECK(status IN('draft','scheduled','sending','completed','partial','failed','cancelled')),
 revision BIGINT NOT NULL DEFAULT 1,request_key TEXT NOT NULL UNIQUE,created_at TIMESTAMPTZ NOT NULL DEFAULT now(),updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),launched_at TIMESTAMPTZ,cancelled_at TIMESTAMPTZ,deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS academy_campaign_course ON academy_campaigns(course_id,id DESC);
CREATE TABLE IF NOT EXISTS academy_automations (
 id BIGSERIAL PRIMARY KEY,course_id BIGINT NOT NULL REFERENCES courses(id),created_by BIGINT NOT NULL REFERENCES users(id),
 name TEXT NOT NULL,trigger TEXT NOT NULL,config JSONB NOT NULL,status TEXT NOT NULL DEFAULT 'draft' CHECK(status IN('draft','active','paused','archived')),
 revision BIGINT NOT NULL DEFAULT 1,generation BIGINT NOT NULL DEFAULT 1,active_since TIMESTAMPTZ,created_at TIMESTAMPTZ NOT NULL DEFAULT now(),updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),request_key TEXT NOT NULL UNIQUE
);
CREATE INDEX IF NOT EXISTS academy_automation_active ON academy_automations(course_id,id) WHERE status='active';
CREATE TABLE IF NOT EXISTS academy_automation_runs (
 id BIGSERIAL PRIMARY KEY,automation_id BIGINT NOT NULL REFERENCES academy_automations(id),generation BIGINT NOT NULL,
 enrollment_id BIGINT NOT NULL REFERENCES academy_enrollments(id),source_key TEXT NOT NULL,event_type TEXT NOT NULL,event_at TIMESTAMPTZ NOT NULL,source_data JSONB NOT NULL DEFAULT '{}',conditions JSONB NOT NULL,skipped_reason TEXT NOT NULL DEFAULT '',created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(automation_id,generation,enrollment_id,source_key)
);
CREATE TABLE IF NOT EXISTS academy_message_deliveries (
 id BIGSERIAL PRIMARY KEY,course_id BIGINT NOT NULL REFERENCES courses(id),actor_id BIGINT NOT NULL REFERENCES users(id),
 campaign_id BIGINT REFERENCES academy_campaigns(id),run_id BIGINT REFERENCES academy_automation_runs(id),enrollment_id BIGINT REFERENCES academy_enrollments(id),
 recipient_id BIGINT NOT NULL REFERENCES users(id),target TEXT NOT NULL DEFAULT 'student' CHECK(target IN('student','curator','owner')),
 channel TEXT NOT NULL CHECK(channel IN('email','in_app')),category TEXT NOT NULL CHECK(category IN('service','marketing')),message JSONB NOT NULL,
 dedup_key TEXT NOT NULL UNIQUE,status TEXT NOT NULL DEFAULT 'queued' CHECK(status IN('queued','sending','sent','delivered','failed','cancelled')),
 attempts INTEGER NOT NULL DEFAULT 0,next_attempt_at TIMESTAMPTZ NOT NULL,queued_at TIMESTAMPTZ NOT NULL DEFAULT now(),claimed_at TIMESTAMPTZ,sent_at TIMESTAMPTZ,
 lease_key TEXT,uncertain BOOLEAN NOT NULL DEFAULT false,error_code TEXT NOT NULL DEFAULT '',message_id TEXT NOT NULL DEFAULT '',provider_id TEXT NOT NULL DEFAULT '',test BOOLEAN NOT NULL DEFAULT false
);
CREATE INDEX IF NOT EXISTS academy_delivery_due ON academy_message_deliveries(next_attempt_at,id) WHERE status='queued';
CREATE INDEX IF NOT EXISTS academy_delivery_run ON academy_message_deliveries(run_id,id) WHERE run_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS academy_delivery_course ON academy_message_deliveries(course_id,id DESC);
CREATE INDEX IF NOT EXISTS academy_delivery_recipient ON academy_message_deliveries(recipient_id,claimed_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS academy_campaign_recipient ON academy_message_deliveries(campaign_id,recipient_id,channel) WHERE campaign_id IS NOT NULL;
CREATE TABLE IF NOT EXISTS academy_delivery_attempts (
 id BIGSERIAL PRIMARY KEY,delivery_id BIGINT NOT NULL REFERENCES academy_message_deliveries(id),number INTEGER NOT NULL,
 started_at TIMESTAMPTZ NOT NULL DEFAULT now(),finished_at TIMESTAMPTZ,outcome TEXT NOT NULL DEFAULT 'sending',error_code TEXT NOT NULL DEFAULT '',UNIQUE(delivery_id,number)
);
CREATE INDEX IF NOT EXISTS academy_delivery_attempts_time ON academy_delivery_attempts(started_at DESC,delivery_id);
CREATE TABLE IF NOT EXISTS academy_communication_preferences (
 user_id BIGINT NOT NULL REFERENCES users(id),course_id BIGINT NOT NULL REFERENCES courses(id),email_service BOOLEAN NOT NULL DEFAULT true,
 email_marketing BOOLEAN NOT NULL DEFAULT false,in_app BOOLEAN NOT NULL DEFAULT true,consented_at TIMESTAMPTZ,updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 unsubscribe_token TEXT NOT NULL UNIQUE,PRIMARY KEY(user_id,course_id)
);
CREATE TABLE IF NOT EXISTS academy_communication_audit (
 id BIGSERIAL PRIMARY KEY,course_id BIGINT REFERENCES courses(id),actor_id BIGINT REFERENCES users(id),entity TEXT NOT NULL,entity_id BIGINT,action TEXT NOT NULL,data JSONB NOT NULL DEFAULT '{}',created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE OR REPLACE FUNCTION academy_can_communicate(eid BIGINT, actor BIGINT) RETURNS BOOLEAN LANGUAGE SQL STABLE AS $$
 SELECT academy_can_manage_enrollment(eid,actor,false) AND EXISTS(SELECT 1 FROM academy_enrollments e JOIN courses c ON c.id=e.course_id WHERE e.id=eid AND(c.owner_id=actor OR EXISTS(SELECT 1 FROM academy_cohort_staff s WHERE s.cohort_id=e.cohort_id AND s.user_id=actor AND s.can_communicate)))
$$;
-- A/B/C/D remain the source of truth for learning events. This view only normalizes them for execution.
CREATE OR REPLACE VIEW academy_communication_events AS
 SELECT 'learning:'||x.id source_key,x.enrollment_id,e.course_id,x.type event_type,x.created_at event_at,x.data,x.lesson_key FROM academy_learning_events x JOIN academy_enrollments e ON e.id=x.enrollment_id
 UNION ALL
 SELECT 'cohort:'||x.id||':'||e.id,e.id,e.course_id,x.type,COALESCE(x.notify_at,x.created_at),x.data,COALESCE(x.data->>'lesson_key','')
 FROM academy_cohort_events x JOIN academy_enrollments e ON e.id=x.enrollment_id OR(x.enrollment_id IS NULL AND e.cohort_id=x.cohort_id)
 WHERE NOT x.cancelled;
