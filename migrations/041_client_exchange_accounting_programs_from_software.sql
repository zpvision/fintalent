WITH software AS (
    SELECT i.value AS name, COALESCE(i.icon, '') AS icon, i.sort_order + 1 AS sort_order
    FROM dictionary_items i
    JOIN dictionaries d ON d.id = i.dictionary_id
    WHERE d.alias = 'software'
      AND i.active = TRUE
      AND i.deleted_at IS NULL
)
UPDATE client_exchange_dictionary_items ce
SET icon = software.icon,
    sort_order = software.sort_order,
    active = TRUE,
    updated_at = NOW()
FROM software
WHERE ce.kind = 'accounting_program'
  AND ce.name = software.name
  AND ce.deleted_at IS NULL;

WITH software AS (
    SELECT i.id, i.value AS name, COALESCE(i.icon, '') AS icon, i.sort_order + 1 AS sort_order
    FROM dictionary_items i
    JOIN dictionaries d ON d.id = i.dictionary_id
    WHERE d.alias = 'software'
      AND i.active = TRUE
      AND i.deleted_at IS NULL
)
INSERT INTO client_exchange_dictionary_items(kind, code, name, icon, sort_order, active)
SELECT 'accounting_program', 'software_' || id, name, icon, sort_order, TRUE
FROM software
WHERE NOT EXISTS (
    SELECT 1
    FROM client_exchange_dictionary_items ce
    WHERE ce.kind = 'accounting_program'
      AND ce.name = software.name
      AND ce.deleted_at IS NULL
)
ON CONFLICT(kind, code) DO UPDATE
SET name = EXCLUDED.name,
    icon = EXCLUDED.icon,
    sort_order = EXCLUDED.sort_order,
    active = TRUE,
    updated_at = NOW();

WITH software AS (
    SELECT i.value AS name
    FROM dictionary_items i
    JOIN dictionaries d ON d.id = i.dictionary_id
    WHERE d.alias = 'software'
      AND i.active = TRUE
      AND i.deleted_at IS NULL
)
UPDATE client_exchange_dictionary_items ce
SET active = FALSE,
    updated_at = NOW()
WHERE ce.kind = 'accounting_program'
  AND ce.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM software WHERE software.name = ce.name);
