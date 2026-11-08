package academy

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

func (h *Handler) versionRoute(w http.ResponseWriter, r *http.Request, course, actor int64, path []string) {
	if len(path) == 0 {
		if r.Method == "GET" {
			h.versionList(w, r, course)
			return
		}
		if r.Method == "POST" {
			var in struct {
				Source         int64  `json:"source_version_id"`
				LegacyRevision *int64 `json:"revision"`
			}
			if !decode(w, r, &in) {
				return
			}
			if in.LegacyRevision != nil {
				result, err := h.snapshot(r.Context(), course, actor, *in.LegacyRevision)
				if err != nil {
					problem(w, err)
					return
				}
				respond(w, 200, result)
				return
			}
			result, err := h.createProgramVersion(r, course, actor, in.Source)
			if err != nil {
				problem(w, err)
				return
			}
			respond(w, 200, result)
			return
		}
		fail(w, 405, "Метод не поддерживается")
		return
	}
	version, err := strconv.ParseInt(path[0], 10, 64)
	if err != nil || version <= 0 {
		problem(w, sql.ErrNoRows)
		return
	}
	action := "draft"
	if len(path) > 1 {
		action = path[1]
	}
	if action == "draft" && r.Method == "PUT" {
		var p Program
		if !decode(w, r, &p) {
			return
		}
		out, err := h.saveVersionDraft(r.Context(), course, version, actor, VersionDraft{VersionID: version, Revision: p.Revision, Program: p})
		if err != nil {
			problem(w, err)
			return
		}
		respond(w, 200, out.Program)
		return
	}
	if action == "actions" && r.Method == "POST" {
		var in Action
		if !decode(w, r, &in) {
			return
		}
		in.VersionID = version
		p, err := h.action(r.Context(), course, actor, in)
		if err != nil {
			problem(w, err)
			return
		}
		respond(w, 200, p)
		return
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, err)
		return
	}
	defer tx.Rollback()
	d, err := lockVersionDraft(r.Context(), tx, course, version, actor)
	if err != nil {
		problem(w, err)
		return
	}
	switch {
	case action == "refresh-tests" && r.Method == "POST":
		var in struct {
			Revision int64 `json:"revision"`
		}
		if !decode(w, r, &in) {
			return
		}
		if in.Revision != d.Revision {
			problem(w, ErrConflict)
			return
		}
		before, _ := json.Marshal(d.Tests)
		if err = seedVersionTests(r.Context(), tx, d, true); err != nil {
			problem(w, err)
			return
		}
		after, _ := json.Marshal(d.Tests)
		if string(before) != string(after) {
			if _, err = tx.ExecContext(r.Context(), `UPDATE academy_program_version_drafts SET test_sources=$2,revision=revision+1,updated_by=$3,updated_at=now() WHERE version_id=$1`, version, string(after), actor); err != nil {
				problem(w, err)
				return
			}
			d.Program.Revision++
		}
		if err = tx.Commit(); err != nil {
			problem(w, err)
			return
		}
		respond(w, 200, d.Program)
	case action == "draft" && r.Method == "GET":
		if err = tx.Commit(); err != nil {
			problem(w, err)
			return
		}
		respond(w, 200, d.Program)
	case action == "history" && r.Method == "GET":
		var raw json.RawMessage
		if len(path) > 2 {
			id, _ := strconv.ParseInt(path[2], 10, 64)
			err = tx.QueryRowContext(r.Context(), `SELECT to_jsonb(r) FROM academy_program_revisions r WHERE r.id=$1 AND r.version_id=$2`, id, version).Scan(&raw)
			if err != nil {
				problem(w, err)
				return
			}
			respond(w, 200, raw)
			return
		}
		err = tx.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(jsonb_build_object('id',id,'number',number,'published_at',published_at,'published_by',published_by) ORDER BY number DESC),'[]') FROM academy_program_revisions WHERE version_id=$1`, version).Scan(&raw)
		if err != nil {
			problem(w, err)
			return
		}
		respond(w, 200, raw)
	case action == "preview" && r.Method == "GET":
		if selected := r.URL.Query().Get("revision"); selected != "" {
			id, _ := strconv.ParseInt(selected, 10, 64)
			var raw []byte
			var tests json.RawMessage
			err = tx.QueryRowContext(r.Context(), `SELECT content,test_metadata FROM academy_program_revisions WHERE id=$1 AND version_id=$2`, id, version).Scan(&raw, &tests)
			if err == nil {
				err = json.Unmarshal(raw, &d.Program)
			}
			if err != nil {
				problem(w, err)
				return
			}
			if err = tx.Commit(); err != nil {
				problem(w, err)
				return
			}
			h.previewRoom(w, r, course, d.Program, tests)
			return
		}
		if err = seedVersionTests(r.Context(), tx, d, false); err != nil {
			problem(w, err)
			return
		}
		testItems := []map[string]any{}
		for _, m := range learningProgram(d.Program).Modules {
			for _, l := range m.Lessons {
				for _, b := range l.Blocks {
					if test, ok := d.Tests[b.Key]; ok {
						testItems = append(testItems, map[string]any{"id": test.VersionID, "block_key": b.Key, "title": test.Title})
					}
				}
			}
		}
		tests, _ := json.Marshal(testItems)
		if err = tx.Commit(); err != nil {
			problem(w, err)
			return
		}
		h.previewRoom(w, r, course, d.Program, tests)
	case action == "publication" && r.Method == "GET":
		plan, err := publicationPlan(r, tx, course, d)
		if err != nil {
			problem(w, err)
			return
		}
		respond(w, 200, plan)
	case action == "publish" && r.Method == "POST":
		var in struct {
			Revision    int64  `json:"revision"`
			Acknowledge bool   `json:"acknowledge_warnings"`
			Key         string `json:"request_key"`
		}
		if !decode(w, r, &in) {
			return
		}
		if in.Key != "" && !keyPattern.MatchString(in.Key) {
			problem(w, ErrInvalid)
			return
		}
		out, err := h.publishProgramDraft(r, tx, course, actor, d, in.Revision, in.Acknowledge, in.Key)
		if err != nil {
			problem(w, err)
			return
		}
		if err = tx.Commit(); err != nil {
			problem(w, err)
			return
		}
		respond(w, 200, out)
	default:
		fail(w, 405, "Метод не поддерживается")
	}
}

func (h *Handler) versionList(w http.ResponseWriter, r *http.Request, course int64) {
	var raw json.RawMessage
	err := h.db.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY x.number DESC),'[]') FROM(
 SELECT v.id,v.number,COALESCE(d.revision,1) revision,rel.published_at,
 (rel.version_id IS NULL OR (d.content-'revision'-'version_id'-'publication_revision_id') IS DISTINCT FROM (v.content-'revision'-'version_id'-'publication_revision_id')
 OR EXISTS(SELECT 1 FROM jsonb_each(d.test_sources) source WHERE source.key IN(SELECT b->>'key' FROM jsonb_array_elements(d.content->'modules') m CROSS JOIN LATERAL jsonb_array_elements(m->'lessons') l CROSS JOIN LATERAL jsonb_array_elements(l->'blocks') b WHERE m->>'status'='published' AND l->>'status'='published' AND b->>'type'='test') AND NOT EXISTS(SELECT 1 FROM academy_current_assessment_versions a WHERE a.program_version_id=v.id AND a.block_key=source.key AND jsonb_build_object('source_version_id',(a.config->>'test_version_id')::bigint,'title',a.title,'questions',a.questions)=source.value))) unpublished_changes,
 (SELECT count(*) FROM academy_cohorts ch WHERE ch.program_version_id=v.id) cohorts,
 (SELECT count(*) FROM academy_enrollments e WHERE e.program_version_id=v.id) students
 FROM academy_current_program_versions v JOIN academy_course_programs cp ON cp.id=v.program_id
 LEFT JOIN academy_program_version_drafts d ON d.version_id=v.id LEFT JOIN academy_current_version_releases rel ON rel.version_id=v.id
 WHERE cp.course_id=$1)x`, course).Scan(&raw)
	if err != nil {
		problem(w, err)
		return
	}
	respond(w, 200, raw)
}

