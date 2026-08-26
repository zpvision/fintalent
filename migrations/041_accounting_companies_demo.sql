DO $$
DECLARE
    seed_hash VARCHAR(60);
    owner_one BIGINT;
    owner_two BIGINT;
    owner_three BIGINT;
    owner_four BIGINT;
    company_one BIGINT;
    company_two BIGINT;
    company_three BIGINT;
    company_four BIGINT;
BEGIN
    SELECT password_hash INTO seed_hash FROM users ORDER BY id LIMIT 1;
    IF seed_hash IS NULL THEN RETURN; END IF;

    INSERT INTO users(full_name,email,password_hash,agreed_to_terms)
    VALUES
        ('Анна Волкова','accounting.demo.one@fintalent.local',seed_hash,TRUE),
        ('Сергей Ковалёв','accounting.demo.two@fintalent.local',seed_hash,TRUE),
        ('Елена Сафина','accounting.demo.three@fintalent.local',seed_hash,TRUE),
        ('Мария Северова','accounting.demo.four@fintalent.local',seed_hash,TRUE)
    ON CONFLICT(email) DO NOTHING;

    SELECT id INTO owner_one FROM users WHERE email='accounting.demo.one@fintalent.local';
    SELECT id INTO owner_two FROM users WHERE email='accounting.demo.two@fintalent.local';
    SELECT id INTO owner_three FROM users WHERE email='accounting.demo.three@fintalent.local';
    SELECT id INTO owner_four FROM users WHERE email='accounting.demo.four@fintalent.local';

    INSERT INTO accounting_companies(owner_user_id,name,slug,short_description,full_description,city,address,remote_all_russia,founded_year,employee_count,phone,email,website,telegram,work_hours,manager_name,manager_position,manager_description,accent_style_id,header_image_type,header_template_id,advantages,status,verified,published_at,current_step)
    SELECT owner_one,'БухЭксперт','buhekspert-demo','Бухгалтерия для IT, маркетплейсов и растущего бизнеса','Берём бухгалтерию на себя: от первичных документов и зарплаты до налогового планирования. Работаем понятным языком и всегда остаёмся на связи.','Москва','Пресненская набережная, 8',TRUE,2017,14,'+7 (495) 120-45-67','hello@buhekspert.example','https://buhekspert.example','https://t.me/buhekspert_demo','Пн–Пт, 9:00–18:00','Анна Волкова','Основатель и руководитель','15 лет в бухгалтерии и налоговом консультировании',(SELECT id FROM accounting_company_accent_styles WHERE color_key='green'),'template',(SELECT id FROM accounting_company_header_templates WHERE slug='header-10'),'["Индивидуальный подход","Всегда на связи","Работа с ЭДО","Конфиденциальность"]'::jsonb,'published',TRUE,NOW()-INTERVAL '12 days',5
    WHERE NOT EXISTS(SELECT 1 FROM accounting_companies WHERE slug='buhekspert-demo');

    INSERT INTO accounting_companies(owner_user_id,name,slug,short_description,full_description,city,address,remote_all_russia,founded_year,employee_count,phone,email,website,telegram,work_hours,manager_name,manager_position,manager_description,accent_style_id,header_image_type,header_template_id,advantages,status,verified,published_at,current_step)
    SELECT owner_two,'Баланс Плюс','balans-plyus-demo','Учёт для производства, оптовой торговли и компаний с ВЭД','Настраиваем прозрачный бухгалтерский и управленческий учёт. Помогаем производственным компаниям контролировать налоги, себестоимость и внешнеэкономические операции.','Санкт-Петербург','Московский проспект, 97',TRUE,2012,28,'+7 (812) 320-18-40','office@balance-plus.example','https://balance-plus.example','https://t.me/balance_plus_demo','Пн–Пт, 8:30–18:30','Сергей Ковалёв','Управляющий партнёр','Эксперт по производственному учёту и ВЭД',(SELECT id FROM accounting_company_accent_styles WHERE color_key='blue'),'template',(SELECT id FROM accounting_company_header_templates WHERE slug='header-03'),'["Опыт сложных проектов","Контроль качества","Профессиональная ответственность"]'::jsonb,'published',TRUE,NOW()-INTERVAL '8 days',5
    WHERE NOT EXISTS(SELECT 1 FROM accounting_companies WHERE slug='balans-plyus-demo');

    INSERT INTO accounting_companies(owner_user_id,name,slug,short_description,full_description,city,address,remote_all_russia,founded_year,employee_count,phone,email,website,telegram,work_hours,manager_name,manager_position,manager_description,accent_style_id,header_image_type,header_template_id,advantages,status,verified,published_at,current_step)
    SELECT owner_three,'Точка Учёта','tochka-ucheta-demo','Современная бухгалтерия для малого бизнеса, HoReCa и розницы','Помогаем предпринимателям спокойно развивать бизнес: ведём учёт, считаем зарплату, сдаём отчётность и заранее предупреждаем о налоговых рисках.','Казань','Петербургская улица, 52',TRUE,2019,9,'+7 (843) 210-09-70','team@uchet-point.example','https://uchet-point.example','https://t.me/uchet_point_demo','Пн–Пт, 9:00–19:00','Елена Сафина','Основатель','Специалист по малому бизнесу и автоматизации учёта',(SELECT id FROM accounting_company_accent_styles WHERE color_key='violet'),'template',(SELECT id FROM accounting_company_header_templates WHERE slug='header-17'),'["Отвечаем до 30 минут","Понятные тарифы","Личный бухгалтер"]'::jsonb,'published',FALSE,NOW()-INTERVAL '5 days',5
    WHERE NOT EXISTS(SELECT 1 FROM accounting_companies WHERE slug='tochka-ucheta-demo');

    INSERT INTO accounting_companies(owner_user_id,name,slug,short_description,full_description,city,address,remote_all_russia,founded_year,employee_count,phone,email,website,telegram,work_hours,manager_name,manager_position,manager_description,accent_style_id,header_image_type,header_template_id,advantages,status,verified,published_at,current_step)
    SELECT owner_four,'Северная Бухгалтерия','severnaya-buhgalteriya-demo','Бухгалтерское сопровождение строительства, логистики и сферы услуг','Ведём бухгалтерский, налоговый и кадровый учёт. Берём на сопровождение компании со сложным документооборотом и несколькими направлениями деятельности.','Екатеринбург','улица Бориса Ельцина, 3',TRUE,2015,18,'+7 (343) 290-33-18','info@north-accounting.example','https://north-accounting.example','https://t.me/north_accounting_demo','Пн–Пт, 9:00–18:00','Мария Северова','Генеральный директор','Практикующий налоговый консультант и аудитор',(SELECT id FROM accounting_company_accent_styles WHERE color_key='turquoise'),'template',(SELECT id FROM accounting_company_header_templates WHERE slug='header-22'),'["Сложный учёт","Работа по всей России","Защищённый документооборот"]'::jsonb,'published',TRUE,NOW()-INTERVAL '2 days',5
    WHERE NOT EXISTS(SELECT 1 FROM accounting_companies WHERE slug='severnaya-buhgalteriya-demo');

    SELECT id INTO company_one FROM accounting_companies WHERE slug='buhekspert-demo';
    SELECT id INTO company_two FROM accounting_companies WHERE slug='balans-plyus-demo';
    SELECT id INTO company_three FROM accounting_companies WHERE slug='tochka-ucheta-demo';
    SELECT id INTO company_four FROM accounting_companies WHERE slug='severnaya-buhgalteriya-demo';

    INSERT INTO accounting_company_direction_links(company_id,direction_id,is_key,sort_order)
    SELECT c.company_id,d.id,c.is_key,c.sort_order FROM (VALUES
      (company_one,'it',TRUE,10),(company_one,'marketplaces',TRUE,20),(company_one,'online-stores',FALSE,30),(company_one,'b2b',FALSE,40),
      (company_two,'manufacturing',TRUE,10),(company_two,'trade-ved',TRUE,20),(company_two,'wholesale',FALSE,30),(company_two,'import',FALSE,40),(company_two,'export',FALSE,50),
      (company_three,'horeca',TRUE,10),(company_three,'retail',TRUE,20),(company_three,'self-employed',FALSE,30),(company_three,'beauty',FALSE,40),
      (company_four,'construction',TRUE,10),(company_four,'logistics',TRUE,20),(company_four,'transport',FALSE,30),(company_four,'b2b',FALSE,40)
    ) AS c(company_id,slug,is_key,sort_order) JOIN accounting_company_directions d ON d.slug=c.slug
    ON CONFLICT(company_id,direction_id) DO NOTHING;

    INSERT INTO accounting_company_tax_system_links(company_id,tax_system_id)
    SELECT c.company_id,t.id FROM (VALUES
      (company_one,'usn-income'),(company_one,'usn-profit'),(company_one,'osno'),
      (company_two,'osno'),(company_two,'usn-profit'),
      (company_three,'usn-income'),(company_three,'psn'),
      (company_four,'osno'),(company_four,'usn-profit')
    ) AS c(company_id,slug) JOIN accounting_company_tax_systems t ON t.slug=c.slug
    ON CONFLICT(company_id,tax_system_id) DO NOTHING;

    INSERT INTO accounting_company_services(company_id,service_id,price_from,price_type,sort_order)
    SELECT c.company_id,s.id,c.price,c.price_type,c.sort_order FROM (VALUES
      (company_one,'accounting-support',2900::numeric,'from_month',10),(company_one,'tax-planning',5900,'from_once',20),(company_one,'payroll',1900,'from_month',30),
      (company_two,'accounting-support',8900,'from_month',10),(company_two,'foreign-trade',15000,'from_month',20),(company_two,'management-accounting',12000,'from_month',30),
      (company_three,'accounting-support',3500,'from_month',10),(company_three,'reporting',1800,'from_once',20),(company_three,'registration',3000,'from_once',30),
      (company_four,'accounting-support',6900,'from_month',10),(company_four,'accounting-recovery',15000,'from_once',20),(company_four,'tax-audit',9000,'from_once',30)
    ) AS c(company_id,slug,price,price_type,sort_order) JOIN accounting_company_service_catalog s ON s.slug=c.slug
    WHERE NOT EXISTS(SELECT 1 FROM accounting_company_services x WHERE x.company_id=c.company_id AND x.service_id=s.id);

    INSERT INTO accounting_company_tariffs(company_id,name,subtitle,price,period,benefits,sort_order,popular,active)
    SELECT c.company_id,c.name,c.subtitle,c.price,'в месяц',c.benefits,c.sort_order,c.popular,TRUE FROM (VALUES
      (company_one,'Старт','Для ИП и небольших ООО',2900::numeric,'["УСН","до 10 операций","отчётность","поддержка в чате"]'::jsonb,10,FALSE),
      (company_one,'Оптимум','Для растущего бизнеса',5900,'["УСН / ОСНО","до 30 операций","кадровый учёт","консультации"]'::jsonb,20,TRUE),
      (company_two,'Производство','Учёт полного цикла',18900,'["ОСНО","производственный учёт","зарплата и кадры","контроль себестоимости"]'::jsonb,10,TRUE),
      (company_two,'ВЭД','Для импортёров и экспортёров',24900,'["валютные операции","импорт / экспорт","ЭДО","налоговое сопровождение"]'::jsonb,20,FALSE),
      (company_three,'Предприниматель','Для ИП',3500,'["УСН или патент","до 20 операций","отчётность","чат с бухгалтером"]'::jsonb,10,TRUE),
      (company_three,'Команда','Для бизнеса с сотрудниками',6900,'["бухгалтерский учёт","зарплата","кадровый учёт","консультации"]'::jsonb,20,FALSE),
      (company_four,'Бизнес','Для компаний сферы услуг',6900,'["УСН / ОСНО","до 40 операций","отчётность","личный бухгалтер"]'::jsonb,10,FALSE),
      (company_four,'Профи','Для строительства и логистики',12900,'["сложный документооборот","зарплата и кадры","налоговое планирование","приоритетная поддержка"]'::jsonb,20,TRUE)
    ) AS c(company_id,name,subtitle,price,benefits,sort_order,popular)
    WHERE NOT EXISTS(SELECT 1 FROM accounting_company_tariffs t WHERE t.company_id=c.company_id AND t.name=c.name);
END $$;
