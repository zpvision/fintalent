DO $demo$
DECLARE
    n integer;
    uid bigint;
    entity_id bigint;
    city_id bigint;
    titles text[] := ARRAY['Главный бухгалтер','Бухгалтер по заработной плате','Бухгалтер на первичную документацию','Финансовый аналитик','Заместитель главного бухгалтера','Бухгалтер по налогам','Экономист','Финансовый контролёр','Бухгалтер по учёту ТМЦ','Казначей','Бухгалтер ВЭД','Методолог бухгалтерского учёта'];
    names text[] := ARRAY['Анна Волкова','Михаил Орлов','Елена Соколова','Дмитрий Морозов','Ольга Лебедева','Алексей Кузнецов','Мария Новикова','Сергей Попов','Наталья Фёдорова','Илья Васильев','Виктория Павлова','Андрей Семёнов'];
    companies text[] := ARRAY['ФинПрофи','Северный Альянс','Технологии Учёта','Городской Проект','Баланс Групп','Альфа Консалт','Новая Логистика','ПромИнвест'];
BEGIN
  FOR n IN 4..51 LOOP
    INSERT INTO users(full_name,email,password_hash,agreed_to_terms,avatar_url)
    VALUES (
      CASE WHEN n < 28 THEN companies[1+((n-4)%array_length(companies,1))] ELSE names[1+((n-28)%array_length(names,1))] END,
      n||'@'||n||'.ru', '__DEMO_PASSWORD_HASH__', TRUE,
      CASE WHEN n >= 28 THEN '/static/profile-3-avatar.png' ELSE '' END
    )
    ON CONFLICT(email) DO UPDATE SET password_hash=EXCLUDED.password_hash;
  END LOOP;

  FOR n IN 4..27 LOOP
    SELECT id INTO uid FROM users WHERE email=n||'@'||n||'.ru';
    SELECT id INTO entity_id FROM vacancies WHERE user_id=uid AND deleted_at IS NULL ORDER BY id LIMIT 1;
    IF entity_id IS NULL THEN
      INSERT INTO vacancies(user_id,title,description,status,salary_from,salary_to,salary_tax_mode,currency,city,address,current_step,published_at,created_at,updated_at)
      VALUES(uid,titles[1+((n-4)%array_length(titles,1))],
      'Ищем внимательного специалиста в дружную финансовую команду. Важны самостоятельность, точность, умение выстраивать процессы и понятно общаться с коллегами.',
      'published',70000+(n-4)*5000,100000+(n-4)*6500,CASE WHEN n%2=0 THEN 'net' ELSE 'gross' END,'RUB',
      (ARRAY['Москва','Санкт-Петербург','Казань','Екатеринбург','Новосибирск','Самара'])[1+((n-4)%6)],
      'Современный офис рядом с метро',10,NOW()-(n||' hours')::interval,NOW()-(n||' days')::interval,NOW())
      RETURNING id INTO entity_id;
    END IF;
    INSERT INTO vacancy_categories(vacancy_id,category_id,block_id,importance,base_weight_snapshot,importance_coefficient_snapshot,effective_weight,sort_order,category_name_snapshot)
    SELECT entity_id,i.id,bd.block_id,CASE WHEN row_number() over() %3=0 THEN 'bonus' WHEN row_number() over()%2=0 THEN 'preferred' ELSE 'required' END,
      COALESCE(i.default_weight,5),100,COALESCE(i.default_weight,5),row_number() over(),i.value
    FROM dictionary_items i
    JOIN vacancy_survey_block_dictionaries bd ON bd.dictionary_id=i.dictionary_id
    WHERE i.active=TRUE AND i.deleted_at IS NULL
    ORDER BY i.dictionary_id,i.sort_order,i.id LIMIT 16
    ON CONFLICT(vacancy_id,category_id) DO NOTHING;
    DELETE FROM vacancy_categories vc USING dictionary_items i,dictionaries d
    WHERE vc.vacancy_id=entity_id AND vc.category_id=i.id AND i.dictionary_id=d.id AND d.alias='position';
    INSERT INTO vacancy_categories(vacancy_id,category_id,block_id,importance,base_weight_snapshot,importance_coefficient_snapshot,effective_weight,sort_order,category_name_snapshot)
    SELECT entity_id,i.id,bd.block_id,'required',COALESCE(i.default_weight,5),100,COALESCE(i.default_weight,5),0,i.value
    FROM dictionary_items i JOIN dictionaries d ON d.id=i.dictionary_id
    JOIN vacancy_survey_block_dictionaries bd ON bd.dictionary_id=d.id
    WHERE d.alias='position' AND i.active=TRUE AND i.deleted_at IS NULL
    ORDER BY i.id OFFSET ((n-4)%12) LIMIT 1;
    INSERT INTO vacancy_duties(vacancy_id,duty_id)
    SELECT entity_id,id FROM duties WHERE is_active=TRUE ORDER BY id OFFSET ((n-4)%5) LIMIT 6
    ON CONFLICT DO NOTHING;
    INSERT INTO vacancy_tests(vacancy_external_id,test_id,test_version_id,sort_order,is_required)
    SELECT entity_id,t.id,v.id,0,TRUE FROM tests t JOIN test_versions v ON v.test_id=t.id AND v.version=t.current_version
    WHERE t.status='published' ORDER BY t.id LIMIT 1 ON CONFLICT DO NOTHING;
    entity_id := NULL;
  END LOOP;

  FOR n IN 28..51 LOOP
    SELECT id INTO uid FROM users WHERE email=n||'@'||n||'.ru';
    SELECT id INTO city_id FROM cities ORDER BY id OFFSET ((n-28)%20) LIMIT 1;
    INSERT INTO resumes(user_id,status,visibility,current_step,desired_salary,available_immediately,search_status_code,preferred_city_id,published_at,work_preferences,created_at,updated_at)
    VALUES(uid,'published','public',10,65000+(n-28)*5500,n%3<>0,CASE WHEN n%4=0 THEN 'considering' ELSE 'open' END,city_id,
      NOW()-(n||' hours')::interval,
      (ARRAY['Ищу команду с понятными процессами и возможностью профессионального роста.','Ценю самостоятельность, доверие и интересные задачи.','Готов(а) развивать учёт и автоматизировать регулярные операции.'])[1+((n-28)%3)],
      NOW()-(n||' days')::interval,NOW())
    ON CONFLICT(user_id) DO UPDATE SET status='published',visibility='public',published_at=EXCLUDED.published_at
    RETURNING id INTO entity_id;
    INSERT INTO resume_categories(resume_id,category_id,block_id,sort_order)
    SELECT entity_id,i.id,bd.block_id,row_number() over()
    FROM dictionary_items i JOIN applicant_survey_block_dictionaries bd ON bd.dictionary_id=i.dictionary_id
    WHERE i.active=TRUE AND i.deleted_at IS NULL ORDER BY i.dictionary_id,i.sort_order,i.id LIMIT 18
    ON CONFLICT DO NOTHING;
    DELETE FROM resume_categories rc USING dictionary_items i,dictionaries d
    WHERE rc.resume_id=entity_id AND rc.category_id=i.id AND i.dictionary_id=d.id AND d.alias='position';
    INSERT INTO resume_categories(resume_id,category_id,block_id,sort_order)
    SELECT entity_id,i.id,bd.block_id,0 FROM dictionary_items i JOIN dictionaries d ON d.id=i.dictionary_id
    JOIN applicant_survey_block_dictionaries bd ON bd.dictionary_id=d.id
    WHERE d.alias='position' AND i.active=TRUE ORDER BY i.id OFFSET ((n-28)%12) LIMIT 1;
    INSERT INTO resume_duties(resume_id,duty_id)
    SELECT entity_id,id FROM duties WHERE is_active=TRUE ORDER BY id OFFSET ((n-28)%5) LIMIT 6 ON CONFLICT DO NOTHING;
    INSERT INTO resume_work_experiences(resume_id,company_name,position,city,start_month,start_year,end_month,end_year,is_current,responsibilities,achievements,sort_order)
    SELECT entity_id,companies[1+((n-28)%array_length(companies,1))],titles[1+((n-28)%array_length(titles,1))],
      (ARRAY['Москва','Санкт-Петербург','Казань','Екатеринбург'])[1+((n-28)%4)],2,2017+((n-28)%4),NULL,NULL,TRUE,
      'Ведение бухгалтерского и налогового учёта, подготовка отчётности, контроль первичных документов и взаимодействие с подразделениями.',
      'Ускорил(а) закрытие месяца и снизил(а) количество ручных операций.',0
    WHERE NOT EXISTS(SELECT 1 FROM resume_work_experiences WHERE resume_id=entity_id);
    INSERT INTO resume_educations(resume_id,education_type,institution,specialization,city,start_year,end_year,is_current,description,sort_order)
    SELECT entity_id,'higher',(ARRAY['РЭУ им. Г. В. Плеханова','Финансовый университет','СПбГЭУ','КФУ'])[1+((n-28)%4)],
      'Экономика и бухгалтерский учёт','',NULL,2014+((n-28)%7),FALSE,'Высшее экономическое образование',0
    WHERE NOT EXISTS(SELECT 1 FROM resume_educations WHERE resume_id=entity_id);
    INSERT INTO resume_languages(resume_id,language_id,sort_order)
    SELECT entity_id,id,row_number() over() FROM languages WHERE code IN ('ru',CASE WHEN n%2=0 THEN 'en' ELSE 'de' END) ON CONFLICT DO NOTHING;
    INSERT INTO resume_preferred_cities(resume_id,city_id,sort_order) VALUES(entity_id,city_id,0) ON CONFLICT DO NOTHING;
    INSERT INTO resume_work_formats(resume_id,dictionary_item_id,sort_order)
    SELECT entity_id,i.id,row_number() over() FROM dictionary_items i JOIN dictionaries d ON d.id=i.dictionary_id
    WHERE d.alias='work_format' AND i.active=TRUE ORDER BY i.id LIMIT 2 ON CONFLICT DO NOTHING;
  END LOOP;
END $demo$;