func publicationPlan(r *http.Request, tx *sql.Tx, course int64, d *VersionDraft) (map[string]any, error) {
	var before Program
	var raw []byte
	var published bool
	err := tx.QueryRowContext(r.Context(), `SELECT content,EXISTS(SELECT 1 FROM academy_version_releases rel WHERE rel.version_id=v.id) FROM academy_current_program_versions v WHERE id=$1`, d.VersionID).Scan(&raw, &published)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(raw, &before); err != nil {
		return nil, err
	}
	if !published {
		before.Modules = nil
	}
	var cohorts, students, unique int
	err = tx.QueryRowContext(r.Context(), `SELECT (SELECT count(*) FROM academy_cohorts WHERE course_id=$1 AND program_version_id=$2),(SELECT count(*) FROM academy_enrollments WHERE course_id=$1 AND program_version_id=$2),(SELECT count(DISTINCT user_id) FROM academy_enrollments WHERE course_id=$1 AND program_version_id=$2)`, course, d.VersionID).Scan(&cohorts, &students, &unique)
	if err != nil {
		return nil, err
	}
	changed, err := changedTestSources(r.Context(), tx, d)
	if err != nil {
		return nil, err
	}
	changes := programChanges(before, d.Program)
	grading, err := testGradingChanged(r.Context(), tx, d, before.PublicationID)
	changes.RequirementsChanged = changes.RequirementsChanged || grading
	return map[string]any{"revision": d.Revision, "changes": changes, "changed_tests": changed, "cohorts": cohorts, "students": students, "unique_students": unique, "readiness": CheckReadiness(d.Program)}, err
}

