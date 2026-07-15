package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"FinTalent/internal/testmodule/domain"
	"FinTalent/internal/testmodule/dto"
)

var ErrNotFound = errors.New("not found")
var ErrForbidden = errors.New("forbidden")

type Repository interface {
	List(context.Context, dto.ListFilter, int64, bool) ([]domain.Test, error)
	Get(context.Context, int64, bool) (*domain.Test, error)
	GetVersion(context.Context, int64, int64, bool) (*domain.Test, error)
	Create(context.Context, int64, dto.CreateTest, string) (*domain.Test, error)
	Update(context.Context, int64, int64, dto.UpdateTest) error
	SoftDelete(context.Context, int64, int64, bool) error
	Moderate(context.Context, int64, string, string) error
	AddQuestion(context.Context, int64, int64, dto.CreateQuestion) (int64, error)
	UpdateQuestion(context.Context, int64, int64, dto.CreateQuestion) error
	DeleteQuestion(context.Context, int64, int64) error
	AddAnswer(context.Context, int64, int64, dto.AnswerInput) (int64, error)
	UpdateAnswer(context.Context, int64, int64, dto.AnswerInput) error
	DeleteAnswer(context.Context, int64, int64) error
	Publish(context.Context, int64, int64) error
	ForkDraft(context.Context, int64, int64) error
	StartAttempt(context.Context, int64, int64) (*domain.Attempt, error)
	GetAttempt(context.Context, int64) (*domain.Attempt, error)
	SaveAttemptAnswer(context.Context, int64, dto.SubmitAnswer) error
	FinishAttempt(context.Context, int64, float64, float64, float64, bool, []domain.AttemptAnswer) error
	ListAttempts(context.Context, int64, int64, bool) ([]domain.Attempt, error)
	Statistics(context.Context, int64) (*domain.Statistics, error)
}

type Postgres struct{ db *sql.DB }

func New(db *sql.DB) *Postgres { return &Postgres{db: db} }

const testSelect = `SELECT t.id,t.author_id,u.full_name,v.title,v.description,t.slug,t.category,t.difficulty,t.status,t.visibility,
	t.price,t.currency,t.is_free,t.current_version,t.passing_percent,t.time_limit_seconds,
	(SELECT COUNT(*) FROM test_questions q WHERE q.test_version_id=v.id),COALESCE(s.attempts_count,0),COALESCE(s.average_percent,0),t.created_at,t.updated_at
	FROM tests t JOIN users u ON u.id=t.author_id JOIN test_versions v ON v.test_id=t.id AND v.version=t.current_version
	LEFT JOIN test_statistics s ON s.test_id=t.id `

