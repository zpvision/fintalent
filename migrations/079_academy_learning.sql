-- Releases refer to append-only Stage A snapshots. They never point at the working tree.
CREATE TABLE IF NOT EXISTS academy_version_releases (
 version_id BIGINT PRIMARY KEY REFERENCES academy_program_versions(id),
 course_id BIGINT NOT NULL REFERENCES courses(id), published_by BIGINT NOT NULL REFERENCES users(id),
 test_metadata JSONB NOT NULL DEFAULT '[]', published_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 UNIQUE(course_id,version_id)
);
CREATE TABLE IF NOT EXISTS academy_enrollments (
 id BIGSERIAL PRIMARY KEY, course_id BIGINT NOT NULL REFERENCES courses(id),
 user_id BIGINT NOT NULL REFERENCES users(id), program_version_id BIGINT NOT NULL,
 cohort_id BIGINT, -- Reserved for a future real cohort FK; independent study uses NULL.
 access_status TEXT NOT NULL DEFAULT 'active' CHECK(access_status IN ('active','suspended','revoked')),
 source TEXT NOT NULL DEFAULT 'manual', request_key TEXT NOT NULL UNIQUE,
 enrolled_by BIGINT NOT NULL REFERENCES users(id), enrolled_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 started_at TIMESTAMPTZ, expires_at TIMESTAMPTZ, completed_at TIMESTAMPTZ,
 last_activity_at TIMESTAMPTZ, last_lesson_key TEXT NOT NULL DEFAULT '',
 FOREIGN KEY(course_id,program_version_id) REFERENCES academy_version_releases(course_id,version_id),
 UNIQUE(id,program_version_id)
);
CREATE UNIQUE INDEX IF NOT EXISTS academy_independent_active_enrollment ON academy_enrollments(course_id,user_id,program_version_id)
 WHERE cohort_id IS NULL AND access_status IN ('active','suspended');
CREATE INDEX IF NOT EXISTS academy_enrollments_user ON academy_enrollments(user_id,enrolled_at DESC,id);
CREATE INDEX IF NOT EXISTS academy_enrollments_course ON academy_enrollments(course_id,enrolled_at DESC,id);
CREATE TABLE IF NOT EXISTS academy_lesson_progress (
 enrollment_id BIGINT NOT NULL, program_version_id BIGINT NOT NULL, lesson_key TEXT NOT NULL,
 status TEXT NOT NULL CHECK(status IN ('in_progress','completed')),
 first_opened_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), last_opened_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 completed_at TIMESTAMPTZ, materials_acknowledged_at TIMESTAMPTZ,
 PRIMARY KEY(enrollment_id,lesson_key),
 FOREIGN KEY(enrollment_id,program_version_id) REFERENCES academy_enrollments(id,program_version_id)
);
CREATE TABLE IF NOT EXISTS academy_learning_events (
 id BIGSERIAL PRIMARY KEY, enrollment_id BIGINT NOT NULL REFERENCES academy_enrollments(id),
 actor_id BIGINT NOT NULL REFERENCES users(id), lesson_key TEXT NOT NULL DEFAULT '',
 type TEXT NOT NULL, event_key TEXT NOT NULL, data JSONB NOT NULL DEFAULT '{}',
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), UNIQUE(enrollment_id,event_key)
);
CREATE INDEX IF NOT EXISTS academy_learning_events_history ON academy_learning_events(enrollment_id,created_at DESC,id);
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgname='academy_release_immutable' AND tgrelid='academy_version_releases'::regclass) THEN
 CREATE TRIGGER academy_release_immutable BEFORE UPDATE OR DELETE ON academy_version_releases FOR EACH ROW EXECUTE FUNCTION academy_immutable_version();
 END IF;
END $$;
