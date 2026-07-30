UPDATE dictionary_items AS item
SET icon = CASE item.value
    WHEN 'НДС' THEN '/static/icons/accounting-areas/vat.svg'
    WHEN 'УСН' THEN '/static/icons/accounting-areas/usn.svg'
    WHEN 'ОСНО' THEN '/static/icons/accounting-areas/osno.svg'
    WHEN 'Зарплата и кадры' THEN '/static/icons/accounting-areas/payroll.svg'
    WHEN 'ТМЦ' THEN '/static/icons/accounting-areas/inventory.svg'
    WHEN 'Банк и касса' THEN '/static/icons/accounting-areas/bank-cash.svg'
    WHEN 'Основные средства' THEN '/static/icons/accounting-areas/fixed-assets.svg'
    WHEN 'Отчетность' THEN '/static/icons/accounting-areas/reporting.svg'
    WHEN 'ВЭД' THEN '/static/icons/accounting-areas/foreign-trade.svg'
    WHEN 'Производство' THEN '/static/icons/accounting-areas/production.svg'
    ELSE item.icon
END
FROM dictionaries AS dictionary
WHERE dictionary.id = item.dictionary_id
  AND dictionary.alias = 'accounting_areas';
