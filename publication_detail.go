package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type publicationDetail struct {
	publicationCard
	Content               []publicationBlock `json:"content"`
	ContentHTML           string             `json:"content_html"`
	SummaryPoints         []string           `json:"summary_points"`
	SEOTitle              string             `json:"seo_title"`
	SEODescription        string             `json:"seo_description"`
	Language              string             `json:"language"`
	AllowComments         bool               `json:"allow_comments"`
	CategorySlug          string             `json:"category_slug"`
	AuthorEmail           string             `json:"author_email"`
	AuthorTitle           string             `json:"author_title"`
	AuthorCompany         string             `json:"author_company"`
	AuthorCity            string             `json:"author_city"`
	AuthorExperience      int                `json:"author_experience"`
	Followers             int64              `json:"followers"`
	AuthorPublications    int64              `json:"author_publications"`
	LastRelevanceCheck    *time.Time         `json:"last_relevance_check_at"`
	NextRelevanceCheck    *time.Time         `json:"next_relevance_check_at"`
	RelevanceComment      string             `json:"relevance_comment"`
	Topics                []string           `json:"topics"`
	Reactions             map[string]int64   `json:"reactions"`
	MyReactions           []string           `json:"my_reactions"`
	Tests                 []map[string]any   `json:"tests"`
	SeriesInfo            map[string]any     `json:"series_info,omitempty"`
	IsAuthor              bool               `json:"is_author"`
	ExpertiseContribution float64            `json:"expertise_contribution"`
}

