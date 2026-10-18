-- Academy authority extends existing FinTalent accounts; it does not create accounts or credentials.
CREATE TABLE IF NOT EXISTS academy_admin_grants (
 user_id BIGINT PRIMARY KEY REFERENCES users(id),role TEXT NOT NULL CHECK(role IN('academy_admin','support','moderator')),
 view_contacts BOOLEAN NOT NULL DEFAULT false,view_answers BOOLEAN NOT NULL DEFAULT false,correct_results BOOLEAN NOT NULL DEFAULT false,
 revision BIGINT NOT NULL DEFAULT 1,updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS academy_admin_audit (
 id BIGSERIAL PRIMARY KEY,actor_key TEXT NOT NULL,actor_name TEXT NOT NULL,actor_user_id BIGINT REFERENCES users(id),
 action TEXT NOT NULL,entity TEXT NOT NULL,entity_id BIGINT NOT NULL DEFAULT 0,course_id BIGINT REFERENCES courses(id),enrollment_id BIGINT REFERENCES academy_enrollments(id),student_id BIGINT REFERENCES users(id),
 reason TEXT NOT NULL,result TEXT NOT NULL DEFAULT 'succeeded',before_value JSONB NOT NULL DEFAULT '{}',after_value JSONB NOT NULL DEFAULT '{}',
 request_key TEXT NOT NULL UNIQUE,fingerprint TEXT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS academy_admin_audit_course ON academy_admin_audit(course_id,id DESC);
CREATE INDEX IF NOT EXISTS academy_admin_audit_student ON academy_admin_audit(student_id,id DESC);
CREATE TABLE IF NOT EXISTS academy_admin_corrections (
 id BIGSERIAL PRIMARY KEY,enrollment_id BIGINT NOT NULL REFERENCES academy_enrollments(id),
 test_attempt_id BIGINT REFERENCES academy_test_attempts(id),submission_id BIGINT REFERENCES academy_assignment_submissions(id),
 payload JSONB NOT NULL,previous_id BIGINT REFERENCES academy_admin_corrections(id),audit_id BIGINT NOT NULL REFERENCES academy_admin_audit(id),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),CHECK((test_attempt_id IS NULL)<>(submission_id IS NULL))
);
CREATE INDEX IF NOT EXISTS academy_admin_test_correction ON academy_admin_corrections(test_attempt_id,id DESC) WHERE test_attempt_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS academy_admin_work_correction ON academy_admin_corrections(submission_id,id DESC) WHERE submission_id IS NOT NULL;
CREATE TABLE IF NOT EXISTS academy_admin_settings (
 id INTEGER PRIMARY KEY CHECK(id=1),settings JSONB NOT NULL,revision BIGINT NOT NULL DEFAULT 1,updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS academy_publication_limits (
 id BIGSERIAL PRIMARY KEY,course_id BIGINT REFERENCES courses(id),author_id BIGINT REFERENCES users(id),blocked BOOLEAN NOT NULL DEFAULT true,
 reason TEXT NOT NULL,revision BIGINT NOT NULL DEFAULT 1,updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),CHECK((course_id IS NULL)<>(author_id IS NULL)),UNIQUE(course_id),UNIQUE(author_id)
);
-- A platform administrator is not impersonated as a FinTalent user. Original actors remain untouched.
ALTER TABLE academy_learning_events ALTER COLUMN actor_id DROP NOT NULL;
ALTER TABLE academy_enrollments ALTER COLUMN enrolled_by DROP NOT NULL;
ALTER TABLE academy_cohort_staff ALTER COLUMN assigned_by DROP NOT NULL;
ALTER TABLE academy_student_curators ALTER COLUMN assigned_by DROP NOT NULL;
ALTER TABLE academy_report_jobs ALTER COLUMN created_by DROP NOT NULL;
ALTER TABLE academy_report_jobs ADD COLUMN IF NOT EXISTS admin_actor_key TEXT NOT NULL DEFAULT '';
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgname='academy_admin_audit_immutable' AND tgrelid='academy_admin_audit'::regclass) THEN
 CREATE TRIGGER academy_admin_audit_immutable BEFORE UPDATE OR DELETE ON academy_admin_audit FOR EACH ROW EXECUTE FUNCTION academy_immutable_version();END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgname='academy_admin_correction_immutable' AND tgrelid='academy_admin_corrections'::regclass) THEN
 CREATE TRIGGER academy_admin_correction_immutable BEFORE UPDATE OR DELETE ON academy_admin_corrections FOR EACH ROW EXECUTE FUNCTION academy_immutable_version();END IF;
