package academy

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/microcosm-cc/bluemonday"
)

type Program struct {
	ID       int64    `json:"id"`
	CourseID int64    `json:"course_id"`
	Revision int64    `json:"revision"`
	Source   string   `json:"source"`
	Modules  []Module `json:"modules"`
}
type Module struct {
	ID           int64    `json:"id"`
	Key          string   `json:"key"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Status       string   `json:"status"`
	CoverAssetID int64    `json:"cover_asset_id"`
	Lessons      []Lesson `json:"lessons"`
}
type Lesson struct {
	ID             int64   `json:"id"`
	Key            string  `json:"key"`
	Title          string  `json:"title"`
	Description    string  `json:"description"`
	Duration       int     `json:"duration_minutes"`
	Status         string  `json:"status"`
	Required       bool    `json:"required"`
	FreePreview    bool    `json:"free_preview"`
	SelfPaced      bool    `json:"self_paced"`
	CompletionRule string  `json:"completion_rule"`
	CoverAssetID   int64   `json:"cover_asset_id"`
	Blocks         []Block `json:"blocks"`
}
type Block struct {
	ID   int64     `json:"id"`
	Key  string    `json:"key"`
	Type string    `json:"type"`
	Data BlockData `json:"data"`
}
type BlockData struct {
	Title         string      `json:"title,omitempty"`
	Text          string      `json:"text,omitempty"`
	URL           string      `json:"url,omitempty"`
	Caption       string      `json:"caption,omitempty"`
	Variant       string      `json:"variant,omitempty"`
	AnswerFormat  string      `json:"answer_format,omitempty"`
	Duration      int         `json:"duration_minutes,omitempty"`
	AssetID       int64       `json:"asset_id,omitempty"`
	PosterID      int64       `json:"poster_id,omitempty"`
	MaterialIDs   []int64     `json:"material_ids,omitempty"`
	TestVersionID int64       `json:"test_version_id,omitempty"`
	Rich          []RichBlock `json:"rich,omitempty"`
}
type RichBlock struct {
	Type  string     `json:"type"`
	Text  string     `json:"text,omitempty"`
	Items []string   `json:"items,omitempty"`
	Rows  [][]string `json:"rows,omitempty"`
}

func Key() string { return rand.Text() }

var keyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,80}$`)
var inlinePolicy = func() *bluemonday.Policy {
	p := bluemonday.StrictPolicy()
	p.AllowElements("b", "strong", "i", "em", "u", "code", "mark", "br", "a")
	p.AllowAttrs("href").OnElements("a")
	p.AllowURLSchemes("https", "http", "mailto")
	p.RequireNoFollowOnLinks(true)
	p.RequireNoReferrerOnLinks(true)
	p.AddTargetBlankToFullyQualifiedLinks(true)
	return p
}()

func validURL(s string) bool {
	u, e := url.Parse(s)
	return e == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil && len(s) <= 2000
}