func loadPublicationDetail(r *http.Request, id int64, recordView bool) (publicationDetail, error) {
	uid := optionalUserID(r)
	var d publicationDetail
	var content, summary, tags, skills, topics, reactions, myReactions, tests []byte
	err := db.QueryRowContext(r.Context(), `SELECT p.id,p.author_id,p.title,p.subtitle,p.excerpt,p.cover_image,p.slug,p.status,p.visibility,p.relevance_status,p.difficulty,p.reading_time,u.full_name,COALESCE(u.avatar_url,''),COALESCE(c.name,''),COALESCE(c.slug,''),p.published_at,p.updated_at,p.content_json,p.content_html,p.summary_points,p.seo_title,p.seo_description,p.language,p.allow_comments,u.email,
	COALESCE((SELECT i.value FROM resumes r JOIN resume_categories rc ON rc.resume_id=r.id JOIN dictionary_items i ON i.id=rc.category_id JOIN dictionaries di ON di.id=i.dictionary_id AND di.alias='position' WHERE r.user_id=u.id LIMIT 1),'Эксперт FinTalent'),'','',14,
	(SELECT COUNT(*) FROM author_subscriptions s WHERE s.author_id=u.id),(SELECT COUNT(*) FROM publications ap WHERE ap.author_id=u.id AND ap.status='published' AND ap.deleted_at IS NULL),p.last_relevance_check_at,p.next_relevance_check_at,p.relevance_comment,
	(SELECT COUNT(*) FROM publication_views v WHERE v.publication_id=p.id),(SELECT COUNT(DISTINCT viewer_hash) FROM publication_views v WHERE v.publication_id=p.id),(SELECT COUNT(*) FROM publication_bookmarks b WHERE b.publication_id=p.id),(SELECT COUNT(*) FROM publication_reactions x WHERE x.publication_id=p.id AND x.reaction_type='useful'),(SELECT COUNT(*) FROM publication_comments m WHERE m.publication_id=p.id AND m.deleted_at IS NULL),
	EXISTS(SELECT 1 FROM publication_bookmarks b WHERE b.publication_id=p.id AND b.user_id=$2),EXISTS(SELECT 1 FROM author_subscriptions s WHERE s.author_id=p.author_id AND s.subscriber_id=$2),
	COALESCE((SELECT jsonb_agg(t.name ORDER BY t.name) FROM publication_tag_links l JOIN publication_tags t ON t.id=l.tag_id WHERE l.publication_id=p.id),'[]'),COALESCE((SELECT jsonb_agg(i.value ORDER BY i.value) FROM publication_skill_links l JOIN dictionary_items i ON i.id=l.skill_id WHERE l.publication_id=p.id),'[]'),COALESCE((SELECT jsonb_agg(t.name ORDER BY t.name) FROM publication_topic_links l JOIN publication_topics t ON t.id=l.topic_id WHERE l.publication_id=p.id),'[]'),
	COALESCE((SELECT jsonb_object_agg(reaction_type,cnt) FROM(SELECT reaction_type,COUNT(*) cnt FROM publication_reactions WHERE publication_id=p.id GROUP BY reaction_type)x),'{}'),COALESCE((SELECT jsonb_agg(reaction_type) FROM publication_reactions WHERE publication_id=p.id AND user_id=$2),'[]'),
	COALESCE((SELECT jsonb_agg(jsonb_build_object('id',t.id,'title',v.title,'difficulty',t.difficulty,'questions',COALESCE((SELECT COUNT(*) FROM test_questions q WHERE q.test_version_id=v.id),0),'duration',COALESCE(t.time_limit_seconds,0),'average',COALESCE(s.average_percent,0),'attempts',COALESCE(s.attempts_count,0)) ORDER BY l.sort_order) FROM publication_test_links l JOIN tests t ON t.id=l.test_id JOIN test_versions v ON v.test_id=t.id AND v.version=t.current_version LEFT JOIN test_statistics s ON s.test_id=t.id WHERE l.publication_id=p.id),'[]')
	FROM publications p JOIN users u ON u.id=p.author_id LEFT JOIN publication_categories c ON c.id=p.category_id WHERE p.id=$1 AND p.deleted_at IS NULL AND (p.author_id=$2 OR (p.status='published' AND p.visibility IN('public','unlisted')))`, id, uid).Scan(&d.ID, &d.AuthorID, &d.Title, &d.Subtitle, &d.Excerpt, &d.CoverImage, &d.Slug, &d.Status, &d.Visibility, &d.RelevanceStatus, &d.Difficulty, &d.ReadingTime, &d.AuthorName, &d.AuthorAvatar, &d.Category, &d.CategorySlug, &d.PublishedAt, &d.UpdatedAt, &content, &d.ContentHTML, &summary, &d.SEOTitle, &d.SEODescription, &d.Language, &d.AllowComments, &d.AuthorEmail, &d.AuthorTitle, &d.AuthorCompany, &d.AuthorCity, &d.AuthorExperience, &d.Followers, &d.AuthorPublications, &d.LastRelevanceCheck, &d.NextRelevanceCheck, &d.RelevanceComment, &d.Views, &d.UniqueViews, &d.Saves, &d.Useful, &d.Discussions, &d.IsSaved, &d.IsFollowing, &tags, &skills, &topics, &reactions, &myReactions, &tests)
	if err != nil {
		return d, err
	}
	_ = json.Unmarshal(content, &d.Content)
	d.SummaryPoints = scanJSONStrings(summary)
	d.Tags = scanJSONStrings(tags)
	d.Skills = scanJSONStrings(skills)
	d.Topics = scanJSONStrings(topics)
	d.MyReactions = scanJSONStrings(myReactions)
	_ = json.Unmarshal(reactions, &d.Reactions)
	_ = json.Unmarshal(tests, &d.Tests)
	d.IsAuthor = uid == d.AuthorID
	den := d.Views
	if den < 1 {
		den = 1
	}
	d.Usefulness = int(d.Useful * 100 / den)
	d.ExpertiseContribution = publicationExpertiseContribution(d.AuthorPublications, d.Useful, d.Saves, d.Reactions["used_at_work"], d.AuthorPublications)
	var sid int64
	var title, slug string
	var order, count int
	if db.QueryRowContext(r.Context(), `SELECT s.id,s.title,s.slug,si.sort_order,(SELECT COUNT(*) FROM publication_series_items x JOIN publications p2 ON p2.id=x.publication_id WHERE x.series_id=s.id AND p2.status='published') FROM publication_series_items si JOIN publication_series s ON s.id=si.series_id WHERE si.publication_id=$1`, id).Scan(&sid, &title, &slug, &order, &count) == nil {
		d.Series = title
		d.SeriesOrder = order
		d.SeriesInfo = map[string]any{"id": sid, "title": title, "slug": slug, "order": order, "count": count}
	}
	if recordView && d.Status == "published" {
		hash := publicationViewerHash(r, uid)
		res, _ := db.ExecContext(r.Context(), `INSERT INTO publication_views(publication_id,user_id,viewer_hash) SELECT $1,NULLIF($2,0),$3 WHERE NOT EXISTS(SELECT 1 FROM publication_views WHERE publication_id=$1 AND viewer_hash=$3 AND viewed_at>NOW()-INTERVAL '6 hours')`, id, uid, hash)
		if n, _ := res.RowsAffected(); n > 0 {
			d.Views++
			d.UniqueViews++
			_, _ = db.ExecContext(r.Context(), `INSERT INTO publication_analytics_daily(publication_id,day,views,unique_views) VALUES($1,CURRENT_DATE,1,1) ON CONFLICT(publication_id,day) DO UPDATE SET views=publication_analytics_daily.views+1,unique_views=publication_analytics_daily.unique_views+1`, id)
		}
	}
	return d, nil
}

