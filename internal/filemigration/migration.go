// Package filemigration inventories legacy references without changing their IDs
// or public URLs. Applying a reviewed manifest is always an explicit operation.
package filemigration

import (
	"FinTalent/internal/objectstorage"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type Source struct {
	Type    string                       `json:"type"`
	Path    string                       `json:"path,omitempty"`
	ID      int64                        `json:"id,omitempty"`
	File    objectstorage.RegisteredFile `json:"file"`
	Problem string                       `json:"problem,omitempty"`
}

var links = regexp.MustCompile(`/static/uploads/[A-Za-z0-9_./%+-]+`)

// ResolveSource refuses traversal and symlinks escaping a recognized upload directory.
func ResolveSource(root, relative string) (string, error) {
	if filepath.IsAbs(relative) || strings.Contains(relative, "\\") {
		return "", objectstorage.ErrInvalid
	}
	if filepath.ToSlash(filepath.Clean(relative)) != relative {
		return "", objectstorage.ErrInvalid
	}
	prefix := "static/uploads/"
	if strings.HasPrefix(relative, "uploads/resume-certificates/") {
		prefix = "uploads/resume-certificates/"
	}
	if !strings.HasPrefix(relative, prefix) {
		return "", objectstorage.ErrInvalid
	}
	for _, part := range strings.Split(relative, "/") {
		if strings.HasPrefix(part, ".") || strings.ContainsRune(part, ':') {
			return "", objectstorage.ErrInvalid
		}
	}
	if prefix == "static/uploads/" && !objectstorage.PublicUploadRef("/"+relative) {
		return "", objectstorage.ErrInvalid
	}
	base, e := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(strings.TrimSuffix(prefix, "/"))))
	if e != nil {
		return "", e
	}
	base, e = filepath.Abs(base)
	if e != nil {
		return "", e
	}
	p, e := filepath.EvalSymlinks(filepath.Join(base, filepath.FromSlash(strings.TrimPrefix(relative, prefix))))
	if e != nil {
		return "", e
	}
	rel, e := filepath.Rel(base, p)
	if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", objectstorage.ErrInvalid
	}
	return p, nil
}

