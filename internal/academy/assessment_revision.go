package academy

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

var ErrProgramUpdated = errors.New("Программа обновлена автором. Обновите программу, чтобы продолжить. Начатые попытки сохранены")

func checkPublication(r *http.Request, e *Enrollment) error {
	if value := r.URL.Query().Get("publication_revision"); value != "" {
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil || id != e.Program.PublicationID {
			return ErrProgramUpdated
		}
	}
	return nil
}

func historicalAssessment(ctx context.Context, q queryer, e *Enrollment, lesson, block string, id int64) (*Assessment, error) {
	var a Assessment
	var config, questions []byte
	err := q.QueryRowContext(ctx, `SELECT id,program_version_id,lesson_key,block_key,kind,title,config,questions,publication_revision_id
	 FROM academy_assessment_versions WHERE id=$1 AND program_version_id=$2 AND lesson_key=$3 AND block_key=$4`, id, e.VersionID, lesson, block).Scan(&a.ID, &a.VersionID, &a.LessonKey, &a.BlockKey, &a.Kind, &a.Title, &config, &questions, &a.PublicationID)
	if err != nil {
		return nil, err
	}
	var b BlockData
	if err = json.Unmarshal(config, &b); err != nil {
		return nil, err
	}
	a.Policy = policy(Block{Data: b})
	a.Definition = b
	err = json.Unmarshal(questions, &a.Questions)
	return &a, err
}

// Existing work owns its definition, including after archival or republishing.
// Every lookup is scoped to the authenticated enrollment and logical keys.
func assessmentForRequest(r *http.Request, q queryer, e *Enrollment, lesson, block string) (*Assessment, bool, error) {
	var id int64
	table := ""
	selected, _ := strconv.ParseInt(r.URL.Query().Get("attempt"), 10, 64)
	if selected > 0 {
		table = "academy_test_attempts"
		id = selected
	}
	if selected, _ := strconv.ParseInt(r.URL.Query().Get("submission"), 10, 64); selected > 0 {
		table = "academy_assignment_submissions"
		id = selected
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	for i, p := range parts {
		if i+1 < len(parts) && (p == "attempts" || p == "submissions") {
			id, _ = strconv.ParseInt(parts[i+1], 10, 64)
			if p == "attempts" {
				table = "academy_test_attempts"
			} else {
				table = "academy_assignment_submissions"
			}
		}
	}
	var aid int64
	var err error
	if table != "" {
		err = q.QueryRowContext(r.Context(), `SELECT a.id FROM `+table+` t JOIN academy_assessment_versions a ON a.id=t.assessment_id
   WHERE t.id=$1 AND t.enrollment_id=$2 AND a.program_version_id=$3 AND a.lesson_key=$4 AND a.block_key=$5`, id, e.ID, e.VersionID, lesson, block).Scan(&aid)
		if err != nil {
			return nil, false, err
		}
	} else {
		err = q.QueryRowContext(r.Context(), `SELECT a.id FROM academy_assessment_versions a WHERE a.program_version_id=$1 AND a.lesson_key=$2 AND a.block_key=$3
   AND(EXISTS(SELECT 1 FROM academy_test_attempts t WHERE t.enrollment_id=$4 AND t.assessment_id=a.id AND t.status='started')
   OR EXISTS(SELECT 1 FROM academy_effective_assignment_submissions s WHERE s.enrollment_id=$4 AND s.assessment_id=a.id AND s.status IN('draft','submitted','in_review')))
   ORDER BY a.id DESC LIMIT 1`, e.VersionID, lesson, block, e.ID).Scan(&aid)
		if err != nil && err != sql.ErrNoRows {
			return nil, false, err
		}
	}
	if aid > 0 {
		a, err := historicalAssessment(r.Context(), q, e, lesson, block, aid)
		return a, true, err
	}
	if err := checkPublication(r, e); err != nil {
		return nil, false, err
	}
	a, err := assessment(r.Context(), q, e, lesson, block)
	return a, false, err
}
