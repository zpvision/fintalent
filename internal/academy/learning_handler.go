package academy

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (h *Handler) learner(w http.ResponseWriter, r *http.Request) {
	accessPath := r.URL.Path == "/api/academy/learning/access"
	user, err := h.user(r)
	if err != nil || user <= 0 {
		if accessPath && r.Method == "GET" {
			respond(w, 200, map[string]bool{"allowed": false})
		} else {
			fail(w, 401, "Войдите в аккаунт")
		}
		return
	}
	var valid bool
	err = h.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND NOT is_blocked AND NOT is_system)`, user).Scan(&valid)
	if err != nil {
		problem(w, err)
		return
	}
	if !valid {
		fail(w, 401, "Войдите в аккаунт")
		return
	}
	if accessPath && r.Method == "GET" {
		var allowed bool
		err = h.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM academy_enrollments WHERE user_id=$1)`, user).Scan(&allowed)
		if err != nil {
			problem(w, err)
			return
		}
		respond(w, 200, map[string]bool{"allowed": allowed})
		return
	}
	if strings.TrimSuffix(r.URL.Path, "/") == "/api/academy/learning" && r.Method == "GET" {
		h.enrollmentList(w, r, user, false, 0)
		return
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/academy/enrollments/"), "/"), "/")
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id <= 0 {
		problem(w, sql.ErrNoRows)
		return
	}
	e, err := loadEnrollment(r.Context(), h.db, id, user, false, false)
	if err != nil {
		problem(w, err)
		return
	}
	if len(parts) == 1 && r.Method == "GET" {
		if err = h.progress(r.Context(), e); err != nil {
			problem(w, err)
			return
		}
		h.room(w, r, e)
		return
	}
	if len(parts) > 1 && parts[1] == "progress" && r.Method == "GET" {
		if err = h.progress(r.Context(), e); err != nil {
			problem(w, err)
			return
		}
		respond(w, 200, e)
		return
	}
	if e.EffectiveStatus != "active" {
		problem(w, ErrAccess)
		return
	}
	if len(parts) > 2 && parts[1] == "assets" {
		asset, _ := strconv.ParseInt(parts[2], 10, 64)
		lesson, ok := findLesson(e.Program, r.URL.Query().Get("lesson"))
		if !ok || !lessonAssets(lesson)[asset] {
			problem(w, sql.ErrNoRows)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			fail(w, 405, "Метод не поддерживается")
			return
		}
		h.serveAsset(w, r, e.ProgramID, asset)
		return
	}
	if len(parts) > 2 && parts[1] == "lessons" {
		key := parts[2]
		l, ok := findLesson(e.Program, key)
		if !ok {
			problem(w, sql.ErrNoRows)
			return
		}
		if r.Method == "GET" && len(parts) == 3 {
			respond(w, 200, map[string]any{"lesson": l, "completion_block": CompletionBlock(l), "next": nextLesson(e.Program, key)})
			return
		}
		if r.Method == "POST" && len(parts) == 4 && oneOf(parts[3], "open", "complete") {
			var v struct {
				EventKey     string `json:"event_key"`
				Acknowledged bool   `json:"materials_acknowledged"`
			}
			if !decode(w, r, &v) {
				return
			}
			if parts[3] == "open" && !keyPattern.MatchString(v.EventKey) {
				fail(w, 400, "Некорректный ключ события")
				return
			}
			e, err = h.recordProgress(r.Context(), id, user, key, parts[3], v.EventKey, v.Acknowledged)
			if err != nil {
				problem(w, err)
				return
			}
			respond(w, 200, e)
			return
		}
	}
	fail(w, 405, "Метод не поддерживается")
}
func nextLesson(p Program, key string) string {
	found := false
	for _, m := range p.Modules {
		if m.Status != "published" {
			continue
		}
		for _, l := range m.Lessons {
			if l.Status != "published" || !l.SelfPaced {
				continue
			}
			if found {
				return l.Key
			}
			if l.Key == key {
				found = true
			}
		}
	}
	return ""
}
func resumeLesson(p Program, last string, progress []Progress) string {
	done := map[string]bool{}
	for _, v := range progress {
		done[v.LessonKey] = v.Status == "completed"
	}
	if _, ok := findLesson(p, last); ok && !done[last] {
		return last
	}
	keys := []string{}
	for _, m := range p.Modules {
		if m.Status != "published" {
			continue
		}
		for _, l := range m.Lessons {
			if l.Status == "published" && l.SelfPaced {
				keys = append(keys, l.Key)
			}
		}
	}
	after := false
	for _, key := range keys {
		if after && !done[key] {
			return key
		}
		if key == last {
			after = true
		}
	}
	for _, key := range keys {
		if !done[key] {
			return key
		}
	}
	if len(keys) > 0 {
		return keys[0]
	}
	return ""
}
func (h *Handler) room(w http.ResponseWriter, r *http.Request, e *Enrollment) {
	if e.EffectiveStatus != "active" {
		respond(w, 200, map[string]any{"enrollment": e, "program": nil})
		return
	}
	p := learningProgram(e.Program)
	assets, err := h.roomAssets(r, e.ProgramID, p)
	if err != nil {
		problem(w, err)
		return
	}
	blocks := map[string]string{}
	for i := range p.Modules {
		for j := range p.Modules[i].Lessons {
			l := &p.Modules[i].Lessons[j]
			blocks[l.Key] = CompletionBlock(*l)
			if !l.SelfPaced {
				l.Blocks = []Block{}
				l.Description = ""
				l.CoverAssetID = 0
			}
		}
	}
	respond(w, 200, map[string]any{"enrollment": e, "program": p, "assets": assets, "tests": e.Tests, "completion_blocks": blocks, "resume_lesson_key": resumeLesson(e.Program, e.LastLessonKey, e.Progress)})
}
func (h *Handler) roomAssets(r *http.Request, program int64, p Program) (json.RawMessage, error) {
	ids := []int64{}
	seen := map[int64]bool{}
	for _, m := range p.Modules {
		if m.Status != "published" {
			continue
		}
		for _, l := range m.Lessons {
			if l.Status != "published" || !l.SelfPaced {
				continue
			}
			for id := range lessonAssets(l) {
				if !seen[id] {
					seen[id] = true
					ids = append(ids, id)
				}
			}
		}
	}
	raw, _ := json.Marshal(ids)
	var result json.RawMessage
	err := h.db.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(jsonb_build_object('id',id,'name',name,'mime',mime,'size',size) ORDER BY id),'[]') FROM academy_media_assets WHERE program_id=$1 AND deleted_at IS NULL AND id IN(SELECT value::bigint FROM jsonb_array_elements_text($2::jsonb))`, program, string(raw)).Scan(&result)
	return result, err
}
func (h *Handler) previewRoom(w http.ResponseWriter, r *http.Request, course int64, p Program, tests json.RawMessage) {
	var title, cover, author string
	err := h.db.QueryRowContext(r.Context(), `SELECT c.title,c.cover_image,u.full_name FROM courses c JOIN users u ON u.id=c.owner_id WHERE c.id=$1`, course).Scan(&title, &cover, &author)
	if err != nil {
		problem(w, err)
		return
	}
	assets, err := h.roomAssets(r, p.ID, p)
	if err != nil {
		problem(w, err)
		return
	}
	view := learningProgram(p)
	blocks := map[string]string{}
	for i := range view.Modules {
		for j := range view.Modules[i].Lessons {
			l := &view.Modules[i].Lessons[j]
			blocks[l.Key] = CompletionBlock(*l)
			if !l.SelfPaced {
				l.Blocks = []Block{}
				l.Description = ""
				l.CoverAssetID = 0
			}
		}
	}
	respond(w, 200, map[string]any{"preview": true, "course": map[string]any{"id": course, "title": title, "cover_image": cover, "author_name": author}, "program": view, "assets": assets, "tests": tests, "completion_blocks": blocks, "resume_lesson_key": resumeLesson(p, "", nil)})
}
func (h *Handler) enrollmentList(w http.ResponseWriter, r *http.Request, user int64, owner bool, course int64) {
	before := int64(9223372036854775807)
	if value := r.URL.Query().Get("before"); value != "" {
		var err error
		before, err = strconv.ParseInt(value, 10, 64)
		if err != nil || before <= 0 {
			fail(w, 400, "Некорректный курсор списка")
			return
		}
	}
	// One query for the collection, version contents and progress; no per-student DB round trips.
	var raw []byte
	err := h.db.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(x.payload ORDER BY x.id DESC),'[]') FROM (SELECT e.id,COALESCE(e.last_activity_at,e.enrolled_at) activity,to_jsonb(e)||jsonb_build_object('title',c.title,'cover_image',c.cover_image,'author_name',author.full_name,'user_name',student.full_name,'avatar',COALESCE(student.avatar_url,''),'version_number',v.number,'program_content',v.content,'unavailable',author.is_blocked OR student.is_blocked OR student.is_system,'progress',COALESCE((SELECT jsonb_agg(jsonb_build_object('lesson_key',p.lesson_key,'status',p.status,'completed_at',p.completed_at) ORDER BY p.lesson_key) FROM academy_lesson_progress p WHERE p.enrollment_id=e.id AND p.program_version_id=e.program_version_id),'[]')) payload FROM academy_enrollments e JOIN courses c ON c.id=e.course_id JOIN users author ON author.id=c.owner_id JOIN users student ON student.id=e.user_id JOIN academy_program_versions v ON v.id=e.program_version_id WHERE e.id<$4 AND (($2 AND c.owner_id=$1 AND c.id=$3) OR (NOT $2 AND e.user_id=$1)) ORDER BY e.id DESC LIMIT 200) x`, user, owner, course, before).Scan(&raw)
	if err != nil {
		problem(w, err)
		return
	}
	var rows []struct {
		Enrollment
		Content     Program `json:"program_content"`
		Unavailable bool    `json:"unavailable"`
	}
	if err = json.Unmarshal(raw, &rows); err != nil {
		problem(w, err)
		return
	}
	items := []Enrollment{}
	for _, row := range rows {
		e := row.Enrollment
		e.EffectiveStatus = e.AccessStatus
		if row.Unavailable {
			e.EffectiveStatus = "suspended"
		} else if e.AccessStatus == "active" && e.ExpiresAt != nil && !e.ExpiresAt.After(time.Now()) {
			e.EffectiveStatus = "expired"
		}
		e.Summary = SummaryFor(row.Content, e.Progress, e.StartedAt != nil)
		if !owner {
			e.Progress = nil
		}
		items = append(items, e)
	}
	respond(w, 200, items)
}
