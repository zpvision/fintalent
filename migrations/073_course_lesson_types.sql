INSERT INTO dictionaries(name,alias) VALUES ('Типы уроков','course_lesson_types') ON CONFLICT DO NOTHING;
INSERT INTO dictionary_items(dictionary_id,value,sort_order)
SELECT d.id,v.name,v.ord FROM dictionaries d CROSS JOIN (VALUES
 ('Видео','video',0),('Текст','text',1),('Практическое задание','practice',2),
 ('Тест','test',3),('Материалы','materials',4)) v(name,key,ord) WHERE d.alias='course_lesson_types'
AND NOT EXISTS(SELECT 1 FROM dictionary_items WHERE dictionary_id=d.id);
CREATE TABLE IF NOT EXISTS course_lesson_type_bindings (
 dictionary_item_id BIGINT PRIMARY KEY REFERENCES dictionary_items(id),
 type_key VARCHAR(20) NOT NULL UNIQUE CHECK(type_key IN ('video','text','practice','test','materials'))
);
INSERT INTO course_lesson_type_bindings(dictionary_item_id,type_key)
SELECT i.id,v.key FROM dictionary_items i JOIN dictionaries d ON d.id=i.dictionary_id
JOIN (VALUES ('Видео','video'),('Текст','text'),('Практическое задание','practice'),
 ('Тест','test'),('Материалы','materials')) v(name,key) ON v.name=i.value
WHERE d.alias='course_lesson_types' ON CONFLICT DO NOTHING;
ALTER TABLE course_lessons ALTER COLUMN type TYPE VARCHAR(40);
ALTER TABLE course_lessons DROP CONSTRAINT IF EXISTS course_lessons_type_check;
CREATE OR REPLACE VIEW course_lesson_types AS
SELECT i.id,COALESCE(b.type_key,'lesson-'||i.id::text) AS key,i.value AS name,
 COALESCE(NULLIF(i.icon,''),CASE b.type_key WHEN 'video' THEN 'video' WHEN 'text' THEN 'book'
 WHEN 'practice' THEN 'briefcase' WHEN 'test' THEN 'check' ELSE 'folder' END) AS icon,
 i.active,i.deleted_at,i.sort_order
FROM dictionary_items i JOIN dictionaries d ON d.id=i.dictionary_id
LEFT JOIN course_lesson_type_bindings b ON b.dictionary_item_id=i.id
WHERE d.alias='course_lesson_types';
