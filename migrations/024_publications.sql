-- Professional publications module. PostgreSQL 14+.
CREATE TABLE IF NOT EXISTS publication_categories (
 id BIGSERIAL PRIMARY KEY, name VARCHAR(160) NOT NULL UNIQUE, slug VARCHAR(160) NOT NULL UNIQUE,
 description TEXT NOT NULL DEFAULT '', sort_order INTEGER NOT NULL DEFAULT 0, is_active BOOLEAN NOT NULL DEFAULT TRUE
);
CREATE TABLE IF NOT EXISTS publication_topics (
 id BIGSERIAL PRIMARY KEY, category_id BIGINT REFERENCES publication_categories(id) ON DELETE SET NULL,
 name VARCHAR(160) NOT NULL, slug VARCHAR(160) NOT NULL UNIQUE, is_active BOOLEAN NOT NULL DEFAULT TRUE
);
CREATE TABLE IF NOT EXISTS publication_tags (
 id BIGSERIAL PRIMARY KEY, name VARCHAR(100) NOT NULL UNIQUE, slug VARCHAR(100) NOT NULL UNIQUE, usage_count BIGINT NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS publication_series (
 id BIGSERIAL PRIMARY KEY, author_id BIGINT NOT NULL REFERENCES users(id), title VARCHAR(240) NOT NULL,
 slug VARCHAR(180) NOT NULL UNIQUE, description TEXT NOT NULL DEFAULT '', cover_image VARCHAR(500) NOT NULL DEFAULT '',
 status VARCHAR(20) NOT NULL DEFAULT 'draft' CHECK(status IN('draft','published','archived')),
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS publications (
 id BIGSERIAL PRIMARY KEY, author_id BIGINT NOT NULL REFERENCES users(id), category_id BIGINT REFERENCES publication_categories(id),
 title VARCHAR(240) NOT NULL, subtitle VARCHAR(300) NOT NULL DEFAULT '', excerpt VARCHAR(800) NOT NULL DEFAULT '',
 cover_image VARCHAR(500) NOT NULL DEFAULT '', content_json JSONB NOT NULL DEFAULT '[]'::jsonb, content_html TEXT NOT NULL DEFAULT '',
 summary_points JSONB NOT NULL DEFAULT '[]'::jsonb, slug VARCHAR(180) NOT NULL UNIQUE,
 seo_title VARCHAR(240) NOT NULL DEFAULT '', seo_description VARCHAR(320) NOT NULL DEFAULT '',
 status VARCHAR(20) NOT NULL DEFAULT 'draft' CHECK(status IN('draft','review','published','rejected','hidden','blocked','archived')),
 visibility VARCHAR(20) NOT NULL DEFAULT 'draft' CHECK(visibility IN('public','unlisted','draft')),
 moderation_status VARCHAR(20) NOT NULL DEFAULT 'draft' CHECK(moderation_status IN('draft','review','published','rejected','hidden','blocked','archived')),
 moderation_reason TEXT NOT NULL DEFAULT '', relevance_status VARCHAR(24) NOT NULL DEFAULT 'current' CHECK(relevance_status IN('current','review_required','outdated','archived')),
 difficulty VARCHAR(20) NOT NULL DEFAULT 'medium' CHECK(difficulty IN('beginner','medium','advanced','expert')),
 language VARCHAR(10) NOT NULL DEFAULT 'ru', reading_time INTEGER NOT NULL DEFAULT 1 CHECK(reading_time BETWEEN 1 AND 300),
 allow_comments BOOLEAN NOT NULL DEFAULT TRUE, is_recommended BOOLEAN NOT NULL DEFAULT FALSE, editor_mark BOOLEAN NOT NULL DEFAULT FALSE,
 last_relevance_check_at TIMESTAMPTZ, next_relevance_check_at TIMESTAMPTZ, relevance_comment TEXT NOT NULL DEFAULT '',
 published_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), deleted_at TIMESTAMPTZ,
 search_vector TSVECTOR GENERATED ALWAYS AS (setweight(to_tsvector('russian',coalesce(title,'')),'A') || setweight(to_tsvector('russian',coalesce(subtitle,'')),'B') || setweight(to_tsvector('russian',coalesce(excerpt,'')),'B') || setweight(to_tsvector('russian',coalesce(content_html,'')),'C')) STORED
);
CREATE TABLE IF NOT EXISTS publication_versions (
 id BIGSERIAL PRIMARY KEY, publication_id BIGINT NOT NULL REFERENCES publications(id) ON DELETE CASCADE, version INTEGER NOT NULL,
 changed_by BIGINT NOT NULL REFERENCES users(id), title VARCHAR(240) NOT NULL, excerpt VARCHAR(800) NOT NULL DEFAULT '',
 content_json JSONB NOT NULL, content_html TEXT NOT NULL DEFAULT '', change_summary TEXT NOT NULL DEFAULT '', change_reason VARCHAR(80) NOT NULL DEFAULT 'updated',
 status VARCHAR(20) NOT NULL DEFAULT 'current', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), UNIQUE(publication_id,version)
);
CREATE TABLE IF NOT EXISTS publication_topic_links (publication_id BIGINT REFERENCES publications(id) ON DELETE CASCADE, topic_id BIGINT REFERENCES publication_topics(id) ON DELETE CASCADE, PRIMARY KEY(publication_id,topic_id));
CREATE TABLE IF NOT EXISTS publication_tag_links (publication_id BIGINT REFERENCES publications(id) ON DELETE CASCADE, tag_id BIGINT REFERENCES publication_tags(id) ON DELETE CASCADE, PRIMARY KEY(publication_id,tag_id));
CREATE TABLE IF NOT EXISTS publication_skill_links (publication_id BIGINT REFERENCES publications(id) ON DELETE CASCADE, skill_id BIGINT REFERENCES dictionary_items(id) ON DELETE CASCADE, contribution NUMERIC(5,2) NOT NULL DEFAULT 1, PRIMARY KEY(publication_id,skill_id));
CREATE TABLE IF NOT EXISTS publication_test_links (publication_id BIGINT REFERENCES publications(id) ON DELETE CASCADE, test_id BIGINT REFERENCES tests(id) ON DELETE CASCADE, sort_order INTEGER NOT NULL DEFAULT 0, PRIMARY KEY(publication_id,test_id));
CREATE TABLE IF NOT EXISTS publication_series_items (series_id BIGINT REFERENCES publication_series(id) ON DELETE CASCADE, publication_id BIGINT UNIQUE REFERENCES publications(id) ON DELETE CASCADE, sort_order INTEGER NOT NULL DEFAULT 0, PRIMARY KEY(series_id,publication_id));
CREATE TABLE IF NOT EXISTS publication_reactions (
 publication_id BIGINT REFERENCES publications(id) ON DELETE CASCADE, user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
 reaction_type VARCHAR(32) CHECK(reaction_type IN('useful','used_at_work','solved_problem','helped_audit')), created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), PRIMARY KEY(publication_id,user_id,reaction_type)
);
CREATE TABLE IF NOT EXISTS publication_collections (id BIGSERIAL PRIMARY KEY,user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,name VARCHAR(160) NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),UNIQUE(user_id,name));
CREATE TABLE IF NOT EXISTS publication_bookmarks (publication_id BIGINT REFERENCES publications(id) ON DELETE CASCADE,user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,collection_id BIGINT REFERENCES publication_collections(id) ON DELETE SET NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),PRIMARY KEY(publication_id,user_id));
CREATE TABLE IF NOT EXISTS author_subscriptions (subscriber_id BIGINT REFERENCES users(id) ON DELETE CASCADE,author_id BIGINT REFERENCES users(id) ON DELETE CASCADE,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),PRIMARY KEY(subscriber_id,author_id),CHECK(subscriber_id<>author_id));
CREATE TABLE IF NOT EXISTS publication_comments (
 id BIGSERIAL PRIMARY KEY, publication_id BIGINT REFERENCES publications(id) ON DELETE CASCADE, author_id BIGINT REFERENCES users(id), parent_id BIGINT REFERENCES publication_comments(id) ON DELETE CASCADE,
 message_type VARCHAR(20) NOT NULL DEFAULT 'opinion' CHECK(message_type IN('question','answer','opinion','clarification')),
 body TEXT NOT NULL, is_best BOOLEAN NOT NULL DEFAULT FALSE, is_confirmed BOOLEAN NOT NULL DEFAULT FALSE, is_pinned BOOLEAN NOT NULL DEFAULT FALSE,
 is_expert BOOLEAN NOT NULL DEFAULT FALSE, helpful_count INTEGER NOT NULL DEFAULT 0, edited_at TIMESTAMPTZ, deleted_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS publication_comment_reactions (comment_id BIGINT REFERENCES publication_comments(id) ON DELETE CASCADE,user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),PRIMARY KEY(comment_id,user_id));
