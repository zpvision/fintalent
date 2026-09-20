ALTER TABLE users
    ADD COLUMN IF NOT EXISTS profile_mode VARCHAR(32);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'users_profile_mode_check'
          AND conrelid = 'users'::regclass
    ) THEN
        ALTER TABLE users
            ADD CONSTRAINT users_profile_mode_check
            CHECK (profile_mode IS NULL OR profile_mode IN ('job_search', 'professional'));
    END IF;
END $$;
