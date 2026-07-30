ALTER TABLE dictionaries ADD COLUMN IF NOT EXISTS vacancy_title VARCHAR(300) NOT NULL DEFAULT '';
UPDATE dictionaries SET vacancy_title = CASE alias
    WHEN 'position' THEN 'Кого вы ищете?'
    WHEN 'experience' THEN 'Какой опыт работы требуется кандидату?'
    WHEN 'business_sector' THEN 'В какой сфере работает компания?'
    WHEN 'company_size' THEN 'Какого размера компания?'
    WHEN 'accounting_areas' THEN 'Какие участки учета должен знать кандидат?'
    WHEN 'software' THEN 'Какими программами должен владеть кандидат?'
    WHEN 'companies_managed_simultaneously' THEN 'Сколько компаний нужно будет вести одновременно?'
    WHEN 'legal_entities_managed_total' THEN 'Сколько юридических фирм нужно будет вести?'
    WHEN 'monthly_primary_documents' THEN 'Какой объем первичных документов будет в месяц?'
    WHEN 'employees_in_payroll' THEN 'Для скольких сотрудников нужно будет рассчитывать зарплату?'
    WHEN 'maximum_company_turnover' THEN 'С компаниями какого оборота предстоит работать?'
    WHEN 'tax_audits' THEN 'Потребуется ли сопровождать налоговые проверки?'
    ELSE name
END
WHERE vacancy_title = '';
