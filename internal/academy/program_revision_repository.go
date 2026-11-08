package academy

import (
	"context"
	"database/sql"
	"encoding/json"
)

// lockVersionDraft deliberately uses the existing owner/admin authorization.
// No learner or cohort-staff permission grants access to unpublished content.
func lockVersionDraft(ctx context.Context, tx *sql.Tx, course, version, actor int64) (*VersionDraft, error) {
	var program, owner int64
	err := tx.QueryRowContext(ctx, `SELECT p.id,c.owner_id FROM academy_program_versions v
 JOIN academy_course_programs p ON p.id=v.program_id JOIN courses c ON c.id=p.course_id
 JOIN users u ON u.id=c.owner_id
 WHERE v.id=$1 AND c.id=$2 AND NOT u.is_blocked AND(c.owner_id=$3 OR $4)
 FOR UPDATE OF c`, version, course, actor, adminPrincipal(ctx).Can("edit")).Scan(&program, &owner)
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO academy_program_revision_heads(version_id) VALUES($1) ON CONFLICT DO NOTHING`, version); err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO academy_program_version_drafts(version_id,content,updated_by)
 SELECT id,content,created_by FROM academy_program_versions WHERE id=$1 ON CONFLICT DO NOTHING`, version)
	if err != nil {
		return nil, err
	}
	d := &VersionDraft{VersionID: version}
	var raw, sources []byte
	err = tx.QueryRowContext(ctx, `SELECT revision,content,test_sources FROM academy_program_version_drafts WHERE version_id=$1 FOR UPDATE`, version).Scan(&d.Revision, &raw, &sources)
	if err == nil {
		err = json.Unmarshal(raw, &d.Program)
	}
	if err != nil {
		return nil, err
	}
	d.Program.ID, d.Program.CourseID = program, course
	d.Program.VersionID, d.Program.Revision = version, d.Revision
	if err = json.Unmarshal(sources, &d.Tests); err != nil {
		return nil, err
	}
	return d, nil
}

// saveVersionDraft never invokes saveTree: relational rows are the legacy
// course-wide draft and cannot isolate edits of two different versions.
func (h *Handler) saveVersionDraft(ctx context.Context, course, version, actor int64, in VersionDraft) (*VersionDraft, error) {
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	d, err := lockVersionDraft(ctx, tx, course, version, actor)
	if err != nil {
		return nil, err
	}
	if d.Revision != in.Revision || in.VersionID != version {
		return nil, ErrConflict
	}
	in.Program.ID, in.Program.CourseID = d.Program.ID, course
	in.Program.VersionID = version
	in.Program.Source = d.Program.Source
	in.Tests = d.Tests
	if err = Validate(&in.Program); err != nil {
		return nil, inputError{err.Error()}
	}
	if err = validateVersionReferences(ctx, tx, in.Program); err != nil {
		return nil, err
	}
	if err = seedVersionTests(ctx, tx, &in, false); err != nil {
		return nil, err
	}
	sources, _ := json.Marshal(in.Tests)
	in.Program.VersionID, in.Program.PublicationID, in.Program.Revision = version, 0, 0
	old := d.Program
	old.VersionID, old.PublicationID, old.Revision = version, 0, 0
	raw, err := json.Marshal(in.Program)
	if err != nil {
		return nil, err
	}
	oldRaw, _ := json.Marshal(old)
	if string(oldRaw) == string(raw) {
		in.Program.Revision = d.Revision
		in.Revision = d.Revision
		return &in, tx.Commit()
	}
	var updated int64
	err = tx.QueryRowContext(ctx, `UPDATE academy_program_version_drafts
 SET content=$3,revision=revision+1,updated_by=$4,updated_at=now(),test_sources=$5
 WHERE version_id=$1 AND revision=$2 RETURNING revision`, version, d.Revision, string(raw), actor, string(sources)).Scan(&updated)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	in.Revision = updated
	in.Program.Revision = updated
	return &in, nil
}