func getPublicationJSON(w http.ResponseWriter, r *http.Request, id int64) {
	d, err := loadPublicationDetail(r, id, true)
	if err != nil {
		status, msg := dbErrorStatus(err)
		writeJSON(w, status, msg)
		return
	}
	writeAdminJSON(w, 200, d)
}

func publicationComments(w http.ResponseWriter, r *http.Request, id int64) {
	if r.Method == http.MethodGet {
		rows, err := db.QueryContext(r.Context(), `SELECT c.id,c.author_id,u.full_name,COALESCE(u.avatar_url,''),c.parent_id,c.message_type,c.body,c.is_best,c.is_confirmed,c.is_pinned,c.is_expert,c.helpful_count,c.edited_at,c.created_at,p.author_id=c.author_id FROM publication_comments c JOIN users u ON u.id=c.author_id JOIN publications p ON p.id=c.publication_id WHERE c.publication_id=$1 AND c.deleted_at IS NULL ORDER BY c.is_pinned DESC,c.created_at LIMIT 50`, id)
		if err != nil {
			writeJSON(w, 500, "Не удалось загрузить обсуждение")
			return
		}
		defer rows.Close()
		items := []map[string]any{}
		for rows.Next() {
			var cid, aid int64
			var name, avatar, typ, body string
			var parent sql.NullInt64
			var best, confirmed, pinned, expert, isAuthor bool
			var helpful int
			var edited sql.NullTime
			var created time.Time
			_ = rows.Scan(&cid, &aid, &name, &avatar, &parent, &typ, &body, &best, &confirmed, &pinned, &expert, &helpful, &edited, &created, &isAuthor)
			items = append(items, map[string]any{"id": cid, "author_id": aid, "author_name": name, "author_avatar": avatar, "parent_id": parent.Int64, "message_type": typ, "body": body, "is_best": best, "is_confirmed": confirmed, "is_pinned": pinned, "is_expert": expert, "helpful_count": helpful, "edited_at": edited.Time, "created_at": created, "is_author": isAuthor})
		}
		writeAdminJSON(w, 200, map[string]any{"items": items})
		return
	}
	u, err := userFromRequest(r)
	if err != nil {
		writeJSON(w, 401, "Требуется авторизация")
		return
	}
	if r.Method == http.MethodPost {
		var in struct {
			Body, Type string
			ParentID   int64
		}
		r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
		if json.NewDecoder(r.Body).Decode(&in) != nil {
			writeJSON(w, 400, "Некорректное сообщение")
			return
		}
		in.Body = strings.TrimSpace(in.Body)
		if len([]rune(in.Body)) < 2 || len([]rune(in.Body)) > 5000 {
			writeJSON(w, 400, "Сообщение должно содержать от 2 до 5000 символов")
			return
		}
		if !map[string]bool{"question": true, "answer": true, "opinion": true, "clarification": true}[in.Type] {
			in.Type = "opinion"
		}
		var cid int64
		err = db.QueryRowContext(r.Context(), `INSERT INTO publication_comments(publication_id,author_id,parent_id,message_type,body) SELECT $1,$2,NULLIF($3,0),$4,$5 WHERE EXISTS(SELECT 1 FROM publications WHERE id=$1 AND status='published' AND allow_comments) AND NOT EXISTS(SELECT 1 FROM publication_comments WHERE author_id=$2 AND created_at>NOW()-INTERVAL '8 seconds') RETURNING id`, id, u.ID, in.ParentID, in.Type, in.Body).Scan(&cid)
		if err != nil {
			writeJSON(w, 400, "Обсуждение закрыто или публикация недоступна")
			return
		}
		_, _ = db.ExecContext(r.Context(), `INSERT INTO notifications(user_id,type,title,body,entity_type,entity_id) SELECT author_id,'publication_comment','Новое обсуждение',$2,'publication',$1 FROM publications WHERE id=$1 AND author_id<>$3`, id, u.FullName+" оставил сообщение к публикации", u.ID)
		writeAdminJSON(w, 201, map[string]any{"id": cid})
		return
	}
	writeJSON(w, 405, "Метод не поддерживается")
}