CREATE TABLE IF NOT EXISTS publication_views (
 id BIGSERIAL PRIMARY KEY,publication_id BIGINT REFERENCES publications(id) ON DELETE CASCADE,user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
 viewer_hash CHAR(64) NOT NULL,viewed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS publication_slug_history (id BIGSERIAL PRIMARY KEY,publication_id BIGINT REFERENCES publications(id) ON DELETE CASCADE,old_slug VARCHAR(180) NOT NULL UNIQUE,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
CREATE TABLE IF NOT EXISTS publication_read_progress (publication_id BIGINT REFERENCES publications(id) ON DELETE CASCADE,user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,progress SMALLINT NOT NULL DEFAULT 0,last_read_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),PRIMARY KEY(publication_id,user_id));
CREATE TABLE IF NOT EXISTS publication_reports (
 id BIGSERIAL PRIMARY KEY,publication_id BIGINT REFERENCES publications(id) ON DELETE CASCADE,comment_id BIGINT REFERENCES publication_comments(id) ON DELETE CASCADE,reporter_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
report_type VARCHAR(40) NOT NULL,details TEXT NOT NULL DEFAULT '',status VARCHAR(20) NOT NULL DEFAULT 'new',resolution TEXT NOT NULL DEFAULT '',handled_by BIGINT REFERENCES users(id),handled_at TIMESTAMPTZ,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS publication_analytics_daily (publication_id BIGINT REFERENCES publications(id) ON DELETE CASCADE,day DATE NOT NULL,views BIGINT NOT NULL DEFAULT 0,unique_views BIGINT NOT NULL DEFAULT 0,profile_clicks BIGINT NOT NULL DEFAULT 0,test_clicks BIGINT NOT NULL DEFAULT 0,test_completions BIGINT NOT NULL DEFAULT 0,new_followers BIGINT NOT NULL DEFAULT 0,citations BIGINT NOT NULL DEFAULT 0,PRIMARY KEY(publication_id,day));
CREATE TABLE IF NOT EXISTS publication_moderation_audit (id BIGSERIAL PRIMARY KEY,publication_id BIGINT REFERENCES publications(id) ON DELETE CASCADE,admin_label VARCHAR(160) NOT NULL DEFAULT '',action VARCHAR(40) NOT NULL,reason TEXT NOT NULL DEFAULT '',created_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
CREATE TABLE IF NOT EXISTS notifications (id BIGSERIAL PRIMARY KEY,user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,type VARCHAR(60) NOT NULL,title VARCHAR(240) NOT NULL,body TEXT NOT NULL DEFAULT '',entity_type VARCHAR(60) NOT NULL DEFAULT '',entity_id BIGINT,is_read BOOLEAN NOT NULL DEFAULT FALSE,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
CREATE INDEX IF NOT EXISTS notifications_user_idx ON notifications(user_id,is_read,created_at DESC);
CREATE INDEX IF NOT EXISTS publications_feed_idx ON publications(status,visibility,published_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS publications_author_idx ON publications(author_id,updated_at DESC);
CREATE INDEX IF NOT EXISTS publications_category_idx ON publications(category_id,published_at DESC);
CREATE INDEX IF NOT EXISTS publications_search_idx ON publications USING GIN(search_vector);
CREATE INDEX IF NOT EXISTS publication_views_dedupe_idx ON publication_views(publication_id,viewer_hash,viewed_at DESC);
CREATE INDEX IF NOT EXISTS publication_comments_feed_idx ON publication_comments(publication_id,created_at);
CREATE INDEX IF NOT EXISTS publication_reports_status_idx ON publication_reports(status,created_at);

INSERT INTO publication_categories(name,slug,description,sort_order) VALUES
 ('Налоги и отчётность','nalogi-i-otchetnost','Практика налогового учёта и отчётности',10),('Бухгалтерский учёт','buhgalterskiy-uchet','Методология и практика бухгалтерского учёта',20),('Финансовый анализ','finansovyy-analiz','Управленческие решения и аналитика',30),('Карьера','karera','Профессиональное развитие',40)
ON CONFLICT(slug) DO NOTHING;
INSERT INTO publication_topics(category_id,name,slug) SELECT id,'УСН','usn' FROM publication_categories WHERE slug='nalogi-i-otchetnost' ON CONFLICT(slug) DO NOTHING;
INSERT INTO publication_topics(category_id,name,slug) SELECT id,'НДС','nds' FROM publication_categories WHERE slug='nalogi-i-otchetnost' ON CONFLICT(slug) DO NOTHING;
