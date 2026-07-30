ALTER TABLE vacancy_survey_block_dictionaries
    ADD COLUMN IF NOT EXISTS selection_color VARCHAR(20) NOT NULL DEFAULT 'blue';
ALTER TABLE applicant_survey_block_dictionaries
    ADD COLUMN IF NOT EXISTS selection_color VARCHAR(20) NOT NULL DEFAULT 'blue';

UPDATE vacancy_survey_block_dictionaries AS relation
SET selection_color = block.selection_color
FROM vacancy_survey_blocks AS block
WHERE block.id = relation.block_id;

UPDATE applicant_survey_block_dictionaries AS relation
SET selection_color = block.selection_color
FROM applicant_survey_blocks AS block
WHERE block.id = relation.block_id;
