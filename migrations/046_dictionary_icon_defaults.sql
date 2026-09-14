-- Match built-in icons by dictionary alias and answer, independently of database IDs
-- and admin sorting. Numeric suffixes below are existing asset filenames, not row IDs.
WITH defaults(alias, value, icon) AS (
    VALUES
    ('position', 'Главный бухгалтер', '/static/icons/positions/position-00.svg'),
    ('position', 'Заместитель главного бухгалтера', '/static/icons/positions/position-01.svg'),
    ('position', 'Бухгалтер', '/static/icons/positions/position-02.svg'),
    ('position', 'Помощник бухгалтера', '/static/icons/positions/position-03.svg'),
    ('position', 'Бухгалтер по заработной плате', '/static/icons/positions/position-04.svg'),
    ('position', 'Бухгалтер по первичной документации', '/static/icons/positions/position-05.svg'),
    ('position', 'Бухгалтер по налогам', '/static/icons/positions/position-06.svg'),
    ('position', 'Финансовый бухгалтер', '/static/icons/positions/position-07.svg'),
    ('position', 'Аудитор', '/static/icons/positions/position-08.svg'),
    ('position', 'Налоговый консультант', '/static/icons/positions/position-09.svg'),
    ('position', 'Финансовый аналитик', '/static/icons/positions/position-10.svg'),
    ('position', 'Экономист', '/static/icons/positions/position-11.svg'),
    ('position', 'Другой вариант', '/static/icons/positions/position-12.svg'),
    ('experience', 'Нет опыта', '/static/icons/dictionaries/experience-14.svg'),
    ('experience', 'До 1 года', '/static/icons/dictionaries/experience-15.svg'),
    ('experience', '1–3 года', '/static/icons/dictionaries/experience-16.svg'),
    ('experience', '3–5 лет', '/static/icons/dictionaries/experience-17.svg'),
    ('experience', '5–10 лет', '/static/icons/dictionaries/experience-18.svg'),
    ('experience', 'Более 10 лет', '/static/icons/dictionaries/experience-19.svg'),
    ('business_sector', 'Производство', '/static/icons/dictionaries/business_sector-20.svg'),
    ('business_sector', 'Торговля', '/static/icons/dictionaries/business_sector-21.svg'),
    ('business_sector', 'Услуги', '/static/icons/dictionaries/business_sector-22.svg'),
    ('business_sector', 'Строительство', '/static/icons/dictionaries/business_sector-23.svg'),
    ('business_sector', 'IT', '/static/icons/dictionaries/business_sector-24.svg'),
    ('business_sector', 'Маркетплейсы', '/static/icons/dictionaries/business_sector-25.svg'),
    ('business_sector', 'Общепит', '/static/icons/dictionaries/business_sector-26.svg'),
    ('business_sector', 'Медицина', '/static/icons/dictionaries/business_sector-27.svg'),
    ('business_sector', 'Образование', '/static/icons/dictionaries/business_sector-28.svg'),
    ('business_sector', 'Государственные учреждения', '/static/icons/dictionaries/business_sector-29.svg'),
    ('business_sector', 'Некоммерческие организации', '/static/icons/dictionaries/business_sector-30.svg'),
    ('business_sector', 'Логистика', '/static/icons/dictionaries/business_sector-31.svg'),
    ('business_sector', 'Другое', '/static/icons/dictionaries/business_sector-32.svg'),
    ('company_size', 'До 10 сотрудников', '/static/icons/dictionaries/company_size-33.svg'),
    ('company_size', 'До 30', '/static/icons/dictionaries/company_size-34.svg'),
    ('company_size', 'До 100', '/static/icons/dictionaries/company_size-35.svg'),
    ('company_size', 'До 300', '/static/icons/dictionaries/company_size-36.svg'),
    ('company_size', 'Более 300', '/static/icons/dictionaries/company_size-37.svg'),
    ('accounting_areas', 'НДС', '/static/icons/accounting-areas/vat.svg'),
    ('accounting_areas', 'УСН', '/static/icons/accounting-areas/usn.svg'),
    ('accounting_areas', 'ОСНО', '/static/icons/accounting-areas/osno.svg'),
    ('accounting_areas', 'Зарплата и кадры', '/static/icons/accounting-areas/payroll.svg'),
    ('accounting_areas', 'ТМЦ', '/static/icons/accounting-areas/inventory.svg'),
    ('accounting_areas', 'Банк и касса', '/static/icons/accounting-areas/bank-cash.svg'),
    ('accounting_areas', 'Основные средства', '/static/icons/accounting-areas/fixed-assets.svg'),
    ('accounting_areas', 'Отчетность', '/static/icons/accounting-areas/reporting.svg'),
    ('accounting_areas', 'ВЭД', '/static/icons/accounting-areas/foreign-trade.svg'),
    ('accounting_areas', 'Производство', '/static/icons/accounting-areas/production.svg'),
    ('software', '1С:Бухгалтерия', '/static/icons/software/1c-accounting.svg'),
    ('software', '1С:ЗУП', '/static/icons/software/1c-zup.svg'),
    ('software', '1С:ERP', '/static/icons/software/1c-erp.svg'),
    ('software', 'СБИС', '/static/icons/software/sbis.svg'),
    ('software', 'Контур.Экстерн', '/static/icons/software/kontur-extern.svg'),
    ('software', 'Диадок', '/static/icons/software/diadoc.svg'),
    ('software', 'Excel', '/static/icons/software/excel.svg'),
    ('software', 'Мое дело', '/static/icons/software/moe-delo.svg'),
    ('software', 'Платформа ОФД', '/static/icons/software/platforma-ofd.svg'),
    ('software', 'Такском', '/static/icons/software/taxcom.svg'),
    ('companies_managed_simultaneously', '1', '/static/icons/dictionaries/companies_managed_simultaneously-56.svg'),
    ('companies_managed_simultaneously', '2-5', '/static/icons/dictionaries/companies_managed_simultaneously-57.svg'),
    ('companies_managed_simultaneously', '6-10', '/static/icons/dictionaries/companies_managed_simultaneously-58.svg'),
    ('companies_managed_simultaneously', '11-20', '/static/icons/dictionaries/companies_managed_simultaneously-59.svg'),
    ('companies_managed_simultaneously', '20-50', '/static/icons/dictionaries/companies_managed_simultaneously-60.svg'),
    ('companies_managed_simultaneously', 'Более 50', '/static/icons/dictionaries/companies_managed_simultaneously-61.svg'),
    ('legal_entities_managed_total', '1-5', '/static/icons/dictionaries/legal_entities_managed_total-62.svg'),
    ('legal_entities_managed_total', '6-20', '/static/icons/dictionaries/legal_entities_managed_total-63.svg'),
    ('legal_entities_managed_total', '21-50', '/static/icons/dictionaries/legal_entities_managed_total-64.svg'),
    ('legal_entities_managed_total', '51-100', '/static/icons/dictionaries/legal_entities_managed_total-65.svg'),
    ('legal_entities_managed_total', 'Более 100', '/static/icons/dictionaries/legal_entities_managed_total-66.svg'),
    ('monthly_primary_documents', 'До 100', '/static/icons/dictionaries/monthly_primary_documents-67.svg'),
    ('monthly_primary_documents', '100-500', '/static/icons/dictionaries/monthly_primary_documents-68.svg'),
    ('monthly_primary_documents', '500-1000', '/static/icons/dictionaries/monthly_primary_documents-69.svg'),
    ('monthly_primary_documents', '1000-5000', '/static/icons/dictionaries/monthly_primary_documents-70.svg'),
    ('monthly_primary_documents', 'Более 5000', '/static/icons/dictionaries/monthly_primary_documents-71.svg'),
    ('employees_in_payroll', 'До 10', '/static/icons/dictionaries/employees_in_payroll-72.svg'),
    ('employees_in_payroll', '10-50', '/static/icons/dictionaries/employees_in_payroll-73.svg'),
    ('employees_in_payroll', '51-100', '/static/icons/dictionaries/employees_in_payroll-74.svg'),
    ('employees_in_payroll', '101-200', '/static/icons/dictionaries/employees_in_payroll-75.svg'),
    ('employees_in_payroll', 'Более 200', '/static/icons/dictionaries/employees_in_payroll-76.svg'),
    ('maximum_company_turnover', 'До 30 млн ₽', '/static/icons/dictionaries/maximum_company_turnover-77.svg'),
    ('maximum_company_turnover', '30-100 млн ₽', '/static/icons/dictionaries/maximum_company_turnover-78.svg'),
    ('maximum_company_turnover', '100-500 млн ₽', '/static/icons/dictionaries/maximum_company_turnover-79.svg'),
    ('maximum_company_turnover', 'Более 500 млн ₽', '/static/icons/dictionaries/maximum_company_turnover-80.svg'),
    ('tax_audits', 'Нет', '/static/icons/dictionaries/tax_audits-81.svg'),
    ('tax_audits', 'Да, 1–2 раза', '/static/icons/dictionaries/tax_audits-82.svg'),
    ('tax_audits', 'Да, регулярно', '/static/icons/dictionaries/tax_audits-83.svg')
)
UPDATE dictionary_items AS item
SET icon = defaults.icon
FROM dictionaries AS dictionary, defaults
WHERE item.dictionary_id = dictionary.id
  AND dictionary.alias = defaults.alias
  AND BTRIM(item.value) = defaults.value
  AND item.deleted_at IS NULL
  AND item.icon IS DISTINCT FROM defaults.icon
  AND (
      BTRIM(COALESCE(item.icon, '')) = ''
      OR (dictionary.alias = 'position' AND item.icon LIKE '/api/assets/position-icon/%')
      OR (dictionary.alias = 'accounting_areas' AND item.icon LIKE '/api/assets/accounting-area-icon/%')
      OR item.icon ~ '^/static/icons/positions/position-[0-9]+[.]svg$'
      OR item.icon ~ '^/static/icons/dictionaries/[a-z_]+-[0-9]+[.]svg$'
  );

-- Run after dictionary creation so new installations get title icons on first start.
UPDATE dictionaries
SET icon = '/api/assets/dictionary-icon/' || id || '.svg'
WHERE BTRIM(COALESCE(icon, '')) = '';
