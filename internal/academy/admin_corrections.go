package academy

import (
	"FinTalent/internal/courses"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"strings"
	"time"
)

func jsonBody(b []byte) io.ReadCloser { return io.NopCloser(strings.NewReader(string(b))) }
func (h *Handler) adminSaveProgram(ctx context.Context, tx *sql.Tx, m *adminMutation, p *Program) error {
	id, revision, err := lock(ctx, tx, m.Course, m.Principal.User)
	if err != nil {
		return err
	}
	if p.Revision != revision {
		return ErrConflict
	}
	p.ID, p.CourseID = id, m.Course
	if err = h.saveTree(ctx, tx, p); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "UPDATE academy_course_programs SET revision=revision+1,updated_at=now() WHERE id=$1", id)
	return err
}
func (h *Handler) adminRestriction(ctx context.Context, tx *sql.Tx, m *adminMutation, data json.RawMessage, author bool) error {
	var input struct {
		Blocked bool `json:"blocked"`
	}
	if json.Unmarshal(data, &input) != nil {
		return ErrInvalid
	}
	return courses.SetPublicationRestriction(ctx, tx, m.ID, author, input.Blocked, m.Reason)
}
func (h *Handler) adminCorrection(ctx context.Context, tx *sql.Tx, m *adminMutation, id int64, kind string, data json.RawMessage) (any, error) {
	var input struct {
		Score    *float64           `json:"score"`
		Status   string             `json:"status"`
		Comment  string             `json:"comment"`
		Criteria map[string]float64 `json:"criteria"`
	}
	if json.Unmarshal(data, &input) != nil || len(input.Comment) > 10000 {
		return nil, ErrInvalid
	}
	e, err := loadEnrollment(ctx, tx, m.Enrollment, m.Principal.User, true, true)
	if err != nil {
		return nil, err
	}
	if err = readProgress(ctx, tx, e); err != nil {
		return nil, err
	}
	before := e.Summary
	column, table := "test_attempt_id", "academy_test_attempts"
	if kind == "submissions" {
		column, table = "submission_id", "academy_assignment_submissions"
	}
	var aid int64
	var original json.RawMessage
	err = tx.QueryRowContext(ctx, "SELECT assessment_id,to_jsonb(x)-'answers'-'answer'-'request_key'-'grades' FROM "+table+" x WHERE id=$1 AND enrollment_id=$2", id, e.ID).Scan(&aid, &original)
	if err != nil {
		return nil, err
	}
	var config []byte
	var lesson string
	err = tx.QueryRowContext(ctx, "SELECT config,lesson_key FROM academy_assessment_versions WHERE id=$1 AND program_version_id=$2", aid, e.VersionID).Scan(&config, &lesson)
	if err != nil {
		return nil, err
	}
	var b BlockData
	if err = json.Unmarshal(config, &b); err != nil {
		return nil, err
	}
	pol := policy(Block{Data: b})
	payload := map[string]any{}
	if kind == "tests" {
		var maxScore float64
		var status string
		err = tx.QueryRowContext(ctx, "SELECT max_score,status FROM academy_test_attempts WHERE id=$1", id).Scan(&maxScore, &status)
		if err != nil {
			return nil, err
		}
		if status != "finished" || input.Score == nil || !finite(*input.Score) || *input.Score < 0 || *input.Score > maxScore || maxScore <= 0 {
			return nil, inputError{"Корректируются только завершённые попытки: балл должен быть в пределах исходного максимума"}
		}
		percent := *input.Score / maxScore * 100
		payload = map[string]any{"score": *input.Score, "percent": percent, "passed": percent >= *pol.PassingPercent}
	} else {
		var status string
		err = tx.QueryRowContext(ctx, "SELECT status FROM academy_assignment_submissions WHERE id=$1", id).Scan(&status)
		if err != nil {
			return nil, err
		}
		if !oneOf(status, "accepted", "rejected", "revision_requested") || !oneOf(input.Status, "accepted", "rejected", "revision_requested") {
			return nil, inputError{"Корректировка применяется к завершённому решению преподавателя. Работы на проверке сначала проверяются обычным способом"}
		}
		score, err := reviewScore(pol, input.Score, input.Criteria, input.Status)
		if err != nil {
			return nil, err
		}
		payload = map[string]any{"score": score, "status": input.Status, "comment": input.Comment, "criteria": input.Criteria}
	}
	var previous *int64
	err = tx.QueryRowContext(ctx, "SELECT max(id) FROM academy_admin_corrections WHERE "+column+"=$1", id).Scan(&previous)
	if err != nil {
		return nil, err
	}
	m.Before = json.RawMessage(jsonValue(map[string]any{"enrollment": m.Before, "original_result": original, "previous_correction": previous}))
	audit, err := adminAudit(ctx, tx, m, map[string]any{"correction": payload, "original_preserved": true})
	if err != nil {
		return nil, err
	}
	m.AuditWritten = true
	var correction int64
	err = tx.QueryRowContext(ctx, "INSERT INTO academy_admin_corrections(enrollment_id,"+column+",payload,previous_id,audit_id) VALUES($1,$2,$3,$4,$5) RETURNING id", e.ID, id, jsonValue(payload), previous, audit).Scan(&correction)
	if err != nil {
		return nil, err
	}
	if err = reconcileAssessmentTx(ctx, tx, e, m.Principal.User, lesson); err != nil {
		return nil, err
	}
	if err = syncCourseTx(ctx, tx, e, m.Principal.User); err != nil {
		return nil, err
	}
	if err = readProgress(ctx, tx, e); err != nil {
		return nil, err
	}
	err = event(ctx, tx, e, m.Principal.User, "administrative_result_corrected", "admin:"+m.Key, lesson, map[string]any{"kind": kind, "result_id": id, "correction_id": correction, "administrator": m.Principal.Name, "reason": m.Reason, "before": before, "after": e.Summary})
	if err == nil {
		now := time.Now()
		err = cohortEvent(ctx, tx, e.CourseID, e.CohortID, &e.ID, m.Principal.User, "administrative_result_corrected", "admin-result:"+m.Key, map[string]any{"reason": m.Reason, "correction_id": correction}, &e.UserID, &now)
	}
	return map[string]any{"correction_id": correction, "audit_id": audit, "before": before, "after": e.Summary, "original_preserved": true}, err
}
