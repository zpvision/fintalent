package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/microcosm-cc/bluemonday"
)

//go:embed migrations/024_publications.sql migrations/025_publications_demo.sql
var publicationMigrationFS embed.FS

type publicationBlock struct {
	Type    string     `json:"type"`
	Text    string     `json:"text,omitempty"`
	URL     string     `json:"url,omitempty"`
	Caption string     `json:"caption,omitempty"`
	Items   []string   `json:"items,omitempty"`
	Rows    [][]string `json:"rows,omitempty"`
}

type publicationInput struct {
	Title, Subtitle, Excerpt, CoverImage, Slug, SEOTitle, SEODescription string
	CategoryID, SeriesID, SeriesOrder, ReadingTime, TestID               int64
	Difficulty, Language, Visibility, ChangeSummary                      string
	AllowComments                                                        bool
	Content                                                              []publicationBlock
	SummaryPoints, Tags, Skills, Topics                                  []string
}

var slugCleanup = regexp.MustCompile(`[^a-z0-9]+`)

func preparePublicationDatabase(ctx context.Context) error {
	schema, err := publicationMigrationFS.ReadFile("migrations/024_publications.sql")
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, string(schema))
	if err != nil {
		return fmt.Errorf("схема публикаций: %w", err)
	}
	return nil
}

func preparePublicationDemo(ctx context.Context) error {
	schema, err := publicationMigrationFS.ReadFile("migrations/025_publications_demo.sql")
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, string(schema))
	if err != nil {
		return fmt.Errorf("демо-данные публикаций: %w", err)
	}
	return nil
}

func registerPublicationRoutes() {
	http.HandleFunc("/publications", serveFrontendPage("static/publications.html"))
	http.HandleFunc("/publications/create", servePage("static/publication-editor.html"))
	http.HandleFunc("/publications/saved", serveFrontendPage("static/publications.html"))
	http.HandleFunc("/publications/analytics", servePage("static/publication-analytics.html"))
	http.HandleFunc("/publications/", publicationPage)
	http.HandleFunc("/api/publications", publicationsAPI)
	http.HandleFunc("/api/publications/meta", publicationMetaAPI)
	http.HandleFunc("/api/publications/slug", publicationSlugAPI)
	http.HandleFunc("/api/publications/summary", publicationSummaryStub)
	http.HandleFunc("/api/publications/upload", publicationUpload)
	http.HandleFunc("/api/publications/", publicationActionAPI)
	http.HandleFunc("/api/publication-authors/", publicationAuthorAPI)
	http.HandleFunc("/api/publication-series", publicationSeriesAPI)
	http.HandleFunc("/api/admin/publications", adminPublicationsAPI)
	http.HandleFunc("/api/admin/publications/", adminPublicationActionAPI)
	http.HandleFunc("/sitemap.xml", publicationSitemap)
	http.HandleFunc("/robots.txt", publicationRobots)
}

func slugify(value string) string {
	repl := map[rune]string{'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ё': "e", 'ж': "zh", 'з': "z", 'и': "i", 'й': "y", 'к': "k", 'л': "l", 'м': "m", 'н': "n", 'о': "o", 'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u", 'ф': "f", 'х': "h", 'ц': "c", 'ч': "ch", 'ш': "sh", 'щ': "sch", 'ъ': "", 'ы': "y", 'ь': "", 'э': "e", 'ю': "yu", 'я': "ya"}
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if v, ok := repl[r]; ok {
			b.WriteString(v)
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if r < 128 {
				b.WriteRune(r)
			}
		} else {
			b.WriteByte('-')
		}
	}
	result := strings.Trim(slugCleanup.ReplaceAllString(b.String(), "-"), "-")
	if len(result) > 170 {
		result = strings.Trim(result[:170], "-")
	}
	return result
}

func safeMediaURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "/static/uploads/publications/") || strings.HasPrefix(value, "/static/publication-") {
		return value
	}
	u, err := url.Parse(value)
	if err == nil && (u.Scheme == "https" || u.Scheme == "http") {
		return value
	}
	return ""
}