// No requests to author-controlled URLs: only browsers load these sources in preview.
func VideoSource(s string) (string, string) {
	u, e := url.Parse(s)
	if e != nil || !validURL(s) {
		return "", ""
	}
	host := strings.ToLower(u.Hostname())
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	safe := regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	if host == "youtu.be" && len(parts) == 1 && safe.MatchString(parts[0]) {
		return "embed", "https://www.youtube-nocookie.com/embed/" + parts[0]
	}
	if host == "www.youtube.com" || host == "youtube.com" {
		id := u.Query().Get("v")
		if len(parts) == 2 && parts[0] == "embed" {
			id = parts[1]
		}
		if safe.MatchString(id) {
			return "embed", "https://www.youtube-nocookie.com/embed/" + id
		}
	}
	if (host == "vimeo.com" || host == "player.vimeo.com") && len(parts) > 0 && regexp.MustCompile(`^[0-9]+$`).MatchString(parts[len(parts)-1]) {
		return "embed", "https://player.vimeo.com/video/" + parts[len(parts)-1]
	}
	if (host == "rutube.ru" || host == "www.rutube.ru") && len(parts) >= 2 && (parts[0] == "video" || parts[0] == "play") {
		id := parts[len(parts)-1]
		if safe.MatchString(id) {
			return "embed", "https://rutube.ru/play/embed/" + id
		}
	}
	if strings.HasSuffix(strings.ToLower(u.Path), ".mp4") || strings.HasSuffix(strings.ToLower(u.Path), ".webm") {
		return "video", s
	}
	return "", ""
}
func Validate(p *Program) error {
	if len(p.Modules) > 100 {
		return fmt.Errorf("Допустимо до 100 модулей")
	}
	seen := map[string]bool{}
	total := 0
	entity := func(id int64, key, title, status string) error {
		if id < 0 || !keyPattern.MatchString(key) || seen[key] {
			return fmt.Errorf("Некорректный идентификатор или повтор элемента")
		}
		seen[key] = true
		if utf8.RuneCountInString(title) > 180 || (status != "draft" && status != "published") {
			return fmt.Errorf("Название — до 180 символов; статус — черновик или опубликован")
		}
		if status == "published" && strings.TrimSpace(title) == "" {
			return fmt.Errorf("Перед публикацией укажите название")
		}
		return nil
	}
	for i := range p.Modules {
		m := &p.Modules[i]
		m.Title = strings.TrimSpace(m.Title)
		if e := entity(m.ID, m.Key, m.Title, m.Status); e != nil {
			return e
		}
		if len(m.Description) > 12000 || utf8.RuneCountInString(m.Description) > 1200 || m.CoverAssetID < 0 {
			return fmt.Errorf("Проверьте описание и обложку модуля")
		}
		for j := range m.Lessons {
			l := &m.Lessons[j]
			l.Title = strings.TrimSpace(l.Title)
			total++
			if e := entity(l.ID, l.Key, l.Title, l.Status); e != nil {
				return e
			}
			if utf8.RuneCountInString(l.Description) > 1200 || l.Duration < 0 || l.Duration > 100000 || l.CoverAssetID < 0 || len(l.Blocks) > 100 {
				return fmt.Errorf("Проверьте описание, длительность и блоки урока")
			}
			if !oneOf(l.CompletionRule, "manual", "materials", "test", "assignment") {
				return fmt.Errorf("Выберите правило завершения урока")
			}
			for k := range l.Blocks {
				b := &l.Blocks[k]
				if e := entity(b.ID, b.Key, "", "draft"); e != nil {
					return e
				}
				if e := validateBlock(b); e != nil {
					return e
				}
			}
		}
	}
	if total > 1000 {
		return fmt.Errorf("Допустимо до 1000 уроков")
	}
	return nil
}
func oneOf(v string, values ...string) bool {
	for _, s := range values {
		if v == s {
			return true
		}
	}
	return false
}
func validateBlock(b *Block) error {
	if !oneOf(b.Type, "text", "video", "image", "file", "link", "note", "test", "assignment") {
		return fmt.Errorf("Неизвестный тип блока")
	}
	d := &b.Data
	if utf8.RuneCountInString(d.Title) > 240 || utf8.RuneCountInString(d.Text) > 20000 || utf8.RuneCountInString(d.Caption) > 500 || d.Duration < 0 || d.Duration > 100000 || len(d.MaterialIDs) > 20 || d.AssetID < 0 || d.PosterID < 0 || d.TestVersionID < 0 {
		return fmt.Errorf("Слишком большой блок или некорректные параметры")
	}
	if d.URL != "" && !validURL(d.URL) {
		return fmt.Errorf("Используйте безопасную HTTPS-ссылку")
	}
	if b.Type == "video" && d.URL != "" {
		if kind, _ := VideoSource(d.URL); kind == "" {
			return fmt.Errorf("Видео: YouTube, Vimeo, Rutube или HTTPS-ссылка на MP4/WebM")
		}
	}
	if b.Type == "note" && !oneOf(d.Variant, "info", "tip", "important", "warning", "example") {
		return fmt.Errorf("Выберите вид примечания")
	}
	if b.Type == "assignment" && !oneOf(d.AnswerFormat, "text", "file", "link") {
		return fmt.Errorf("Выберите формат ответа")
	}
	if len(d.Rich) > 200 {
		return fmt.Errorf("Разделите длинный текст на несколько блоков")
	}
	for i := range d.Rich {
		r := &d.Rich[i]
		if !oneOf(r.Type, "paragraph", "h2", "h3", "bullets", "numbered", "checklist", "quote", "code", "table", "divider") {
			return fmt.Errorf("Недопустимый элемент текста")
		}
		if len(r.Text) > 100000 || len(r.Items) > 200 || len(r.Rows) > 100 {
			return fmt.Errorf("Слишком большой текст")
		}
		r.Text = inlinePolicy.Sanitize(r.Text)
		for j := range r.Items {
			if len(r.Items[j]) > 20000 {
				return fmt.Errorf("Слишком длинный пункт списка")
			}
			r.Items[j] = inlinePolicy.Sanitize(r.Items[j])
		}
		for j := range r.Rows {
			if len(r.Rows[j]) > 20 {
				return fmt.Errorf("До 20 столбцов таблицы")
			}
			for k := range r.Rows[j] {
				if len(r.Rows[j][k]) > 20000 {
					return fmt.Errorf("Слишком большая ячейка")
				}
				r.Rows[j][k] = inlinePolicy.Sanitize(r.Rows[j][k])
			}
		}
	}
	raw, _ := json.Marshal(d)
	if len(raw) > 500000 {
		return fmt.Errorf("Слишком большой блок")
	}
	return nil
}