func Inventory(ctx context.Context, db *sql.DB, root string) ([]Source, error) {
	owners := map[string]objectstorage.Owner{}
	originalNames := map[string]string{}
	ambiguous := map[string]bool{}
	queries := []struct{ kind, q string }{
		{"user", `SELECT id,json_build_object('avatar_url',avatar_url) FROM users`},
		{"user", `SELECT owner_id,to_jsonb(c) FROM courses c`},
		{"accounting_company", `SELECT id,to_jsonb(c) FROM accounting_companies c`},
		{"user", `SELECT author_id,to_jsonb(p) FROM publications p`},
		{"user", `SELECT author_id,to_jsonb(p) FROM publication_series p`},
		{"user", `SELECT p.author_id,to_jsonb(v) FROM publication_versions v JOIN publications p ON p.id=v.publication_id`},
		{"user", `SELECT author_user_id,to_jsonb(s) FROM profimarket_solutions s`},
		{"user", `SELECT s.author_user_id,to_jsonb(m) FROM profimarket_media m JOIN profimarket_solutions s ON s.id=m.solution_id`},
		{"user", `SELECT s.author_user_id,to_jsonb(m) FROM profimarket_regulation_sections m JOIN profimarket_solutions s ON s.id=m.solution_id`},
		{"user", `SELECT s.author_user_id,to_jsonb(i) FROM profimarket_regulation_items i JOIN profimarket_regulation_sections m ON m.id=i.section_id JOIN profimarket_solutions s ON s.id=m.solution_id`},
	}
	for _, query := range queries {
		rows, e := db.QueryContext(ctx, query.q)
		if e != nil {
			return nil, e
		}
		for rows.Next() {
			var id int64
			var raw []byte
			if e = rows.Scan(&id, &raw); e != nil {
				rows.Close()
				return nil, e
			}
			var v any
			if e = json.Unmarshal(raw, &v); e != nil {
				rows.Close()
				return nil, e
			}
			var walk func(any)
			walk = func(v any) {
				switch x := v.(type) {
				case string:
					for _, ref := range links.FindAllString(x, -1) {
						ref, e := url.PathUnescape(ref)
						if e != nil {
							continue
						}
						candidate := objectstorage.Owner{Kind: query.kind, ID: id}
						if old, ok := owners[ref]; ok && old != candidate {
							ambiguous[ref] = true
						} else {
							owners[ref] = candidate
						}
					}
				case map[string]any:
					for _, v := range x {
						walk(v)
					}
				case []any:
					for _, v := range x {
						walk(v)
					}
				}
			}
			walk(v)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return nil, e
		}
	}
	var registryExists bool
	if e := db.QueryRowContext(ctx, `SELECT to_regclass('object_files') IS NOT NULL`).Scan(&registryExists); e != nil {
		return nil, e
	}
	if registryExists {
		rows, e := db.QueryContext(ctx, `SELECT legacy_ref,owner_kind,owner_id,original_name FROM object_files`)
		if e != nil {
			return nil, e
		}
		for rows.Next() {
			var ref, kind, name string
			var id int64
			if e = rows.Scan(&ref, &kind, &id, &name); e != nil {
				rows.Close()
				return nil, e
			}
			if strings.HasPrefix(ref, "/static/uploads/") {
				owners[ref] = objectstorage.Owner{Kind: kind, ID: id}
				originalNames[ref] = name
				delete(ambiguous, ref)
			}
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return nil, e
		}
	}
	sources := []Source{}
	seen := map[string]bool{}
	addFile := func(ref, path string, owner objectstorage.Owner, problem string) {
		seen[ref] = true
		s := Source{Type: "file", Path: path, File: objectstorage.RegisteredFile{Ref: ref, Owner: owner, Category: "files", Name: filepath.Base(path)}, Problem: problem}
		if name := originalNames[ref]; name != "" {
			s.File.Name = name
		}
		p, e := ResolveSource(root, path)
		if e != nil {
			s.Problem = "source_missing_or_unsafe"
		} else {
			f, e := os.Open(p)
			if e != nil {
				s.Problem = "source_unreadable"
			} else {
				st, e := f.Stat()
				if e != nil || !st.Mode().IsRegular() {
					s.Problem = "source_not_regular"
				} else {
					s.File.Size = st.Size()
					prefix := make([]byte, 512)
					n, _ := f.Read(prefix)
					s.File.MIME = http.DetectContentType(prefix[:n])
					if strings.HasSuffix(strings.ToLower(path), ".svg") {
						s.File.MIME = "image/svg+xml"
					}
					if strings.HasPrefix(ref, "/static/uploads/") && st.Mode().Perm()&0004 == 0 {
						s.Problem = "source_not_publicly_readable"
					}
					if _, e = f.Seek(0, 0); e != nil {
						s.Problem = "source_unreadable"
					} else {
						digest := sha256.New()
						copied, err := io.Copy(digest, io.LimitReader(f, st.Size()+1))
						if err != nil || copied != st.Size() {
							s.Problem = "source_changed"
						} else {
							s.File.SHA256 = hex.EncodeToString(digest.Sum(nil))
						}
					}
				}
				f.Close()
			}
		}
		sources = append(sources, s)
	}
	for ref, owner := range owners {
		problem := ""
		if ambiguous[ref] {
			problem = "owner_ambiguous"
		}
		addFile(ref, strings.TrimPrefix(ref, "/"), owner, problem)
	}
	e := filepath.WalkDir(filepath.Join(root, "static", "uploads"), func(path string, d os.DirEntry, e error) error {
		if os.IsNotExist(e) {
			return nil
		}
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		rel, e := filepath.Rel(root, path)
		if e != nil {
			return e
		}
		rel = filepath.ToSlash(rel)
		ref := "/" + rel
		if seen[ref] {
			return nil
		}
		owner := objectstorage.Owner{}
		problem := "owner_unknown"
		if strings.HasPrefix(ref, "/static/uploads/position-icons/") {
			owner = objectstorage.Owner{Kind: "platform"}
			problem = ""
		}
		addFile(ref, rel, owner, problem)
		return nil
	})
	if e != nil {
		return nil, e
	}
	rows, e := db.QueryContext(ctx, `SELECT id,user_id,storage_name,original_name,content_type,file_size FROM resume_education_certificates ORDER BY id`)
	if e != nil {
		return nil, e
	}
	for rows.Next() {
		var id, owner, size int64
		var key, name, typ string
		if e = rows.Scan(&id, &owner, &key, &name, &typ, &size); e != nil {
			rows.Close()
			return nil, e
		}
		addFile("certificate:"+key, "uploads/resume-certificates/"+key, objectstorage.Owner{Kind: "user", ID: owner}, "")
		last := &sources[len(sources)-1]
		last.ID = id
		last.File.Name = name
		if last.File.Size != size {
			last.Problem = "source_size_mismatch"
		}
		last.File.MIME = typ
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	e = filepath.WalkDir(filepath.Join(root, "uploads", "resume-certificates"), func(path string, d os.DirEntry, err error) error {
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		ref := "certificate:" + strings.TrimPrefix(rel, "uploads/resume-certificates/")
		if !seen[ref] {
			addFile(ref, rel, objectstorage.Owner{}, "record_missing")
		}
		return nil
	})
	if e != nil {
		return nil, e
	}
	rows, e = db.QueryContext(ctx, `SELECT id,owner_id,name,mime,size,content FROM test_question_materials ORDER BY id`)
	if e != nil {
		return nil, e
	}
	for rows.Next() {
		s := Source{Type: "test_material"}
		s.File.Category = "tests"
		s.File.Owner.Kind = "user"
		var content []byte
		if e = rows.Scan(&s.ID, &s.File.Owner.ID, &s.File.Name, &s.File.MIME, &s.File.Size, &content); e != nil {
			rows.Close()
			return nil, e
		}
		s.File.Ref = "test-material:" + strconv.FormatInt(s.ID, 10)
		digest := sha256.Sum256(content)
		s.File.SHA256 = hex.EncodeToString(digest[:])
		sources = append(sources, s)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].File.Ref < sources[j].File.Ref })
	return sources, nil
}