func renderPublicationBlocks(blocks []publicationBlock) string {
	var out strings.Builder
	writeText := func(tag, class, text string) {
		if strings.TrimSpace(text) != "" {
			fmt.Fprintf(&out, "<%s class=\"%s\">%s</%s>", tag, class, safePublicationInlineHTML(text), tag)
		}
	}
	for i, block := range blocks {
		id := fmt.Sprintf("section-%d", i+1)
		switch block.Type {
		case "h2":
			fmt.Fprintf(&out, `<h2 id="%s">%s</h2>`, id, safePublicationInlineHTML(block.Text))
		case "h3":
			fmt.Fprintf(&out, `<h3 id="%s">%s</h3>`, id, safePublicationInlineHTML(block.Text))
		case "paragraph":
			writeText("p", "", block.Text)
		case "quote":
			writeText("blockquote", "", block.Text)
		case "warning", "info", "avoid", "note", "example", "conclusion":
			labels := map[string]string{"warning": "Предупреждение", "info": "Информация", "avoid": "Как избежать", "note": "Обратите внимание", "example": "Практический пример", "conclusion": "Вывод"}
			fmt.Fprintf(&out, `<aside class="article-callout %s"><b>%s</b><p>%s</p></aside>`, block.Type, labels[block.Type], safePublicationInlineHTML(block.Text))
		case "bullets", "numbered", "checklist":
			tag := "ul"
			if block.Type == "numbered" {
				tag = "ol"
			}
			fmt.Fprintf(&out, "<%s class=\"%s\">", tag, block.Type)
			for _, item := range block.Items {
				if strings.TrimSpace(item) != "" {
					fmt.Fprintf(&out, "<li>%s</li>", safePublicationInlineHTML(item))
				}
			}
			fmt.Fprintf(&out, "</%s>", tag)
		case "code":
			fmt.Fprintf(&out, `<pre><code>%s</code></pre>`, html.EscapeString(block.Text))
		case "divider":
			out.WriteString("<hr>")
		case "image":
			if src := safeMediaURL(block.URL); src != "" {
				fmt.Fprintf(&out, `<figure><img src="%s" alt="%s" loading="lazy"><figcaption>%s</figcaption></figure>`, html.EscapeString(src), html.EscapeString(block.Caption), html.EscapeString(block.Caption))
			}
		case "video":
			if src := safeMediaURL(block.URL); src != "" {
				fmt.Fprintf(&out, `<p class="article-video"><a href="%s" target="_blank" rel="noopener noreferrer">Открыть видео ↗</a></p>`, html.EscapeString(src))
			}
		case "table":
			out.WriteString(`<div class="article-table"><table>`)
			for ri, row := range block.Rows {
				if ri == 0 {
					out.WriteString("<thead>")
				} else if ri == 1 {
					out.WriteString("</thead><tbody>")
				}
				out.WriteString("<tr>")
				for _, cell := range row {
					tag := "td"
					if ri == 0 {
						tag = "th"
					}
					fmt.Fprintf(&out, "<%s>%s</%s>", tag, safePublicationInlineHTML(cell), tag)
				}
				out.WriteString("</tr>")
			}
			out.WriteString("</tbody></table></div>")
		}
	}
	return out.String()
}

var publicationInlinePolicy = func() *bluemonday.Policy {
	policy := bluemonday.StrictPolicy()
	policy.AllowElements("b", "strong", "i", "em", "u", "code", "mark", "br", "a")
	policy.AllowAttrs("href").OnElements("a")
	policy.AllowStandardURLs()
	policy.RequireNoFollowOnLinks(true)
	policy.RequireNoReferrerOnLinks(true)
	policy.AddTargetBlankToFullyQualifiedLinks(true)
	return policy
}()

func safePublicationInlineHTML(value string) string {
	return publicationInlinePolicy.Sanitize(strings.TrimSpace(value))
}

func validatePublicationInput(in *publicationInput) error {
	in.Title = strings.Join(strings.Fields(in.Title), " ")
	if strings.TrimSpace(in.SEOTitle) == "" {
		in.SEOTitle = truncatePublicationSEO(in.Title, 60)
	}
	if strings.TrimSpace(in.SEODescription) == "" {
		description := strings.TrimSpace(in.Excerpt)
		if description == "" {
			description = strings.TrimSpace(in.Subtitle)
		}
		in.SEODescription = truncatePublicationSEO(description, 160)
	}
	in.Slug = slugify(in.Slug)
	if in.Slug == "" {
		in.Slug = slugify(in.Title)
	}
	if len([]rune(in.Title)) < 5 || len([]rune(in.Title)) > 240 {
		return errors.New("Заголовок должен содержать от 5 до 240 символов")
	}
	if in.Slug == "" {
		return errors.New("Не удалось сформировать URL публикации")
	}
	if len([]rune(in.Excerpt)) > 800 || len([]rune(in.SEODescription)) > 320 {
		return errors.New("Описание слишком длинное")
	}
	if len(in.Content) > 300 {
		return errors.New("В публикации слишком много блоков")
	}
	allowed := map[string]bool{"paragraph": true, "h2": true, "h3": true, "quote": true, "warning": true, "info": true, "bullets": true, "numbered": true, "checklist": true, "table": true, "code": true, "video": true, "avoid": true, "note": true, "example": true, "conclusion": true, "image": true, "divider": true}
	for _, b := range in.Content {
		if !allowed[b.Type] {
			return errors.New("Обнаружен неподдерживаемый блок")
		}
		if len([]rune(b.Text)) > 20000 {
			return errors.New("Один из блоков слишком большой")
		}
	}
	if !map[string]bool{"beginner": true, "medium": true, "advanced": true, "expert": true}[in.Difficulty] {
		in.Difficulty = "medium"
	}
	if !map[string]bool{"public": true, "unlisted": true, "draft": true}[in.Visibility] {
		in.Visibility = "draft"
	}
	if in.Language == "" {
		in.Language = "ru"
	}
	if in.ReadingTime < 1 {
		in.ReadingTime = int64(max(1, len(renderPublicationBlocks(in.Content))/1200))
	}
	if in.ReadingTime > 300 {
		in.ReadingTime = 300
	}
	in.CoverImage = safeMediaURL(in.CoverImage)
	return nil
}

