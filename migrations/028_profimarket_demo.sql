DO $seed$
DECLARE author_id BIGINT; ai_author_id BIGINT; regulation_id BIGINT; ai_id BIGINT; section_id BIGINT;
BEGIN
  SELECT id INTO author_id FROM users WHERE email='3@3.ru' LIMIT 1;
  IF author_id IS NULL THEN SELECT id INTO author_id FROM users ORDER BY id LIMIT 1; END IF;
  SELECT id INTO ai_author_id FROM users ORDER BY id LIMIT 1;
  IF author_id IS NULL THEN RETURN; END IF;

  INSERT INTO profimarket_solutions(author_user_id,type,status,title,slug,short_description,description,price,old_price,pricing_type,tags,topics,audiences,is_featured,is_new,published_at)
  VALUES(author_id,'REGULATION','PUBLISHED','Пакет регламентов «МАРКЕТПЛЕЙСЫ»','paket-reglamentov-marketpleysy',
    '15 регламентов для работы с Wildberries и Ozon в 1С:Бухгалтерия 8.3.',
    'Готовый пакет рабочих процессов для внедрения в CRM бухгалтерской компании.',17900,21900,'ONE_TIME',
    ARRAY['Wildberries','Ozon','1С:Бухгалтерия 8.3'],ARRAY['Маркетплейсы','Бухгалтерский учёт'],ARRAY['Бухгалтерские компании','Бухгалтеры маркетплейсов'],TRUE,FALSE,NOW()-INTERVAL '8 days')
  ON CONFLICT(slug) DO UPDATE SET author_user_id=EXCLUDED.author_user_id,status='PUBLISHED',title=EXCLUDED.title,short_description=EXCLUDED.short_description,description=EXCLUDED.description,price=EXCLUDED.price,old_price=EXCLUDED.old_price,pricing_type=EXCLUDED.pricing_type,tags=EXCLUDED.tags,topics=EXCLUDED.topics,audiences=EXCLUDED.audiences,is_featured=TRUE,published_at=COALESCE(profimarket_solutions.published_at,NOW())
  RETURNING id INTO regulation_id;
  DELETE FROM profimarket_regulation_sections WHERE solution_id=regulation_id;
  DELETE FROM profimarket_access_features WHERE solution_id=regulation_id;
  DELETE FROM profimarket_solution_crm WHERE solution_id=regulation_id;
  INSERT INTO profimarket_regulation_sections(solution_id,title,description,sort_order) VALUES(regulation_id,'Wildberries','8 регламентов',10) RETURNING id INTO section_id;
  INSERT INTO profimarket_regulation_items(section_id,title,sort_order) VALUES
  (section_id,'Разнесение банковских поступлений от Wildberries',10),(section_id,'Загрузка отчётов комиссионеров (агентов) Wildberries в 1С',20),(section_id,'Учет продаж Wildberries. Работа с юрлицами',30),(section_id,'Поступление на расчетный счет Wildberries (выкупы)',40),(section_id,'Учет услуг и расходов Wildberries',50),(section_id,'Списания и компенсации Wildberries',60),(section_id,'Учет электронного документооборота (ЭДО) по Wildberries',70),(section_id,'Сверки с Wildberries',80);
  INSERT INTO profimarket_regulation_sections(solution_id,title,description,sort_order) VALUES(regulation_id,'Ozon','7 регламентов',20) RETURNING id INTO section_id;
  INSERT INTO profimarket_regulation_items(section_id,title,sort_order) VALUES
  (section_id,'Загрузка отчётов комиссионеров (агентов) Ozon в 1С',10),(section_id,'Поступление на расчетный счет Ozon (выкупы)',20),(section_id,'Учет услуг и расходов Ozon',30),(section_id,'Списания и компенсации Ozon',40),(section_id,'Учет электронного документооборота (ЭДО) по Ozon',50),(section_id,'Сверки с Ozon',60),(section_id,'Проверка НДС по маркетплейсам. Закрытие месяца',70);
  INSERT INTO profimarket_access_features(solution_id,icon,title,description,sort_order) VALUES
  (regulation_id,'workflow','15 регламентов','Готовые процессы и инструкции для работы с Wildberries и Ozon.',10),
  (regulation_id,'calculator','Работа в 1С:Бухгалтерия 8.3','Пошаговые действия внутри 1С:Бухгалтерия 8.3.',20),
  (regulation_id,'users','Доступ к закрытому сообществу «Регламенты бухбизнеса»','',30),
  (regulation_id,'message','Обратная связь при внедрении и адаптации','',40),
  (regulation_id,'video','Мастер-класс «Автоматизация процессов в бухгалтерской компании»','',50),
  (regulation_id,'gift','1 месяц FinKoper бесплатно по промокоду','',60),
  (regulation_id,'infinity','Доступ к регламентам навсегда','',70),
  (regulation_id,'cloud','Регламенты хранятся в FinKoper','',80);
  INSERT INTO profimarket_solution_crm(solution_id,crm_id) SELECT regulation_id,id FROM profimarket_crm WHERE code IN ('finkoper','other') ON CONFLICT DO NOTHING;

  INSERT INTO profimarket_solutions(author_user_id,type,status,title,slug,short_description,description,price,pricing_type,trial_days,delivery_type,external_url,tags,topics,audiences,is_featured,is_new,published_at)
  VALUES(ai_author_id,'AI_ASSISTANT','PUBLISHED','ИИ-помощник по требованиям ФНС','ii-pomoschnik-po-trebovaniyam-fns',
    'Анализирует требования из ФНС, выделяет суть запроса, сроки и риски. Помогает подготовить ответ и собрать необходимые документы.',
    'Профессиональный ИИ-помощник для ежедневной работы бухгалтера с требованиями ФНС.',490,'MONTHLY',3,'LINK','https://t.me/',
    ARRAY['ФНС','Telegram','ИИ'],ARRAY['Налоговые требования','Автоматизация'],ARRAY['Бухгалтеры','Налоговые консультанты'],TRUE,TRUE,NOW()-INTERVAL '2 days')
  ON CONFLICT(slug) DO UPDATE SET author_user_id=EXCLUDED.author_user_id,status='PUBLISHED',title=EXCLUDED.title,short_description=EXCLUDED.short_description,description=EXCLUDED.description,price=EXCLUDED.price,pricing_type=EXCLUDED.pricing_type,trial_days=3,tags=EXCLUDED.tags,topics=EXCLUDED.topics,audiences=EXCLUDED.audiences,is_featured=TRUE,published_at=COALESCE(profimarket_solutions.published_at,NOW())
  RETURNING id INTO ai_id;
  DELETE FROM profimarket_ai_features WHERE solution_id=ai_id;
  DELETE FROM profimarket_solution_platforms WHERE solution_id=ai_id;
  INSERT INTO profimarket_ai_features(solution_id,icon,title,description,sort_order) VALUES
  (ai_id,'scan','Анализирует требования ФНС','Определяет тип требования, сроки ответа и возможные риски.',10),
  (ai_id,'language','Объясняет простым языком','Переводит сложные формулировки ФНС на понятный бухгалтеру язык.',20),
  (ai_id,'list','Предлагает план ответа','Формирует структуру ответа и список необходимых документов.',30),
  (ai_id,'clock','Экономит время','Помогает быстрее разобрать требование и подготовить ответ.',40),
  (ai_id,'telegram','Всегда под рукой','Работает через Telegram.',50);
  INSERT INTO profimarket_solution_platforms(solution_id,platform_id) SELECT ai_id,id FROM profimarket_platforms WHERE code='telegram' ON CONFLICT DO NOTHING;
END $seed$;