func publicationRecommendations(w http.ResponseWriter, r *http.Request, id int64) {
	rows, err := db.QueryContext(r.Context(), `SELECT p.id,p.title,p.slug,p.excerpt,p.cover_image,u.full_name,COALESCE(c.name,'') FROM publications p JOIN users u ON u.id=p.author_id LEFT JOIN publication_categories c ON c.id=p.category_id WHERE p.id<>$1 AND p.status='published' AND p.visibility='public' AND p.deleted_at IS NULL ORDER BY (p.category_id=(SELECT category_id FROM publications WHERE id=$1)) DESC,(SELECT COUNT(*) FROM publication_tag_links a JOIN publication_tag_links b ON b.tag_id=a.tag_id WHERE a.publication_id=$1 AND b.publication_id=p.id) DESC,p.published_at DESC LIMIT 3`, id)
	if err != nil {
		writeJSON(w, 500, "Не удалось подобрать рекомендации")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var pid int64
		var title, slug, excerpt, cover, author, category string
		_ = rows.Scan(&pid, &title, &slug, &excerpt, &cover, &author, &category)
		items = append(items, map[string]any{"id": pid, "title": title, "slug": slug, "excerpt": excerpt, "cover_image": cover, "author": author, "category": category})
	}
	writeAdminJSON(w, 200, map[string]any{"items": items})
}

func publicationVersions(w http.ResponseWriter, r *http.Request, id int64) {
	rows, err := db.QueryContext(r.Context(), `SELECT version,created_at,change_summary,change_reason,status FROM publication_versions WHERE publication_id=$1 ORDER BY version DESC LIMIT 20`, id)
	if err != nil {
		writeJSON(w, 500, "Не удалось загрузить версии")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var version int
		var created time.Time
		var summary, reason, status string
		_ = rows.Scan(&version, &created, &summary, &reason, &status)
		items = append(items, map[string]any{"version": version, "created_at": created, "summary": summary, "reason": reason, "status": status})
	}
	writeAdminJSON(w, 200, map[string]any{"items": items})
}

func publicationAnalytics(w http.ResponseWriter, r *http.Request, id int64) {
	u, err := userFromRequest(r)
	if err != nil {
		writeJSON(w, 401, "Требуется авторизация")
		return
	}
	var author int64
	_ = db.QueryRowContext(r.Context(), `SELECT author_id FROM publications WHERE id=$1`, id).Scan(&author)
	if author != u.ID && !isAdmin(r) {
		writeJSON(w, 403, "Аналитика доступна только автору")
		return
	}
	var views, unique, saves, comments, followers int64
	_ = db.QueryRowContext(r.Context(), `SELECT (SELECT COUNT(*) FROM publication_views WHERE publication_id=$1),(SELECT COUNT(DISTINCT viewer_hash) FROM publication_views WHERE publication_id=$1),(SELECT COUNT(*) FROM publication_bookmarks WHERE publication_id=$1),(SELECT COUNT(*) FROM publication_comments WHERE publication_id=$1 AND deleted_at IS NULL),(SELECT COUNT(*) FROM author_subscriptions WHERE author_id=$2)`, id, author).Scan(&views, &unique, &saves, &comments, &followers)
	writeAdminJSON(w, 200, map[string]any{"views": views, "unique_views": unique, "saves": saves, "discussions": comments, "followers": followers})
}

func publicationReport(w http.ResponseWriter, r *http.Request, id int64) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, "Метод не поддерживается")
		return
	}
	u, err := userFromRequest(r)
	if err != nil {
		writeJSON(w, 401, "Требуется авторизация")
		return
	}
	var in struct{ Type, Details string }
	_ = json.NewDecoder(r.Body).Decode(&in)
	allowed := map[string]bool{"outdated": true, "false_information": true, "copyright": true, "advertising": true, "insult": true, "spam": true, "other": true}
	if !allowed[in.Type] {
		writeJSON(w, 400, "Выберите причину жалобы")
		return
	}
	_, err = db.ExecContext(r.Context(), `INSERT INTO publication_reports(publication_id,reporter_id,report_type,details) VALUES($1,$2,$3,$4)`, id, u.ID, in.Type, strings.TrimSpace(in.Details))
	if err != nil {
		writeJSON(w, 500, "Не удалось отправить сообщение")
		return
	}
	writeJSON(w, 201, "Сообщение отправлено автору и модератору")
}

func publicationProgress(w http.ResponseWriter, r *http.Request, id int64) {
	if r.Method != http.MethodPost {
		return
	}
	u, err := userFromRequest(r)
	if err != nil {
		return
	}
	var in struct{ Progress int }
	_ = json.NewDecoder(r.Body).Decode(&in)
	if in.Progress < 0 {
		in.Progress = 0
	}
	if in.Progress > 100 {
		in.Progress = 100
	}
	_, _ = db.ExecContext(r.Context(), `INSERT INTO publication_read_progress(publication_id,user_id,progress) VALUES($1,$2,$3) ON CONFLICT(publication_id,user_id) DO UPDATE SET progress=EXCLUDED.progress,last_read_at=NOW()`, id, u.ID, in.Progress)
	writeJSON(w, 200, "Прогресс сохранён")
}

