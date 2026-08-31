CREATE TABLE IF NOT EXISTS help_topics (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL UNIQUE,
    category VARCHAR(160) NOT NULL DEFAULT '',
    icon VARCHAR(500) NOT NULL DEFAULT '',
    short_description TEXT NOT NULL DEFAULT '',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS resume_help_topics (
    resume_id BIGINT NOT NULL REFERENCES resumes(id) ON DELETE CASCADE,
    topic_id BIGINT NOT NULL REFERENCES help_topics(id) ON DELETE RESTRICT,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(resume_id, topic_id)
);

CREATE TABLE IF NOT EXISTS help_requests (
    id BIGSERIAL PRIMARY KEY,
    requester_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expert_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    topic_id BIGINT NOT NULL REFERENCES help_topics(id) ON DELETE RESTRICT,
    request_text TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'new' CHECK(status IN ('new','accepted','declined','completed','cancelled')),
    accepted_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK(requester_id <> expert_id)
);

CREATE TABLE IF NOT EXISTS help_request_messages (
    id BIGSERIAL PRIMARY KEY,
    help_request_id BIGINT NOT NULL REFERENCES help_requests(id) ON DELETE CASCADE,
    author_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    text TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS help_reviews (
    id BIGSERIAL PRIMARY KEY,
    author_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    recipient_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    help_request_id BIGINT NOT NULL REFERENCES help_requests(id) ON DELETE CASCADE UNIQUE,
    rating INTEGER NOT NULL CHECK(rating BETWEEN 1 AND 5),
    text TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK(author_id <> recipient_id)
);

CREATE INDEX IF NOT EXISTS help_topics_catalog_idx ON help_topics(is_active, sort_order, id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS resume_help_topics_resume_idx ON resume_help_topics(resume_id, sort_order);
CREATE INDEX IF NOT EXISTS help_requests_expert_idx ON help_requests(expert_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS help_requests_requester_idx ON help_requests(requester_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS help_request_messages_request_idx ON help_request_messages(help_request_id, created_at);
CREATE INDEX IF NOT EXISTS help_reviews_recipient_idx ON help_reviews(recipient_id, created_at DESC);

INSERT INTO help_topics(name, category, icon, short_description, sort_order) VALUES
('ВЭД', 'Налоги и учет', '🌐', 'Валютный контроль, импорт, экспорт и документы по внешнеэкономической деятельности', 10),
('ТСЖ и управляющие компании', 'Отрасли', '🏢', 'Учет, начисления и отчетность для ТСЖ и управляющих компаний', 20),
('Яндекс Такси', 'Платформы', '🚕', 'Разбор учета, актов и расчетов по Яндекс Такси', 30),
('Маркетплейсы', 'Платформы', '🛒', 'Общие вопросы учета и документов по маркетплейсам', 40),
('Ozon', 'Маркетплейсы', '🟦', 'Комиссии, отчеты, возвраты и учет продаж на Ozon', 50),
('Wildberries', 'Маркетплейсы', '🟣', 'Отчеты, удержания, возвраты и документы Wildberries', 60),
('Яндекс Маркет', 'Маркетплейсы', '🟨', 'Учет продаж, актов и закрывающих документов Яндекс Маркета', 70),
('Восстановление учета', 'Учет', '🧩', 'Помощь с восстановлением первички, регистров и отчетности', 80),
('Налоговые требования', 'Налоги', '📩', 'Ответы на требования ФНС и подготовка пояснений', 90),
('Налоговые проверки', 'Налоги', '🔎', 'Камеральные и выездные проверки, сбор документов и риски', 100),
('Валютный контроль', 'ВЭД', '💱', 'Контракты, справки, документы банка и сроки валютного контроля', 110),
('Импорт', 'ВЭД', '📦', 'Документы, пошлины, НДС и учет импортных операций', 120),
('Экспорт', 'ВЭД', '🚢', 'Подтверждение нулевой ставки, документы и учет экспорта', 130),
('Кадровый учет', 'Персонал', '👥', 'Кадры, зарплата, отпуска, больничные и кадровые документы', 140),
('Управленческий учет', 'Управление', '📊', 'Отчеты, бюджеты, управленческая аналитика и контроль', 150),
('Автоматизация учета', 'Инструменты', '⚙️', 'Настройка процессов, обменов, регламентов и учетных систем', 160),
('1С', 'Инструменты', '🧮', 'Работа в 1С, настройки, обмены и типовые ошибки', 170),
('СБИС', 'Инструменты', '📄', 'Отчетность, ЭДО и рабочие процессы в СБИС', 180),
('Контур', 'Инструменты', '✅', 'Контур.Экстерн, Диадок и смежные сервисы', 190),
('Самозанятые', 'Налоги', '🧾', 'НПД, договоры, чеки, ограничения и учет выплат', 200)
ON CONFLICT(name) DO UPDATE SET
    category = EXCLUDED.category,
    icon = CASE WHEN BTRIM(help_topics.icon) = '' THEN EXCLUDED.icon ELSE help_topics.icon END,
    short_description = CASE WHEN BTRIM(help_topics.short_description) = '' THEN EXCLUDED.short_description ELSE help_topics.short_description END,
    sort_order = CASE WHEN help_topics.sort_order = 0 THEN EXCLUDED.sort_order ELSE help_topics.sort_order END,
    updated_at = NOW();
