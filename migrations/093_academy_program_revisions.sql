-- Logical version IDs remain the original snapshot IDs. Old snapshots and
-- results remain immutable; current content is resolved through the head.
ALTER TABLE academy_lessons ADD COLUMN IF NOT EXISTS access_mode TEXT NOT NULL DEFAULT 'schedule' CHECK(access_mode IN('schedule','immediate'));
CREATE TABLE IF NOT EXISTS academy_program_revision_heads (
 version_id BIGINT PRIMARY KEY REFERENCES academy_program_versions(id),
 published_revision_id BIGINT,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS academy_program_version_drafts (
 version_id BIGINT PRIMARY KEY REFERENCES academy_program_versions(id),
 revision BIGINT NOT NULL DEFAULT 1 CHECK(revision > 0),
 content JSONB NOT NULL CHECK(jsonb_typeof(content)='object'),
 updated_by BIGINT NOT NULL REFERENCES users(id),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE academy_program_version_drafts ADD COLUMN IF NOT EXISTS test_sources JSONB NOT NULL DEFAULT '{}';
CREATE TABLE IF NOT EXISTS academy_program_revisions (
 id BIGSERIAL PRIMARY KEY,
 version_id BIGINT NOT NULL REFERENCES academy_program_versions(id),
 number INTEGER NOT NULL CHECK(number > 0),
 draft_revision BIGINT NOT NULL CHECK(draft_revision > 0),
 content JSONB NOT NULL CHECK(jsonb_typeof(content)='object'),
 assessment_definitions JSONB NOT NULL CHECK(jsonb_typeof(assessment_definitions)='array'),
 test_metadata JSONB NOT NULL DEFAULT '[]',
 published_by BIGINT NOT NULL REFERENCES users(id),
 published_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(version_id,number), UNIQUE(version_id,draft_revision), UNIQUE(id,version_id)
);
ALTER TABLE academy_program_revisions ADD COLUMN IF NOT EXISTS request_key TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS academy_program_publication_request ON academy_program_revisions(version_id,request_key) WHERE request_key IS NOT NULL;
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='academy_revision_head_scope' AND conrelid='academy_program_revision_heads'::regclass) THEN
  ALTER TABLE academy_program_revision_heads ADD CONSTRAINT academy_revision_head_scope
   FOREIGN KEY(published_revision_id,version_id) REFERENCES academy_program_revisions(id,version_id);
 END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgname='academy_program_revision_immutable' AND tgrelid='academy_program_revisions'::regclass) THEN
  CREATE TRIGGER academy_program_revision_immutable BEFORE UPDATE OR DELETE ON academy_program_revisions
   FOR EACH ROW EXECUTE FUNCTION academy_immutable_version();
 END IF;
END $$;

-- Deterministic correspondence: old version N becomes logical version N, and
-- its existing published content becomes revision 1. Never copy the landing.
INSERT INTO academy_program_revision_heads(version_id)
 SELECT id FROM academy_program_versions ON CONFLICT DO NOTHING;
INSERT INTO academy_program_version_drafts(version_id,content,updated_by)
 SELECT id,content,created_by FROM academy_program_versions ON CONFLICT DO NOTHING;
INSERT INTO academy_program_revisions(version_id,number,draft_revision,content,assessment_definitions,test_metadata,published_by,published_at)
 SELECT v.id,1,1,v.content,
  COALESCE((SELECT jsonb_agg(to_jsonb(a) ORDER BY a.id) FROM academy_assessment_versions a WHERE a.program_version_id=v.id),'[]'),
  r.test_metadata,r.published_by,r.published_at
 FROM academy_program_versions v JOIN academy_version_releases r ON r.version_id=v.id
 ON CONFLICT(version_id,number) DO NOTHING;
UPDATE academy_program_revision_heads h SET published_revision_id=r.id
 FROM academy_program_revisions r
 WHERE h.version_id=r.version_id AND r.number=1 AND h.published_revision_id IS NULL;



ALTER TABLE academy_assessment_versions ADD COLUMN IF NOT EXISTS publication_revision_id BIGINT;
DO $$ DECLARE c record; BEGIN
 FOR c IN SELECT conname FROM pg_constraint
 WHERE conrelid='academy_assessment_versions'::regclass AND contype='u'
 AND pg_get_constraintdef(oid)='UNIQUE (program_version_id, lesson_key, block_key)'
 LOOP EXECUTE format('ALTER TABLE academy_assessment_versions DROP CONSTRAINT %I',c.conname); END LOOP;
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='academy_assessment_revision_scope' AND conrelid='academy_assessment_versions'::regclass) THEN
  ALTER TABLE academy_assessment_versions ADD CONSTRAINT academy_assessment_revision_scope
   FOREIGN KEY(publication_revision_id,program_version_id) REFERENCES academy_program_revisions(id,version_id)
   DEFERRABLE INITIALLY DEFERRED;
 END IF;
END $$;
CREATE UNIQUE INDEX IF NOT EXISTS academy_assessment_per_revision
 ON academy_assessment_versions(program_version_id,lesson_key,block_key,publication_revision_id) NULLS NOT DISTINCT;