END $$;
CREATE OR REPLACE VIEW academy_effective_test_attempts AS
 SELECT t.id,t.enrollment_id,t.program_version_id,t.assessment_id,t.number,t.request_key,t.status,t.answers,t.answer_revision,t.grades,
 COALESCE((c.payload->>'score')::numeric,t.score) score,t.max_score,COALESCE((c.payload->>'percent')::numeric,t.percent) percent,
 COALESCE((c.payload->>'passed')::boolean,t.passed) passed,t.started_at,t.deadline_at,t.finished_at,COALESCE(c.id,0) correction_id
 FROM academy_test_attempts t LEFT JOIN LATERAL(SELECT id,payload FROM academy_admin_corrections WHERE test_attempt_id=t.id ORDER BY id DESC LIMIT 1)c ON true;
CREATE OR REPLACE VIEW academy_effective_assignment_submissions AS
 SELECT s.id,s.enrollment_id,s.program_version_id,s.assessment_id,s.number,s.request_key,COALESCE(c.payload->>'status',s.status) status,
 s.answer,s.revision,s.created_at,s.updated_at,s.submitted_at,s.assigned_to,s.assigned_at,
 COALESCE((c.payload->>'score')::numeric,s.score) score,COALESCE(c.payload->>'comment',s.comment) comment,s.reviewed_by,s.reviewed_at,
 s.submitted_deadline_at,s.is_late,COALESCE(c.id,0) correction_id
 FROM academy_assignment_submissions s LEFT JOIN LATERAL(SELECT id,payload FROM academy_admin_corrections WHERE submission_id=s.id ORDER BY id DESC LIMIT 1)c ON true;

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
 AND NOT EXISTS(SELECT 1 FROM academy_effective_assignment_submissions s JOIN academy_assessment_versions a ON a.id=s.assessment_id WHERE s.enrollment_id=e.id AND a.block_key=d->>'key' AND s.status IN('submitted','in_review','accepted'))) overdue_assignments
 FROM academy_enrollments e JOIN users u ON u.id=e.user_id JOIN courses course ON course.id=e.course_id JOIN users owner ON owner.id=course.owner_id
 JOIN academy_program_versions v ON v.id=e.program_version_id
 LEFT JOIN academy_student_curators sc ON sc.enrollment_id=e.id LEFT JOIN users curator ON curator.id=sc.user_id
 CROSS JOIN LATERAL(SELECT count(*) required,count(*) FILTER(WHERE p.status='completed') completed
 FROM jsonb_array_elements(v.content->'modules') m CROSS JOIN LATERAL jsonb_array_elements(m->'lessons') l
 LEFT JOIN academy_lesson_progress p ON p.enrollment_id=e.id AND p.lesson_key=l->>'key'
 WHERE m->>'status'='published' AND l->>'status'='published' AND COALESCE((l->>'required')::boolean,false)
 AND(e.cohort_id IS NOT NULL OR COALESCE((l->>'self_paced')::boolean,false))) lessons
 CROSS JOIN LATERAL(SELECT round(avg(percent),2) test_average,count(DISTINCT assessment_id) FILTER(WHERE passed) tests_passed,count(*) FILTER(WHERE NOT passed) failed_attempts,count(*) test_attempts FROM academy_effective_test_attempts t WHERE t.enrollment_id=e.id AND t.status='finished') tests
 CROSS JOIN LATERAL(SELECT count(*) FILTER(WHERE status IN('submitted','in_review')) pending,count(*) FILTER(WHERE status='accepted') accepted,count(*) FILTER(WHERE status='revision_requested') revision
 FROM academy_effective_assignment_submissions s WHERE s.enrollment_id=e.id AND s.submitted_at IS NOT NULL AND NOT EXISTS(SELECT 1 FROM academy_effective_assignment_submissions n WHERE n.enrollment_id=s.enrollment_id AND n.assessment_id=s.assessment_id AND n.number>s.number AND n.submitted_at IS NOT NULL)) work;

ALTER TABLE academy_communication_audit ALTER COLUMN actor_id DROP NOT NULL;
