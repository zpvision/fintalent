CREATE TABLE IF NOT EXISTS academy_cohorts (
 id BIGSERIAL PRIMARY KEY, course_id BIGINT NOT NULL REFERENCES courses(id),
 program_version_id BIGINT NOT NULL, name TEXT NOT NULL CHECK(length(name) BETWEEN 1 AND 180),
 description TEXT NOT NULL DEFAULT '', type TEXT NOT NULL DEFAULT 'group' CHECK(type IN('group','corporate','individual')),
 status TEXT NOT NULL DEFAULT 'draft' CHECK(status IN('draft','scheduled','active','completed','archived')),
 starts_at TIMESTAMPTZ NOT NULL, ends_at TIMESTAMPTZ, timezone TEXT NOT NULL DEFAULT 'Europe/Moscow',
 capacity INTEGER CHECK(capacity>0 AND capacity<=100000), schedule_mode TEXT NOT NULL DEFAULT 'all' CHECK(schedule_mode IN('all','calendar','relative')),
 rules JSONB NOT NULL DEFAULT '[]', deadlines JSONB NOT NULL DEFAULT '[]', revision BIGINT NOT NULL DEFAULT 1,
 created_by BIGINT NOT NULL REFERENCES users(id), created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 request_key TEXT NOT NULL UNIQUE, updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 CHECK(ends_at IS NULL OR ends_at>starts_at),
 FOREIGN KEY(course_id,program_version_id) REFERENCES academy_version_releases(course_id,version_id),
 UNIQUE(id,course_id,program_version_id)
);
CREATE INDEX IF NOT EXISTS academy_cohorts_course ON academy_cohorts(course_id,starts_at DESC,id);
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='academy_enrollment_cohort_fk' AND conrelid='academy_enrollments'::regclass) THEN
 ALTER TABLE academy_enrollments ADD CONSTRAINT academy_enrollment_cohort_fk FOREIGN KEY(cohort_id,course_id,program_version_id) REFERENCES academy_cohorts(id,course_id,program_version_id);
 END IF;
