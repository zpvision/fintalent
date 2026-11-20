package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func revisionBackfillExec(t *testing.T, database *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := database.Exec(query, args...); err != nil {
		t.Fatal(err)
	}
}

func revisionBackfillState(t *testing.T, database *sql.DB) map[string]string {
	t.Helper()
	state := map[string]string{}
	for _, table := range []string{"academy_program_versions", "academy_version_releases", "academy_program_revision_heads", "academy_program_version_drafts", "academy_program_revisions", "academy_assessment_versions", "academy_cohorts", "academy_enrollments", "academy_lesson_progress", "academy_test_attempts", "academy_assignment_submissions", "academy_submission_reviews"} {
		var value string
		if err := database.QueryRow(`SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text),'[]')::text FROM ` + table + ` x`).Scan(&value); err != nil {
			t.Fatal(err)
		}
		state[table] = value
	}
	return state
}

func TestAcademyProgramRevisionEmptyAndRepeatPostgres(t *testing.T) {
	configureProgramRevisionTestDatabase(t)
	database := newAcademyFocusedDatabase(t) // Full preparation, including 093, before any Academy fixtures.
	before := revisionBackfillState(t, database)
	for table, value := range before {
		if value != "[]" {
			t.Fatal("clean Academy fixture contains rows", table)
		}
	}
	started := time.Now()
	revisionBackfillExec(t, database, academyProgramRevisionsSchema)
	t.Logf("093 repeat on empty database: %.3f ms", float64(time.Since(started).Microseconds())/1000)
	if !reflect.DeepEqual(before, revisionBackfillState(t, database)) {
		t.Fatal("empty repeat changed data")
	}
}

