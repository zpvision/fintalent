CREATE TABLE IF NOT EXISTS course_commission_defaults (
 id BOOLEAN PRIMARY KEY DEFAULT true CHECK(id),
 standard_bps INTEGER NOT NULL DEFAULT 2000 CHECK(standard_bps BETWEEN 0 AND 10000),
 partner_bps INTEGER NOT NULL DEFAULT 1200 CHECK(partner_bps BETWEEN 0 AND 10000)
);
INSERT INTO course_commission_defaults(id) VALUES(true) ON CONFLICT DO NOTHING;
CREATE TABLE IF NOT EXISTS course_commission_terms (
 course_id BIGINT PRIMARY KEY REFERENCES courses(id),
 standard_bps INTEGER NOT NULL CHECK(standard_bps BETWEEN 0 AND 10000),
 partner_bps INTEGER NOT NULL CHECK(partner_bps BETWEEN 0 AND 10000),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO course_commission_terms(course_id,standard_bps,partner_bps)
 SELECT c.id,d.standard_bps,d.partner_bps FROM courses c CROSS JOIN course_commission_defaults d
 ON CONFLICT(course_id) DO NOTHING;
CREATE OR REPLACE FUNCTION course_copy_commission_terms() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 INSERT INTO course_commission_terms(course_id,standard_bps,partner_bps)
 SELECT NEW.id,standard_bps,partner_bps FROM course_commission_defaults WHERE id=true;
 RETURN NEW;
END; $$;
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgname='course_initial_commission_terms' AND tgrelid='courses'::regclass) THEN
 CREATE TRIGGER course_initial_commission_terms AFTER INSERT ON courses FOR EACH ROW EXECUTE FUNCTION course_copy_commission_terms();
 END IF;
END; $$;
ALTER TABLE course_interests ADD COLUMN IF NOT EXISTS paid_at TIMESTAMPTZ;
ALTER TABLE course_interests ADD COLUMN IF NOT EXISTS commission_override_bps INTEGER CHECK(commission_override_bps BETWEEN 0 AND 10000);
ALTER TABLE course_interests ADD COLUMN IF NOT EXISTS paid_by BIGINT REFERENCES users(id);
ALTER TABLE course_interests ADD COLUMN IF NOT EXISTS paid_commission_bps INTEGER CHECK(paid_commission_bps BETWEEN 0 AND 10000);
ALTER TABLE course_interests ADD COLUMN IF NOT EXISTS paid_referral_link_id BIGINT;
ALTER TABLE course_interests ADD COLUMN IF NOT EXISTS paid_enrollment_id BIGINT REFERENCES academy_enrollments(id);
ALTER TABLE course_interests ADD COLUMN IF NOT EXISTS invitation_outbox_id BIGINT REFERENCES notification_outbox(id);
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='course_interest_paid_source' AND conrelid='course_interests'::regclass) THEN
 ALTER TABLE course_interests ADD CONSTRAINT course_interest_paid_source FOREIGN KEY(paid_referral_link_id,course_id) REFERENCES course_referral_links(id,course_id);
 END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='course_interest_paid_complete' AND conrelid='course_interests'::regclass) THEN
 ALTER TABLE course_interests ADD CONSTRAINT course_interest_paid_complete CHECK(paid_at IS NULL OR (user_id IS NOT NULL AND paid_commission_bps IS NOT NULL AND paid_enrollment_id IS NOT NULL AND invitation_outbox_id IS NOT NULL));
 END IF;
END; $$;
CREATE INDEX IF NOT EXISTS course_interests_unpaid ON course_interests(course_id,first_clicked_at DESC,id) WHERE user_id IS NOT NULL AND paid_at IS NULL;
CREATE OR REPLACE FUNCTION course_interest_preserve_payment() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF OLD.paid_at IS NOT NULL AND ROW(NEW.paid_at,NEW.paid_by,NEW.paid_commission_bps,NEW.paid_referral_link_id,NEW.paid_enrollment_id,NEW.invitation_outbox_id) IS DISTINCT FROM ROW(OLD.paid_at,OLD.paid_by,OLD.paid_commission_bps,OLD.paid_referral_link_id,OLD.paid_enrollment_id,OLD.invitation_outbox_id) THEN
 RAISE EXCEPTION 'Confirmed course payment is immutable';
 END IF;
 RETURN NEW;
END; $$;
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgname='course_interest_payment_immutable' AND tgrelid='course_interests'::regclass) THEN
 CREATE TRIGGER course_interest_payment_immutable BEFORE UPDATE ON course_interests FOR EACH ROW EXECUTE FUNCTION course_interest_preserve_payment();
 END IF;
END; $$;
