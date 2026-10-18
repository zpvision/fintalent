package main

import (
	"FinTalent/internal/academy"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// Called inside the existing isolated-cloud-schema course suite, never against application data.
func testAcademyStageA(t *testing.T, db *sql.DB, mux *http.ServeMux, gate func(http.HandlerFunc) http.HandlerFunc, owner, other, course int64) {
	t.Helper()
	if e := prepareAcademyDatabase(context.Background()); e != nil {
		t.Fatal("repeat Academy migration", e)
	}
	h := academy.New(db, func(r *http.Request) (int64, error) { return strconv.ParseInt(r.Header.Get("X-Test-User"), 10, 64) }, gate, t.TempDir())
	h.Register(mux)
	base := fmt.Sprintf("/api/academy/courses/%d", course)
	request := func(method, path string, user int64, v any, status int) []byte {
		t.Helper()
		b, _ := json.Marshal(v)
		r := httptest.NewRequest(method, path, bytes.NewReader(b))
		if user > 0 {
			r.Header.Set("X-Test-User", strconv.FormatInt(user, 10))
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s: got %d want %d: %s", method, path, w.Code, status, w.Body.String())
		}
		return w.Body.Bytes()
	}
	program := func(raw []byte) academy.Program {
		t.Helper()
		var p academy.Program
		if e := json.Unmarshal(raw, &p); e != nil {
			t.Fatal(e)
		}
		return p
	}
	request("GET", base, 0, nil, 401)
	request("GET", base, other, nil, 404)
	before := request("GET", fmt.Sprintf("/api/courses/my/%d", course), owner, nil, 200)
	p := program(request("POST", base+"/program", owner, map[string]string{"source": "landing"}, 200))
	if len(p.Modules) != 2 || len(p.Modules[0].Lessons) != 2 {
		t.Fatal("landing not copied")
	}
	again := program(request("POST", base+"/program", owner, map[string]string{"source": "empty"}, 200))
	if again.ID != p.ID || len(again.Modules) != 2 {
		t.Fatal("retry overwrote program")
	}
	request("GET", base+"/modules", owner, nil, 200)
	request("GET", base+"/modules/"+p.Modules[0].Key+"/lessons", owner, nil, 200)
	request("GET", base+"/lessons/"+p.Modules[0].Lessons[0].Key+"/blocks", owner, nil, 200)
	request("GET", base+"/modules/unknown-key", owner, nil, 404)
	p = program(request("POST", base+"/modules", owner, academy.Action{Revision: p.Revision, Module: &academy.Module{Title: "Временный модуль"}}, 200))
	removeKey := p.Modules[len(p.Modules)-1].Key
	p = program(request("DELETE", base+"/modules/"+removeKey, owner, academy.Action{Revision: p.Revision}, 200))
	savedCopy := p.Modules[0].Lessons[0].Key
	action := func(typ, key, target string, index int) {
		t.Helper()
		p = program(request("POST", base+"/actions", owner, academy.Action{Revision: p.Revision, Type: typ, Key: key, Target: target, Index: index}, 200))
	}
	action("lesson.move", savedCopy, p.Modules[1].Key, 0)
	if p.Modules[1].Lessons[0].Key != savedCopy {
		t.Fatal("lesson move failed")
	}
	action("module.move", p.Modules[1].Key, "", 0)
	if p.Modules[0].Lessons[0].Key != savedCopy {
		t.Fatal("module reorder failed")
	}
	l := &p.Modules[0].Lessons[0]
	l.Title = "Независимый урок Академии"
	l.Blocks = []academy.Block{{Key: academy.Key(), Type: "text", Data: academy.BlockData{Rich: []academy.RichBlock{{Type: "paragraph", Text: `<b>Учебный текст</b><img src=x onerror=alert(1)>`}}}}, {Key: academy.Key(), Type: "video", Data: academy.BlockData{URL: "https://youtu.be/abc123"}}, {Key: academy.Key(), Type: "assignment", Data: academy.BlockData{Title: "Практика", Text: "Инструкция", AnswerFormat: "file"}}}
	old := p
	p = program(request("PUT", base+"/program", owner, p, 200))
	request("PUT", base+"/program", owner, old, 409)
	// Test links use the existing engine and enforce authorship at a fixed test version.
	var ownVersion, foreignVersion int64
	for i, uid := range []int64{owner, other} {
		var testID, versionID int64
		if e := db.QueryRow(`INSERT INTO tests(author_id,slug) VALUES($1,$2) RETURNING id`, uid, fmt.Sprintf("academy-test-%d", i)).Scan(&testID); e != nil {
			t.Fatal(e)
		}
		if e := db.QueryRow(`INSERT INTO test_versions(test_id,version,title,created_by) VALUES($1,1,'Academy test',$2) RETURNING id`, testID, uid).Scan(&versionID); e != nil {
			t.Fatal(e)
		}
		if uid == owner {
			ownVersion = versionID
		} else {
			foreignVersion = versionID
		}
	}
	p.Modules[0].Lessons[0].Blocks = append(p.Modules[0].Lessons[0].Blocks, academy.Block{Key: academy.Key(), Type: "test", Data: academy.BlockData{TestVersionID: ownVersion}})
	p = program(request("PUT", base+"/program", owner, p, 200))
	foreignJSON, _ := json.Marshal(p)
	var forbidden academy.Program
	_ = json.Unmarshal(foreignJSON, &forbidden)
	forbidden.Modules[0].Lessons[0].Blocks[len(forbidden.Modules[0].Lessons[0].Blocks)-1].Data.TestVersionID = foreignVersion
	request("PUT", base+"/program", owner, forbidden, 400)
	if bytes.Contains(request("GET", base+"/program", owner, nil, 200), []byte("onerror")) {
		t.Fatal("unsafe rich text persisted")
	}
	blockKey := p.Modules[0].Lessons[0].Blocks[0].Key
	action("block.move", blockKey, "", 2)
	if p.Modules[0].Lessons[0].Blocks[2].Key != blockKey {
		t.Fatal("block reorder failed")
	}
	// Upload and download a real (small) private PDF; filenames never become storage paths.
	var upload bytes.Buffer
	mw := multipart.NewWriter(&upload)
	_ = mw.WriteField("entity_key", savedCopy)
	part, _ := mw.CreateFormFile("file", "../../private.pdf")
	_, _ = part.Write([]byte("%PDF-1.7\nAcademy fixture"))
	_ = mw.Close()
	r := httptest.NewRequest("POST", base+"/assets", &upload)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	r.Header.Set("X-Test-User", strconv.FormatInt(owner, 10))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 201 {
		t.Fatal("upload failed", w.Code, w.Body.String())
	}
	var asset struct{ ID int64 }
	_ = json.Unmarshal(w.Body.Bytes(), &asset)
	assetPath := fmt.Sprintf("%s/assets/%d", base, asset.ID)
	request("GET", assetPath, other, nil, 404)
	request("GET", assetPath, 0, nil, 401)
	request("GET", assetPath, owner, nil, 200)
	p.Modules[0].Lessons[0].Blocks = append(p.Modules[0].Lessons[0].Blocks, academy.Block{Key: academy.Key(), Type: "file", Data: academy.BlockData{AssetID: asset.ID}})
	p = program(request("PUT", base+"/program", owner, p, 200))
	bad := p
	bad.Modules = append([]academy.Module{}, p.Modules...)
	bad.Modules[1] = p.Modules[1]
	bad.Modules[1].Lessons = append([]academy.Lesson{}, p.Modules[1].Lessons...)
	bad.Modules[1].Lessons[0].Blocks = []academy.Block{{Key: academy.Key(), Type: "file", Data: academy.BlockData{AssetID: asset.ID}}}
	request("PUT", base+"/program", owner, bad, 400)
	action("lesson.duplicate", savedCopy, "", 0)
	if len(p.Modules[0].Lessons) != 3 {
		t.Fatal("duplicate lesson failed")
	}
	action("module.duplicate", p.Modules[0].Key, "", 0)
	if len(p.Modules) != 3 {
		t.Fatal("duplicate module failed")
	}
	preview := request("GET", base+"/preview/"+savedCopy, owner, nil, 200)
	if !bytes.Contains(preview, []byte("Независимый урок")) {
		t.Fatal("draft preview missing")
	}
	snap := request("POST", base+"/versions", owner, map[string]int64{"revision": p.Revision}, 200)
	snap2 := request("POST", base+"/versions", owner, map[string]int64{"revision": p.Revision}, 200)
	if !bytes.Equal(snap, snap2) {
		t.Fatal("snapshot retry not idempotent")
	}
	if _, e := db.Exec(`UPDATE academy_program_versions SET content='{}' WHERE program_id=$1`, p.ID); e == nil {
		t.Fatal("snapshot was mutable")
	}
	p.Modules[0].Lessons[0].Title = "Новая редакция"
	p = program(request("PUT", base+"/program", owner, p, 200))
	var snapshot string
	if e := db.QueryRow(`SELECT content::text FROM academy_program_versions WHERE program_id=$1`, p.ID).Scan(&snapshot); e != nil || strings.Contains(snapshot, "Новая редакция") {
		t.Fatal("snapshot changed", e)
	}
	request("DELETE", assetPath, owner, nil, 409)
	// A second owner cannot adopt any child of the first program.
	var second struct{ ID int64 }
	_ = json.Unmarshal(request("POST", "/api/courses/my", other, nil, 201), &second)
	otherBase := fmt.Sprintf("/api/academy/courses/%d", second.ID)
	otherP := program(request("POST", otherBase+"/program", other, map[string]string{"source": "empty"}, 200))
	otherP.Modules = p.Modules
	request("PUT", otherBase+"/program", other, otherP, 400)
	action("module.delete", p.Modules[0].Key, "", 0)
	action("lesson.delete", p.Modules[0].Lessons[0].Key, "", 0)
	after := request("GET", fmt.Sprintf("/api/courses/my/%d", course), owner, nil, 200)
	if !bytes.Equal(before, after) {
		t.Fatal("Academy changed landing")
	}
	// Marketing edits never touch Academy rows.
	if _, e := db.Exec(`UPDATE course_modules SET title='Изменён только лендинг' WHERE course_id=$1`, course); e != nil {
		t.Fatal(e)
	}
	latest := program(request("GET", base+"/program", owner, nil, 200))
	for _, m := range latest.Modules {
		if m.Title == "Изменён только лендинг" {
			t.Fatal("landing edit changed Academy")
		}
	}
	t.Run("Stage B learning", func(t *testing.T) { testAcademyStageB(t, db, mux, owner, other, course) })
}
