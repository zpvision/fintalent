ALTER TABLE applicant_survey_blocks
    ADD COLUMN IF NOT EXISTS columns_per_row INTEGER NOT NULL DEFAULT 4;

UPDATE applicant_survey_blocks SET columns_per_row = 4;
