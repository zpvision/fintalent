DO $$
DECLARE
    crm_id BIGINT;
BEGIN
    INSERT INTO dictionaries(name, alias, use_importance_in_vacancy, single_choice, vacancy_title)
    VALUES ('CRM-системы', 'crm', TRUE, FALSE, 'С какими CRM-системами нужно уметь работать?')
    ON CONFLICT (alias) WHERE alias IS NOT NULL
    DO UPDATE SET
        name = EXCLUDED.name,
        vacancy_title = EXCLUDED.vacancy_title,
        use_importance_in_vacancy = EXCLUDED.use_importance_in_vacancy,
        single_choice = EXCLUDED.single_choice,
        updated_at = NOW()
    RETURNING id INTO crm_id;

    DELETE FROM dictionary_items WHERE dictionary_id = crm_id;

    INSERT INTO dictionary_items(dictionary_id, value, comment, icon, sort_order, active)
    VALUES
        (crm_id, '1С:CRM', 'CRM на платформе 1С, интеграция с бухгалтерией и торговлей', '/static/icons/crm/crm-00.svg', 0, TRUE),
        (crm_id, 'FinKoper', 'CRM для бухгалтерских компаний: клиенты, задачи, отчётность и документы', '/static/icons/crm/crm-15.svg', 1, TRUE),
        (crm_id, 'Saby (СБИС) CRM', 'Продажи, клиенты и документы в экосистеме Saby/СБИС', '/static/icons/crm/crm-01.svg', 2, TRUE),
        (crm_id, 'Битрикс24', 'CRM, сделки, задачи и коммуникации', '/static/icons/crm/crm-02.svg', 3, TRUE),
        (crm_id, 'amoCRM', 'CRM для отделов продаж и работы с лидами', '/static/icons/crm/crm-03.svg', 4, TRUE),
        (crm_id, 'Мегаплан', 'Продажи, клиенты, проекты и задачи', '/static/icons/crm/crm-04.svg', 5, TRUE),
        (crm_id, 'ПланФикс', 'Управление клиентами и бизнес-процессами', '/static/icons/crm/crm-05.svg', 6, TRUE),
        (crm_id, 'retailCRM', 'CRM для интернет-магазинов и розницы', '/static/icons/crm/crm-06.svg', 7, TRUE),
        (crm_id, 'МойСклад', 'Клиенты, продажи, складской и товарный учёт', '/static/icons/crm/crm-07.svg', 8, TRUE),
        (crm_id, 'ELMA365 CRM', 'Low-code CRM и автоматизация процессов', '/static/icons/crm/crm-08.svg', 9, TRUE),
        (crm_id, 'BPMSoft', 'Корпоративная CRM и управление процессами', '/static/icons/crm/crm-09.svg', 10, TRUE),
        (crm_id, 'RegionSoft CRM', 'CRM для продаж, сервиса и аналитики', '/static/icons/crm/crm-10.svg', 11, TRUE),
        (crm_id, 'РосБизнесСофт CRM', 'CRM и комплексная автоматизация предприятия', '/static/icons/crm/crm-11.svg', 12, TRUE),
        (crm_id, 'Простой бизнес', 'CRM, проекты, документы и коммуникации', '/static/icons/crm/crm-12.svg', 13, TRUE),
        (crm_id, 'Клиентская база', 'Конструктор CRM и базы клиентов', '/static/icons/crm/crm-13.svg', 14, TRUE);
END $$;
