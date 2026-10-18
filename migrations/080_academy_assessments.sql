-- Private, append-only assessment definitions belonging to an immutable curriculum version.
CREATE TABLE IF NOT EXISTS academy_assessment_versions (
 id BIGSERIAL PRIMARY KEY, program_version_id BIGINT NOT NULL REFERENCES academy_program_versions(id),
 lesson_key TEXT NOT NULL, block_key TEXT NOT NULL, kind TEXT NOT NULL CHECK(kind IN('test','assignment')),
 config JSONB NOT NULL, questions JSONB NOT NULL DEFAULT '[]', title TEXT NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 UNIQUE(program_version_id,lesson_key,block_key), UNIQUE(id,program_version_id)
);
CREATE TABLE IF NOT EXISTS academy_test_attempts (
 id BIGSERIAL PRIMARY KEY, enrollment_id BIGINT NOT NULL, program_version_id BIGINT NOT NULL,
 assessment_id BIGINT NOT NULL, number INTEGER NOT NULL CHECK(number>0), request_key TEXT NOT NULL,
 status TEXT NOT NULL DEFAULT 'started' CHECK(status IN('started','finished')),
 answers JSONB NOT NULL DEFAULT '[]', answer_revision BIGINT NOT NULL DEFAULT 0,
 grades JSONB NOT NULL DEFAULT '[]', score NUMERIC NOT NULL DEFAULT 0, max_score NUMERIC NOT NULL DEFAULT 0,
 percent NUMERIC NOT NULL DEFAULT 0, passed BOOLEAN NOT NULL DEFAULT FALSE,
 started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), deadline_at TIMESTAMPTZ, finished_at TIMESTAMPTZ,
 FOREIGN KEY(enrollment_id,program_version_id) REFERENCES academy_enrollments(id,program_version_id),
 FOREIGN KEY(assessment_id,program_version_id) REFERENCES academy_assessment_versions(id,program_version_id),
 UNIQUE(enrollment_id,assessment_id,number), UNIQUE(enrollment_id,request_key)
);
CREATE UNIQUE INDEX IF NOT EXISTS academy_test_one_started ON academy_test_attempts(enrollment_id,assessment_id) WHERE status='started';
CREATE TABLE IF NOT EXISTS academy_assignment_submissions (
 id BIGSERIAL PRIMARY KEY, enrollment_id BIGINT NOT NULL, program_version_id BIGINT NOT NULL,
 assessment_id BIGINT NOT NULL, number INTEGER NOT NULL CHECK(number>0), request_key TEXT NOT NULL,
 status TEXT NOT NULL DEFAULT 'draft' CHECK(status IN('draft','submitted','in_review','revision_requested','accepted','rejected')),
 answer JSONB NOT NULL DEFAULT '{"text":"","links":[],"asset_ids":[]}', revision BIGINT NOT NULL DEFAULT 0,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), submitted_at TIMESTAMPTZ,
 assigned_to BIGINT REFERENCES users(id), assigned_at TIMESTAMPTZ,
 score NUMERIC, comment TEXT NOT NULL DEFAULT '', reviewed_by BIGINT REFERENCES users(id), reviewed_at TIMESTAMPTZ,
 FOREIGN KEY(enrollment_id,program_version_id) REFERENCES academy_enrollments(id,program_version_id),
 FOREIGN KEY(assessment_id,program_version_id) REFERENCES academy_assessment_versions(id,program_version_id),
 UNIQUE(enrollment_id,assessment_id,number), UNIQUE(enrollment_id,request_key)
);
CREATE UNIQUE INDEX IF NOT EXISTS academy_submission_one_pending ON academy_assignment_submissions(enrollment_id,assessment_id) WHERE status IN('draft','submitted','in_review');
CREATE TABLE IF NOT EXISTS academy_submission_reviews (
 id BIGSERIAL PRIMARY KEY, submission_id BIGINT NOT NULL REFERENCES academy_assignment_submissions(id),
 reviewer_id BIGINT REFERENCES users(id), source TEXT NOT NULL CHECK(source IN('manual','automatic','ai')),
 request_key TEXT NOT NULL,
 action TEXT NOT NULL CHECK(action IN('in_review','accepted','revision_requested','rejected')),
 score NUMERIC, criterion_scores JSONB NOT NULL DEFAULT '{}', comment TEXT NOT NULL DEFAULT '', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), UNIQUE(submission_id,request_key)
);
ALTER TABLE academy_media_assets ADD COLUMN IF NOT EXISTS submission_id BIGINT REFERENCES academy_assignment_submissions(id);
CREATE INDEX IF NOT EXISTS academy_submission_files ON academy_media_assets(submission_id,id) WHERE submission_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS academy_test_result ON academy_test_attempts(enrollment_id,assessment_id,status,number DESC);
CREATE INDEX IF NOT EXISTS academy_submission_queue ON academy_assignment_submissions(status,submitted_at,id);
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgname='academy_assessment_immutable' AND tgrelid='academy_assessment_versions'::regclass) THEN
 CREATE TRIGGER academy_assessment_immutable BEFORE UPDATE OR DELETE ON academy_assessment_versions FOR EACH ROW EXECUTE FUNCTION academy_immutable_version();
 END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgname='academy_review_immutable' AND tgrelid='academy_submission_reviews'::regclass) THEN
 CREATE TRIGGER academy_review_immutable BEFORE UPDATE OR DELETE ON academy_submission_reviews FOR EACH ROW EXECUTE FUNCTION academy_immutable_version();
 END IF;
END $$;
CREATE OR REPLACE FUNCTION academy_protect_submission() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' OR OLD.status IN('accepted','rejected','revision_requested') OR (OLD.status<>'draft' AND (NEW.answer IS DISTINCT FROM OLD.answer OR NEW.enrollment_id<>OLD.enrollment_id OR NEW.assessment_id<>OLD.assessment_id OR NEW.program_version_id<>OLD.program_version_id OR NEW.number<>OLD.number)) THEN
 RAISE EXCEPTION 'Submitted answers are immutable';
 END IF;
 RETURN NEW;
END $$;
CREATE OR REPLACE FUNCTION academy_protect_test_result() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' OR OLD.status='finished' THEN RAISE EXCEPTION 'Finished Academy attempts are immutable'; END IF;
 RETURN NEW;
END $$;
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgname='academy_test_result_protected' AND tgrelid='academy_test_attempts'::regclass) THEN
 CREATE TRIGGER academy_test_result_protected BEFORE UPDATE OR DELETE ON academy_test_attempts FOR EACH ROW EXECUTE FUNCTION academy_protect_test_result();
 END IF;
END $$;
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgname='academy_submission_protected' AND tgrelid='academy_assignment_submissions'::regclass) THEN
 CREATE TRIGGER academy_submission_protected BEFORE UPDATE OR DELETE ON academy_assignment_submissions FOR EACH ROW EXECUTE FUNCTION academy_protect_submission();
 END IF;
END $$;