func publicationPage(w http.ResponseWriter, r *http.Request) {
	tail := strings.Trim(strings.TrimPrefix(r.URL.Path, "/publications/"), "/")
	if strings.HasSuffix(tail, "/edit") {
		idText := strings.TrimSuffix(tail, "/edit")
		if _, err := strconv.ParseInt(idText, 10, 64); err == nil {
			servePage("static/publication-editor.html")(w, r)
			return
		}
	}
	if tail == "" {
		http.Redirect(w, r, "/publications", 302)
		return
	}
	var id int64
	err := db.QueryRowContext(r.Context(), `SELECT id FROM publications WHERE slug=$1 AND deleted_at IS NULL`, tail).Scan(&id)
	if err == sql.ErrNoRows {
		var newSlug string
		if db.QueryRowContext(r.Context(), `SELECT p.slug FROM publication_slug_history h JOIN publications p ON p.id=h.publication_id WHERE h.old_slug=$1 AND p.status='published' AND p.deleted_at IS NULL`, tail).Scan(&newSlug) == nil {
			http.Redirect(w, r, "/publications/"+newSlug, http.StatusMovedPermanently)
			return
		}
		http.NotFound(w, r)
		return
	}
	d, err := loadPublicationDetail(r, id, true)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	renderPublicationSEOPage(w, r, d)
}

func renderPublicationSEOPage(w http.ResponseWriter, r *http.Request, d publicationDetail) {
	title := d.SEOTitle
	if title == "" {
		title = d.Title + " — FinTalent"
	}
	description := d.SEODescription
	if description == "" {
		description = d.Excerpt
	}
	canonical := publicationBaseURL(r) + "/publications/" + d.Slug
	image := d.CoverImage
	if strings.HasPrefix(image, "/") {
		image = publicationBaseURL(r) + image
	}
	robots := "index,follow"
	if d.Status != "published" || d.Visibility != "public" {
		robots = "noindex,nofollow"
	}
	schema, _ := json.Marshal(map[string]any{"@context": "https://schema.org", "@type": "Article", "headline": d.Title, "description": description, "image": image, "datePublished": d.PublishedAt, "dateModified": d.UpdatedAt, "inLanguage": d.Language, "author": map[string]string{"@type": "Person", "name": d.AuthorName}, "mainEntityOfPage": canonical})
	templateBytes, err := os.ReadFile("static/publication-view.html")
	if err != nil {
		http.Error(w, "Шаблон недоступен", 500)
		return
	}
	page := string(templateBytes)
	replacements := map[string]string{"{{SEO_TITLE}}": html.EscapeString(title), "{{SEO_DESCRIPTION}}": html.EscapeString(description), "{{CANONICAL}}": html.EscapeString(canonical), "{{SEO_IMAGE}}": html.EscapeString(image), "{{ROBOTS}}": robots, "{{STRUCTURED_DATA}}": string(schema), "{{ARTICLE_TITLE}}": html.EscapeString(d.Title), "{{ARTICLE_EXCERPT}}": html.EscapeString(d.Excerpt), "{{ARTICLE_HTML}}": d.ContentHTML, "{{PUBLICATION_ID}}": strconv.FormatInt(d.ID, 10)}
	for key, value := range replacements {
		page = strings.ReplaceAll(page, key, value)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=60")
	_, _ = w.Write([]byte(page))
}

func publicationBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func publicationSitemap(w http.ResponseWriter, r *http.Request) {
	rows, err := db.QueryContext(r.Context(), `SELECT slug,updated_at FROM publications WHERE status='published' AND visibility='public' AND deleted_at IS NULL ORDER BY updated_at DESC`)
	if err != nil {
		http.Error(w, "", 500)
		return
	}
	defer rows.Close()
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`)
	fmt.Fprintf(w, "<url><loc>%s/publications</loc></url>", publicationBaseURL(r))
	for rows.Next() {
		var slug string
		var updated time.Time
		_ = rows.Scan(&slug, &updated)
		fmt.Fprintf(w, "<url><loc>%s/publications/%s</loc><lastmod>%s</lastmod></url>", publicationBaseURL(r), html.EscapeString(slug), updated.Format("2006-01-02"))
	}
	fmt.Fprint(w, "</urlset>")
}
func publicationRobots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "User-agent: *\nAllow: /\nDisallow: /admin\nDisallow: /publications/create\nSitemap: %s/sitemap.xml\n", publicationBaseURL(r))
}
