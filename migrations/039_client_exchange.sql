CREATE TABLE IF NOT EXISTS client_exchange_dictionary_items (
    id BIGSERIAL PRIMARY KEY,
    kind VARCHAR(40) NOT NULL,
    code VARCHAR(100) NOT NULL,
    name VARCHAR(300) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    min_value NUMERIC(18,2),
    max_value NUMERIC(18,2),
    color VARCHAR(30) NOT NULL DEFAULT 'blue',
    icon VARCHAR(500) NOT NULL DEFAULT '',
    legal_name VARCHAR(300) NOT NULL DEFAULT '',
    operator_code VARCHAR(100) NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL DEFAULT 0,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(kind, code)
);
ALTER TABLE client_exchange_dictionary_items ADD COLUMN IF NOT EXISTS icon VARCHAR(500) NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS client_exchange_dictionary_kind_idx ON client_exchange_dictionary_items(kind, active, sort_order) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS client_exchange_listings (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    seller_user_id BIGINT NOT NULL REFERENCES users(id),
    company_id BIGINT,
    selected_buyer_user_id BIGINT REFERENCES users(id),
    title VARCHAR(240) NOT NULL DEFAULT '',
    client_inn VARCHAR(12) NOT NULL DEFAULT '',
    client_legal_name VARCHAR(500) NOT NULL DEFAULT '',
    industry_id BIGINT REFERENCES client_exchange_dictionary_items(id),
    employee_range_id BIGINT REFERENCES client_exchange_dictionary_items(id),
    tax_system_id BIGINT REFERENCES client_exchange_dictionary_items(id),
    revenue_range_id BIGINT REFERENCES client_exchange_dictionary_items(id),
    accounting_state_id BIGINT REFERENCES client_exchange_dictionary_items(id),
    transfer_reason_id BIGINT REFERENCES client_exchange_dictionary_items(id),
    transfer_type_id BIGINT REFERENCES client_exchange_dictionary_items(id),
    transfer_reason_comment TEXT NOT NULL DEFAULT '',
    transfer_price NUMERIC(18,2),
    monthly_commission_percent NUMERIC(5,2),
    commission_months INTEGER,
    current_monthly_fee NUMERIC(18,2),
    revenue_from NUMERIC(18,2),
    revenue_to NUMERIC(18,2),
    operations_per_month INTEGER,
    banks_count INTEGER,
    has_vat BOOLEAN NOT NULL DEFAULT FALSE,
    foreign_trade BOOLEAN NOT NULL DEFAULT FALSE,
    bargain_allowed BOOLEAN NOT NULL DEFAULT FALSE,
    region VARCHAR(200) NOT NULL DEFAULT '',
    city VARCHAR(200) NOT NULL DEFAULT '',
    client_since DATE,
    desired_transfer_date DATE,
    comment TEXT NOT NULL DEFAULT '',
    status VARCHAR(40) NOT NULL DEFAULT 'draft',
    match_percent INTEGER,
    views_count INTEGER NOT NULL DEFAULT 0,
    current_step INTEGER NOT NULL DEFAULT 1,
    published_at TIMESTAMPTZ,
    transferred_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (status IN ('draft','active','has_responses','buyer_selected','transfer_in_progress','transferred','archived','cancelled')),
    CHECK (transfer_price IS NULL OR transfer_price >= 0),
    CHECK (current_monthly_fee IS NULL OR current_monthly_fee >= 0),
    CHECK (monthly_commission_percent IS NULL OR monthly_commission_percent BETWEEN 0 AND 100),
    CHECK (match_percent IS NULL OR match_percent BETWEEN 0 AND 100),
    CHECK (operations_per_month IS NULL OR operations_per_month >= 0),
    CHECK (banks_count IS NULL OR banks_count >= 0)
);
CREATE INDEX IF NOT EXISTS client_exchange_listing_catalog_idx ON client_exchange_listings(status, published_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS client_exchange_listing_industry_idx ON client_exchange_listings(industry_id);
CREATE INDEX IF NOT EXISTS client_exchange_listing_tax_idx ON client_exchange_listings(tax_system_id);
CREATE INDEX IF NOT EXISTS client_exchange_listing_region_idx ON client_exchange_listings(region);
CREATE INDEX IF NOT EXISTS client_exchange_listing_employees_idx ON client_exchange_listings(employee_range_id);
CREATE INDEX IF NOT EXISTS client_exchange_listing_transfer_idx ON client_exchange_listings(transfer_type_id, transfer_price);

CREATE TABLE IF NOT EXISTS client_exchange_listing_options (
    listing_id BIGINT NOT NULL REFERENCES client_exchange_listings(id) ON DELETE CASCADE,
    item_id BIGINT NOT NULL REFERENCES client_exchange_dictionary_items(id),
    kind VARCHAR(40) NOT NULL,
    PRIMARY KEY(listing_id, item_id)
);
CREATE INDEX IF NOT EXISTS client_exchange_listing_options_filter_idx ON client_exchange_listing_options(kind, item_id, listing_id);

CREATE TABLE IF NOT EXISTS client_exchange_responses (
    id BIGSERIAL PRIMARY KEY,
    listing_id BIGINT NOT NULL REFERENCES client_exchange_listings(id),
    buyer_user_id BIGINT NOT NULL REFERENCES users(id),
    proposed_price NUMERIC(18,2),
    accept_original_price BOOLEAN NOT NULL DEFAULT FALSE,
    ready_to_discuss BOOLEAN NOT NULL DEFAULT FALSE,
    comment TEXT NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (status IN ('pending','accepted','rejected','withdrawn')),
    CHECK (proposed_price IS NULL OR proposed_price >= 0)
);
CREATE UNIQUE INDEX IF NOT EXISTS client_exchange_one_active_response_idx ON client_exchange_responses(listing_id,buyer_user_id) WHERE status='pending';
CREATE INDEX IF NOT EXISTS client_exchange_response_seller_idx ON client_exchange_responses(listing_id,status,created_at DESC);
CREATE INDEX IF NOT EXISTS client_exchange_response_buyer_idx ON client_exchange_responses(buyer_user_id,created_at DESC);

CREATE TABLE IF NOT EXISTS client_exchange_favorites (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    listing_id BIGINT NOT NULL REFERENCES client_exchange_listings(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(user_id,listing_id)
);
CREATE TABLE IF NOT EXISTS client_exchange_views (
    listing_id BIGINT NOT NULL REFERENCES client_exchange_listings(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    viewed_on DATE NOT NULL DEFAULT CURRENT_DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(listing_id,user_id,viewed_on)
);
CREATE TABLE IF NOT EXISTS client_exchange_notifications (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type VARCHAR(60) NOT NULL,
    title VARCHAR(240) NOT NULL,
    message TEXT NOT NULL DEFAULT '',
    listing_id BIGINT REFERENCES client_exchange_listings(id) ON DELETE CASCADE,
    response_id BIGINT REFERENCES client_exchange_responses(id) ON DELETE CASCADE,
    read_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS client_exchange_notifications_user_idx ON client_exchange_notifications(user_id,read_at,created_at DESC);

INSERT INTO client_exchange_dictionary_items(kind,code,name,min_value,max_value,sort_order) VALUES
('employee_range','none','Нет сотрудников',0,0,1),('employee_range','1_2','1–2',1,2,2),('employee_range','3_5','3–5',3,5,3),('employee_range','6_10','6–10',6,10,4),('employee_range','11_20','11–20',11,20,5),('employee_range','21_50','21–50',21,50,6),('employee_range','51_100','51–100',51,100,7),('employee_range','101_250','101–250',101,250,8),('employee_range','251_500','251–500',251,500,9),('employee_range','over_500','Более 500',501,NULL,10)
ON CONFLICT(kind,code) DO NOTHING;

INSERT INTO client_exchange_dictionary_items(kind,code,name,sort_order) VALUES
('industry','online_stores','Интернет-магазины',1),('industry','retail','Розничная торговля',2),('industry','wholesale','Оптовая торговля',3),('industry','wholesale_retail','Оптово-розничная торговля',4),('industry','marketplaces','Маркетплейсы',5),('industry','manufacturing','Производство',6),('industry','construction','Строительство',7),('industry','repair','Ремонт и отделочные работы',8),('industry','real_estate','Недвижимость',9),('industry','management_companies','Управляющие компании',10),('industry','utilities','ЖКХ',11),('industry','transport','Транспорт',12),('industry','cargo','Грузоперевозки',13),('industry','logistics','Логистика и складские услуги',14),('industry','courier','Курьерские услуги',15),('industry','car_service','Автосервисы',16),('industry','car_dealers','Автосалоны',17),('industry','it','IT-компании',18),('industry','software','Разработка программного обеспечения',19),('industry','saas','Интернет-сервисы / SaaS',20),('industry','marketing','Маркетинг и реклама',21),('industry','consulting','Консалтинг',22),('industry','legal','Юридические услуги',23),('industry','education','Образовательные услуги',24),('industry','online_schools','Онлайн-школы',25),('industry','medical','Медицинские услуги',26),('industry','dentistry','Стоматологии',27),('industry','beauty','Салоны красоты',28),('industry','fitness','Фитнес и спорт',29),('industry','restaurants','Рестораны и кафе',30),('industry','catering','Общественное питание',31),('industry','hotels','Гостиницы и размещение',32),('industry','tourism','Туризм',33),('industry','agriculture','Сельское хозяйство',34),('industry','food','Пищевая промышленность',35),('industry','light_industry','Лёгкая промышленность',36),('industry','import','Импорт',37),('industry','export','Экспорт',38),('industry','foreign_trade','ВЭД',39),('industry','finance','Финансовые услуги',40),('industry','insurance','Страхование',41),('industry','rental','Аренда имущества',42),('industry','cleaning','Клининговые услуги',43),('industry','security','Охранные услуги',44),('industry','hr','Подбор персонала / HR',45),('industry','creative','Фото / видео / креативные услуги',46),('industry','bloggers','Блогеры / инфобизнес',47),('industry','ngo','НКО и фонды',48),('industry','franchise','Франчайзинг',49),('industry','other_services','Прочие услуги',50)
ON CONFLICT(kind,code) DO NOTHING;

INSERT INTO client_exchange_dictionary_items(kind,code,name,sort_order) VALUES
('marketplace','ozon','Ozon',1),('marketplace','wildberries','Wildberries',2),('marketplace','yandex_market','Яндекс Маркет',3),('marketplace','megamarket','Мегамаркет',4),('marketplace','lamoda','Lamoda',5),('marketplace','aliexpress','AliExpress',6),('marketplace','avito','Авито',7),('marketplace','other','Другой',8),
('tax_system','osno','ОСНО',1),('tax_system','usn_income','УСН Доходы',2),('tax_system','usn_profit','УСН Доходы минус расходы',3),('tax_system','ausn_income','АУСН Доходы',4),('tax_system','ausn_profit','АУСН Доходы минус расходы',5),('tax_system','psn','ПСН',6),('tax_system','eshn','ЕСХН',7),('tax_system','npd','НПД',8),('tax_system','other','Другое',9),
('transfer_type','fixed','Фиксированная цена',1),('transfer_type','negotiable','По договорённости',2),('transfer_type','monthly_commission','Ежемесячная комиссия',3),('transfer_type','term_commission','Комиссия на определённый срок',4),('transfer_type','exchange','Только обмен',5),('transfer_type','free','Бесплатная передача',6),
('revenue_range','under_1m','до 1 млн ₽',1),('revenue_range','1_5m','1–5 млн ₽',2),('revenue_range','5_10m','5–10 млн ₽',3),('revenue_range','10_25m','10–25 млн ₽',4),('revenue_range','25_50m','25–50 млн ₽',5),('revenue_range','50_100m','50–100 млн ₽',6),('revenue_range','100_250m','100–250 млн ₽',7),('revenue_range','250_500m','250–500 млн ₽',8),('revenue_range','500m_1b','500 млн – 1 млрд ₽',9),('revenue_range','over_1b','более 1 млрд ₽',10),
('accounting_program','1c_accounting','1С:Бухгалтерия',1),('accounting_program','1c_unf','1С:УНФ',2),('accounting_program','1c_complex','1С:Комплексная автоматизация',3),('accounting_program','kontur','Контур.Бухгалтерия',4),('accounting_program','saby','СБИС / Saby',5),('accounting_program','moedelo','Моё дело',6),('accounting_program','buhsoft','БухСофт',7),('accounting_program','other','Другая программа',8)
ON CONFLICT(kind,code) DO NOTHING;

INSERT INTO client_exchange_dictionary_items(kind,code,name,color,sort_order) VALUES
('accounting_state','excellent','Отличное','green',1),('accounting_state','good','Хорошее','green',2),('accounting_state','attention','Требует внимания','yellow',3),('accounting_state','problems','Есть отдельные проблемы','orange',4),('accounting_state','partial_restore','Требуется частичное восстановление','orange',5),('accounting_state','full_restore','Требуется полное восстановление','red',6),('accounting_state','unknown','Состояние неизвестно','gray',7)
ON CONFLICT(kind,code) DO NOTHING;

INSERT INTO client_exchange_dictionary_items(kind,code,name,sort_order) VALUES
('transfer_reason','too_small','Клиент слишком маленький для нашей компании',1),('transfer_reason','too_large','Клиент слишком крупный для нашей компании',2),('transfer_reason','not_specialization','Не наша специализация',3),('transfer_reason','other_region','Клиент находится в другом регионе',4),('transfer_reason','staff_shortage','Не хватает сотрудников для обслуживания',5),('transfer_reason','optimize_base','Оптимизируем клиентскую базу',6),('transfer_reason','other_service_level','Клиенту требуется другой уровень сервиса',7),('transfer_reason','close_direction','Закрываем данное направление',8),('transfer_reason','high_workload','Высокая трудоёмкость обслуживания',9),('transfer_reason','client_searches','Клиент самостоятельно ищет новую бухгалтерскую компанию',10),('transfer_reason','specialization_change','Изменение специализации нашей компании',11),('transfer_reason','reduce_clients','Сокращаем количество клиентов',12),('transfer_reason','portfolio_part','Передаём часть клиентского портфеля',13),('transfer_reason','temporary_shortage','Временная нехватка ресурсов',14),('transfer_reason','other','Другая причина',15)
ON CONFLICT(kind,code) DO NOTHING;

INSERT INTO client_exchange_dictionary_items(kind,code,name,sort_order) VALUES
('edo_provider','diadoc','Контур.Диадок',1),('edo_provider','saby','СБИС / Saby',2),('edo_provider','taxcom','Такском / Файлер',3),('edo_provider','astral','Астрал.ЭДО',4),('edo_provider','sfera','СФЕРА Курьер / СберКорус',5),('edo_provider','edo_light','ЭДО Лайт',6),('edo_provider','1c_edo','1С-ЭДО',7),('edo_provider','potok','ЭДО Поток',8),('edo_provider','stek','СТЭК',9),('edo_provider','express','Электронный Экспресс',10),('edo_provider','komita','Комита',11),('edo_provider','edisoft','Edisoft / Эдивеб',12),('edo_provider','synerdocs','Synerdocs',13),('edo_provider','directum','Directum',14),('edo_provider','other','Другой оператор ЭДО',15),('edo_provider','none','ЭДО не используется',16)
ON CONFLICT(kind,code) DO NOTHING;