func validateVersionReferences(ctx context.Context, tx *sql.Tx, p Program) error {
	type assetReference struct {
		ID    int64  `json:"id"`
		Key   string `json:"key"`
		Image bool   `json:"image"`
		Video bool   `json:"video"`
	}
	refs := []assetReference{}
	tests := []int64{}
	add := func(id int64, key string, image, video bool) {
		if id != 0 {
			refs = append(refs, assetReference{id, key, image, video})
		}
	}
	for _, m := range p.Modules {
		add(m.CoverAssetID, m.Key, true, false)
		for _, l := range m.Lessons {
			add(l.CoverAssetID, l.Key, true, false)
			for _, b := range l.Blocks {
				add(b.Data.AssetID, l.Key, b.Type == "image", b.Type == "video")
				add(b.Data.PosterID, l.Key, true, false)
				for _, id := range b.Data.MaterialIDs {
					if id <= 0 {
						return ErrInvalid
					}
					add(id, l.Key, false, false)
				}
				if b.Data.TestVersionID != 0 {
					tests = append(tests, b.Data.TestVersionID)
				}
			}
		}
	}
	raw, _ := json.Marshal(refs)
	var valid bool
	err := tx.QueryRowContext(ctx, `SELECT NOT EXISTS(
 SELECT 1 FROM jsonb_to_recordset($2::jsonb) x(id bigint,key text,image boolean,video boolean)
 WHERE NOT EXISTS(SELECT 1 FROM academy_media_assets a JOIN academy_asset_links l ON l.asset_id=a.id
 WHERE a.id=x.id AND a.program_id=$1 AND l.program_id=$1 AND l.entity_key=x.key
 AND a.submission_id IS NULL AND a.deleted_at IS NULL
 AND(NOT x.image OR a.mime LIKE 'image/%') AND(NOT x.video OR a.mime='video/mp4')))`, p.ID, string(raw)).Scan(&valid)
	if err != nil {
		return err
	}
	if !valid {
		return ErrInvalid
	}
	raw, _ = json.Marshal(tests)
	err = tx.QueryRowContext(ctx, `SELECT NOT EXISTS(SELECT 1 FROM jsonb_array_elements_text($2::jsonb) x
 WHERE NOT EXISTS(SELECT 1 FROM test_versions v JOIN tests t ON t.id=v.test_id JOIN courses c ON c.owner_id=t.author_id
 WHERE v.id=x.value::bigint AND c.id=$1 AND t.deleted_at IS NULL AND t.status NOT IN('deleted','blocked','archived')))`, p.CourseID, string(raw)).Scan(&valid)
	if err != nil {
		return err
	}
	if !valid {
		return ErrInvalid
	}
	if p.VersionID > 0 {
		type identity struct {
			Key  string `json:"key"`
			Kind string `json:"kind"`
		}
		identities := []identity{}
		for _, m := range p.Modules {
			identities = append(identities, identity{m.Key, "module"})
			for _, l := range m.Lessons {
				identities = append(identities, identity{l.Key, "lesson"})
				for _, b := range l.Blocks {
					kind := "content"
					if oneOf(b.Type, "test", "assignment") {
						kind = b.Type
					}
					identities = append(identities, identity{b.Key, kind})
				}
			}
		}
		raw, _ = json.Marshal(identities)
		err = tx.QueryRowContext(ctx, `WITH contents AS(SELECT content FROM academy_program_versions WHERE id=$1 UNION ALL SELECT content FROM academy_program_revisions WHERE version_id=$1),
 modules AS(SELECT m FROM contents c CROSS JOIN LATERAL jsonb_array_elements(c.content->'modules') m),
 lessons AS(SELECT l FROM modules CROSS JOIN LATERAL jsonb_array_elements(m->'lessons') l),
 kinds AS(SELECT m->>'key' key,'module' kind FROM modules UNION SELECT l->>'key','lesson' FROM lessons UNION SELECT b->>'key',CASE WHEN b->>'type' IN('test','assignment') THEN b->>'type' ELSE 'content' END FROM lessons CROSS JOIN LATERAL jsonb_array_elements(l->'blocks') b)
 SELECT NOT EXISTS(SELECT 1 FROM kinds old JOIN jsonb_to_recordset($2::jsonb) current(key text,kind text) ON old.key=current.key WHERE old.kind<>current.kind)`, p.VersionID, string(raw)).Scan(&valid)
		if err != nil {
			return err
		}
		if !valid {
			return inputError{"Для другого типа проверки создайте новый блок. История существующего блока защищена"}
		}
	}
	return nil
}
