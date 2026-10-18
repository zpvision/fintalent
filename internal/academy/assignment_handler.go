package academy

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (h *Handler) assignmentAction(w http.ResponseWriter, r *http.Request, tx *sql.Tx, e *Enrollment, a *Assessment, user int64, tail []string, finish func()) {
	history, err := submissions(r.Context(), tx, e.ID, a.ID)
	if err != nil {
		problem(w, err)
		return
	}
	if len(tail) == 1 && tail[0] == "draft" && r.Method == "POST" {
		var body struct {
			Key string `json:"request_key"`
		}
		if !decode(w, r, &body) {
			return
		}
		if !keyPattern.MatchString(body.Key) {
			problem(w, ErrInvalid)
			return
		}
		for _, s := range history {
			if s.Status == "draft" {
				finish()
				return
			}
		}
		var exists bool
		if err = tx.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM academy_effective_assignment_submissions WHERE enrollment_id=$1 AND assessment_id=$2 AND request_key=$3)`, e.ID, a.ID, body.Key).Scan(&exists); err != nil {
			problem(w, err)
			return
		}
		if exists {
			finish()
			return
		}
		if len(history) > 0 {
			previous := history[0]
			if oneOf(previous.Status, "submitted", "in_review", "accepted") {
				fail(w, 409, "Предыдущая работа уже принята или находится на проверке")
				return
			}
			if previous.Status == "revision_requested" && !yes(a.Policy.AllowRevision) || previous.Status == "rejected" && !yes(a.Policy.AllowRetry) {
				fail(w, 409, "Повторная отправка не разрешена автором")
				return
			}
		}
		if len(history) >= a.Policy.MaxAttempts {
			fail(w, 409, "Доступные попытки отправки использованы")
			return
		}
		var id int64
		err = tx.QueryRowContext(r.Context(), `INSERT INTO academy_assignment_submissions(enrollment_id,program_version_id,assessment_id,number,request_key,answer) VALUES($1,$2,$3,$4,$5,$6) RETURNING id`, e.ID, e.VersionID, a.ID, len(history)+1, body.Key, func() string {
			if len(history) == 0 {
				return `{"text":"","links":[],"asset_ids":[]}`
			}
			v := history[0].Answer
			v.AssetIDs = []int64{}
			raw, _ := json.Marshal(v)
			return string(raw)
		}()).Scan(&id)
		if err == nil {
			err = touchLearningTx(r.Context(), tx, e, user, a.LessonKey)
		}
		if err != nil {
			problem(w, err)
			return
		}
		finish()
		return
	}
	if len(tail) >= 2 && tail[0] == "submissions" {
		id, _ := strconv.ParseInt(tail[1], 10, 64)
		var s *Submission
		for i := range history {
			if history[i].ID == id {
				s = &history[i]
			}
		}
		if s == nil {
			problem(w, sql.ErrNoRows)
			return
		}
		if len(tail) >= 3 && tail[2] == "files" {
			h.submissionFile(w, r, tx, e, a, user, s, tail[3:], false, finish)
			return
		}
		if len(tail) == 2 && r.Method == "PUT" {
			if s.Status != "draft" {
				fail(w, 409, "Отправленная работа не изменяется. Создайте новую попытку")
				return
			}
			var body struct {
				Answer   SubmissionAnswer `json:"answer"`
				Revision int64            `json:"revision"`
			}
			if !decode(w, r, &body) {
				return
			}
			if body.Revision != s.Revision {
				fail(w, 409, "Черновик изменён. Обновите задание, чтобы не затереть ответ")
				return
			}
			if err = validateSubmission(&body.Answer, a.Policy, false); err == nil {
				err = validateSubmissionFiles(r.Context(), tx, id, body.Answer.AssetIDs)
			}
			if err != nil {
				problem(w, err)
				return
			}
			raw, _ := json.Marshal(body.Answer)
			_, err = tx.ExecContext(r.Context(), `UPDATE academy_assignment_submissions SET answer=$2,revision=revision+1,updated_at=NOW() WHERE id=$1`, id, string(raw))
			if err == nil {
				_, err = tx.ExecContext(r.Context(), `UPDATE academy_media_assets SET deleted_at=NOW() WHERE submission_id=$1 AND deleted_at IS NULL AND id NOT IN(SELECT value::bigint FROM jsonb_array_elements_text($2::jsonb))`, id, jsonValue(body.Answer.AssetIDs))
			}
			if err == nil {
				err = touchLearningTx(r.Context(), tx, e, user, a.LessonKey)
			}
			if err != nil {
				problem(w, err)
				return
			}
			finish()
			return
		}
		if len(tail) == 3 && tail[2] == "submit" && r.Method == "POST" {
			if s.Status != "draft" {
				finish()
				return
			} // Retrying the same submission never creates another attempt.
			var body struct {
				Revision int64 `json:"revision"`
			}
			if !decode(w, r, &body) {
				return
			}
			due, late := assignmentDeadline(e.Cohort, a.BlockKey)
			if due != nil && !late && !due.After(time.Now()) {
				fail(w, 403, "Срок сдачи задания истёк. Поздняя отправка отключена преподавателем")
				return
			}
			if body.Revision != s.Revision {
				fail(w, 409, "Черновик изменён. Обновите задание перед отправкой")
				return
			}
			if err = validateSubmission(&s.Answer, a.Policy, true); err == nil {
				err = validateSubmissionFiles(r.Context(), tx, id, s.Answer.AssetIDs)
			}
			if err != nil {
				problem(w, err)
				return
			}
			_, err = tx.ExecContext(r.Context(), `UPDATE academy_assignment_submissions SET status='submitted',submitted_at=NOW(),updated_at=NOW(),revision=revision+1,submitted_deadline_at=$2,is_late=($2::timestamptz IS NOT NULL AND clock_timestamp()>$2) WHERE id=$1`, id, due)
			if err == nil {
				err = touchLearningTx(r.Context(), tx, e, user, a.LessonKey)
			}
			if err == nil {
				err = assessmentEvent(r.Context(), tx, e, user, a, "assignment_submitted", id, nil)
			}
			if err == nil {
				var owner int64
				err = tx.QueryRowContext(r.Context(), `SELECT owner_id FROM courses WHERE id=$1`, e.CourseID).Scan(&owner)
				if err == nil {
					err = notifyAssessment(r.Context(), tx, e, owner, "submission", "Новая работа на проверку", a.Title, true)
				}
			}
			if err == nil && a.Policy.ReviewMode == "automatic" {
				comment := "Автоматически принят формат ответа. Содержательная оценка работы не проводилась."
				_, err = tx.ExecContext(r.Context(), `UPDATE academy_assignment_submissions SET status='accepted',comment=$2,reviewed_at=NOW(),revision=revision+1 WHERE id=$1`, id, comment)
				if err == nil {
					_, err = tx.ExecContext(r.Context(), `INSERT INTO academy_submission_reviews(submission_id,source,action,comment,request_key) VALUES($1,'automatic','accepted',$2,'automatic')`, id, comment)
				}
				if err == nil {
					err = assessmentEvent(r.Context(), tx, e, user, a, "assignment_accepted", id, map[string]string{"source": "automatic"})
				}
				if err == nil {
					err = notifyAssessment(r.Context(), tx, e, user, "accepted", "Задание принято", a.Title, false)
				}
				if err == nil {
					err = reconcileAssessmentTx(r.Context(), tx, e, user, a.LessonKey)
				}
			}
			if err != nil {
				problem(w, err)
				return
			}
			finish()
			return
		}
	}
	fail(w, 405, "Метод не поддерживается")
}
func jsonValue(v any) string { raw, _ := json.Marshal(v); return string(raw) }
func validateSubmissionFiles(ctx context.Context, tx *sql.Tx, id int64, ids []int64) error {
	seen := map[int64]bool{}
	for _, id := range ids {
		if id <= 0 || seen[id] {
			return ErrInvalid
		}
		seen[id] = true
	}
	var count int
	err := tx.QueryRowContext(ctx, `SELECT count(*) FROM academy_media_assets WHERE submission_id=$1 AND deleted_at IS NULL AND id IN(SELECT value::bigint FROM jsonb_array_elements_text($2::jsonb))`, id, jsonValue(ids)).Scan(&count)
	if err != nil {
		return err
	}
	if count != len(ids) {
		return ErrInvalid
	}
	return nil
}
func (h *Handler) submissionFile(w http.ResponseWriter, r *http.Request, tx *sql.Tx, e *Enrollment, a *Assessment, user int64, s *Submission, tail []string, owner bool, finish func()) {
	if len(tail) == 0 && r.Method == "POST" && !owner {
		if s.Status != "draft" || !strings.Contains("|"+strings.Join(a.Policy.Formats, "|")+"|", "|file|") {
			fail(w, 409, "Прикрепить файл можно только к черновику с файловым форматом ответа")
			return
		}
		if len(s.Answer.AssetIDs) >= 10 {
			fail(w, 400, "До 10 файлов в одном ответе")
			return
		}
		if err := parseUpload(w, r); err != nil {
			fail(w, 400, err.Error())
			return
		}
		defer r.MultipartForm.RemoveAll()
		upload, err := h.storeUpload(r)
		if err != nil {
			problem(w, err)
			return
		}
		committed := false
		defer func() {
			if !committed {
				upload.remove()
			}
		}()
		var id int64
		err = tx.QueryRowContext(r.Context(), `INSERT INTO academy_media_assets(program_id,entity_key,storage_key,name,mime,size,created_by,submission_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`, e.ProgramID, a.BlockKey, upload.storage, upload.name, upload.mime, upload.size, user, s.ID).Scan(&id)
		if err == nil {
			s.Answer.AssetIDs = append(s.Answer.AssetIDs, id)
			_, err = tx.ExecContext(r.Context(), `UPDATE academy_assignment_submissions SET answer=$2,revision=revision+1,updated_at=NOW() WHERE id=$1`, s.ID, jsonValue(s.Answer))
		}
		if err != nil {
			problem(w, err)
			return
		}
		if err = tx.Commit(); err != nil {
			problem(w, err)
			return
		}
		committed = true
		h.assessmentState(w, r, e, user, a.LessonKey, a.BlockKey)
		return
	}
	if len(tail) == 1 && (r.Method == "GET" || r.Method == "HEAD") {
		id, _ := strconv.ParseInt(tail[0], 10, 64)
		found := false
		for _, v := range s.Answer.AssetIDs {
			found = found || v == id
		}
		if !found {
			problem(w, sql.ErrNoRows)
			return
		}
		var storage, name, mime string
		err := tx.QueryRowContext(r.Context(), `SELECT storage_key,name,mime FROM academy_media_assets WHERE id=$1 AND submission_id=$2 AND deleted_at IS NULL`, id, s.ID).Scan(&storage, &name, &mime)
		if err != nil {
			problem(w, err)
			return
		}
		if err = tx.Commit(); err != nil {
			problem(w, err)
			return
		}
		h.sendAssetFile(w, r, storage, name, mime)
		return
	}
	fail(w, 405, "Метод не поддерживается")
}
