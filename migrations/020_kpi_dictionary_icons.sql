UPDATE dictionaries
SET icon = CASE alias
    WHEN 'legal_entities_managed_total' THEN '/static/icons/dictionaries/kpi-organizations.svg'
    WHEN 'maximum_company_turnover' THEN '/static/icons/dictionaries/kpi-turnover.svg'
    WHEN 'tax_audits' THEN '/static/icons/dictionaries/kpi-tax-audit.svg'
    ELSE icon
END,
updated_at = NOW()
WHERE alias IN (
    'legal_entities_managed_total',
    'maximum_company_turnover',
    'tax_audits'
);
