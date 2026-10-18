package academy

import (
	test "FinTalent/internal/testmodule/domain"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

func (h *Handler) assessmentTx(r *http.Request, id, user int64, lesson, block string) (*sql.Tx, *Enrollment, *Assessment, error) {
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		return nil, nil, nil, err
	}
	e, err := loadEnrollment(r.Context(), tx, id, user, false, true)
	if err == nil && e.EffectiveStatus != "active" {
		err = ErrAccess
	}
	if err == nil {
		err = e.checkLesson(lesson)
	}
	var a *Assessment
	if err == nil {
		a, err = assessment(r.Context(), tx, e, lesson, block)
	}
	if err != nil {
		tx.Rollback()
		return nil, nil, nil, err
	}
	return tx, e, a, nil
}
func (h *Handler) learningAssessment(w http.ResponseWriter, r *http.Request, e *Enrollment, user int64, lesson, block string, tail []string) {
	tx, e, a, err := h.assessmentTx(r, e.ID, user, lesson, block)
	if err != nil {
		problem(w, err)
		return
	}
	defer tx.Rollback()
	finish := func() {
		if err = tx.Commit(); err != nil {
			problem(w, err)
			return
		}
		h.assessmentState(w, r, e, user, lesson, block)
	}
	if len(tail) == 0 && r.Method == "GET" {
		if a.Kind == "test" {
			attempts, er := testAttempts(r.Context(), tx, e.ID, a.ID)
			if er != nil {
				problem(w, er)
				return
			}
			if len(attempts) > 0 && attempts[0].Status == "started" && attempts[0].DeadlineAt != nil && !attempts[0].DeadlineAt.After(time.Now()) {
				if err = finishAcademyTest(r.Context(), tx, e, a, user, &attempts[0]); err != nil {
					problem(w, err)
					return
				}
			}
		}
		finish()
		return
	}
	if a.Kind == "assignment" {
		h.assignmentAction(w, r, tx, e, a, user, tail, finish)
		return
	}
	attempts, err := testAttempts(r.Context(), tx, e.ID, a.ID)
	if err != nil {
		problem(w, err)
		return
	}
	if len(tail) == 1 && tail[0] == "start" && r.Method == "POST" {
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
		if len(attempts) > 0 && attempts[0].Status == "started" {
			if attempts[0].DeadlineAt == nil || attempts[0].DeadlineAt.After(time.Now()) {
				finish()
				return
			}
			if err = finishAcademyTest(r.Context(), tx, e, a, user, &attempts[0]); err != nil {
				problem(w, err)
				return
			}
		}
		var existing bool
		if err = tx.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM academy_effective_test_attempts WHERE enrollment_id=$1 AND assessment_id=$2 AND request_key=$3)`, e.ID, a.ID, body.Key).Scan(&existing); err != nil {
			problem(w, err)
			return
		}
		if existing {
			finish()
			return
		}
		if len(attempts) >= a.Policy.MaxAttempts || len(attempts) > 0 && !yes(a.Policy.AllowRetry) {
			fail(w, 409, "Доступные попытки использованы")
			return
		}
		var id int64
		err = tx.QueryRowContext(r.Context(), `INSERT INTO academy_test_attempts(enrollment_id,program_version_id,assessment_id,number,request_key,deadline_at) VALUES($1,$2,$3,$4,$5,CASE WHEN $6>0 THEN clock_timestamp()+$6*INTERVAL '1 second' END) RETURNING id`, e.ID, e.VersionID, a.ID, len(attempts)+1, body.Key, a.Policy.TimeLimitSeconds).Scan(&id)
		if err == nil {
			err = touchLearningTx(r.Context(), tx, e, user, lesson)
		}
		if err == nil {
			err = assessmentEvent(r.Context(), tx, e, user, a, "test_started", id, nil)
		}
		if err != nil {
			problem(w, err)
			return
		}
		finish()
		return
	}
	if len(tail) == 3 && tail[0] == "attempts" {
		id, _ := strconv.ParseInt(tail[1], 10, 64)
		var attempt *AcademyTestAttempt
		for i := range attempts {
			if attempts[i].ID == id {
				attempt = &attempts[i]
			}
		}
		if attempt == nil {
			problem(w, sql.ErrNoRows)
			return
		}
		if tail[2] == "finish" && r.Method == "POST" {
			if err = finishAcademyTest(r.Context(), tx, e, a, user, attempt); err != nil {
				problem(w, err)
				return
			}
			finish()
			return
		}
		if tail[2] == "answer" && r.Method == "PUT" {
			if attempt.Status != "started" {
				fail(w, 409, "Попытка уже завершена")
				return
			}
			if attempt.DeadlineAt != nil && !attempt.DeadlineAt.After(time.Now()) {
				if err = finishAcademyTest(r.Context(), tx, e, a, user, attempt); err == nil {
					err = tx.Commit()
				}
				if err != nil {
					problem(w, err)
				} else {
					fail(w, 409, "Время истекло. Результат сохранён, обновите тест")
				}
				return
			}
			var body struct {
				TestAnswer
				Revision int64 `json:"revision"`
			}
			if !decode(w, r, &body) {
				return
			}
			if body.Revision != attempt.Revision {
				fail(w, 409, "Ответ изменён в другой вкладке. Обновите попытку")
				return
			}
			if err = validateTestAnswer(body.TestAnswer, a.Questions); err != nil {
				problem(w, err)
				return
			}
			found := false
			for i := range attempt.Answers {
				if attempt.Answers[i].QuestionID == body.QuestionID {
					attempt.Answers[i] = body.TestAnswer
					found = true
				}
			}
			if !found {
				attempt.Answers = append(attempt.Answers, body.TestAnswer)
			}
			raw, _ := json.Marshal(attempt.Answers)
			var result sql.Result
			result, err = tx.ExecContext(r.Context(), `UPDATE academy_test_attempts SET answers=$2,answer_revision=answer_revision+1 WHERE id=$1 AND status='started' AND (deadline_at IS NULL OR deadline_at>clock_timestamp())`, id, string(raw))
			if err == nil {
				n, _ := result.RowsAffected()
				if n != 1 {
					fail(w, 409, "Время истекло. Обновите результат теста")
					return
				}
			}
			if err == nil {
				err = touchLearningTx(r.Context(), tx, e, user, lesson)
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
func finishAcademyTest(ctx context.Context, tx *sql.Tx, e *Enrollment, a *Assessment, user int64, attempt *AcademyTestAttempt) error {
	if attempt.Status == "finished" {
		return nil
	}
	result := test.Grade(a.Questions, testAnswers(attempt.Answers), *a.Policy.PassingPercent)
	grades, _ := json.Marshal(result.Answers)
	_, err := tx.ExecContext(ctx, `UPDATE academy_test_attempts SET status='finished',finished_at=LEAST(clock_timestamp(),COALESCE(deadline_at,clock_timestamp())),score=$2,max_score=$3,percent=$4,passed=$5,grades=$6 WHERE id=$1`, attempt.ID, result.Score, result.MaxScore, result.Percent, result.Passed, string(grades))
	if err != nil {
		return err
	}
	if err = touchLearningTx(ctx, tx, e, user, a.LessonKey); err != nil {
		return err
	}
	if err = assessmentEvent(ctx, tx, e, user, a, "test_finished", attempt.ID, map[string]any{"percent": result.Percent, "passed": result.Passed}); err != nil {
		return err
	}
	if result.Passed {
		if err = notifyAssessment(ctx, tx, e, user, "test_passed", "Тест успешно пройден", a.Title, false); err != nil {
			return err
		}
	}
	return reconcileAssessmentTx(ctx, tx, e, user, a.LessonKey)
}
func (h *Handler) assessmentState(w http.ResponseWriter, r *http.Request, e *Enrollment, user int64, lesson, block string) {
	e, err := loadEnrollment(r.Context(), h.db, e.ID, user, false, false)
	if err != nil {
		problem(w, err)
		return
	}
	if e.EffectiveStatus != "active" {
		problem(w, ErrAccess)
		return
	}
	if err = e.checkLesson(lesson); err != nil {
		problem(w, err)
		return
	}
	a, err := assessment(r.Context(), h.db, e, lesson, block)
	if err != nil {
		problem(w, err)
		return
	}
	if a.Kind == "assignment" {
		history, err := submissions(r.Context(), h.db, e.ID, a.ID)
		if err != nil {
			problem(w, err)
			return
		}
		var files json.RawMessage
		err = h.db.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(jsonb_build_object('id',m.id,'submission_id',m.submission_id,'name',m.name,'mime',m.mime,'size',m.size) ORDER BY m.id),'[]') FROM academy_media_assets m JOIN academy_assignment_submissions s ON s.id=m.submission_id WHERE s.enrollment_id=$1 AND s.assessment_id=$2 AND m.deleted_at IS NULL`, e.ID, a.ID).Scan(&files)
		if err != nil {
			problem(w, err)
			return
		}
		respond(w, 200, map[string]any{"assessment": a, "submissions": history, "files": files, "deadline_at": func() *time.Time { at, _ := assignmentDeadline(e.Cohort, block); return at }(), "allow_late": func() bool { _, allow := assignmentDeadline(e.Cohort, block); return allow }()})
		return
	}
	attempts, err := testAttempts(r.Context(), h.db, e.ID, a.ID)
	if err != nil {
		problem(w, err)
		return
	}
	selected, _ := strconv.ParseInt(r.URL.Query().Get("attempt"), 10, 64)
	var active *AcademyTestAttempt
	for i := range attempts {
		if selected == 0 && i == 0 || attempts[i].ID == selected {
			active = &attempts[i]
		}
		if attempts[i].Status != "finished" || !a.Policy.ShowCorrect {
			attempts[i].Grades = nil
		}
	}
	questions := []test.Question{}
	if active != nil {
		raw, _ := json.Marshal(a.Questions)
		_ = json.Unmarshal(raw, &questions)
		test.PrepareAnswerOrder(questions, active.ID, true)
		for i := range questions {
			questions[i].Settings = nil
		}
		if active.Status != "finished" || !a.Policy.ShowCorrect {
			test.HideQuestionSolutions(questions)
		}
		if active.Status == "finished" && a.Policy.ShowExplanations {
			for i := range questions {
				for _, q := range a.Questions {
					if questions[i].ID == q.ID {
						questions[i].Explanation = q.Explanation
					}
				}
			}
		} else {
			for i := range questions {
				questions[i].Explanation = ""
			}
		}
	}
	respond(w, 200, map[string]any{"assessment": a, "attempts": attempts, "questions": questions, "question_count": len(a.Questions), "selected_attempt_id": func() int64 {
		if active != nil {
			return active.ID
		}
		return 0
	}(), "remaining_attempts": remainingTestAttempts(a.Policy, len(attempts))})
}
