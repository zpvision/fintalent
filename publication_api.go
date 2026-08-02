package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type publicationCard struct {
	ID              int64      `json:"id"`
	AuthorID        int64      `json:"author_id"`
	Title           string     `json:"title"`
	Subtitle        string     `json:"subtitle"`
	Excerpt         string     `json:"excerpt"`
	CoverImage      string     `json:"cover_image"`
	Slug            string     `json:"slug"`
	Status          string     `json:"status"`
	Visibility      string     `json:"visibility"`
	RelevanceStatus string     `json:"relevance_status"`
	Difficulty      string     `json:"difficulty"`
	ReadingTime     int        `json:"reading_time"`
	AuthorName      string     `json:"author_name"`
	AuthorAvatar    string     `json:"author_avatar"`
	Category        string     `json:"category"`
	PublishedAt     *time.Time `json:"published_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	Views           int64      `json:"views"`
	UniqueViews     int64      `json:"unique_views"`
	Saves           int64      `json:"saves"`
	Useful          int64      `json:"useful"`
	Discussions     int64      `json:"discussions"`
	Usefulness      int        `json:"usefulness"`
	IsSaved         bool       `json:"is_saved"`
	IsFollowing     bool       `json:"is_following"`
	Tags            []string   `json:"tags"`
	Skills          []string   `json:"skills"`
	Series          string     `json:"series,omitempty"`
	SeriesOrder     int        `json:"series_order,omitempty"`
}

func publicationsAPI(w http.ResponseWriter, r *http.Request) {
	if !requirePublicationSameOrigin(w, r) {
		return
	}
	if r.Method == http.MethodPost {
		createPublication(w, r)
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, 405, "Метод не поддерживается")
		return
	}
	uid := optionalUserID(r)
	q := r.URL.Query()
	scope := q.Get("scope")
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	limit := 12
	offset := (page - 1) * limit
	where := []string{"p.deleted_at IS NULL"}
	args := []any{}
	add := func(s string, v any) { args = append(args, v); where = append(where, fmt.Sprintf(s, len(args))) }
	if scope == "mine" || scope == "drafts" {
		if uid == 0 {
			writeJSON(w, 401, "Требуется авторизация")
			return
		}
		add("p.author_id=$%d", uid)
		if scope == "drafts" {
			where = append(where, "p.status='draft'")
		}
	} else {
		where = append(where, "p.status='published' AND p.visibility='public'")
		if scope == "saved" {
			if uid == 0 {
				writeJSON(w, 401, "Требуется авторизация")
				return
			}
			add("EXISTS(SELECT 1 FROM publication_bookmarks b WHERE b.publication_id=p.id AND b.user_id=$%d)", uid)
		}
		if scope == "subscriptions" {
			if uid == 0 {
				writeJSON(w, 401, "Требуется авторизация")
				return
			}
			add("EXISTS(SELECT 1 FROM author_subscriptions s WHERE s.author_id=p.author_id AND s.subscriber_id=$%d)", uid)
		}
		if scope == "recommended" {
			where = append(where, "p.is_recommended=TRUE")
		}
	}
	if search := strings.TrimSpace(q.Get("q")); search != "" {
		args = append(args, search)
		where = append(where, fmt.Sprintf("(p.search_vector @@ websearch_to_tsquery('russian',$%d) OR u.full_name ILIKE '%%'||$%d||'%%' OR c.name ILIKE '%%'||$%d||'%%' OR EXISTS(SELECT 1 FROM publication_tag_links pl JOIN publication_tags pt ON pt.id=pl.tag_id WHERE pl.publication_id=p.id AND pt.name ILIKE '%%'||$%d||'%%') OR EXISTS(SELECT 1 FROM publication_skill_links sl JOIN dictionary_items si ON si.id=sl.skill_id WHERE sl.publication_id=p.id AND si.value ILIKE '%%'||$%d||'%%'))", len(args), len(args), len(args), len(args), len(args)))
	}
	if cat := q.Get("category"); cat != "" {
		add("c.slug=$%d", cat)
	}
	if diff := q.Get("difficulty"); diff != "" {
		add("p.difficulty=$%d", diff)
	}
	order := "p.published_at DESC NULLS LAST,p.updated_at DESC"
	switch q.Get("sort") {
	case "popular":
		order = "views DESC,p.published_at DESC"
	case "useful":
		order = "useful DESC,p.published_at DESC"
	case "saved":
		order = "saves DESC,p.published_at DESC"
	case "discussed":
		order = "discussions DESC,p.published_at DESC"
	}
	args = append(args, uid, limit, offset)
	userArg := len(args) - 2
	limitArg := len(args) - 1
	offsetArg := len(args)
	query := fmt.Sprintf(`SELECT p.id,p.author_id,p.title,p.subtitle,p.excerpt,p.cover_image,p.slug,p.status,p.visibility,p.relevance_status,p.difficulty,p.reading_time,u.full_name,COALESCE(u.avatar_url,''),COALESCE(c.name,''),p.published_at,p.updated_at,
	(SELECT COUNT(*) FROM publication_views v WHERE v.publication_id=p.id) views,(SELECT COUNT(DISTINCT viewer_hash) FROM publication_views v WHERE v.publication_id=p.id) unique_views,
	(SELECT COUNT(*) FROM publication_bookmarks b WHERE b.publication_id=p.id) saves,(SELECT COUNT(*) FROM publication_reactions x WHERE x.publication_id=p.id AND x.reaction_type='useful') useful,
	(SELECT COUNT(*) FROM publication_comments m WHERE m.publication_id=p.id AND m.deleted_at IS NULL) discussions,
	EXISTS(SELECT 1 FROM publication_bookmarks b WHERE b.publication_id=p.id AND b.user_id=$%d),EXISTS(SELECT 1 FROM author_subscriptions s WHERE s.author_id=p.author_id AND s.subscriber_id=$%d),
	COALESCE((SELECT jsonb_agg(t.name ORDER BY t.name) FROM publication_tag_links l JOIN publication_tags t ON t.id=l.tag_id WHERE l.publication_id=p.id),'[]'),
	COALESCE((SELECT jsonb_agg(i.value ORDER BY i.value) FROM publication_skill_links l JOIN dictionary_items i ON i.id=l.skill_id WHERE l.publication_id=p.id),'[]'),COALESCE((SELECT s.title FROM publication_series_items si JOIN publication_series s ON s.id=si.series_id WHERE si.publication_id=p.id),''),COALESCE((SELECT si.sort_order FROM publication_series_items si WHERE si.publication_id=p.id),0)
	FROM publications p JOIN users u ON u.id=p.author_id LEFT JOIN publication_categories c ON c.id=p.category_id WHERE %s ORDER BY %s LIMIT $%d OFFSET $%d`, userArg, userArg, strings.Join(where, " AND "), order, limitArg, offsetArg)
	rows, err := db.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeJSON(w, 500, "Не удалось загрузить публикации")
		return
	}
	defer rows.Close()
	items := []publicationCard{}
	for rows.Next() {
		var x publicationCard
		var tags, skills []byte
		if err = rows.Scan(&x.ID, &x.AuthorID, &x.Title, &x.Subtitle, &x.Excerpt, &x.CoverImage, &x.Slug, &x.Status, &x.Visibility, &x.RelevanceStatus, &x.Difficulty, &x.ReadingTime, &x.AuthorName, &x.AuthorAvatar, &x.Category, &x.PublishedAt, &x.UpdatedAt, &x.Views, &x.UniqueViews, &x.Saves, &x.Useful, &x.Discussions, &x.IsSaved, &x.IsFollowing, &tags, &skills, &x.Series, &x.SeriesOrder); err != nil {
			writeJSON(w, 500, "Не удалось загрузить публикации")
			return
		}
		x.Tags = scanJSONStrings(tags)
		x.Skills = scanJSONStrings(skills)
		den := x.Views
		if den < 1 {
			den = 1
		}
		x.Usefulness = int(x.Useful * 100 / den)
		items = append(items, x)
	}
	writeAdminJSON(w, 200, map[string]any{"items": items, "page": page, "has_more": len(items) == limit})
}

func createPublication(w http.ResponseWriter, r *http.Request) {
	u, err := userFromRequest(r)
	if err != nil {
		writeJSON(w, 401, "Требуется авторизация")
		return
	}
	in, ok := decodePublicationInput(w, r)
	if !ok {
		return
	}
	contentJSON, _ := json.Marshal(in.Content)
	summary, _ := json.Marshal(in.SummaryPoints)
	htmlBody := renderPublicationBlocks(in.Content)
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		writeJSON(w, 500, "Не удалось создать публикацию")
		return
	}
	defer tx.Rollback()
	var id int64
	err = tx.QueryRowContext(r.Context(), `INSERT INTO publications(author_id,category_id,title,subtitle,excerpt,cover_image,content_json,content_html,summary_points,slug,seo_title,seo_description,visibility,difficulty,language,reading_time,allow_comments) VALUES($1,NULLIF($2,0),$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17) RETURNING id`, u.ID, in.CategoryID, in.Title, in.Subtitle, in.Excerpt, in.CoverImage, contentJSON, htmlBody, summary, in.Slug, in.SEOTitle, in.SEODescription, in.Visibility, in.Difficulty, in.Language, in.ReadingTime, in.AllowComments).Scan(&id)
	if err != nil {
		status, msg := dbErrorStatus(err)
		writeJSON(w, status, msg)
		return
	}
	if err = savePublicationLinks(r.Context(), tx, id, in); err != nil {
		writeJSON(w, 500, "Не удалось сохранить настройки публикации")
		return
	}
	if err = tx.Commit(); err != nil {
		writeJSON(w, 500, "Не удалось создать публикацию")
		return
	}
	writeAdminJSON(w, 201, map[string]any{"id": id, "slug": in.Slug, "status": "draft"})
}

func savePublicationLinks(ctx context.Context, tx *sql.Tx, id int64, in publicationInput) error {
	for _, table := range []string{"publication_tag_links", "publication_skill_links", "publication_topic_links", "publication_test_links", "publication_series_items"} {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE publication_id=$1", id); err != nil {
			return err
		}
	}
	for _, name := range in.Tags {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		slug := slugify(name)
		var tagID int64
		if err := tx.QueryRowContext(ctx, `INSERT INTO publication_tags(name,slug) VALUES($1,$2) ON CONFLICT(slug) DO UPDATE SET name=EXCLUDED.name RETURNING id`, name, slug).Scan(&tagID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO publication_tag_links VALUES($1,$2) ON CONFLICT DO NOTHING`, id, tagID); err != nil {
			return err
		}
	}
	for _, value := range in.Skills {
		if _, err := tx.ExecContext(ctx, `INSERT INTO publication_skill_links(publication_id,skill_id) SELECT $1,id FROM dictionary_items WHERE value=$2 AND deleted_at IS NULL ORDER BY id LIMIT 1 ON CONFLICT DO NOTHING`, id, value); err != nil {
			return err
		}
	}
	for _, slug := range in.Topics {
		if _, err := tx.ExecContext(ctx, `INSERT INTO publication_topic_links SELECT $1,id FROM publication_topics WHERE slug=$2 ON CONFLICT DO NOTHING`, id, slug); err != nil {
			return err
		}
	}
	if in.TestID > 0 {
		if _, err := tx.ExecContext(ctx, `INSERT INTO publication_test_links SELECT $1,id,0 FROM tests WHERE id=$2 AND status='published' ON CONFLICT DO NOTHING`, id, in.TestID); err != nil {
			return err
		}
	}
	if in.SeriesID > 0 {
		if _, err := tx.ExecContext(ctx, `INSERT INTO publication_series_items(series_id,publication_id,sort_order) SELECT id,$2,$3 FROM publication_series WHERE id=$1 ON CONFLICT(publication_id) DO UPDATE SET series_id=EXCLUDED.series_id,sort_order=EXCLUDED.sort_order`, in.SeriesID, id, in.SeriesOrder); err != nil {
			return err
		}
	}
	return nil
}

func publicationMetaAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, "Метод не поддерживается")
		return
	}
	uid := optionalUserID(r)
	categories := queryPairs(r, `SELECT id,name,slug FROM publication_categories WHERE is_active ORDER BY sort_order,name`)
	topics := queryPairs(r, `SELECT id,name,slug FROM publication_topics WHERE is_active ORDER BY name`)
	skills := queryPairs(r, `SELECT i.id,i.value,d.alias FROM dictionary_items i JOIN dictionaries d ON d.id=i.dictionary_id WHERE d.alias IN('accounting_areas','software','crm','position') AND i.active AND i.deleted_at IS NULL ORDER BY i.value LIMIT 100`)
	tests := queryPairs(r, `SELECT t.id,v.title,t.slug FROM tests t JOIN test_versions v ON v.test_id=t.id AND v.version=t.current_version WHERE t.status='published' ORDER BY v.title LIMIT 100`)
	series := []map[string]any{}
	if uid > 0 {
		rows, _ := db.QueryContext(r.Context(), `SELECT id,title,slug FROM publication_series WHERE author_id=$1 AND status<>'archived' ORDER BY title`, uid)
		if rows != nil {
			defer rows.Close()
			for rows.Next() {
				var id int64
				var name, slug string
				_ = rows.Scan(&id, &name, &slug)
				series = append(series, map[string]any{"id": id, "name": name, "slug": slug})
			}
		}
	}
	writeAdminJSON(w, 200, map[string]any{"categories": categories, "topics": topics, "skills": skills, "tests": tests, "series": series})
}

func queryPairs(r *http.Request, q string) []map[string]any {
	rows, err := db.QueryContext(r.Context(), q)
	if err != nil {
		return []map[string]any{}
	}
	defer rows.Close()
	result := []map[string]any{}
	for rows.Next() {
		var id int64
		var name, slug string
		if rows.Scan(&id, &name, &slug) == nil {
			result = append(result, map[string]any{"id": id, "name": name, "slug": slug})
		}
	}
	return result
}

func publicationSlugAPI(w http.ResponseWriter, r *http.Request) {
	slug := slugify(r.URL.Query().Get("value"))
	var exists bool
	_ = db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM publications WHERE slug=$1 UNION SELECT 1 FROM publication_slug_history WHERE old_slug=$1)`, slug).Scan(&exists)
	writeAdminJSON(w, 200, map[string]any{"slug": slug, "available": slug != "" && !exists})
}
func publicationSummaryStub(w http.ResponseWriter, r *http.Request) {
	if optionalUserID(r) == 0 {
		writeJSON(w, 401, "Требуется авторизация")
		return
	}
	writeAdminJSON(w, 200, map[string]any{"available": false, "message": "Автоматическая генерация будет доступна после подключения AI. Заполните тезисы вручную — неподтверждённые факты не создаются."})
}