func scanTest(scanner interface{ Scan(...any) error }) (*domain.Test, error) {
	var t domain.Test
	var limit sql.NullInt64
	err := scanner.Scan(&t.ID, &t.AuthorID, &t.AuthorName, &t.Title, &t.Description, &t.Slug, &t.Category, &t.Difficulty, &t.Status, &t.Visibility, &t.Price, &t.Currency, &t.IsFree, &t.Version, &t.PassingPercent, &limit, &t.QuestionCount, &t.AttemptsCount, &t.AveragePercent, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if limit.Valid {
		x := int(limit.Int64)
		t.TimeLimitSeconds = &x
	}
	return &t, nil
}

func (p *Postgres) List(ctx context.Context, f dto.ListFilter, userID int64, admin bool) ([]domain.Test, error) {
	where := []string{"t.deleted_at IS NULL"}
	args := []any{}
	add := func(condition string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(condition, len(args)))
	}
	if !admin {
		if f.Scope == "mine" {
			add("t.author_id=$%d", userID)
		} else {
			where = append(where, "t.status='published' AND t.visibility IN ('public','marketplace')")
		}
	}
	if f.Status != "" {
		add("t.status=$%d", f.Status)
	}
	if f.Category != "" {
		add("t.category=$%d", f.Category)
	}
	if f.Author != "" {
		add("u.full_name ILIKE '%%'||$%d||'%%'", f.Author)
	}
	if f.Search != "" {
		add("(v.title ILIKE '%%'||$%d||'%%' OR v.description ILIKE '%%'||$%d||'%%')", f.Search)
	}
	if f.Price == "free" {
		where = append(where, "t.is_free")
	}
	if f.Price == "paid" {
		where = append(where, "NOT t.is_free")
	}
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	args = append(args, f.Limit, f.Offset)
	q := testSelect + " WHERE " + strings.Join(where, " AND ") + fmt.Sprintf(" ORDER BY t.updated_at DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	rows, err := p.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Test{}
	for rows.Next() {
		t, err := scanTest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

func (p *Postgres) Get(ctx context.Context, id int64, includeCorrect bool) (*domain.Test, error) {
	t, err := scanTest(p.db.QueryRowContext(ctx, testSelect+" WHERE t.id=$1 AND t.deleted_at IS NULL", id))
	if err != nil {
		return nil, err
	}
	return p.loadQuestions(ctx, t, includeCorrect)
}

func (p *Postgres) GetVersion(ctx context.Context, testID, versionID int64, includeCorrect bool) (*domain.Test, error) {
	query := strings.Replace(testSelect, "JOIN test_versions v ON v.test_id=t.id AND v.version=t.current_version", "JOIN test_versions v ON v.test_id=t.id", 1)
	t, err := scanTest(p.db.QueryRowContext(ctx, query+" WHERE t.id=$1 AND v.id=$2 AND t.deleted_at IS NULL", testID, versionID))
	if err != nil {
		return nil, err
	}
	return p.loadQuestions(ctx, t, includeCorrect)
}

func (p *Postgres) loadQuestions(ctx context.Context, t *domain.Test, includeCorrect bool) (*domain.Test, error) {
	rows, err := p.db.QueryContext(ctx, `SELECT q.id,q.test_version_id,q.sort_order,q.question,q.question_type,q.explanation,q.points,q.settings FROM test_questions q WHERE q.test_version_id=(SELECT id FROM test_versions WHERE test_id=$1 AND version=$2) ORDER BY q.sort_order,q.id`, t.ID, t.Version)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var q domain.Question
		var settings []byte
		if err := rows.Scan(&q.ID, &q.TestVersionID, &q.SortOrder, &q.Question, &q.Type, &q.Explanation, &q.Points, &settings); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(settings, &q.Settings)
		aRows, err := p.db.QueryContext(ctx, `SELECT id,question_id,answer,is_correct,sort_order FROM test_answers WHERE question_id=$1 ORDER BY sort_order,id`, q.ID)
		if err != nil {
			return nil, err
		}
		for aRows.Next() {
			var a domain.Answer
			if err := aRows.Scan(&a.ID, &a.QuestionID, &a.Answer, &a.IsCorrect, &a.SortOrder); err != nil {
				aRows.Close()
				return nil, err
			}
			if !includeCorrect {
				a.IsCorrect = false
			}
			q.Answers = append(q.Answers, a)
		}
		aRows.Close()
		t.Questions = append(t.Questions, q)
	}
	return t, rows.Err()
}

func (p *Postgres) Create(ctx context.Context, author int64, in dto.CreateTest, slug string) (*domain.Test, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var id int64
	err = tx.QueryRowContext(ctx, `INSERT INTO tests(author_id,slug,category,difficulty,visibility,price,is_free,passing_percent,time_limit_seconds) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`, author, slug, in.Category, in.Difficulty, in.Visibility, in.Price, in.IsFree, in.PassingPercent, in.TimeLimitSeconds).Scan(&id)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO test_versions(test_id,version,title,description,created_by) VALUES($1,1,$2,$3,$4)`, id, strings.TrimSpace(in.Title), strings.TrimSpace(in.Description), author)
	if err == nil {
		_, err = tx.ExecContext(ctx, `INSERT INTO test_statistics(test_id) VALUES($1)`, id)
	}
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return p.Get(ctx, id, true)
}

func (p *Postgres) Update(ctx context.Context, id, user int64, in dto.UpdateTest) error {
	res, err := p.db.ExecContext(ctx, `UPDATE tests t SET category=$1,difficulty=$2,visibility=$3,price=$4,is_free=$5,passing_percent=$6,time_limit_seconds=$7,updated_at=NOW() FROM test_versions v WHERE t.id=$8 AND t.author_id=$9 AND t.status='draft' AND v.test_id=t.id AND v.version=t.current_version`, in.Category, in.Difficulty, in.Visibility, in.Price, in.IsFree, in.PassingPercent, in.TimeLimitSeconds, id, user)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrForbidden
	}
	_, err = p.db.ExecContext(ctx, `UPDATE test_versions v SET title=$1,description=$2,updated_at=NOW() FROM tests t WHERE t.id=$3 AND t.author_id=$4 AND v.test_id=t.id AND v.version=t.current_version`, strings.TrimSpace(in.Title), strings.TrimSpace(in.Description), id, user)
	return err
}
func (p *Postgres) SoftDelete(ctx context.Context, id, user int64, admin bool) error {
	q := `UPDATE tests SET status='deleted',deleted_at=NOW(),updated_at=NOW() WHERE id=$1`
	args := []any{id}
	if !admin {
		q += ` AND author_id=$2`
		args = append(args, user)
	}
	res, err := p.db.ExecContext(ctx, q, args...)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrForbidden
	}
	return nil
}
func (p *Postgres) Moderate(ctx context.Context, id int64, action, reason string) error {
	status := map[string]string{"publish": "published", "block": "blocked", "archive": "archived"}[action]
	if status == "" {
		return errors.New("invalid action")
	}
	_, err := p.db.ExecContext(ctx, `UPDATE tests SET status=$1,blocked_reason=$2,blocked_at=CASE WHEN $1='blocked' THEN NOW() ELSE NULL END,updated_at=NOW() WHERE id=$3`, status, reason, id)
	return err
}

func (p *Postgres) currentVersion(ctx context.Context, testID, user int64) (int64, error) {
	var v int64
	err := p.db.QueryRowContext(ctx, `SELECT tv.id FROM test_versions tv JOIN tests t ON t.id=tv.test_id AND t.current_version=tv.version WHERE t.id=$1 AND t.author_id=$2 AND t.status='draft'`, testID, user).Scan(&v)
	if err == sql.ErrNoRows {
		return 0, ErrForbidden
	}
	return v, err
}
func insertAnswers(ctx context.Context, tx *sql.Tx, qID int64, answers []dto.AnswerInput) error {
	for i, a := range answers {
		order := a.SortOrder
		if order == 0 {
			order = i
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO test_answers(question_id,answer,is_correct,sort_order) VALUES($1,$2,$3,$4)`, qID, strings.TrimSpace(a.Answer), a.IsCorrect, order); err != nil {
			return err
		}
	}
	return nil
}
func (p *Postgres) AddQuestion(ctx context.Context, testID, user int64, in dto.CreateQuestion) (int64, error) {
	v, err := p.currentVersion(ctx, testID, user)
	if err != nil {
		return 0, err
	}
	settings, _ := json.Marshal(in.Settings)
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var id int64
	err = tx.QueryRowContext(ctx, `INSERT INTO test_questions(test_version_id,sort_order,question,question_type,explanation,points,settings) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id`, v, in.SortOrder, strings.TrimSpace(in.Question), in.QuestionType, in.Explanation, in.Points, settings).Scan(&id)
	if err != nil {
		return 0, err
	}
	if err = insertAnswers(ctx, tx, id, in.Answers); err != nil {
		return 0, err
	}
	return id, tx.Commit()
}
func (p *Postgres) UpdateQuestion(ctx context.Context, id, user int64, in dto.CreateQuestion) error {
	settings, _ := json.Marshal(in.Settings)
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `UPDATE test_questions q SET sort_order=$1,question=$2,question_type=$3,explanation=$4,points=$5,settings=$6,updated_at=NOW() FROM test_versions v JOIN tests t ON t.id=v.test_id WHERE q.id=$7 AND q.test_version_id=v.id AND t.author_id=$8 AND t.status='draft'`, in.SortOrder, strings.TrimSpace(in.Question), in.QuestionType, in.Explanation, in.Points, settings, id, user)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrForbidden
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM test_answers WHERE question_id=$1`, id); err != nil {
		return err
	}
	if err = insertAnswers(ctx, tx, id, in.Answers); err != nil {
		return err
	}
	return tx.Commit()
}
func (p *Postgres) DeleteQuestion(ctx context.Context, id, user int64) error {
	res, err := p.db.ExecContext(ctx, `DELETE FROM test_questions q USING test_versions v,tests t WHERE q.id=$1 AND q.test_version_id=v.id AND v.test_id=t.id AND t.author_id=$2 AND t.status='draft'`, id, user)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrForbidden
	}
	return nil
}
func (p *Postgres) AddAnswer(ctx context.Context, qID, user int64, in dto.AnswerInput) (int64, error) {
	var id int64
	err := p.db.QueryRowContext(ctx, `INSERT INTO test_answers(question_id,answer,is_correct,sort_order) SELECT q.id,$1,$2,$3 FROM test_questions q JOIN test_versions v ON v.id=q.test_version_id JOIN tests t ON t.id=v.test_id WHERE q.id=$4 AND t.author_id=$5 AND t.status='draft' RETURNING id`, in.Answer, in.IsCorrect, in.SortOrder, qID, user).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, ErrForbidden
	}
	return id, err
}
func (p *Postgres) UpdateAnswer(ctx context.Context, id, user int64, in dto.AnswerInput) error {
	res, err := p.db.ExecContext(ctx, `UPDATE test_answers a SET answer=$1,is_correct=$2,sort_order=$3,updated_at=NOW() FROM test_questions q JOIN test_versions v ON v.id=q.test_version_id JOIN tests t ON t.id=v.test_id WHERE a.id=$4 AND a.question_id=q.id AND t.author_id=$5 AND t.status='draft'`, in.Answer, in.IsCorrect, in.SortOrder, id, user)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrForbidden
	}
	return nil
}
func (p *Postgres) DeleteAnswer(ctx context.Context, id, user int64) error {
	res, err := p.db.ExecContext(ctx, `DELETE FROM test_answers a USING test_questions q,test_versions v,tests t WHERE a.id=$1 AND a.question_id=q.id AND q.test_version_id=v.id AND v.test_id=t.id AND t.author_id=$2 AND t.status='draft'`, id, user)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrForbidden
	}
	return nil
}
func (p *Postgres) Publish(ctx context.Context, id, user int64) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `UPDATE tests t SET status='published',visibility=CASE WHEN visibility='private' THEN 'public' ELSE visibility END,updated_at=NOW() WHERE id=$1 AND author_id=$2 AND status='draft' AND EXISTS(SELECT 1 FROM test_versions v JOIN test_questions q ON q.test_version_id=v.id WHERE v.test_id=t.id AND v.version=t.current_version)`, id, user)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrForbidden
	}
	if _, err = tx.ExecContext(ctx, `UPDATE test_versions v SET published_at=NOW() FROM tests t WHERE t.id=$1 AND t.author_id=$2 AND v.test_id=t.id AND v.version=t.current_version`, id, user); err != nil {
		return err
	}
	return tx.Commit()
}

func (p *Postgres) ForkDraft(ctx context.Context, id, user int64) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var status string
	var oldVersion, newVersion int
	var oldVersionID, newVersionID int64
	err = tx.QueryRowContext(ctx, `SELECT t.status,t.current_version,v.id FROM tests t JOIN test_versions v ON v.test_id=t.id AND v.version=t.current_version WHERE t.id=$1 AND t.author_id=$2 AND t.deleted_at IS NULL FOR UPDATE OF t`, id, user).Scan(&status, &oldVersion, &oldVersionID)
	if err == sql.ErrNoRows {
		return ErrForbidden
	}
	if err != nil {
		return err
	}
	if status == domain.StatusDraft {
		return tx.Commit()
	}
	if status == domain.StatusDeleted || status == domain.StatusBlocked {
		return ErrForbidden
	}
	newVersion = oldVersion + 1
	err = tx.QueryRowContext(ctx, `INSERT INTO test_versions(test_id,version,title,description,changelog,created_by) SELECT test_id,$1,title,description,'Черновик на основе версии '||version,$2 FROM test_versions WHERE id=$3 RETURNING id`, newVersion, user, oldVersionID).Scan(&newVersionID)
	if err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,sort_order,question,question_type,explanation,points,settings FROM test_questions WHERE test_version_id=$1 ORDER BY sort_order,id`, oldVersionID)
	if err != nil {
		return err
	}
	type oldQuestion struct {
		id                          int64
		order                       int
		question, kind, explanation string
		points                      float64
		settings                    []byte
	}
	var questions []oldQuestion
	for rows.Next() {
		var q oldQuestion
		if err = rows.Scan(&q.id, &q.order, &q.question, &q.kind, &q.explanation, &q.points, &q.settings); err != nil {
			rows.Close()
			return err
		}
		questions = append(questions, q)
	}
	rows.Close()
	for _, q := range questions {
		var newQuestionID int64
		err = tx.QueryRowContext(ctx, `INSERT INTO test_questions(test_version_id,sort_order,question,question_type,explanation,points,settings) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id`, newVersionID, q.order, q.question, q.kind, q.explanation, q.points, q.settings).Scan(&newQuestionID)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO test_answers(question_id,answer,is_correct,sort_order) SELECT $1,answer,is_correct,sort_order FROM test_answers WHERE question_id=$2`, newQuestionID, q.id); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE tests SET current_version=$1,status='draft',updated_at=NOW() WHERE id=$2`, newVersion, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (p *Postgres) StartAttempt(ctx context.Context, testID, user int64) (*domain.Attempt, error) {
	var a domain.Attempt
	err := p.db.QueryRowContext(ctx, `INSERT INTO test_attempts(test_id,test_version_id,user_id,max_score) SELECT t.id,v.id,$2,COALESCE((SELECT SUM(points) FROM test_questions WHERE test_version_id=v.id),0) FROM tests t JOIN test_versions v ON v.test_id=t.id AND v.version=t.current_version WHERE t.id=$1 AND t.status='published' AND (t.visibility IN ('public','marketplace') OR t.author_id=$2) RETURNING id,test_id,test_version_id,user_id,max_score,started_at,status`, testID, user).Scan(&a.ID, &a.TestID, &a.TestVersionID, &a.UserID, &a.MaxScore, &a.StartedAt, &a.Status)
	if err == sql.ErrNoRows {
		return nil, ErrForbidden
	}
	return &a, err
}
func (p *Postgres) GetAttempt(ctx context.Context, id int64) (*domain.Attempt, error) {
	var a domain.Attempt
	var finished sql.NullTime
	var passed sql.NullBool
	err := p.db.QueryRowContext(ctx, `SELECT a.id,a.test_id,a.test_version_id,a.user_id,u.full_name,v.title,a.score,a.max_score,a.percent,a.passed,a.started_at,a.finished_at,a.duration_seconds,a.status FROM test_attempts a JOIN users u ON u.id=a.user_id JOIN test_versions v ON v.id=a.test_version_id WHERE a.id=$1`, id).Scan(&a.ID, &a.TestID, &a.TestVersionID, &a.UserID, &a.UserName, &a.TestTitle, &a.Score, &a.MaxScore, &a.Percent, &passed, &a.StartedAt, &finished, &a.DurationSeconds, &a.Status)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if finished.Valid {
		a.FinishedAt = &finished.Time
	}
	if passed.Valid {
		a.Passed = &passed.Bool
	}
	rows, err := p.db.QueryContext(ctx, `SELECT aa.question_id,q.question,aa.selected_answer_id,COALESCE(sa.answer,''),COALESCE(aa.text_answer,''),aa.is_correct,aa.earned_points,COALESCE((SELECT string_agg(answer,', ' ORDER BY sort_order) FROM test_answers WHERE question_id=q.id AND is_correct),''),aa.answered_at,aa.response_seconds FROM test_attempt_answers aa JOIN test_questions q ON q.id=aa.question_id LEFT JOIN test_answers sa ON sa.id=aa.selected_answer_id WHERE aa.attempt_id=$1 ORDER BY q.sort_order,aa.id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var x domain.AttemptAnswer
		var selected sql.NullInt64
		var correct sql.NullBool
		var answeredAt sql.NullTime
		if err := rows.Scan(&x.QuestionID, &x.Question, &selected, &x.SelectedAnswer, &x.TextAnswer, &correct, &x.EarnedPoints, &x.CorrectAnswer, &answeredAt, &x.ResponseSeconds); err != nil {
			return nil, err
		}
		if selected.Valid {
			x.SelectedAnswerID = &selected.Int64
		}
		if correct.Valid {
			x.IsCorrect = &correct.Bool
		}
		if answeredAt.Valid {
			x.AnsweredAt = &answeredAt.Time
		}
		a.Answers = append(a.Answers, x)
	}
	return &a, rows.Err()
}
func (p *Postgres) SaveAttemptAnswer(ctx context.Context, attempt int64, in dto.SubmitAnswer) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var responseSeconds int
	if err = tx.QueryRowContext(ctx, `SELECT GREATEST(0,EXTRACT(EPOCH FROM NOW()-COALESCE((SELECT MAX(answered_at) FROM test_attempt_answers WHERE attempt_id=$1),(SELECT started_at FROM test_attempts WHERE id=$1)))::int)`, attempt).Scan(&responseSeconds); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM test_attempt_answers WHERE attempt_id=$1 AND question_id=$2`, attempt, in.QuestionID); err != nil {
		return err
	}
	if len(in.SelectedAnswerIDs) == 0 {
		_, err = tx.ExecContext(ctx, `INSERT INTO test_attempt_answers(attempt_id,question_id,text_answer,response_seconds) SELECT a.id,q.id,$3,$4 FROM test_attempts a JOIN test_questions q ON q.test_version_id=a.test_version_id WHERE a.id=$1 AND q.id=$2 AND a.status='started'`, attempt, in.QuestionID, in.TextAnswer, responseSeconds)
	} else {
		for _, answer := range in.SelectedAnswerIDs {
			if _, err = tx.ExecContext(ctx, `INSERT INTO test_attempt_answers(attempt_id,question_id,selected_answer_id,response_seconds) SELECT a.id,q.id,ta.id,$4 FROM test_attempts a JOIN test_questions q ON q.test_version_id=a.test_version_id JOIN test_answers ta ON ta.question_id=q.id WHERE a.id=$1 AND q.id=$2 AND ta.id=$3 AND a.status='started'`, attempt, in.QuestionID, answer, responseSeconds); err != nil {
				return err
			}
		}
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (p *Postgres) FinishAttempt(ctx context.Context, id int64, score, max, percent float64, passed bool, answers []domain.AttemptAnswer) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, a := range answers {
		_, err = tx.ExecContext(ctx, `UPDATE test_attempt_answers SET is_correct=$1,earned_points=$2 WHERE attempt_id=$3 AND question_id=$4`, a.IsCorrect, a.EarnedPoints, id, a.QuestionID)
		if err != nil {
			return err
		}
	}
	res, err := tx.ExecContext(ctx, `UPDATE test_attempts SET score=$1,max_score=$2,percent=$3,passed=$4,finished_at=NOW(),duration_seconds=EXTRACT(EPOCH FROM NOW()-started_at)::int,status='finished' WHERE id=$5 AND status='started'`, score, max, percent, passed, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrForbidden
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO test_statistics(test_id,attempts_count,completed_count,passed_count,failed_count,average_percent,average_duration_seconds,updated_at) SELECT test_id,COUNT(*),COUNT(*) FILTER(WHERE status='finished'),COUNT(*) FILTER(WHERE passed),COUNT(*) FILTER(WHERE passed=FALSE),COALESCE(AVG(percent) FILTER(WHERE status='finished'),0),COALESCE(AVG(duration_seconds) FILTER(WHERE status='finished'),0),NOW() FROM test_attempts WHERE test_id=(SELECT test_id FROM test_attempts WHERE id=$1) GROUP BY test_id ON CONFLICT(test_id) DO UPDATE SET attempts_count=EXCLUDED.attempts_count,completed_count=EXCLUDED.completed_count,passed_count=EXCLUDED.passed_count,failed_count=EXCLUDED.failed_count,average_percent=EXCLUDED.average_percent,average_duration_seconds=EXCLUDED.average_duration_seconds,updated_at=NOW()`, id)
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (p *Postgres) ListAttempts(ctx context.Context, testID, user int64, admin bool) ([]domain.Attempt, error) {
	q := `SELECT a.id,a.test_id,a.test_version_id,a.user_id,u.full_name,v.title,a.score,a.max_score,a.percent,a.passed,a.started_at,a.finished_at,a.duration_seconds,a.status,(SELECT COUNT(DISTINCT aa.question_id) FROM test_attempt_answers aa WHERE aa.attempt_id=a.id AND aa.is_correct=TRUE),(SELECT COUNT(*) FROM test_questions tq WHERE tq.test_version_id=a.test_version_id) FROM test_attempts a JOIN users u ON u.id=a.user_id JOIN test_versions v ON v.id=a.test_version_id WHERE 1=1`
	args := []any{}
	if testID > 0 {
		args = append(args, testID)
		q += fmt.Sprintf(" AND a.test_id=$%d", len(args))
	}
	if !admin {
		args = append(args, user)
		q += fmt.Sprintf(" AND a.user_id=$%d", len(args))
	}
	q += " ORDER BY a.started_at DESC LIMIT 200"
	rows, err := p.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Attempt{}
	for rows.Next() {
		var a domain.Attempt
		var finished sql.NullTime
		var passed sql.NullBool
		if err := rows.Scan(&a.ID, &a.TestID, &a.TestVersionID, &a.UserID, &a.UserName, &a.TestTitle, &a.Score, &a.MaxScore, &a.Percent, &passed, &a.StartedAt, &finished, &a.DurationSeconds, &a.Status, &a.CorrectAnswers, &a.TotalQuestions); err != nil {
			return nil, err
		}
		if passed.Valid {
			a.Passed = &passed.Bool
		}
		if finished.Valid {
			a.FinishedAt = &finished.Time
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
func (p *Postgres) Statistics(ctx context.Context, id int64) (*domain.Statistics, error) {
	var s domain.Statistics
	err := p.db.QueryRowContext(ctx, `SELECT test_id,attempts_count,completed_count,passed_count,failed_count,average_percent,average_duration_seconds FROM test_statistics WHERE test_id=$1`, id).Scan(&s.TestID, &s.AttemptsCount, &s.CompletedCount, &s.PassedCount, &s.FailedCount, &s.AveragePercent, &s.AverageDurationSeconds)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return &s, err
}

var _ = time.Now
