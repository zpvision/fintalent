UPDATE accounting_company_service_catalog AS service
SET icon = icons.path,
    updated_at = NOW()
FROM (VALUES
    ('accounting-support', '/static/icons/acs/01.svg'),
    ('tax-planning', '/static/icons/acs/02.svg'),
    ('payroll', '/static/icons/acs/03.svg'),
    ('reporting', '/static/icons/acs/04.svg'),
    ('consulting', '/static/icons/acs/05.svg'),
    ('one-time', '/static/icons/acs/06.svg'),
    ('accounting-recovery', '/static/icons/acs/07.svg'),
    ('registration', '/static/icons/acs/08.svg'),
    ('foreign-trade', '/static/icons/acs/09.svg'),
    ('marketplace-accounting', '/static/icons/acs/10.svg'),
    ('management-accounting', '/static/icons/acs/11.svg'),
    ('outsourced-cfo', '/static/icons/acs/12.svg'),
    ('tax-audit', '/static/icons/acs/13.svg'),
    ('one-c', '/static/icons/acs/14.svg'),
    ('edo', '/static/icons/acs/15.svg')
) AS icons(slug, path)
WHERE service.slug = icons.slug
  AND service.icon IS DISTINCT FROM icons.path;