func Apply(ctx context.Context, registry objectstorage.Registry, root string, s Source) error {
	if s.Problem != "" {
		return errors.New("manifest contains unresolved source")
	}
	digest, e := hex.DecodeString(s.File.SHA256)
	if e != nil || len(digest) != 32 {
		return objectstorage.ErrInvalid
	}
	if s.Type == "test_material" {
		var data []byte
		var owner, size int64
		var name, typ string
		if e := registry.DB.QueryRowContext(ctx, `SELECT content,owner_id,size,name,mime FROM test_question_materials WHERE id=$1`, s.ID).Scan(&data, &owner, &size, &name, &typ); e != nil {
			return e
		}
		if s.File.Ref != "test-material:"+strconv.FormatInt(s.ID, 10) || s.File.Owner != (objectstorage.Owner{Kind: "user", ID: owner}) || s.File.Category != "tests" || s.File.Size != size || s.File.MIME != typ || s.File.Name != name {
			return objectstorage.ErrInvalid
		}
		return registry.CopyVerified(ctx, s.File, bytes.NewReader(data))
	}
	if e = validateFileSource(ctx, registry.DB, s); e != nil {
		return e
	}
	p, e := ResolveSource(root, s.Path)
	if e != nil {
		return e
	}
	f, e := os.Open(p)
	if e != nil {
		return e
	}
	defer f.Close()
	if strings.HasPrefix(s.File.Ref, "/static/uploads/") {
		st, err := f.Stat()
		if err != nil {
			return err
		}
		if st.Mode().Perm()&0004 == 0 {
			return objectstorage.ErrInvalid
		}
	}
	prefix := make([]byte, 512)
	n, _ := f.Read(prefix)
	typ := http.DetectContentType(prefix[:n])
	if strings.HasSuffix(strings.ToLower(s.Path), ".svg") {
		typ = "image/svg+xml"
	}
	if typ != s.File.MIME {
		return objectstorage.ErrInvalid
	}
	if _, e = f.Seek(0, 0); e != nil {
		return e
	}
	return registry.CopyVerified(ctx, s.File, f)
}

func ReadManifest(r io.Reader) ([]Source, error) {
	var sources []Source
	decoder := json.NewDecoder(io.LimitReader(r, 64<<20))
	decoder.DisallowUnknownFields()
	if e := decoder.Decode(&sources); e != nil {
		return nil, e
	}
	var extra any
	if e := decoder.Decode(&extra); e != io.EOF {
		return nil, objectstorage.ErrInvalid
	}
	seen := map[string]bool{}
	for _, s := range sources {
		if s.File.Ref == "" || seen[s.File.Ref] {
			return nil, objectstorage.ErrInvalid
		}
		seen[s.File.Ref] = true
		switch s.Problem {
		case "", "source_missing_or_unsafe", "source_unreadable", "source_not_regular", "source_changed", "source_size_mismatch", "source_not_publicly_readable", "owner_ambiguous", "owner_unknown", "record_missing":
		default:
			return nil, objectstorage.ErrInvalid
		}
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].File.Ref < sources[j].File.Ref })
	return sources, nil
}

func validateFileSource(ctx context.Context, db *sql.DB, s Source) error {
	if s.Type != "file" {
		return objectstorage.ErrInvalid
	}
	if strings.HasPrefix(s.File.Ref, "certificate:") {
		var user, size int64
		var key, typ string
		if e := db.QueryRowContext(ctx, `SELECT user_id,storage_name,content_type,file_size FROM resume_education_certificates WHERE id=$1`, s.ID).Scan(&user, &key, &typ, &size); e != nil {
			return e
		}
		if s.File.Owner != (objectstorage.Owner{Kind: "user", ID: user}) || s.File.Category != "files" || s.File.Ref != "certificate:"+key || s.Path != "uploads/resume-certificates/"+key || s.File.Size != size || s.File.MIME != typ {
			return objectstorage.ErrInvalid
		}
	}
	if s.File.Ref != "/"+s.Path && !strings.HasPrefix(s.File.Ref, "certificate:") {
		return objectstorage.ErrInvalid
	}

	return nil
}
