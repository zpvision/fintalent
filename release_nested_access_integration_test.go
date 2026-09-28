package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"FinTalent/internal/accountingcompany"
)

// The caller has already created and verified a fresh cloud test schema.
func testReleaseNestedAccess(t *testing.T, ctx context.Context) {
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.ExecContext(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	insert := func(q string, args ...any) int64 {
		t.Helper()
		var id int64
		if err := db.QueryRowContext(ctx, q, args...).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	owner := insert(`INSERT INTO users(full_name,email,password_hash) VALUES('Nested owner','nested-owner@example.invalid',repeat('*',60)) RETURNING id`)
	other := insert(`INSERT INTO users(full_name,email,password_hash) VALUES('Nested reader','nested-reader@example.invalid',repeat('*',60)) RETURNING id`)
	cookies := map[int64]*http.Cookie{}
	for _, uid := range []int64{owner, other} {
		w := httptest.NewRecorder()
		if err := createSession(w, uid); err != nil {
			t.Fatal(err)
		}
		cookies[uid] = w.Result().Cookies()[0]
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/resumes/", resumeKnowledgeActionHandler)
	mux.HandleFunc("/api/publications/", publicationActionAPI)
	mux.HandleFunc("/api/publication-authors/", publicationAuthorAPI)
	accountingcompany.New(db, func(r *http.Request) (accountingcompany.User, error) {
		uid, _ := strconv.ParseInt(r.Header.Get("X-Fixture-User"), 10, 64)
		if uid == 0 {
			return accountingcompany.User{}, errors.New("guest")
		}
		return accountingcompany.User{ID: uid, FullName: "Fixture"}, nil
	}, func(r *http.Request) bool { return r.Header.Get("X-Fixture-Admin") == "1" }).Register(mux)
	request := func(uid int64, method, path, body string, admin bool) *httptest.ResponseRecorder {
		bounded, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		r := httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(bounded)
		if uid > 0 {
			r.AddCookie(cookies[uid])
			r.Header.Set("X-Fixture-User", strconv.FormatInt(uid, 10))
		}
		if admin {
			r.Header.Set("X-Fixture-Admin", "1")
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	resume := insert(`INSERT INTO resumes(user_id,status,visibility) VALUES($1,'published','public') RETURNING id`, owner)
	company := insert(`INSERT INTO accounting_companies(owner_user_id,name,slug,status) VALUES($1,'Nested company','nested-company','published') RETURNING id`, owner)
	pub := insert(`INSERT INTO publications(author_id,title,slug,status,visibility) VALUES($1,'Nested publication','nested-publication','published','public') RETURNING id`, owner)
	knowledgePath := fmt.Sprintf("/api/resumes/%d/test-knowledge", resume)
	companyPath := fmt.Sprintf("/api/accounting-companies/%d", company)
	pubPath := fmt.Sprintf("/api/publications/%d", pub)
	for _, state := range []struct {
		name, status             string
		blocked, system, deleted bool
	}{
		{"public", "published", false, false, false}, {"draft", "draft", false, false, false},
		{"blocked", "published", true, false, false}, {"system", "published", true, true, false},
		{"deleted", "published", false, false, true},
	} {
		t.Run(state.name, func(t *testing.T) {
			exec(`UPDATE users SET is_blocked=$2,is_system=$3 WHERE id=$1`, owner, state.blocked, state.system)
			for _, table := range []string{"resumes", "accounting_companies", "publications"} {
				id := resume
				if table == "accounting_companies" {
					id = company
				}
				if table == "publications" {
					id = pub
				}
				exec("UPDATE "+table+" SET status=$2,deleted_at=CASE WHEN $3 THEN NOW() ELSE NULL END WHERE id=$1", id, state.status, state.deleted)
			}
			public := state.status == "published" && !state.deleted && (!state.blocked || state.system)
			for _, uid := range []int64{0, owner, other} {
				for _, path := range []string{knowledgePath, companyPath + "/passport", pubPath + "/comments", pubPath + "/versions"} {
					allowed := public || (state.name == "draft" && uid == owner && path != knowledgePath)
					want := 404
					if allowed {
						want = 200
					}
					if w := request(uid, "GET", path, "", false); w.Code != want {
						t.Fatalf("%s user=%d: %d want %d: %s", path, uid, w.Code, want, w.Body.String())
					}
				}
			}
			wantReview := 404
			if public {
				wantReview = 201
			}
			if w := request(other, "POST", companyPath+"/reviews", `{"text":"Fixture company review","rating":5}`, false); w.Code != wantReview {
				t.Fatalf("review: %d want %d %s", w.Code, wantReview, w.Body.String())
			}
			if !public {
				for _, action := range []string{"comments", "reaction", "bookmark", "progress", "report"} {
					if w := request(other, "POST", pubPath+"/"+action, `{"body":"Fixture comment"}`, false); w.Code != 404 {
						t.Fatalf("hidden %s: %d", action, w.Code)
					}
				}
			}
			wantAdmin := 404
			if public || state.name == "draft" {
				wantAdmin = 200
			}
			if w := request(other, "GET", companyPath+"/passport", "", true); w.Code != wantAdmin {
				t.Fatalf("admin passport: %d want %d", w.Code, wantAdmin)
			}
		})
	}
	exec(`UPDATE users SET is_blocked=false,is_system=false WHERE id=$1`, owner)
	exec(`UPDATE resumes SET status='published',deleted_at=NULL WHERE id=$1`, resume)
	exec(`UPDATE accounting_companies SET status='published',deleted_at=NULL WHERE id=$1`, company)
	exec(`UPDATE publications SET status='published',deleted_at=NULL WHERE id=$1`, pub)
	subscriptionPath := fmt.Sprintf("/api/publication-authors/%d/subscribe", owner)
	if w := request(other, "POST", subscriptionPath, "", false); w.Code != 200 || !strings.Contains(w.Body.String(), `"active":true`) {
		t.Fatalf("subscribe: %d %s", w.Code, w.Body.String())
	}
	exec(`UPDATE users SET is_blocked=true WHERE id=$1`, owner)
	if w := request(other, "POST", subscriptionPath, "", false); w.Code != 404 {
		t.Fatalf("blocked subscription: %d", w.Code)
	}
	assertCount(t, `SELECT count(*) FROM notifications WHERE user_id=$1 AND type='new_follower'`, 1, owner)
	exec(`UPDATE users SET is_blocked=false WHERE id=$1`, owner)
	if w := request(other, "POST", subscriptionPath, "", false); w.Code != 200 || !strings.Contains(w.Body.String(), `"active":false`) {
		t.Fatalf("unsubscribe: %d %s", w.Code, w.Body.String())
	}
	for _, action := range []struct {
		name, body string
		status     int
	}{
		{"reaction", `{"type":"useful"}`, 200},
		{"bookmark", `{}`, 200},
		{"progress", `{"Progress":65}`, 200},
		{"report", `{"Type":"other","Details":"Fixture report"}`, 201},
	} {
		if w := request(other, "POST", pubPath+"/"+action.name, action.body, false); w.Code != action.status {
			t.Fatalf("public %s: %d %s", action.name, w.Code, w.Body.String())
		}
	}
	for _, table := range []string{"publication_reactions", "publication_bookmarks", "publication_read_progress", "publication_reports"} {
		assertCount(t, "SELECT count(*) FROM "+table+" WHERE publication_id=$1", 1, pub)
	}
	t.Run("interaction holds parent visibility lock", func(t *testing.T) {
		w := httptest.NewRecorder()
		tx, ok := beginPublicationInteraction(w, httptest.NewRequest("POST", pubPath, nil).WithContext(ctx), pub)
		if !ok {
			t.Fatalf("begin: %d %s", w.Code, w.Body.String())
		}
		defer tx.Rollback()
		blocked, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
		defer cancel()
		change, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer change.Rollback()
		_, err = change.ExecContext(blocked, `UPDATE publications SET status='draft' WHERE id=$1`, pub)
		// A timed-out autocommit statement can have an uncertain outcome during
		// cancellation. This competing test write must never be committed.
		change.Rollback()
		if err == nil || blocked.Err() == nil {
			t.Fatalf("visibility update did not wait for interaction: %v", err)
		}
		if !commitPublicationInteraction(w, tx) {
			t.Fatalf("commit: %d", w.Code)
		}
	})
	test := insert(`INSERT INTO tests(author_id,slug,status,visibility) VALUES($1,'nested-test','published','public') RETURNING id`, owner)
	version := insert(`INSERT INTO test_versions(test_id,version,title,created_by) VALUES($1,1,'Nested competency',$2) RETURNING id`, test, owner)
	body := fmt.Sprintf(`{"test_id":%d}`, test)
	if w := request(other, "POST", knowledgePath+"/confirmations", body, false); w.Code != 404 {
		t.Fatalf("missing result confirmed: %d", w.Code)
	}
	attempt := insert(`INSERT INTO test_attempts(test_id,test_version_id,user_id,status,finished_at,show_in_resume,context,percent) VALUES($1,$2,$3,'finished',NOW(),true,'{"employee_invitation_id":999}',75) RETURNING id`, test, version, owner)
	if w := request(other, "POST", knowledgePath+"/confirmations", body, false); w.Code != 404 {
		t.Fatalf("employee result confirmed as owner: %d", w.Code)
	}
	exec(`UPDATE test_attempts SET context='{}'::jsonb WHERE id=$1`, attempt)
	for i := 0; i < 2; i++ {
		if w := request(other, "POST", knowledgePath+"/confirmations", body, false); w.Code != 200 {
			t.Fatalf("valid confirmation: %d %s", w.Code, w.Body.String())
		}
	}
	assertCount(t, `SELECT count(*) FROM resume_test_confirmations WHERE resume_id=$1`, 1, resume)
	if w := request(owner, "POST", knowledgePath+"/confirmations", body, false); w.Code != 403 {
		t.Fatalf("self confirmation: %d", w.Code)
	}
	db.SetMaxOpenConns(1)
	for _, path := range []string{knowledgePath, companyPath + "/passport"} {
		if w := request(other, "GET", path, "", false); w.Code != 200 || !strings.Contains(w.Body.String(), "Nested competency") {
			db.SetMaxOpenConns(16)
			t.Fatalf("single connection read: %d %s", w.Code, w.Body.String())
		}
	}
	db.SetMaxOpenConns(16)
	if w := request(other, "DELETE", knowledgePath+"/confirmations", body, false); w.Code != 200 {
		t.Fatalf("remove confirmation: %d", w.Code)
	}
	foreignPub := insert(`INSERT INTO publications(author_id,title,slug,status,visibility) VALUES($1,'Other publication','other-nested-publication','published','public') RETURNING id`, owner)
	parent := insert(`INSERT INTO publication_comments(publication_id,author_id,body) VALUES($1,$2,'Parent fixture') RETURNING id`, foreignPub, owner)
	if w := request(other, "POST", pubPath+"/comments", fmt.Sprintf(`{"Body":"Reply fixture","ParentID":%d}`, parent), false); w.Code != 400 {
		t.Fatalf("foreign parent: %d %s", w.Code, w.Body.String())
	}
	exec(`UPDATE publication_comments SET publication_id=$2 WHERE id=$1`, parent, pub)
	if w := request(other, "POST", pubPath+"/comments", fmt.Sprintf(`{"Body":"Reply fixture","ParentID":%d}`, parent), false); w.Code != 201 {
		t.Fatalf("valid parent: %d %s", w.Code, w.Body.String())
	}
}
