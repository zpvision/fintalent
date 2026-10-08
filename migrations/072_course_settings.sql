INSERT INTO dictionaries(name,alias) SELECT 'Возможности курса','course_features'
WHERE NOT EXISTS(SELECT 1 FROM dictionaries WHERE alias='course_features');
INSERT INTO dictionary_items(dictionary_id,value,sort_order)
SELECT d.id,v.name,v.ord FROM dictionaries d CROSS JOIN (VALUES
 ('Сертификат после завершения',0),('Чат с автором и участниками',1),
 ('Обновления курса включены',2),('Доступ с любого устройства',3),
 ('Доступно корпоративное обучение',4)
) v(name,ord) WHERE d.alias='course_features'
AND NOT EXISTS(SELECT 1 FROM dictionary_items WHERE dictionary_id=d.id);
CREATE TABLE IF NOT EXISTS course_feature_bindings (
 dictionary_item_id BIGINT PRIMARY KEY REFERENCES dictionary_items(id),
 feature_key VARCHAR(40) NOT NULL UNIQUE CHECK(feature_key IN ('certificate','chat','updates','any_device','corporate'))
);
INSERT INTO course_feature_bindings(dictionary_item_id,feature_key)
SELECT i.id,v.key FROM dictionary_items i JOIN dictionaries d ON d.id=i.dictionary_id
JOIN (VALUES ('Сертификат после завершения','certificate'),('Чат с автором и участниками','chat'),
 ('Обновления курса включены','updates'),('Доступ с любого устройства','any_device'),
 ('Доступно корпоративное обучение','corporate')) v(name,key) ON v.name=i.value
WHERE d.alias='course_features' ON CONFLICT DO NOTHING;
CREATE TABLE IF NOT EXISTS course_feature_selections (
 course_id BIGINT NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
 dictionary_item_id BIGINT NOT NULL REFERENCES dictionary_items(id),
 PRIMARY KEY(course_id,dictionary_item_id)
);
INSERT INTO course_feature_selections(course_id,dictionary_item_id)
SELECT c.id,b.dictionary_item_id FROM courses c CROSS JOIN course_feature_bindings b
WHERE CASE b.feature_key WHEN 'certificate' THEN c.certificate WHEN 'chat' THEN c.chat
 WHEN 'updates' THEN c.updates WHEN 'any_device' THEN c.any_device WHEN 'corporate' THEN c.corporate END
ON CONFLICT DO NOTHING;
ALTER TABLE courses ADD COLUMN IF NOT EXISTS bonuses_color VARCHAR(40) NOT NULL DEFAULT '';
