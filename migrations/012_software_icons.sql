UPDATE dictionary_items AS item
SET icon = CASE item.value
    WHEN '1С:Бухгалтерия' THEN '/static/icons/software/1c-accounting.svg'
    WHEN '1С:ЗУП' THEN '/static/icons/software/1c-zup.svg'
    WHEN '1С:ERP' THEN '/static/icons/software/1c-erp.svg'
    WHEN 'СБИС' THEN '/static/icons/software/sbis.svg'
    WHEN 'Контур.Экстерн' THEN '/static/icons/software/kontur-extern.svg'
    WHEN 'Диадок' THEN '/static/icons/software/diadoc.svg'
    WHEN 'Excel' THEN '/static/icons/software/excel.svg'
    WHEN 'Мое дело' THEN '/static/icons/software/moe-delo.svg'
    WHEN 'Платформа ОФД' THEN '/static/icons/software/platforma-ofd.svg'
    WHEN 'Такском' THEN '/static/icons/software/taxcom.svg'
    ELSE item.icon
END
FROM dictionaries AS dictionary
WHERE dictionary.id = item.dictionary_id
  AND dictionary.alias = 'software';
