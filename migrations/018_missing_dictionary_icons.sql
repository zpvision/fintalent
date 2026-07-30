UPDATE dictionary_items AS item
SET icon = '/static/icons/dictionaries/' || dictionary.alias || '-' || item.id || '.svg'
FROM dictionaries AS dictionary
WHERE dictionary.id = item.dictionary_id
  AND BTRIM(COALESCE(item.icon, '')) = ''
  AND item.id IN (
      14,15,16,17,18,19,
      20,21,22,23,24,25,26,27,28,29,30,31,32,
      33,34,35,36,37,
      56,57,58,59,60,61,
      62,63,64,65,66,
      67,68,69,70,71,
      72,73,74,75,76,
      77,78,79,80,
      81,82,83
  );
