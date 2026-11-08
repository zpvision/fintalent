package academy

import (
	test "FinTalent/internal/testmodule/domain"
	"context"
	"database/sql"
	"encoding/json"
	"reflect"
	"sort"
)

type capturedAcademyTest struct {
	VersionID int64           `json:"source_version_id"`
	Title     string          `json:"title"`
	Questions []test.Question `json:"questions"`
}

func captureAcademyTest(ctx context.Context, q queryer, version, owner int64) (capturedAcademyTest, error) {
	var raw []byte
	var out capturedAcademyTest
	err := q.QueryRowContext(ctx, `SELECT jsonb_build_object('title',v.title,'questions',COALESCE((SELECT jsonb_agg(to_jsonb(q)||jsonb_build_object('answers',COALESCE((SELECT jsonb_agg(to_jsonb(a) ORDER BY a.sort_order,a.id) FROM test_answers a WHERE a.question_id=q.id),'[]')) ORDER BY q.sort_order,q.id) FROM test_questions q WHERE q.test_version_id=v.id),'[]')) FROM test_versions v JOIN tests t ON t.id=v.test_id WHERE v.id=$1 AND t.author_id=$2 AND t.deleted_at IS NULL AND t.status NOT IN('deleted','blocked','archived')`, version, owner).Scan(&raw)
	if err != nil {
		return out, inputError{"Тест недоступен автору курса. Выберите доступный тест перед публикацией"}
	}
	err = json.Unmarshal(raw, &out)
	out.VersionID = version
	return out, err
}

func changedTestSources(ctx context.Context, tx *sql.Tx, d *VersionDraft) (int, error) {
	var raw []byte
	var owner int64
	if err := tx.QueryRowContext(ctx, `SELECT v.content,c.owner_id FROM academy_current_program_versions v JOIN academy_course_programs p ON p.id=v.program_id JOIN courses c ON c.id=p.course_id WHERE v.id=$1`, d.VersionID).Scan(&raw, &owner); err != nil {
		return 0, err
	}
	var published Program
	if err := json.Unmarshal(raw, &published); err != nil {
		return 0, err
	}
	count := 0
	for _, m := range learningProgram(d.Program).Modules {
		for _, l := range m.Lessons {
			for _, b := range l.Blocks {
				if b.Type != "test" || b.Data.TestVersionID == 0 {
					continue
				}
				live, ok := d.Tests[b.Key]
				if !ok {
					count++
					continue
				}
				old, err := publishedTestSource(ctx, tx, d.VersionID, published.PublicationID, l.Key, b.Key, live.VersionID)
				if err == sql.ErrNoRows || err == ErrAssessmentUnprepared {
					count++
					continue
				}
				if err != nil {
					return 0, err
				}
				if live.Title != old.Title || !reflect.DeepEqual(live.Questions, old.Questions) {
					count++
				}
			}
		}
	}
	return count, nil
}

func seedVersionTests(ctx context.Context, tx *sql.Tx, d *VersionDraft, force bool) error {
	var owner int64
	var raw []byte
	if err := tx.QueryRowContext(ctx, `SELECT c.owner_id,v.content FROM academy_current_program_versions v JOIN academy_course_programs p ON p.id=v.program_id JOIN courses c ON c.id=p.course_id WHERE v.id=$1`, d.VersionID).Scan(&owner, &raw); err != nil {
		return err
	}
	var current Program
	if err := json.Unmarshal(raw, &current); err != nil {
		return err
	}
	out := map[string]capturedAcademyTest{}
	for _, m := range d.Program.Modules {
		for _, l := range m.Lessons {
			for _, b := range l.Blocks {
				if b.Type != "test" || b.Data.TestVersionID == 0 {
					continue
				}
				if old, ok := d.Tests[b.Key]; ok && old.VersionID == b.Data.TestVersionID && !force {
					out[b.Key] = old
					continue
				}
				if !force {
					saved, err := publishedTestSource(ctx, tx, d.VersionID, current.PublicationID, l.Key, b.Key, b.Data.TestVersionID)
					if err == nil {
						out[b.Key] = saved
						continue
					}
					if err != sql.ErrNoRows {
						return err
					}
				}
				captured, err := captureAcademyTest(ctx, tx, b.Data.TestVersionID, owner)
				if err != nil {
					return err
				}
				out[b.Key] = captured
			}
		}
	}
	d.Tests = out
	return nil
}

func publishedTestSource(ctx context.Context, q queryer, version, publication int64, lesson, block string, source int64) (capturedAcademyTest, error) {
	var out capturedAcademyTest
	var raw []byte
	err := q.QueryRowContext(ctx, `SELECT title,questions FROM academy_assessment_versions a
 WHERE program_version_id=$1 AND lesson_key=$2 AND block_key=$3 AND kind='test' AND (config->>'test_version_id')::bigint=$5
 AND(publication_revision_id=$4 OR publication_revision_id IS NULL AND($4=0 OR EXISTS(SELECT 1 FROM academy_program_revisions r WHERE r.id=$4 AND r.version_id=$1 AND r.number=1)))
 ORDER BY publication_revision_id NULLS LAST LIMIT 1`, version, lesson, block, publication, source).Scan(&out.Title, &raw)
	if err != nil {
		return out, err
	}
	out.VersionID = source
	err = json.Unmarshal(raw, &out.Questions)
	return out, err
}

func testGradingChanged(ctx context.Context, tx *sql.Tx, d *VersionDraft, publication int64) (bool, error) {
	for _, m := range learningProgram(d.Program).Modules {
		for _, l := range m.Lessons {
			for _, b := range l.Blocks {
				if !l.Required || b.Type != "test" || !requiredBlock(l, b) || b.Data.TestVersionID == 0 {
					continue
				}
				old, err := publishedTestSource(ctx, tx, d.VersionID, publication, l.Key, b.Key, b.Data.TestVersionID)
				if err == sql.ErrNoRows {
					return true, nil
				}
				if err != nil {
					return false, err
				}
				current := d.Tests[b.Key]
				if !reflect.DeepEqual(gradingSignature(old.Questions), gradingSignature(current.Questions)) {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

func gradingSignature(questions []test.Question) map[int64]test.Question {
	out := map[int64]test.Question{}
	for _, q := range clone(questions) {
		q.Question = ""
		q.Explanation = ""
		q.SortOrder = 0
		for i := range q.Answers {
			q.Answers[i].SortOrder = 0
			if q.Type != test.QuestionText {
				q.Answers[i].Answer = ""
			}
		}
		sort.Slice(q.Answers, func(i, j int) bool { return q.Answers[i].ID < q.Answers[j].ID })
		out[q.ID] = q
	}
	return out
}