CREATE OR REPLACE VIEW academy_current_program_versions AS
 SELECT v.id,v.program_id,v.number,v.source_revision,
 COALESCE(r.content,v.content)||jsonb_build_object('version_id',v.id,'publication_revision_id',COALESCE(r.id,0)) content,
 v.created_by,v.created_at
 FROM academy_program_versions v LEFT JOIN academy_program_revision_heads h ON h.version_id=v.id
 LEFT JOIN academy_program_revisions r ON r.id=h.published_revision_id AND r.version_id=v.id;
CREATE OR REPLACE VIEW academy_current_version_releases AS
 SELECT old.version_id,old.course_id,COALESCE(r.published_by,old.published_by) published_by,
 COALESCE(r.test_metadata,old.test_metadata) test_metadata,COALESCE(r.published_at,old.published_at) published_at
 FROM academy_version_releases old LEFT JOIN academy_program_revision_heads h ON h.version_id=old.version_id
 LEFT JOIN academy_program_revisions r ON r.id=h.published_revision_id AND r.version_id=old.version_id;

CREATE OR REPLACE VIEW academy_current_assessment_versions AS
 SELECT a.* FROM academy_assessment_versions a JOIN academy_current_program_versions v ON v.id=a.program_version_id
 WHERE (a.publication_revision_id=(v.content->>'publication_revision_id')::bigint
 OR a.publication_revision_id IS NULL
 AND ((v.content->>'publication_revision_id')::bigint=0 OR EXISTS(SELECT 1 FROM academy_program_revisions r WHERE r.id=(v.content->>'publication_revision_id')::bigint AND r.version_id=v.id AND r.number=1))
 AND NOT EXISTS(SELECT 1 FROM academy_assessment_versions fresh WHERE fresh.program_version_id=a.program_version_id AND fresh.lesson_key=a.lesson_key AND fresh.block_key=a.block_key AND fresh.publication_revision_id=(v.content->>'publication_revision_id')::bigint))
 AND EXISTS(SELECT 1 FROM jsonb_array_elements(v.content->'modules') m CROSS JOIN LATERAL jsonb_array_elements(m->'lessons') l CROSS JOIN LATERAL jsonb_array_elements(l->'blocks') b WHERE m->>'status'='published' AND l->>'status'='published' AND l->>'key'=a.lesson_key AND b->>'key'=a.block_key AND b->>'type'=a.kind);

CREATE OR REPLACE VIEW academy_all_program_contents AS
 SELECT program_id,content FROM academy_program_versions
 UNION ALL SELECT v.program_id,d.content FROM academy_program_version_drafts d JOIN academy_program_versions v ON v.id=d.version_id
 UNION ALL SELECT v.program_id,r.content FROM academy_program_revisions r JOIN academy_program_versions v ON v.id=r.version_id;
CREATE OR REPLACE VIEW academy_editable_entities AS
 SELECT program_id,key FROM academy_modules UNION SELECT program_id,key FROM academy_lessons
 UNION SELECT v.program_id,m->>'key' FROM academy_program_version_drafts d JOIN academy_program_versions v ON v.id=d.version_id
 CROSS JOIN LATERAL jsonb_array_elements(d.content->'modules') m
 UNION SELECT v.program_id,l->>'key' FROM academy_program_version_drafts d JOIN academy_program_versions v ON v.id=d.version_id
 CROSS JOIN LATERAL jsonb_array_elements(d.content->'modules') m CROSS JOIN LATERAL jsonb_array_elements(m->'lessons') l;

CREATE OR REPLACE FUNCTION academy_block_satisfied(eid bigint,version bigint,block text) RETURNS boolean LANGUAGE SQL STABLE AS $$
 SELECT COALESCE(bool_or(CASE WHEN a.kind='test' THEN
 COALESCE((SELECT t.passed FROM academy_effective_test_attempts t WHERE t.enrollment_id=eid AND t.assessment_id=a.id AND t.status='finished'
 ORDER BY CASE WHEN COALESCE((a.config->'assessment'->>'use_best')::boolean,true) THEN t.percent END DESC,t.number DESC LIMIT 1),false)
 ELSE EXISTS(SELECT 1 FROM academy_effective_assignment_submissions s WHERE s.enrollment_id=eid AND s.assessment_id=a.id AND s.status='accepted') END),false)
 FROM academy_assessment_versions a WHERE a.program_version_id=version AND a.block_key=block
$$;
UPDATE academy_program_version_drafts d SET test_sources=(SELECT jsonb_object_agg(a.block_key,jsonb_build_object('source_version_id',(a.config->>'test_version_id')::bigint,'title',a.title,'questions',a.questions)) FROM academy_current_assessment_versions a WHERE a.program_version_id=d.version_id AND a.kind='test')
 WHERE d.test_sources='{}' AND EXISTS(SELECT 1 FROM academy_current_assessment_versions a WHERE a.program_version_id=d.version_id AND a.kind='test');