func (h *Handler) createProgramVersion(r *http.Request, course, actor, source int64) (*Program, error) {
	ctx := r.Context()
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var owner int64
	if err = tx.QueryRowContext(ctx, `SELECT c.owner_id FROM courses c JOIN users u ON u.id=c.owner_id WHERE c.id=$1 AND NOT u.is_blocked AND(c.owner_id=$2 OR $3) FOR UPDATE OF c`, course, actor, adminPrincipal(ctx).Can("edit")).Scan(&owner); err != nil {
		return nil, err
	}
	program, _, err := lock(ctx, tx, course, actor)
	if err != nil {
		return nil, err
	}
	var p *Program
	if source == 0 {
		var hasPublished bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM academy_version_releases WHERE course_id=$1)`, course).Scan(&hasPublished); err != nil {
			return nil, err
		}
		if hasPublished {
			return nil, inputError{"Выберите опубликованную исходную версию"}
		}
		p, err = load(ctx, tx, course)
	} else {
		var raw []byte
		err = tx.QueryRowContext(ctx, `SELECT v.content FROM academy_current_program_versions v JOIN academy_version_releases rel ON rel.version_id=v.id WHERE v.id=$1 AND v.program_id=$2`, source, program).Scan(&raw)
		if err == nil {
			p = &Program{}
			err = json.Unmarshal(raw, p)
		}
	}
	if err != nil {
		return nil, err
	}
	seeds := map[string]capturedAcademyTest{}
	if source > 0 {
		for _, m := range learningProgram(*p).Modules {
			for _, l := range m.Lessons {
				for _, b := range l.Blocks {
					if b.Type == "test" && b.Data.TestVersionID > 0 {
						captured, e := publishedTestSource(ctx, tx, source, p.PublicationID, l.Key, b.Key, b.Data.TestVersionID)
						if e != nil {
							if e == sql.ErrNoRows {
								return nil, inputError{"Подготовьте и опубликуйте проверки исходной версии в редакторе программы"}
							}
							return nil, e
						}
						seeds[b.Key] = captured
					}
				}
			}
		}
	}
	if source > 0 {
		copied, keys := independentProgram(*p)
		p = &copied
		remapped := map[string]capturedAcademyTest{}
		for old, captured := range seeds {
			remapped[keys[old]] = captured
		}
		seeds = remapped
		for from, to := range keys {
			if _, err = tx.ExecContext(ctx, `INSERT INTO academy_asset_links(asset_id,program_id,entity_key) SELECT asset_id,program_id,$3 FROM academy_asset_links WHERE program_id=$1 AND entity_key=$2 ON CONFLICT DO NOTHING`, program, from, to); err != nil {
				return nil, err
			}
		}
	}
	p.ID, p.CourseID, p.VersionID, p.PublicationID, p.Revision = program, course, 0, 0, 0
	raw, _ := json.Marshal(p)
	var version int64
	err = tx.QueryRowContext(ctx, `INSERT INTO academy_program_versions(program_id,number,source_revision,content,created_by)
 VALUES($1,(SELECT COALESCE(max(number),0)+1 FROM academy_program_versions WHERE program_id=$1),
 (SELECT GREATEST(COALESCE((SELECT max(source_revision) FROM academy_program_versions WHERE program_id=$1),0),revision)+1 FROM academy_course_programs WHERE id=$1),$2,$3) RETURNING id`, program, string(raw), owner).Scan(&version)
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO academy_program_revision_heads(version_id) VALUES($1);`, version); err != nil {
		return nil, err
	}
	sourceJSON, _ := json.Marshal(seeds)
	if _, err = tx.ExecContext(ctx, `INSERT INTO academy_program_version_drafts(version_id,content,updated_by,test_sources) VALUES($1,$2,$3,$4)`, version, string(raw), owner, string(sourceJSON)); err != nil {
		return nil, err
	}
	// Keep the legacy save token ahead of reserved source revisions.
	if _, err = tx.ExecContext(ctx, `UPDATE academy_course_programs SET revision=(SELECT source_revision+1 FROM academy_program_versions WHERE id=$2) WHERE id=$1`, program, version); err != nil {
		return nil, err
	}
	p.VersionID, p.Revision = version, 1
	return p, tx.Commit()
}

