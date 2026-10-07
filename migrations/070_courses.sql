-- Course marketing pages; lesson content and enrolments are separate future domains.
INSERT INTO dictionaries(name,alias) SELECT 'Категории курсов','course_categories'
WHERE NOT EXISTS(SELECT 1 FROM dictionaries WHERE alias='course_categories');
INSERT INTO dictionary_items(dictionary_id,value,sort_order)
SELECT d.id,v.name,v.ord FROM dictionaries d CROSS JOIN (VALUES
 ('ИИ и автоматизация',0),('Бухгалтерский учёт',1),('Налоги и отчётность',2),
 ('Зарплата и кадры',3),('1С',4),('ВЭД',5),('Управление',6),('Excel',7),('Другое',8)
) v(name,ord) WHERE d.alias='course_categories'
AND NOT EXISTS(SELECT 1 FROM dictionary_items i WHERE i.dictionary_id=d.id);

CREATE TABLE IF NOT EXISTS courses (
 id BIGSERIAL PRIMARY KEY, owner_id BIGINT NOT NULL REFERENCES users(id),
 slug VARCHAR(240) NOT NULL UNIQUE, title VARCHAR(180) NOT NULL DEFAULT '',
 description VARCHAR(1200) NOT NULL DEFAULT '', category_id BIGINT REFERENCES dictionary_items(id),
 level VARCHAR(20) NOT NULL DEFAULT 'beginner' CHECK(level IN ('beginner','intermediate','advanced')),
 cover_image TEXT NOT NULL DEFAULT '', promo_url TEXT NOT NULL DEFAULT '',
 price BIGINT NOT NULL DEFAULT 0 CHECK(price>=0), old_price BIGINT CHECK(old_price>0),
 hours NUMERIC(8,2) NOT NULL DEFAULT 0 CHECK(hours>=0), access_months INTEGER NOT NULL DEFAULT 12 CHECK(access_months>0),
 certificate BOOLEAN NOT NULL DEFAULT FALSE, chat BOOLEAN NOT NULL DEFAULT FALSE,
 updates BOOLEAN NOT NULL DEFAULT TRUE, any_device BOOLEAN NOT NULL DEFAULT TRUE,
 corporate BOOLEAN NOT NULL DEFAULT FALSE,
 status VARCHAR(20) NOT NULL DEFAULT 'draft' CHECK(status IN ('draft','moderation','published','hidden')),
 revision BIGINT NOT NULL DEFAULT 1, moderation_note TEXT NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), published_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS courses_owner_idx ON courses(owner_id,updated_at DESC,id);
CREATE INDEX IF NOT EXISTS courses_catalog_idx ON courses(category_id,published_at DESC,id) WHERE status='published';
CREATE TABLE IF NOT EXISTS course_instructors (
 course_id BIGINT NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
 user_id BIGINT NOT NULL REFERENCES users(id), sort_order INTEGER NOT NULL DEFAULT 0,
 PRIMARY KEY(course_id,user_id)
);
CREATE TABLE IF NOT EXISTS course_benefits (
 id BIGSERIAL PRIMARY KEY, course_id BIGINT NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
 icon VARCHAR(40) NOT NULL DEFAULT 'sparkles',title VARCHAR(120) NOT NULL DEFAULT '',description VARCHAR(500) NOT NULL DEFAULT '',sort_order INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS course_bonuses (
 id BIGSERIAL PRIMARY KEY,course_id BIGINT NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
 title VARCHAR(180) NOT NULL DEFAULT '',sort_order INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS course_modules (
 id BIGSERIAL PRIMARY KEY,course_id BIGINT NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
 title VARCHAR(180) NOT NULL DEFAULT '',description VARCHAR(1200) NOT NULL DEFAULT '',sort_order INTEGER NOT NULL DEFAULT 0,
 UNIQUE(id,course_id)
);
CREATE TABLE IF NOT EXISTS course_lessons (
 id BIGSERIAL PRIMARY KEY,module_id BIGINT NOT NULL REFERENCES course_modules(id) ON DELETE CASCADE,
 title VARCHAR(180) NOT NULL DEFAULT '',description VARCHAR(1200) NOT NULL DEFAULT '',
 duration_minutes INTEGER NOT NULL DEFAULT 0 CHECK(duration_minutes>=0),
 type VARCHAR(20) NOT NULL DEFAULT 'video' CHECK(type IN ('video','text','practice','test','materials')),
 preview_enabled BOOLEAN NOT NULL DEFAULT FALSE,sort_order INTEGER NOT NULL DEFAULT 0,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS course_faq (
 id BIGSERIAL PRIMARY KEY,course_id BIGINT NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
 question VARCHAR(300) NOT NULL DEFAULT '',answer VARCHAR(3000) NOT NULL DEFAULT '',sort_order INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS course_reviews (
 id BIGSERIAL PRIMARY KEY,course_id BIGINT NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
 user_id BIGINT NOT NULL REFERENCES users(id),rating SMALLINT NOT NULL CHECK(rating BETWEEN 1 AND 5),
 body TEXT NOT NULL, status VARCHAR(20) NOT NULL DEFAULT 'moderation' CHECK(status IN ('moderation','published','hidden')),
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),UNIQUE(course_id,user_id)
);
CREATE TABLE IF NOT EXISTS course_favorites (
 course_id BIGINT NOT NULL REFERENCES courses(id) ON DELETE CASCADE,user_id BIGINT NOT NULL REFERENCES users(id),
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),PRIMARY KEY(course_id,user_id)
);
CREATE INDEX IF NOT EXISTS course_modules_order_idx ON course_modules(course_id,sort_order,id);
CREATE INDEX IF NOT EXISTS course_benefits_order_idx ON course_benefits(course_id,sort_order,id);
CREATE INDEX IF NOT EXISTS course_bonuses_order_idx ON course_bonuses(course_id,sort_order,id);
CREATE INDEX IF NOT EXISTS course_faq_order_idx ON course_faq(course_id,sort_order,id);
CREATE INDEX IF NOT EXISTS course_lessons_order_idx ON course_lessons(module_id,sort_order,id);
CREATE INDEX IF NOT EXISTS course_reviews_public_idx ON course_reviews(course_id,created_at DESC) WHERE status='published';