END $$;
CREATE UNIQUE INDEX IF NOT EXISTS academy_cohort_student_unique ON academy_enrollments(cohort_id,user_id) WHERE cohort_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS academy_enrollment_cohort ON academy_enrollments(cohort_id,id);
ALTER TABLE academy_assignment_submissions ADD COLUMN IF NOT EXISTS submitted_deadline_at TIMESTAMPTZ;
ALTER TABLE academy_assignment_submissions ADD COLUMN IF NOT EXISTS is_late BOOLEAN NOT NULL DEFAULT false;
CREATE TABLE IF NOT EXISTS academy_cohort_staff (
 cohort_id BIGINT NOT NULL REFERENCES academy_cohorts(id), user_id BIGINT NOT NULL REFERENCES users(id),
 role TEXT NOT NULL CHECK(role IN('teacher','curator')), can_review BOOLEAN NOT NULL DEFAULT false,
 assigned_by BIGINT NOT NULL REFERENCES users(id), assigned_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY(cohort_id,user_id)
);
CREATE TABLE IF NOT EXISTS academy_student_curators (
 enrollment_id BIGINT PRIMARY KEY REFERENCES academy_enrollments(id), user_id BIGINT NOT NULL REFERENCES users(id),
 assigned_by BIGINT NOT NULL REFERENCES users(id), assigned_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS academy_student_notes (
 id BIGSERIAL PRIMARY KEY, enrollment_id BIGINT NOT NULL REFERENCES academy_enrollments(id),
 author_id BIGINT NOT NULL REFERENCES users(id), text TEXT NOT NULL CHECK(length(text) BETWEEN 1 AND 10000),
 revision BIGINT NOT NULL DEFAULT 1, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(), deleted_at TIMESTAMPTZ
);
CREATE TABLE IF NOT EXISTS academy_student_tags (
 id BIGSERIAL PRIMARY KEY, course_id BIGINT NOT NULL REFERENCES courses(id), name TEXT NOT NULL CHECK(length(name) BETWEEN 1 AND 60),
 color TEXT NOT NULL DEFAULT 'blue' CHECK(color IN('blue','teal','purple','amber','rose')), created_by BIGINT NOT NULL REFERENCES users(id),
 UNIQUE(course_id,name), UNIQUE(id,course_id)
);
CREATE TABLE IF NOT EXISTS academy_student_tag_assignments (
 enrollment_id BIGINT NOT NULL REFERENCES academy_enrollments(id), tag_id BIGINT NOT NULL REFERENCES academy_student_tags(id),
 assigned_by BIGINT NOT NULL REFERENCES users(id), assigned_at TIMESTAMPTZ NOT NULL DEFAULT now(), PRIMARY KEY(enrollment_id,tag_id)
);
-- One audit/outbox for cohort and CRM actions. A notification and delivery marker commit together.
CREATE TABLE IF NOT EXISTS academy_cohort_events (
 id BIGSERIAL PRIMARY KEY, course_id BIGINT NOT NULL REFERENCES courses(id), cohort_id BIGINT REFERENCES academy_cohorts(id),
 enrollment_id BIGINT REFERENCES academy_enrollments(id), actor_id BIGINT REFERENCES users(id),
 type TEXT NOT NULL, event_key TEXT NOT NULL UNIQUE, data JSONB NOT NULL DEFAULT '{}', created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 cancelled BOOLEAN NOT NULL DEFAULT false, recipient_id BIGINT REFERENCES users(id), notify_at TIMESTAMPTZ, delivered_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS academy_cohort_events_pending ON academy_cohort_events(notify_at,id) WHERE recipient_id IS NOT NULL AND delivered_at IS NULL;
CREATE INDEX IF NOT EXISTS academy_cohort_events_history ON academy_cohort_events(enrollment_id,id DESC);
CREATE OR REPLACE FUNCTION academy_can_manage_enrollment(eid BIGINT, actor BIGINT, reviewing BOOLEAN DEFAULT false) RETURNS BOOLEAN LANGUAGE SQL STABLE AS $$
 SELECT EXISTS(SELECT 1 FROM academy_enrollments e JOIN courses c ON c.id=e.course_id JOIN users owner ON owner.id=c.owner_id JOIN users u ON u.id=actor
 WHERE e.id=eid AND NOT owner.is_blocked AND NOT u.is_blocked AND NOT u.is_system AND
 (c.owner_id=actor OR EXISTS(SELECT 1 FROM academy_cohort_staff s WHERE s.cohort_id=e.cohort_id AND s.user_id=actor
 AND (NOT reviewing OR s.can_review) AND (s.role='teacher' OR EXISTS(SELECT 1 FROM academy_student_curators sc WHERE sc.enrollment_id=e.id AND sc.user_id=actor)))))
$$;
CREATE OR REPLACE FUNCTION academy_guard_cohort_version() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NEW.program_version_id<>OLD.program_version_id AND EXISTS(SELECT 1 FROM academy_enrollments WHERE cohort_id=OLD.id) THEN
 RAISE EXCEPTION 'cohort version is locked after enrollment' USING ERRCODE='23514'; END IF;
 RETURN NEW; END $$;
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgname='academy_cohort_version_lock' AND tgrelid='academy_cohorts'::regclass) THEN
 CREATE TRIGGER academy_cohort_version_lock BEFORE UPDATE ON academy_cohorts FOR EACH ROW EXECUTE FUNCTION academy_guard_cohort_version(); END IF;
END $$;
CREATE OR REPLACE FUNCTION academy_guard_student_scope() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF TG_TABLE_NAME='academy_student_curators' THEN
 IF NOT EXISTS(SELECT 1 FROM academy_enrollments e JOIN academy_cohort_staff s ON s.cohort_id=e.cohort_id AND s.user_id=NEW.user_id WHERE e.id=NEW.enrollment_id) THEN RAISE EXCEPTION 'curator outside cohort' USING ERRCODE='23514'; END IF;
 ELSE
 IF NOT EXISTS(SELECT 1 FROM academy_enrollments e JOIN academy_student_tags t ON t.course_id=e.course_id WHERE e.id=NEW.enrollment_id AND t.id=NEW.tag_id) THEN RAISE EXCEPTION 'tag outside course' USING ERRCODE='23514'; END IF;
 END IF;RETURN NEW;END $$;
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgname='academy_student_curator_scope' AND tgrelid='academy_student_curators'::regclass) THEN CREATE TRIGGER academy_student_curator_scope BEFORE INSERT OR UPDATE ON academy_student_curators FOR EACH ROW EXECUTE FUNCTION academy_guard_student_scope(); END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgname='academy_student_tag_scope' AND tgrelid='academy_student_tag_assignments'::regclass) THEN CREATE TRIGGER academy_student_tag_scope BEFORE INSERT OR UPDATE ON academy_student_tag_assignments FOR EACH ROW EXECUTE FUNCTION academy_guard_student_scope(); END IF;
END $$;

CREATE OR REPLACE FUNCTION academy_cohort_rule_at(start_time TIMESTAMPTZ, zone TEXT, rule JSONB) RETURNS TIMESTAMPTZ LANGUAGE SQL STABLE AS $$
 SELECT CASE WHEN COALESCE(rule->>'at','')<>'' THEN (rule->>'at')::timestamptz ELSE (((start_time AT TIME ZONE zone)::date+COALESCE((rule->>'day')::integer,0)) + COALESCE(NULLIF(rule->>'clock','')::time,date_trunc('minute',start_time AT TIME ZONE zone)::time)) AT TIME ZONE zone END
