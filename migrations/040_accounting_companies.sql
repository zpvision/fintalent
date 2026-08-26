CREATE TABLE IF NOT EXISTS accounting_company_directions (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(180) NOT NULL,
    slug VARCHAR(180) NOT NULL UNIQUE,
    icon VARCHAR(40) NOT NULL DEFAULT 'briefcase',
    description TEXT NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL DEFAULT 0,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS accounting_company_service_catalog (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(220) NOT NULL,
    slug VARCHAR(180) NOT NULL UNIQUE,
    icon VARCHAR(40) NOT NULL DEFAULT 'calculator',
    category VARCHAR(140) NOT NULL DEFAULT 'Бухгалтерские услуги',
    description TEXT NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL DEFAULT 0,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS accounting_company_header_templates (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(180) NOT NULL,
    slug VARCHAR(180) NOT NULL UNIQUE,
    image_url VARCHAR(500) NOT NULL,
    category VARCHAR(100) NOT NULL DEFAULT 'Финансы',
    sort_order INTEGER NOT NULL DEFAULT 0,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS accounting_company_accent_styles (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(80) NOT NULL,
    color_key VARCHAR(40) NOT NULL UNIQUE,
    color_value VARCHAR(20) NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS accounting_company_tax_systems (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    slug VARCHAR(100) NOT NULL UNIQUE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    active BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE IF NOT EXISTS accounting_companies (
    id BIGSERIAL PRIMARY KEY,
    owner_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    manager_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    name VARCHAR(240) NOT NULL DEFAULT '',
    slug VARCHAR(260) UNIQUE,
    short_description VARCHAR(500) NOT NULL DEFAULT '',
    full_description TEXT NOT NULL DEFAULT '',
    logo VARCHAR(500) NOT NULL DEFAULT '',
    city VARCHAR(180) NOT NULL DEFAULT '',
    address VARCHAR(500) NOT NULL DEFAULT '',
    remote_all_russia BOOLEAN NOT NULL DEFAULT FALSE,
    founded_year INTEGER,
    employee_count INTEGER,
    inn VARCHAR(12) NOT NULL DEFAULT '',
    phone VARCHAR(80) NOT NULL DEFAULT '',
    email VARCHAR(254) NOT NULL DEFAULT '',
    website VARCHAR(500) NOT NULL DEFAULT '',
    telegram VARCHAR(500) NOT NULL DEFAULT '',
    whatsapp VARCHAR(500) NOT NULL DEFAULT '',
    vk VARCHAR(500) NOT NULL DEFAULT '',
    work_hours VARCHAR(180) NOT NULL DEFAULT '',
    manager_name VARCHAR(240) NOT NULL DEFAULT '',
    manager_position VARCHAR(180) NOT NULL DEFAULT '',
    manager_photo VARCHAR(500) NOT NULL DEFAULT '',
    manager_description VARCHAR(700) NOT NULL DEFAULT '',
    accent_style_id BIGINT REFERENCES accounting_company_accent_styles(id) ON DELETE SET NULL,
    header_image_type VARCHAR(20) NOT NULL DEFAULT 'template' CHECK(header_image_type IN ('template','custom')),
    header_template_id BIGINT REFERENCES accounting_company_header_templates(id) ON DELETE SET NULL,
    custom_header_image VARCHAR(500) NOT NULL DEFAULT '',
    advantages JSONB NOT NULL DEFAULT '[]'::jsonb,
    status VARCHAR(20) NOT NULL DEFAULT 'draft' CHECK(status IN ('draft','published','archived')),
    current_step INTEGER NOT NULL DEFAULT 1 CHECK(current_step BETWEEN 1 AND 5),
    verified BOOLEAN NOT NULL DEFAULT FALSE,
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT accounting_companies_founded_year_check CHECK(founded_year IS NULL OR founded_year BETWEEN 1900 AND 2200),
    CONSTRAINT accounting_companies_employee_count_check CHECK(employee_count IS NULL OR employee_count >= 0)
);
CREATE UNIQUE INDEX IF NOT EXISTS accounting_companies_owner_active_uidx ON accounting_companies(owner_user_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS accounting_companies_catalog_idx ON accounting_companies(status,published_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS accounting_companies_city_idx ON accounting_companies(city) WHERE status='published' AND deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS accounting_company_direction_links (
    company_id BIGINT NOT NULL REFERENCES accounting_companies(id) ON DELETE CASCADE,
    direction_id BIGINT NOT NULL REFERENCES accounting_company_directions(id) ON DELETE RESTRICT,
    is_key BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY(company_id,direction_id)
);

CREATE TABLE IF NOT EXISTS accounting_company_tax_system_links (
    company_id BIGINT NOT NULL REFERENCES accounting_companies(id) ON DELETE CASCADE,
    tax_system_id BIGINT NOT NULL REFERENCES accounting_company_tax_systems(id) ON DELETE RESTRICT,
    PRIMARY KEY(company_id,tax_system_id)
);

CREATE TABLE IF NOT EXISTS accounting_company_services (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES accounting_companies(id) ON DELETE CASCADE,
    service_id BIGINT REFERENCES accounting_company_service_catalog(id) ON DELETE RESTRICT,
    custom_name VARCHAR(220) NOT NULL DEFAULT '',
    price_from NUMERIC(14,2),
    price_type VARCHAR(30) NOT NULL DEFAULT 'from_month' CHECK(price_type IN ('from_month','month','from_hour','hour','from_once','request')),
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT accounting_company_service_price_check CHECK(price_from IS NULL OR price_from >= 0),
    CONSTRAINT accounting_company_service_name_check CHECK(service_id IS NOT NULL OR length(trim(custom_name)) > 0)
);
CREATE INDEX IF NOT EXISTS accounting_company_services_company_idx ON accounting_company_services(company_id,sort_order);

CREATE TABLE IF NOT EXISTS accounting_company_tariffs (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES accounting_companies(id) ON DELETE CASCADE,
    name VARCHAR(160) NOT NULL,
    subtitle VARCHAR(300) NOT NULL DEFAULT '',
    price NUMERIC(14,2),
    period VARCHAR(80) NOT NULL DEFAULT 'в месяц',
    benefits JSONB NOT NULL DEFAULT '[]'::jsonb,
    sort_order INTEGER NOT NULL DEFAULT 0,
    popular BOOLEAN NOT NULL DEFAULT FALSE,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT accounting_company_tariff_price_check CHECK(price IS NULL OR price >= 0)
);
CREATE INDEX IF NOT EXISTS accounting_company_tariffs_company_idx ON accounting_company_tariffs(company_id,sort_order);

CREATE TABLE IF NOT EXISTS accounting_company_team (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES accounting_companies(id) ON DELETE CASCADE,
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    test_employee_id BIGINT,
    full_name VARCHAR(240) NOT NULL,
    email VARCHAR(254) NOT NULL DEFAULT '',
    position VARCHAR(180) NOT NULL DEFAULT '',
    photo VARCHAR(500) NOT NULL DEFAULT '',
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS accounting_company_team_company_idx ON accounting_company_team(company_id,active);

CREATE TABLE IF NOT EXISTS accounting_company_reviews (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES accounting_companies(id) ON DELETE CASCADE,
    author_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    author_name VARCHAR(180) NOT NULL,
    author_company VARCHAR(220) NOT NULL DEFAULT '',
    text TEXT NOT NULL,
    rating INTEGER NOT NULL CHECK(rating BETWEEN 1 AND 5),
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','published','rejected')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS accounting_company_reviews_public_idx ON accounting_company_reviews(company_id,created_at DESC) WHERE status='published';

CREATE TABLE IF NOT EXISTS accounting_company_competency_scores (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES accounting_companies(id) ON DELETE CASCADE,
    competency_name VARCHAR(240) NOT NULL,
    average_percent NUMERIC(5,2) NOT NULL CHECK(average_percent BETWEEN 0 AND 100),
    specialists_count INTEGER NOT NULL DEFAULT 0,
    tests_count INTEGER NOT NULL DEFAULT 0,
    last_confirmed_at TIMESTAMPTZ,
    calculated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(company_id,competency_name)
);

INSERT INTO accounting_company_accent_styles(name,color_key,color_value,sort_order) VALUES
('Зелёный','green','#16a36a',10),('Синий','blue','#3478f6',20),('Фиолетовый','violet','#7657d6',30),('Бирюзовый','turquoise','#109c9a',40),('Оранжевый','orange','#e78324',50)
ON CONFLICT(color_key) DO UPDATE SET name=EXCLUDED.name,color_value=EXCLUDED.color_value,sort_order=EXCLUDED.sort_order;

INSERT INTO accounting_company_directions(name,slug,icon,sort_order) VALUES
('IT-сфера','it','laptop',10),('Услуги B2B','b2b','briefcase',20),('Торговля с ВЭД','trade-ved','globe',30),('Оптово-розничная торговля','wholesale-retail','store',40),
('Производство','manufacturing','factory',50),('Инфобизнес','infobusiness','video',60),('Wildberries','wildberries','basket',70),('OZON','ozon','package',80),
('Строительство','construction','building',90),('Медицина','medicine','heart',100),('HoReCa','horeca','coffee',110),('Логистика','logistics','truck',120),
('Транспорт','transport','car',130),('Образование','education','book',140),('Консалтинг','consulting','dialog',150),('Агентства','agencies','users',160),
('Недвижимость','real-estate','home',170),('Салоны красоты','beauty','sparkles',180),('Автосервисы','car-service','wrench',190),('Розничная торговля','retail','cart',200),
('Оптовая торговля','wholesale','boxes',210),('Импорт','import','arrow-in',220),('Экспорт','export','arrow-out',230),('Самозанятые','self-employed','person',240),
('НКО','nonprofit','hands',250),('Маркетплейсы','marketplaces','market',260),('Онлайн-магазины','online-stores','monitor',270),('Производственные компании','production-companies','gears',280)
ON CONFLICT(slug) DO UPDATE SET name=EXCLUDED.name,icon=EXCLUDED.icon,sort_order=EXCLUDED.sort_order;

INSERT INTO accounting_company_service_catalog(name,slug,icon,category,sort_order) VALUES
('Бухгалтерское сопровождение','accounting-support','calculator','Бухгалтерский учёт',10),
('Налоговое планирование','tax-planning','chart','Налоги',20),
('Кадровый учёт и расчёт зарплаты','payroll','users','Кадры',30),
('Подготовка и сдача отчётности','reporting','document','Отчётность',40),
('Консультации по учёту и налогам','consulting','dialog','Консультации',50),
('Разовые услуги','one-time','sparkles','Разовые услуги',60),
('Восстановление бухгалтерского учёта','accounting-recovery','restore','Бухгалтерский учёт',70),
('Регистрация ИП и ООО','registration','building','Регистрация бизнеса',80),
('Ведение ВЭД','foreign-trade','globe','ВЭД',90),
('Сопровождение маркетплейсов','marketplace-accounting','market','Маркетплейсы',100),
('Управленческий учёт','management-accounting','dashboard','Финансы',110),
('Финансовый директор на аутсорсе','outsourced-cfo','briefcase','Финансы',120),
('Налоговые проверки и требования','tax-audit','shield','Налоги',130),
('Миграция и настройка 1С','one-c','gears','Автоматизация',140),
('Электронный документооборот','edo','signature','Автоматизация',150)
ON CONFLICT(slug) DO UPDATE SET name=EXCLUDED.name,icon=EXCLUDED.icon,category=EXCLUDED.category,sort_order=EXCLUDED.sort_order;

INSERT INTO accounting_company_tax_systems(name,slug,sort_order) VALUES
('ОСНО','osno',10),('УСН Доходы','usn-income',20),('УСН Доходы минус расходы','usn-profit',30),('АУСН','ausn',40),('ПСН','psn',50),('ЕСХН','eshn',60),('НПД','npd',70)
ON CONFLICT(slug) DO UPDATE SET name=EXCLUDED.name,sort_order=EXCLUDED.sort_order;

INSERT INTO accounting_company_header_templates(name,slug,image_url,category,sort_order)
SELECT 'Обложка '||n, 'header-'||lpad(n::text,2,'0'), '/static/accounting-company-headers/header-'||lpad(n::text,2,'0')||'.jpg',
       CASE WHEN n<=6 THEN 'Рабочий стол' WHEN n<=12 THEN 'Документы' WHEN n<=18 THEN 'Цифровые финансы' ELSE 'Минимализм' END, n*10
FROM generate_series(1,24) n
ON CONFLICT(slug) DO UPDATE SET image_url=EXCLUDED.image_url,category=EXCLUDED.category,sort_order=EXCLUDED.sort_order;