func truncatePublicationSEO(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	short := strings.TrimSpace(string(runes[:limit-1]))
	if index := strings.LastIndex(short, " "); index > limit/2 {
		short = short[:index]
	}
	return strings.TrimSpace(short) + "…"
}

func optionalUserID(r *http.Request) int64 {
	u, err := userFromRequest(r)
	if err != nil {
		return 0
	}
	return u.ID
}

func requirePublicationSameOrigin(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		return true
	}
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || !strings.EqualFold(u.Host, r.Host) {
		writeJSON(w, http.StatusForbidden, "Запрос отклонён проверкой безопасности")
		return false
	}
	return true
}

func publicationViewerHash(r *http.Request, userID int64) string {
	identity := strconv.FormatInt(userID, 10)
	if userID == 0 {
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		identity = host + "|" + r.UserAgent() + "|" + r.Header.Get("Accept-Language")
	}
	sum := sha256.Sum256([]byte(identity))
	return hex.EncodeToString(sum[:])
}

func decodePublicationInput(w http.ResponseWriter, r *http.Request) (publicationInput, bool) {
	var raw struct {
		Title, Subtitle, Excerpt, CoverImage, Slug, SEOTitle, SEODescription, Difficulty, Language, Visibility, ChangeSummary string
		CategoryID, SeriesID, SeriesOrder, ReadingTime, TestID                                                                int64
		AllowComments                                                                                                         *bool
		Content                                                                                                               []publicationBlock
		SummaryPoints, Tags, Skills, Topics                                                                                   []string
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	if json.NewDecoder(r.Body).Decode(&raw) != nil {
		writeJSON(w, 400, "Некорректные данные публикации")
		return publicationInput{}, false
	}
	in := publicationInput{Title: raw.Title, Subtitle: raw.Subtitle, Excerpt: raw.Excerpt, CoverImage: raw.CoverImage, Slug: raw.Slug, SEOTitle: raw.SEOTitle, SEODescription: raw.SEODescription, Difficulty: raw.Difficulty, Language: raw.Language, Visibility: raw.Visibility, ChangeSummary: raw.ChangeSummary, CategoryID: raw.CategoryID, SeriesID: raw.SeriesID, SeriesOrder: raw.SeriesOrder, ReadingTime: raw.ReadingTime, TestID: raw.TestID, Content: raw.Content, SummaryPoints: raw.SummaryPoints, Tags: raw.Tags, Skills: raw.Skills, Topics: raw.Topics, AllowComments: true}
	if raw.AllowComments != nil {
		in.AllowComments = *raw.AllowComments
	}
	if err := validatePublicationInput(&in); err != nil {
		writeJSON(w, 400, err.Error())
		return publicationInput{}, false
	}
	return in, true
}

func scanJSONStrings(value []byte) []string {
	var result []string
	_ = json.Unmarshal(value, &result)
	return result
}

func dbErrorStatus(err error) (int, string) {
	if errors.Is(err, sql.ErrNoRows) {
		return http.StatusNotFound, "Публикация не найдена"
	}
	if strings.Contains(err.Error(), "23505") {
		return http.StatusConflict, "Такой URL уже используется"
	}
	return http.StatusInternalServerError, "Не удалось выполнить операцию"
}

// publicationExpertiseContribution is deliberately capped: publishing many weak
// materials cannot dominate the professional index. The result is recalculated
// from source metrics and is never stored as an opaque score.
func publicationExpertiseContribution(publications, useful, saves, usedAtWork, confirmedCurrent int64) float64 {
	quality := float64(min(useful, 500))*0.04 + float64(min(saves, 300))*0.025 + float64(min(usedAtWork, 200))*0.06
	activity := float64(min(publications, 12)) * 1.2
	relevance := float64(min(confirmedCurrent, publications)) * 0.8
	return min(30, quality+activity+relevance)
}
