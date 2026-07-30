ALTER TABLE vacancy_survey_blocks
    ADD COLUMN IF NOT EXISTS selection_color VARCHAR(20) NOT NULL DEFAULT 'blue';
ALTER TABLE applicant_survey_blocks
    ADD COLUMN IF NOT EXISTS selection_color VARCHAR(20) NOT NULL DEFAULT 'blue';

UPDATE vacancy_survey_blocks SET selection_color = CASE name
    WHEN 'Участки' THEN 'green'
    WHEN 'Программы' THEN 'violet'
    WHEN 'CRM' THEN 'teal'
    WHEN 'Общая информация' THEN 'orange'
    WHEN 'Дополнительная информация' THEN 'rose'
    ELSE 'blue'
END;

UPDATE applicant_survey_blocks SET selection_color = CASE name
    WHEN 'Профессиональные навыки' THEN 'green'
    WHEN 'Общая информация' THEN 'orange'
    WHEN 'Проверки' THEN 'rose'
    ELSE 'blue'
END;