func TestAcademyProgramRevisionPartialBackfillPostgres(t *testing.T) {
	f := newProgramRevisionFixture(t, true)
	database := f.DB
	exec := func(query string, args ...any) { t.Helper(); revisionBackfillExec(t, database, query, args...) }
	id := func(query string, args ...any) int64 {
		t.Helper()
		var n int64
		if err := database.QueryRow(query, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	// Partially applied 093: its tables exist, but neither data backfill has run.
	prefix, _, ok := strings.Cut(academyProgramRevisionsSchema, "-- Deterministic correspondence:")
	if !ok {
		t.Fatal("migration initialization boundary missing")
	}
	exec(prefix)
	exec(`ALTER TABLE academy_assessment_versions ADD COLUMN publication_revision_id bigint`)
	var base map[string]any
	var content []byte
	if err := database.QueryRow(`SELECT content FROM academy_program_versions WHERE id=$1`, f.Version).Scan(&content); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(content, &base); err != nil {
		t.Fatal(err)
	}
	lesson := base["modules"].([]any)[0].(map[string]any)["lessons"].([]any)[0].(map[string]any)
	lesson["blocks"] = []any{map[string]any{"key": "legacy-test", "type": "test", "data": map[string]any{"title": "Frozen test", "test_version_id": 77}}}
	content, _ = json.Marshal(base)
	legacy := id(`INSERT INTO academy_program_versions(program_id,number,source_revision,content,created_by) VALUES($1,2,2,$2,$3) RETURNING id`, f.Program, content, f.Owner)
	partial := id(`INSERT INTO academy_program_versions(program_id,number,source_revision,content,created_by) VALUES($1,3,3,$2,$3) RETURNING id`, f.Program, content, f.Owner)
	for _, version := range []int64{legacy, partial} {
		exec(`INSERT INTO academy_version_releases(version_id,course_id,published_by) VALUES($1,$2,$3)`, version, f.Course, f.Owner)
	}
	pub1 := id(`INSERT INTO academy_program_revisions(version_id,number,draft_revision,content,assessment_definitions,published_by) VALUES($1,1,1,$2,'[]',$3) RETURNING id`, partial, content, f.Owner)
	pub2 := id(`INSERT INTO academy_program_revisions(version_id,number,draft_revision,content,assessment_definitions,published_by) VALUES($1,2,2,jsonb_set($2::jsonb,'{modules,0,title}','"Published second revision"'),'[]',$3) RETURNING id`, partial, content, f.Owner)
	exec(`INSERT INTO academy_program_revision_heads(version_id,published_revision_id) VALUES($1,$2)`, partial, pub2)
	// This row predates this run: neither '{}' nor revision/content can prove its origin.
	exec(`INSERT INTO academy_program_version_drafts(version_id,revision,content,updated_by,test_sources) VALUES($1,7,jsonb_set($2::jsonb,'{modules,0,title}','"Author draft"'),$3,'{}')`, partial, content, f.Owner)
	var originalDraft, originalPub string
	if err := database.QueryRow(`SELECT to_jsonb(d)::text FROM academy_program_version_drafts d WHERE version_id=$1`, partial).Scan(&originalDraft); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow(`SELECT to_jsonb(r)::text FROM academy_program_revisions r WHERE id=$1`, pub1).Scan(&originalPub); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		version     int64
		publication any
	}{{legacy, nil}, {partial, pub2}} {
		exec(`INSERT INTO academy_assessment_versions(program_version_id,lesson_key,block_key,kind,config,questions,title,publication_revision_id)
 SELECT $1,content->'modules'->0->'lessons'->0->>'key','legacy-test','test','{"test_version_id":77}','[{"id":1,"question":"Frozen question","points":1}]','Frozen test',$2 FROM academy_program_versions WHERE id=$1`, item.version, item.publication)
	}
	aid := id(`SELECT id FROM academy_assessment_versions WHERE program_version_id=$1`, legacy)
	cohort := id(`INSERT INTO academy_cohorts(course_id,program_version_id,name,type,status,starts_at,timezone,schedule_mode,request_key,created_by) VALUES($1,$2,'Preserve cohort','group','active',now(),'UTC','all','backfill-cohort',$3) RETURNING id`, f.Course, legacy, f.Owner)
	enrollment := id(`INSERT INTO academy_enrollments(course_id,user_id,program_version_id,cohort_id,enrolled_by,request_key,started_at,completed_at) VALUES($1,$2,$3,$4,$5,'backfill-student','2026-01-01','2026-01-02') RETURNING id`, f.Course, f.Student, legacy, cohort, f.Owner)
	exec(`INSERT INTO academy_lesson_progress(enrollment_id,program_version_id,lesson_key,status,completed_at) SELECT $1,$2,content->'modules'->0->'lessons'->0->>'key','completed','2026-01-02' FROM academy_program_versions WHERE id=$2`, enrollment, legacy)
	exec(`INSERT INTO academy_test_attempts(enrollment_id,program_version_id,assessment_id,number,request_key,status,answers,grades,score,max_score,percent,passed,finished_at) VALUES($1,$2,$3,1,'backfill-attempt','finished','[{"question_id":1,"selected_answer_ids":[1]}]','[{"earned_points":1}]',1,1,100,true,'2026-01-02')`, enrollment, legacy, aid)
	assignment := id(`INSERT INTO academy_assessment_versions(program_version_id,lesson_key,block_key,kind,config,title) SELECT id,content->'modules'->0->'lessons'->0->>'key','legacy-assignment','assignment','{}','Frozen assignment' FROM academy_program_versions WHERE id=$1 RETURNING id`, legacy)
	submission := id(`INSERT INTO academy_assignment_submissions(enrollment_id,program_version_id,assessment_id,number,request_key,status,answer,submitted_at,reviewed_at,reviewed_by,score) VALUES($1,$2,$3,1,'backfill-submission','accepted','{"text":"Original answer"}','2026-01-01','2026-01-02',$4,90) RETURNING id`, enrollment, legacy, assignment, f.Owner)
	exec(`INSERT INTO academy_submission_reviews(submission_id,reviewer_id,source,request_key,action,score,comment) VALUES($1,$2,'manual','backfill-review','accepted',90,'Original grade')`, submission, f.Owner)
	// Fail even a zero-row UPDATE of drafts: re-running 093 must never issue it.
	exec(`CREATE FUNCTION forbid_backfill_draft_update() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'Existing drafts must not be updated'; END $$;
 CREATE TRIGGER forbid_backfill_draft_update BEFORE UPDATE ON academy_program_version_drafts FOR EACH STATEMENT EXECUTE FUNCTION forbid_backfill_draft_update()`)
	before := revisionBackfillState(t, database)
	initializedAt := time.Now()
	exec(academyProgramRevisionsSchema)
	t.Logf("093 first backfill on partial safety fixture (3 versions, 3 assessments): %.3f ms", float64(time.Since(initializedAt).Microseconds())/1000)
	after := revisionBackfillState(t, database)
	for _, table := range []string{"academy_program_versions", "academy_version_releases", "academy_assessment_versions", "academy_cohorts", "academy_enrollments", "academy_lesson_progress", "academy_test_attempts", "academy_assignment_submissions", "academy_submission_reviews"} {
		if before[table] != after[table] {
			t.Fatal("initialization modified existing data", table)
		}
	}
	var draft, pub string
	if err := database.QueryRow(`SELECT to_jsonb(d)::text FROM academy_program_version_drafts d WHERE version_id=$1`, partial).Scan(&draft); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow(`SELECT to_jsonb(r)::text FROM academy_program_revisions r WHERE id=$1`, pub1).Scan(&pub); err != nil {
		t.Fatal(err)
	}
	if draft != originalDraft || pub != originalPub || id(`SELECT published_revision_id FROM academy_program_revision_heads WHERE version_id=$1`, partial) != pub2 {
		t.Fatal("author draft or existing publication overwritten")
	}
	if id(`SELECT count(*) FROM academy_program_version_drafts WHERE version_id=$1 AND test_sources->'legacy-test'->>'title'='Frozen test' AND (test_sources->'legacy-test'->>'source_version_id')::bigint=77`, legacy) != 1 {
		t.Fatal("new backfill draft did not capture frozen tests")
	}
	if id(`SELECT count(*) FROM academy_program_revision_heads h JOIN academy_program_revisions r ON r.id=h.published_revision_id WHERE h.version_id=$1 AND r.number=1`, legacy) != 1 {
		t.Fatal("missing legacy publication not initialized")
	}
	sequence := id(`SELECT last_value FROM academy_program_revisions_id_seq`)
	for repeat := 1; repeat <= 3; repeat++ {
		start := time.Now()
		exec(academyProgramRevisionsSchema)
		t.Logf("093 repeat %d on small safety fixture: %.3f ms (not a production-size benchmark)", repeat, float64(time.Since(start).Microseconds())/1000)
		if !reflect.DeepEqual(after, revisionBackfillState(t, database)) {
			t.Fatal("repeat modified data", repeat)
		}
		if id(`SELECT last_value FROM academy_program_revisions_id_seq`) != sequence {
			t.Fatal("repeat tried to insert existing publications")
		}
	}

	t.Run("legacy_snapshot_and_new_version_after_migration", func(t *testing.T) {
		request := func(path string, body any) []byte {
			t.Helper()
			raw, _ := json.Marshal(body)
			r := httptest.NewRequest("POST", fmt.Sprintf("/api/academy/courses/%d", f.Course)+path, bytes.NewReader(raw))
			r.Header.Set("X-Test-User", strconv.FormatInt(f.Owner, 10))
			w := httptest.NewRecorder()
			f.Mux.ServeHTTP(w, r)
			if w.Code != 200 {
				t.Fatalf("POST %s status=%d: %s", path, w.Code, w.Body.String())
			}
			return w.Body.Bytes()
		}
		module := id(`INSERT INTO academy_modules(program_id,key,title,status) VALUES($1,'snapshot-module','Snapshot module','published') RETURNING id`, f.Program)
		lesson := id(`INSERT INTO academy_lessons(program_id,module_id,key,title,status,self_paced,completion_rule) VALUES($1,$2,'snapshot-lesson','Snapshot lesson','published',true,'manual') RETURNING id`, f.Program, module)
		exec(`INSERT INTO academy_lesson_blocks(program_id,lesson_id,key,type,data) VALUES($1,$2,'snapshot-text','text','{"text":"Snapshot text"}')`, f.Program, lesson)
		exec(`UPDATE academy_course_programs SET revision=10 WHERE id=$1`, f.Program)
		rootRevision := id(`SELECT revision FROM academy_course_programs WHERE id=$1`, f.Program)
		request("/versions", map[string]any{"revision": rootRevision})
		snapshot := id(`SELECT id FROM academy_program_versions WHERE program_id=$1 AND source_revision=$2`, f.Program, rootRevision)
		if id(`SELECT count(*) FROM academy_program_version_drafts WHERE version_id=$1`, snapshot) != 0 {
			t.Fatal("legacy snapshot fixture already had a draft")
		}
		exec(academyProgramRevisionsSchema)
		request("/releases", map[string]any{"version_id": snapshot, "acknowledge_warnings": true})
		var created struct {
			VersionID int64 `json:"version_id"`
		}
		if err := json.Unmarshal(request("/versions", map[string]any{"source_version_id": snapshot}), &created); err != nil || created.VersionID == 0 {
			t.Fatal("new version failed", err)
		}
		state := revisionBackfillState(t, database)
		exec(academyProgramRevisionsSchema)
		if !reflect.DeepEqual(state, revisionBackfillState(t, database)) {
			t.Fatal("repeat modified new unpublished version")
		}
		request(fmt.Sprintf("/versions/%d/publish", created.VersionID), map[string]any{"revision": 1, "acknowledge_warnings": true})
		state = revisionBackfillState(t, database)
		exec(academyProgramRevisionsSchema)
		if !reflect.DeepEqual(state, revisionBackfillState(t, database)) {
			t.Fatal("repeat modified new publication")
		}
	})
}