$$;
CREATE OR REPLACE VIEW academy_student_metrics AS
 SELECT e.id enrollment_id,e.course_id,e.cohort_id,e.user_id,e.program_version_id,e.enrolled_at,e.started_at,e.completed_at,e.last_activity_at,e.expires_at,e.access_status,
 u.full_name user_name,COALESCE(u.avatar_url,'') avatar,v.number version_number,
 CASE WHEN u.is_blocked OR owner.is_blocked THEN 'suspended' WHEN e.access_status='active' AND e.expires_at<=now() THEN 'expired' ELSE e.access_status END effective_status,
 lessons.required,lessons.completed,
 CASE WHEN lessons.required>0 THEN round(100.0*lessons.completed/lessons.required)::integer ELSE 0 END percent,
 tests.test_average,tests.tests_passed,tests.failed_attempts,work.pending,work.accepted,work.revision,
 COALESCE(curator.full_name,'') curator_name,sc.user_id curator_id,tests.test_attempts,
 (SELECT count(*) FROM academy_cohorts ch CROSS JOIN LATERAL jsonb_array_elements(v.content->'modules') m CROSS JOIN LATERAL jsonb_array_elements(m->'lessons') l
 WHERE ch.id=e.cohort_id AND ch.status IN('active','completed','archived') AND m->>'status'='published' AND l->>'status'='published' AND COALESCE((l->>'required')::boolean,false)
 AND GREATEST(ch.starts_at,CASE WHEN ch.schedule_mode='all' THEN ch.starts_at ELSE COALESCE((SELECT academy_cohort_rule_at(ch.starts_at,ch.timezone,r) FROM jsonb_array_elements(ch.rules) r WHERE(r->>'kind'='lesson' AND r->>'key'=l->>'key') OR(r->>'kind'='module' AND r->>'key'=m->>'key') ORDER BY (r->>'kind'='lesson') DESC LIMIT 1),ch.starts_at) END)<=now()) expected_lessons,
 (SELECT count(*) FROM academy_cohorts ch CROSS JOIN LATERAL jsonb_array_elements(ch.deadlines) d WHERE ch.id=e.cohort_id AND academy_cohort_rule_at(ch.starts_at,ch.timezone,d)<now()
 AND NOT EXISTS(SELECT 1 FROM academy_assignment_submissions s JOIN academy_assessment_versions a ON a.id=s.assessment_id WHERE s.enrollment_id=e.id AND a.block_key=d->>'key' AND s.status IN('submitted','in_review','accepted'))) overdue_assignments
 FROM academy_enrollments e JOIN users u ON u.id=e.user_id JOIN courses course ON course.id=e.course_id JOIN users owner ON owner.id=course.owner_id
 JOIN academy_program_versions v ON v.id=e.program_version_id
 LEFT JOIN academy_student_curators sc ON sc.enrollment_id=e.id LEFT JOIN users curator ON curator.id=sc.user_id
 CROSS JOIN LATERAL(SELECT count(*) required,count(*) FILTER(WHERE p.status='completed') completed
 FROM jsonb_array_elements(v.content->'modules') m CROSS JOIN LATERAL jsonb_array_elements(m->'lessons') l
 LEFT JOIN academy_lesson_progress p ON p.enrollment_id=e.id AND p.lesson_key=l->>'key'
 WHERE m->>'status'='published' AND l->>'status'='published' AND COALESCE((l->>'required')::boolean,false)
 AND(e.cohort_id IS NOT NULL OR COALESCE((l->>'self_paced')::boolean,false))) lessons
 CROSS JOIN LATERAL(SELECT round(avg(percent),2) test_average,count(DISTINCT assessment_id) FILTER(WHERE passed) tests_passed,count(*) FILTER(WHERE NOT passed) failed_attempts,count(*) test_attempts FROM academy_test_attempts t WHERE t.enrollment_id=e.id AND t.status='finished') tests
 CROSS JOIN LATERAL(SELECT count(*) FILTER(WHERE status IN('submitted','in_review')) pending,count(*) FILTER(WHERE status='accepted') accepted,count(*) FILTER(WHERE status='revision_requested') revision
 FROM academy_assignment_submissions s WHERE s.enrollment_id=e.id AND s.submitted_at IS NOT NULL AND NOT EXISTS(SELECT 1 FROM academy_assignment_submissions n WHERE n.enrollment_id=s.enrollment_id AND n.assessment_id=s.assessment_id AND n.number>s.number AND n.submitted_at IS NOT NULL)) work;