CREATE OR REPLACE VIEW academy_student_metrics AS
 SELECT e.id enrollment_id,e.course_id,e.cohort_id,e.user_id,e.program_version_id,e.enrolled_at,e.started_at,e.completed_at,e.last_activity_at,e.expires_at,e.access_status,
 u.full_name user_name,COALESCE(u.avatar_url,'') avatar,v.number version_number,
 CASE WHEN u.is_blocked OR owner.is_blocked THEN 'suspended' WHEN e.access_status='active' AND e.expires_at<=now() THEN 'expired' ELSE e.access_status END effective_status,
 lessons.required,lessons.completed,
 CASE WHEN lessons.required>0 THEN floor(100.0*lessons.completed/lessons.required)::integer ELSE 0 END percent,
 tests.test_average,tests.tests_passed,tests.failed_attempts,work.pending,work.accepted,work.revision,
 COALESCE(curator.full_name,'') curator_name,sc.user_id curator_id,tests.test_attempts,
 (SELECT count(*) FROM academy_cohorts ch CROSS JOIN LATERAL jsonb_array_elements(v.content->'modules') m CROSS JOIN LATERAL jsonb_array_elements(m->'lessons') l
 WHERE ch.id=e.cohort_id AND ch.status IN('active','completed','archived') AND m->>'status'='published' AND l->>'status'='published' AND COALESCE((l->>'required')::boolean,false)
 AND (l->>'access_mode'='immediate' OR GREATEST(ch.starts_at,CASE WHEN ch.schedule_mode='all' THEN ch.starts_at ELSE COALESCE((SELECT academy_cohort_rule_at(ch.starts_at,ch.timezone,r) FROM jsonb_array_elements(ch.rules) r WHERE(r->>'kind'='lesson' AND r->>'key'=l->>'key') OR(r->>'kind'='module' AND r->>'key'=m->>'key') ORDER BY (r->>'kind'='lesson') DESC LIMIT 1),ch.starts_at) END)<=now())) expected_lessons,
 (SELECT count(*) FROM academy_cohorts ch CROSS JOIN LATERAL jsonb_array_elements(ch.deadlines) d WHERE ch.id=e.cohort_id AND academy_cohort_rule_at(ch.starts_at,ch.timezone,d)<now()
 AND EXISTS(SELECT 1 FROM academy_current_assessment_versions active WHERE active.program_version_id=e.program_version_id AND active.block_key=d->>'key' AND active.kind='assignment')
 AND NOT EXISTS(SELECT 1 FROM academy_effective_assignment_submissions s JOIN academy_assessment_versions a ON a.id=s.assessment_id WHERE s.enrollment_id=e.id AND a.block_key=d->>'key' AND s.status IN('submitted','in_review','accepted'))) overdue_assignments
 FROM academy_enrollments e JOIN users u ON u.id=e.user_id JOIN courses course ON course.id=e.course_id JOIN users owner ON owner.id=course.owner_id
 JOIN academy_current_program_versions v ON v.id=e.program_version_id
 LEFT JOIN academy_student_curators sc ON sc.enrollment_id=e.id LEFT JOIN users curator ON curator.id=sc.user_id
 CROSS JOIN LATERAL(SELECT count(*) required,count(*) FILTER(WHERE p.status='completed' AND(e.completed_at IS NOT NULL OR NOT EXISTS(
 SELECT 1 FROM jsonb_array_elements(l->'blocks') b WHERE b->>'type' IN('test','assignment')
 AND(COALESCE((b->'data'->'assessment'->>'required')::boolean,true) OR l->>'completion_rule'=b->>'type')
 AND NOT academy_block_satisfied(e.id,e.program_version_id,b->>'key')))) completed
 FROM jsonb_array_elements(v.content->'modules') m CROSS JOIN LATERAL jsonb_array_elements(m->'lessons') l
 LEFT JOIN academy_lesson_progress p ON p.enrollment_id=e.id AND p.lesson_key=l->>'key'
 WHERE m->>'status'='published' AND l->>'status'='published' AND COALESCE((l->>'required')::boolean,false)
 AND(e.cohort_id IS NOT NULL OR COALESCE((l->>'self_paced')::boolean,false))) lessons
 CROSS JOIN LATERAL(SELECT round(avg(percent),2) test_average,count(DISTINCT assessment_id) FILTER(WHERE passed) tests_passed,count(*) FILTER(WHERE NOT passed) failed_attempts,count(*) test_attempts FROM academy_effective_test_attempts t WHERE t.enrollment_id=e.id AND t.status='finished') tests
 CROSS JOIN LATERAL(SELECT count(*) FILTER(WHERE status IN('submitted','in_review')) pending,count(*) FILTER(WHERE status='accepted') accepted,count(*) FILTER(WHERE status='revision_requested') revision
 FROM academy_effective_assignment_submissions s WHERE s.enrollment_id=e.id AND s.submitted_at IS NOT NULL AND NOT EXISTS(SELECT 1 FROM academy_effective_assignment_submissions n WHERE n.enrollment_id=s.enrollment_id AND n.assessment_id=s.assessment_id AND n.number>s.number AND n.submitted_at IS NOT NULL)) work;