func (h *Handler) publishProgramDraft(r *http.Request, tx *sql.Tx, course, actor int64, d *VersionDraft, revision int64, ack bool, key string) (map[string]any, error) {
	ctx := r.Context()
	var existing int64
	if key != "" {
		err := tx.QueryRowContext(ctx, `SELECT id FROM academy_program_revisions WHERE version_id=$1 AND request_key=$2`, d.VersionID, key).Scan(&existing)
		if err == nil {
			return map[string]any{"published": true, "publication_revision_id": existing, "replayed": true}, nil
		}
		if err != sql.ErrNoRows {
			return nil, err
		}
	}
	if err := seedVersionTests(ctx, tx, d, false); err != nil {
		return nil, err
	}
	err := tx.QueryRowContext(ctx, `SELECT id FROM academy_program_revisions WHERE version_id=$1 AND draft_revision=$2`, d.VersionID, revision).Scan(&existing)
	if err == nil {
		if revision != d.Revision {
			return map[string]any{"published": true, "publication_revision_id": existing, "replayed": true}, nil
		}
		changed, checkErr := changedTestSources(ctx, tx, d)
		if checkErr != nil {
			return nil, checkErr
		}
		if changed == 0 {
			return map[string]any{"published": true, "publication_revision_id": existing, "replayed": true}, nil
		}
		if _, err = tx.ExecContext(ctx, `UPDATE academy_program_version_drafts SET revision=revision+1,updated_by=$2,updated_at=now() WHERE version_id=$1`, d.VersionID, actor); err != nil {
			return nil, err
		}
		d.Revision++
		revision = d.Revision
		err = sql.ErrNoRows
	}
	if err != sql.ErrNoRows {
		return nil, err
	}
	if revision != d.Revision {
		return nil, ErrConflict
	}
	if err = Validate(&d.Program); err != nil {
		return nil, inputError{err.Error()}
	}
	if err = validateVersionReferences(ctx, tx, d.Program); err != nil {
		return nil, err
	}
	ready := CheckReadiness(d.Program)
	if len(ready.Blockers) > 0 {
		return nil, inputError{strings.Join(ready.Blockers, ". ")}
	}
	if len(ready.Warnings) > 0 && !ack {
		return nil, inputError{"Подтвердите предупреждения перед публикацией"}
	}
	var publication, owner int64
	err = tx.QueryRowContext(ctx, `SELECT nextval(pg_get_serial_sequence('academy_program_revisions','id')),owner_id FROM courses WHERE id=$1`, course).Scan(&publication, &owner)
	if err != nil {
		return nil, err
	}
	if err = freezeRevisionAssessments(ctx, tx, d.VersionID, d.Program, owner, publication, d.Tests); err != nil {
		return nil, err
	}
	var tests json.RawMessage
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(jsonb_agg(jsonb_build_object('id',(config->>'test_version_id')::bigint,'block_key',block_key,'title',title) ORDER BY id),'[]') FROM academy_assessment_versions WHERE publication_revision_id=$1 AND kind='test'`, publication).Scan(&tests)
	if err != nil {
		return nil, err
	}
	d.Program.Revision, d.Program.PublicationID = 0, 0
	raw, _ := json.Marshal(d.Program)
	_, err = tx.ExecContext(ctx, `INSERT INTO academy_program_revisions(id,version_id,number,draft_revision,content,assessment_definitions,test_metadata,published_by,request_key)
 SELECT $1,$2,(SELECT COALESCE(max(number),0)+1 FROM academy_program_revisions WHERE version_id=$2),$3,$4,
 COALESCE((SELECT jsonb_agg(to_jsonb(a) ORDER BY a.id) FROM academy_assessment_versions a WHERE a.publication_revision_id=$1),'[]'),$5,$6,NULLIF($7,'')`, publication, d.VersionID, revision, string(raw), string(tests), actor, key)
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO academy_version_releases(version_id,course_id,published_by,test_metadata) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, d.VersionID, course, actor, string(tests)); err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO academy_program_revision_heads(version_id,published_revision_id) VALUES($1,$2) ON CONFLICT(version_id) DO UPDATE SET published_revision_id=excluded.published_revision_id`, d.VersionID, publication)
	return map[string]any{"published": true, "version_id": d.VersionID, "publication_revision_id": publication}, err
}
