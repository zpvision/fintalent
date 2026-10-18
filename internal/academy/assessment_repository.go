package academy

import (
	test "FinTalent/internal/testmodule/domain"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var ErrAssessmentUnprepared = errors.New("Проверка этой версии ещё не подготовлена. Преподавателю нужно подготовить проверки в кабинете курса")

// Capture a complete, MVCC-consistent test in one statement. Academy never writes
// professional attempts/statistics; both contexts use testmodule/domain.Grade.
func freezeAssessments(ctx context.Context, tx *sql.Tx, version int64, p Program, owner int64) error {
	for _, m := range p.Modules {
		if m.Status != "published" {
			continue
		}
		for _, l := range m.Lessons {
			if l.Status != "published" {
				continue
			}
			for _, b := range l.Blocks {
				if !oneOf(b.Type, "test", "assignment") {
					continue
				}
				var exists bool
				if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM academy_assessment_versions WHERE program_version_id=$1 AND lesson_key=$2 AND block_key=$3)`, version, l.Key, b.Key).Scan(&exists); err != nil {
					return err
				}
				if exists {
					continue
				}
				questions := json.RawMessage(`[]`)
				title := b.Data.Title
				if b.Type == "test" {
					if b.Data.TestVersionID == 0 {
						continue
					}
					var raw []byte
					err := tx.QueryRowContext(ctx, `SELECT jsonb_build_object('title',v.title,'questions',COALESCE((SELECT jsonb_agg(to_jsonb(q)||jsonb_build_object('answers',COALESCE((SELECT jsonb_agg(to_jsonb(a) ORDER BY a.sort_order,a.id) FROM test_answers a WHERE a.question_id=q.id),'[]')) ORDER BY q.sort_order,q.id) FROM test_questions q WHERE q.test_version_id=v.id),'[]')) FROM test_versions v JOIN tests t ON t.id=v.test_id WHERE v.id=$1 AND t.author_id=$2 AND t.deleted_at IS NULL AND t.status NOT IN('deleted','blocked','archived')`, b.Data.TestVersionID, owner).Scan(&raw)
					if err != nil {
						return inputError{"Тест недоступен владельцу курса. Выберите актуальную версию теста и зафиксируйте программу заново"}
					}
					var captured struct {
						Title     string          `json:"title"`
						Questions []test.Question `json:"questions"`
					}
					if err = json.Unmarshal(raw, &captured); err != nil {
						return err
					}
					if len(captured.Questions) == 0 {
						return inputError{"В прикреплённом тесте нет вопросов"}
					}
					for _, q := range captured.Questions {
						if !finite(q.Points) || q.Points <= 0 {
							return inputError{"У каждого вопроса теста должен быть положительный балл"}
						}
						correct := 0
						for _, a := range q.Answers {
							if a.IsCorrect {
								correct++
							}
						}
						if correct == 0 {
							return inputError{"Укажите правильные ответы в прикреплённом тесте"}
						}
					}
					title = captured.Title
					questions, _ = json.Marshal(captured.Questions)
				}
				if title == "" {
					title = "Практическое задание"
				}
				data := b.Data
				data.Assessment = new(AssessmentPolicy)
				*data.Assessment = policy(b)
				config, _ := json.Marshal(data)
				if _, err := tx.ExecContext(ctx, `INSERT INTO academy_assessment_versions(program_version_id,lesson_key,block_key,kind,config,questions,title) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(program_version_id,lesson_key,block_key) DO NOTHING`, version, l.Key, b.Key, b.Type, string(config), string(questions), title); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
func assessment(ctx context.Context, q queryer, e *Enrollment, lesson, block string) (*Assessment, error) {
	l, ok := findLesson(e.Program, lesson)
	if !ok {
		return nil, sql.ErrNoRows
	}
	found := false
	for _, b := range l.Blocks {
		found = found || b.Key == block && oneOf(b.Type, "test", "assignment")
	}
	if !found {
		return nil, sql.ErrNoRows
	}
	var a Assessment
	var config, questions []byte
	err := q.QueryRowContext(ctx, `SELECT id,program_version_id,lesson_key,block_key,kind,title,config,questions FROM academy_assessment_versions WHERE program_version_id=$1 AND lesson_key=$2 AND block_key=$3`, e.VersionID, lesson, block).Scan(&a.ID, &a.VersionID, &a.LessonKey, &a.BlockKey, &a.Kind, &a.Title, &config, &questions)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAssessmentUnprepared
	}
	if err != nil {
		return nil, err
	}
	var d BlockData
	if err = json.Unmarshal(config, &d); err != nil {
		return nil, err
	}
	a.Policy = policy(Block{Data: d})
	err = json.Unmarshal(questions, &a.Questions)
	return &a, err
}

type AcademyTestAttempt struct {
	CorrectionID int64                `json:"correction_id,omitempty"`
	ID           int64                `json:"id"`
	Number       int                  `json:"number"`
	Status       string               `json:"status"`
	Revision     int64                `json:"revision"`
	Answers      []TestAnswer         `json:"answers"`
	Grades       []test.AttemptAnswer `json:"grades,omitempty"`
	Score        float64              `json:"score"`
	MaxScore     float64              `json:"max_score"`
	Percent      float64              `json:"percent"`
	Passed       bool                 `json:"passed"`
	StartedAt    time.Time            `json:"started_at"`
	DeadlineAt   *time.Time           `json:"deadline_at"`
	FinishedAt   *time.Time           `json:"finished_at"`
	Remaining    int                  `json:"remaining_seconds"`
}
type Submission struct {
	CorrectionID    int64            `json:"correction_id,omitempty"`
	ID              int64            `json:"id"`
	Number          int              `json:"number"`
	Status          string           `json:"status"`
	Revision        int64            `json:"revision"`
	Answer          SubmissionAnswer `json:"answer"`
	CreatedAt       time.Time        `json:"created_at"`
	SubmittedAt     *time.Time       `json:"submitted_at"`
	Score           *float64         `json:"score"`
	Comment         string           `json:"comment"`
	ReviewedAt      *time.Time       `json:"reviewed_at"`
	ReviewerID      *int64           `json:"reviewer_id"`
	ReviewerName    string           `json:"reviewer_name"`
	CriterionScores json.RawMessage  `json:"criterion_scores"`
	ReviewSource    string           `json:"review_source"`
}

func testAttempts(ctx context.Context, db interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, eid, aid int64) ([]AcademyTestAttempt, error) {
	rows, err := db.QueryContext(ctx, `SELECT id,number,status,answer_revision,answers,grades,score,max_score,percent,passed,started_at,deadline_at,finished_at,correction_id FROM academy_effective_test_attempts WHERE enrollment_id=$1 AND assessment_id=$2 ORDER BY number DESC`, eid, aid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AcademyTestAttempt{}
	for rows.Next() {
		var a AcademyTestAttempt
		var answers, grades []byte
		if err = rows.Scan(&a.ID, &a.Number, &a.Status, &a.Revision, &answers, &grades, &a.Score, &a.MaxScore, &a.Percent, &a.Passed, &a.StartedAt, &a.DeadlineAt, &a.FinishedAt, &a.CorrectionID); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(answers, &a.Answers); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(grades, &a.Grades); err != nil {
			return nil, err
		}
		if a.DeadlineAt != nil {
			a.Remaining = max(0, int(time.Until(*a.DeadlineAt).Seconds()))
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
func submissions(ctx context.Context, db interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, eid, aid int64) ([]Submission, error) {
	rows, err := db.QueryContext(ctx, `SELECT s.id,s.number,s.status,s.revision,s.answer,s.created_at,s.submitted_at,s.score,s.comment,s.reviewed_at,s.reviewed_by,COALESCE(u.full_name,''),COALESCE((SELECT payload->'criteria' FROM academy_admin_corrections WHERE submission_id=s.id ORDER BY id DESC LIMIT 1),(SELECT criterion_scores FROM academy_submission_reviews WHERE submission_id=s.id ORDER BY id DESC LIMIT 1),'{}'),CASE WHEN s.correction_id>0 THEN 'administrative' ELSE COALESCE((SELECT source FROM academy_submission_reviews WHERE submission_id=s.id ORDER BY id DESC LIMIT 1),'') END,s.correction_id FROM academy_effective_assignment_submissions s LEFT JOIN users u ON u.id=COALESCE(s.reviewed_by,s.assigned_to) WHERE s.enrollment_id=$1 AND s.assessment_id=$2 ORDER BY s.number DESC`, eid, aid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Submission{}
	for rows.Next() {
		var s Submission
		var answer []byte
		if err = rows.Scan(&s.ID, &s.Number, &s.Status, &s.Revision, &answer, &s.CreatedAt, &s.SubmittedAt, &s.Score, &s.Comment, &s.ReviewedAt, &s.ReviewerID, &s.ReviewerName, &s.CriterionScores, &s.ReviewSource, &s.CorrectionID); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(answer, &s.Answer); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
func notifyAssessment(ctx context.Context, tx *sql.Tx, e *Enrollment, recipient int64, typ, title, body string, owner bool) error {
	entity := "academy_enrollment"
	id := e.ID
	if owner {
		entity = "academy_course"
		id = e.CourseID
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO notifications(user_id,type,title,body,entity_type,entity_id) VALUES($1,$2,$3,$4,$5,$6)`, recipient, "academy_"+typ, title, body, entity, id)
	return err
}
func assessmentEvent(ctx context.Context, tx *sql.Tx, e *Enrollment, actor int64, a *Assessment, typ string, id int64, data any) error {
	return event(ctx, tx, e, actor, typ, fmt.Sprintf("%s:%d", typ, id), a.LessonKey, map[string]any{"assessment_id": a.ID, "attempt_id": id, "details": data})
}
