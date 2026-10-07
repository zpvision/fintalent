package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"FinTalent/internal/accountingcompany"
	"FinTalent/internal/testmodule/domain"
	"FinTalent/internal/testmodule/dto"
	"FinTalent/internal/testmodule/repository"
	"FinTalent/internal/testmodule/service"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

// Explicit opt-in; only a fresh, verified schema is used, never public. No app
// server or notification workers start, so these fixtures cannot send mail.
func TestReleaseSecurityIsolatedPostgres(t *testing.T) {
	if os.Getenv("RUN_RELEASE_SECURITY_DB_TESTS") != "1" {
		t.Skip("requires opt-in isolated cloud PostgreSQL test")
	}
	loadLocalEnv(".env")
	config, err := pgx.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal("invalid DATABASE_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	admin := stdlib.OpenDB(*config)
	t.Cleanup(func() { admin.Close() })
	schema := fmt.Sprintf("release_security_%d", time.Now().UnixNano())
	t.Logf("isolated test schema: %s", schema)
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = admin.ExecContext(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if !strings.HasPrefix(schema, "release_security_") {
			t.Fatal("unsafe cleanup target")
		}
		if _, err := admin.ExecContext(context.Background(), "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Errorf("isolated schema cleanup: %v", err)
		}
	})
	// admin stays open until the cleanup callback has removed only this schema.
	config.RuntimeParams["search_path"] = schema
	isolated := stdlib.OpenDB(*config)
	original := db
	db = isolated
	defer func() { db = original; isolated.Close() }()
	var actual string
	if err = isolated.QueryRowContext(ctx, "SELECT current_schema()").Scan(&actual); err != nil || actual != schema {
		t.Fatal("schema isolation could not be verified")
	}
	t.Setenv("APP_ENV", "production")
	t.Setenv("SEED_DEMO_DATA", "false")
	t.Setenv("SYNC_GEOGRAPHY", "false")
	if err = prepareDatabase(); err != nil {
		t.Fatal(err)
	}
	t.Log("isolated schema initialization complete")
	t.Run("credential session lifecycle", func(t *testing.T) { testReleaseCredentialSessions(t, ctx) })
	t.Run("contact access and concurrency", func(t *testing.T) { testReleaseContacts(t, ctx) })
	t.Run("help access and concurrency", func(t *testing.T) { testReleaseHelp(t, ctx) })
	t.Run("client exchange access and races", func(t *testing.T) { testReleaseClientExchange(t, ctx) })
	t.Run("nested public access matrix", func(t *testing.T) { testReleaseNestedAccess(t, ctx) })
	t.Run("all participant question types", func(t *testing.T) { testReleaseQuestionTypes(t, ctx) })
	t.Run("vacancy attached version and availability", func(t *testing.T) { testReleaseVacancyVersion(t, ctx) })
	var author, reader int64
	for i, target := range []*int64{&author, &reader} {
		if err = db.QueryRowContext(ctx, `INSERT INTO users(full_name,email,password_hash) VALUES('Audit fixture',$1,repeat('*',60)) RETURNING id`, fmt.Sprintf("fixture%d@example.invalid", i)).Scan(target); err != nil {
			t.Fatal(err)
		}
	}
	var cookie *http.Cookie
	recorder := httptest.NewRecorder()
	if err = createSession(recorder, reader); err != nil {
		t.Fatal(err)
	}
	cookie = recorder.Result().Cookies()[0]
	run := func(handler http.HandlerFunc, method, path, body string, auth bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if auth {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		handler(w, r)
		return w
	}

	t.Run("company rollback and review moderation", func(t *testing.T) {
		mux := http.NewServeMux()
		adminAccess := false
		accountingcompany.New(db, func(*http.Request) (accountingcompany.User, error) {
			return accountingcompany.User{ID: author, FullName: "Fixture", Email: "fixture@example.invalid"}, nil
		}, func(*http.Request) bool { return adminAccess }).Register(mux)
		request := func(method, path, body string) *httptest.ResponseRecorder {
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
			return w
		}
		w := request("POST", "/api/accounting-companies", `{"name":"Fixture company","direction_ids":[99999999]}`)
		if w.Code < 400 {
			t.Fatalf("invalid dictionary accepted: %d", w.Code)
		}
		assertCount(t, "SELECT count(*) FROM accounting_companies WHERE owner_user_id=$1", 0, author)
		var companyID, reviewID int64
		if err := db.QueryRowContext(ctx, `INSERT INTO accounting_companies(owner_user_id,name,slug) VALUES($1,'Fixture company','fixture-company') RETURNING id`, author).Scan(&companyID); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRowContext(ctx, `INSERT INTO accounting_company_reviews(company_id,author_name,text,rating) VALUES($1,'Fixture','Review fixture',5) RETURNING id`, companyID).Scan(&reviewID); err != nil {
			t.Fatal(err)
		}
		path := fmt.Sprintf("/api/admin/community/company-reviews/%d/approve", reviewID)
		if w = request("POST", path, ""); w.Code != 403 {
			t.Fatalf("nonadmin moderation: %d", w.Code)
		}
		adminAccess = true
		if w = request("GET", "/api/admin/community/company-reviews", ""); w.Code != 200 || !strings.Contains(w.Body.String(), "Review fixture") {
			t.Fatalf("review list: %d %s", w.Code, w.Body.String())
		}
		if w = request("POST", path, ""); w.Code != 200 {
			t.Fatalf("review approval: %d %s", w.Code, w.Body.String())
		}
		if w = request("POST", path, ""); w.Code != 409 {
			t.Fatalf("repeat moderation: %d", w.Code)
		}
		assertCount(t, "SELECT count(*) FROM accounting_company_reviews WHERE id=$1 AND status='published'", 1, reviewID)
	})
	t.Run("admin session expiry and revocation", func(t *testing.T) {
		t.Setenv("ADMIN_LOGIN", "release-audit")
		t.Setenv("ADMIN_PASSWORD", "release-audit-password")
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/api/admin/login", nil)
		if err = createAdminSession(w, r); err != nil {
			t.Fatal(err)
		}
		r.AddCookie(w.Result().Cookies()[0])
		if !isAdmin(r) {
			t.Fatal("valid session rejected")
		}
		adminLogout(httptest.NewRecorder(), r)
		if isAdmin(r) {
			t.Fatal("logout token still valid")
		}
		w = httptest.NewRecorder()
		if err = createAdminSession(w, r); err != nil {
			t.Fatal(err)
		}
		r = httptest.NewRequest("GET", "/api/admin/session", nil)
		r.AddCookie(w.Result().Cookies()[0])
		if _, err = db.ExecContext(ctx, "UPDATE admin_sessions SET expires_at=NOW()-INTERVAL '1 second'"); err != nil {
			t.Fatal(err)
		}
		if isAdmin(r) {
			t.Fatal("expired token accepted")
		}
	})
	t.Run("solution child endpoints and purchase idempotency", func(t *testing.T) {
		var id int64
		if err = db.QueryRowContext(ctx, `INSERT INTO profimarket_solutions(author_user_id,type,title,slug,status,pricing_type,price) VALUES($1,'AI_ASSISTANT','Fixture solution','audit-fixture','DRAFT','FREE',0) RETURNING id`, author).Scan(&id); err != nil {
			t.Fatal(err)
		}
		for _, h := range []http.HandlerFunc{profiMarketReviewsAPI, profiMarketQuestionsAPI} {
			if w := run(h, "GET", fmt.Sprintf("/?solution_id=%d", id), "", false); w.Code != 404 {
				t.Fatalf("draft child: %d %s", w.Code, w.Body.String())
			}
		}
		if _, err = db.ExecContext(ctx, "UPDATE profimarket_solutions SET status='PUBLISHED' WHERE id=$1", id); err != nil {
			t.Fatal(err)
		}
		t.Run("regulation sections load without nested connection", func(t *testing.T) {
			var sectionID int64
			if e := db.QueryRowContext(ctx, `INSERT INTO profimarket_regulation_sections(solution_id,title) VALUES($1,'Fixture section') RETURNING id`, id).Scan(&sectionID); e != nil {
				t.Fatal(e)
			}
			if _, e := db.ExecContext(ctx, `INSERT INTO profimarket_regulation_items(section_id,title) VALUES($1,'Fixture item')`, sectionID); e != nil {
				t.Fatal(e)
			}
			db.SetMaxOpenConns(1)
			defer db.SetMaxOpenConns(16)
			bounded, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			value, e := loadProfiSolution(bounded, fmt.Sprint(id), nil)
			if e != nil {
				t.Fatal(e)
			}
			if len(value.Sections) != 1 || len(value.Sections[0].Items) != 1 {
				t.Fatal("regulation sections lost")
			}
		})
		for range 2 {
			r := httptest.NewRequest("POST", "/", strings.NewReader("{}"))
			r.Header.Set("Idempotency-Key", "fixture-action")
			w := httptest.NewRecorder()
			profiPurchaseAction(w, r, id, &user{ID: reader, FullName: "Fixture", Email: "fixture1@example.invalid"})
			if w.Code != 201 && w.Code != 200 {
				t.Fatalf("purchase: %d %s", w.Code, w.Body.String())
			}
		}
		assertCount(t, "SELECT count(*) FROM profimarket_purchases WHERE solution_id=$1", 1, id)
		assertCount(t, "SELECT count(*) FROM notification_outbox WHERE kind IN('order','event')", 2)
		if _, err = db.ExecContext(ctx, "UPDATE users SET is_blocked=TRUE WHERE id=$1", author); err != nil {
			t.Fatal(err)
		}
		for _, h := range []http.HandlerFunc{profiMarketReviewsAPI, profiMarketQuestionsAPI} {
			if w := run(h, "GET", fmt.Sprintf("/?solution_id=%d", id), "", false); w.Code != 404 {
				t.Fatalf("blocked child: %d", w.Code)
			}
		}
		if _, err = db.ExecContext(ctx, "UPDATE users SET is_blocked=FALSE WHERE id=$1", author); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("test answers deadlines versions and concurrent revision", func(t *testing.T) {
		repo := repository.New(db)
		svc := service.New(repo)
		limit := 60
		test, err := svc.Create(ctx, author, dto.CreateTest{Title: "Release security test", Difficulty: "easy", Visibility: "public", IsFree: true, PassingPercent: 60, TimeLimitSeconds: &limit})
		if err != nil {
			t.Fatal(err)
		}
		qid, err := svc.AddQuestion(ctx, test.ID, author, dto.CreateQuestion{Question: "Fixture question", QuestionType: domain.QuestionText, Explanation: "secret explanation", Points: 1, Answers: []dto.AnswerInput{{Answer: "secret answer", IsCorrect: true}}})
		if err != nil {
			t.Fatal(err)
		}
		if err = svc.Publish(ctx, test.ID, author); err != nil {
			t.Fatal(err)
		}
		public, err := svc.Get(ctx, test.ID, reader, false)
		if err != nil {
			t.Fatal(err)
		}
		if len(public.Questions[0].Answers) != 0 || public.Questions[0].Explanation != "" {
			t.Fatal("public grading material exposed")
		}
		a, err := svc.Start(ctx, test.ID, reader, 0)
		if err != nil {
			t.Fatal(err)
		}
		if a.TimeLimitSeconds != 60 || len(a.Questions[0].Answers) != 0 {
			t.Fatal("attempt secret or timer contract")
		}
		if err = svc.SaveAnswer(ctx, a.ID, reader, dto.SubmitAnswer{QuestionID: qid + 999999, TextAnswer: "x"}); err == nil {
			t.Fatal("foreign question accepted")
		}
		if err = svc.SaveAnswer(ctx, a.ID, reader, dto.SubmitAnswer{QuestionID: qid, TextAnswer: "secret answer"}); err != nil {
			t.Fatal(err)
		}
		if err = repo.FinishAttempt(ctx, a.ID, 0, 0, 1, 0, false, nil); err != repository.ErrConflict {
			t.Fatalf("stale grade accepted: %v", err)
		}
		result, err := svc.Finish(ctx, a.ID, reader)
		if err != nil {
			t.Fatal(err)
		}
		if result.Percent != 100 {
			t.Fatal(result.Percent)
		}
		if err = svc.SaveAnswer(ctx, a.ID, reader, dto.SubmitAnswer{QuestionID: qid, TextAnswer: "changed"}); err == nil {
			t.Fatal("finished answer changed")
		}
		assertCount(t, "SELECT count(*) FROM test_attempt_answers WHERE attempt_id=$1 AND text_answer='secret answer'", 1, a.ID)
		a, err = svc.Start(ctx, test.ID, reader, 0)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.ExecContext(ctx, "UPDATE test_attempts SET started_at=NOW()-INTERVAL '2 minutes' WHERE id=$1", a.ID); err != nil {
			t.Fatal(err)
		}
		if err = repo.SaveAttemptAnswer(ctx, a.ID, dto.SubmitAnswer{QuestionID: qid, TextAnswer: "late"}); err == nil {
			t.Fatal("late answer accepted")
		}
		t.Run("parallel answer and finish preserve committed answers", func(t *testing.T) {
			for range 3 {
				attempt, e := svc.Start(ctx, test.ID, reader, 0)
				if e != nil {
					t.Fatal(e)
				}
				if e = svc.SaveAnswer(ctx, attempt.ID, reader, dto.SubmitAnswer{QuestionID: qid, TextAnswer: "wrong"}); e != nil {
					t.Fatal(e)
				}
				var wg sync.WaitGroup
				start := make(chan struct{})
				for i := 0; i < 3; i++ {
					wg.Add(1)
					go func(i int) {
						defer wg.Done()
						<-start
						if i == 0 {
							_ = svc.SaveAnswer(ctx, attempt.ID, reader, dto.SubmitAnswer{QuestionID: qid, TextAnswer: "secret answer"})
						} else {
							_, _ = svc.Finish(ctx, attempt.ID, reader)
						}
					}(i)
				}
				close(start)
				wg.Wait()
				current, e := repo.GetAttempt(ctx, attempt.ID)
				if e != nil {
					t.Fatal(e)
				}
				if current.Status == "started" {
					if _, e = svc.Finish(ctx, attempt.ID, reader); e != nil {
						t.Fatal(e)
					}
				}
				var answer string
				var percent float64
				if e = db.QueryRowContext(ctx, `SELECT aa.text_answer,a.percent FROM test_attempts a JOIN test_attempt_answers aa ON aa.attempt_id=a.id WHERE a.id=$1`, attempt.ID).Scan(&answer, &percent); e != nil {
					t.Fatal(e)
				}
				if (answer == "secret answer") != (percent == 100) {
					t.Fatalf("answer/result race: %q %v", answer, percent)
				}
				assertCount(t, "SELECT count(*) FROM test_attempt_answers WHERE attempt_id=$1", 1, attempt.ID)
			}
		})
		t.Run("employee expiry revocation and personal isolation", func(t *testing.T) {
			var employeeID, invitationID, versionID int64
			if e := db.QueryRowContext(ctx, `INSERT INTO company_test_employees(owner_user_id,full_name,email) VALUES($1,'Employee fixture','employee@example.invalid') RETURNING id`, author).Scan(&employeeID); e != nil {
				t.Fatal(e)
			}
			if e := db.QueryRowContext(ctx, `SELECT v.id FROM test_versions v JOIN tests t ON t.id=v.test_id AND t.current_version=v.version WHERE t.id=$1`, test.ID).Scan(&versionID); e != nil {
				t.Fatal(e)
			}
			token := strings.Repeat("e", 64)
			if e := db.QueryRowContext(ctx, `INSERT INTO company_test_invitations(owner_user_id,employee_id,test_id,test_version_id,token) VALUES($1,$2,$3,$4,$5) RETURNING id`, author, employeeID, test.ID, versionID, token).Scan(&invitationID); e != nil {
				t.Fatal(e)
			}
			call := func(action, body string) *httptest.ResponseRecorder {
				method := "POST"
				if action == "" {
					method = "GET"
				}
				return run(publicEmployeeTest, method, "/api/employee-test/"+token+action, body, false)
			}
			if w := call("/start", ""); w.Code != 200 {
				t.Fatalf("employee start: %d %s", w.Code, w.Body.String())
			}
			if w := call("", ""); w.Code != 200 || strings.Contains(w.Body.String(), "secret answer") {
				t.Fatalf("employee participant DTO: %d %s", w.Code, w.Body.String())
			}
			var employeeAttempt int64
			if e := db.QueryRowContext(ctx, "SELECT attempt_id FROM company_test_invitations WHERE id=$1", invitationID).Scan(&employeeAttempt); e != nil {
				t.Fatal(e)
			}
			if e := repo.SetAttemptResumeVisibility(ctx, employeeAttempt, author, true); e == nil {
				t.Fatal("employee result can be published as owner result")
			}
			body := fmt.Sprintf(`{"question_id":%d,"text_answer":"secret answer"}`, qid)
			for _, state := range []string{"expired", "revoked", "blocked"} {
				var e error
				switch state {
				case "expired":
					_, e = db.ExecContext(ctx, "UPDATE company_test_invitations SET expires_at=NOW()-INTERVAL '1 second' WHERE id=$1", invitationID)
				case "revoked":
					_, e = db.ExecContext(ctx, "UPDATE company_test_invitations SET status='revoked' WHERE id=$1", invitationID)
				case "blocked":
					_, e = db.ExecContext(ctx, "UPDATE users SET is_blocked=TRUE WHERE id=$1", author)
				}
				if e != nil {
					t.Fatal(e)
				}
				for _, action := range []string{"", "/start", "/answer", "/finish"} {
					if w := call(action, body); w.Code < 400 {
						t.Fatalf("%s %s accepted: %d", state, action, w.Code)
					}
				}
				if _, e = db.ExecContext(ctx, "UPDATE company_test_invitations SET status='started',expires_at=NOW()+INTERVAL '1 day' WHERE id=$1", invitationID); e != nil {
					t.Fatal(e)
				}
				if _, e = db.ExecContext(ctx, "UPDATE users SET is_blocked=FALSE WHERE id=$1", author); e != nil {
					t.Fatal(e)
				}
			}
			if w := call("/answer", body); w.Code != 200 {
				t.Fatalf("employee answer: %d %s", w.Code, w.Body.String())
			}
			if w := call("/finish", ""); w.Code != 200 {
				t.Fatalf("employee finish: %d %s", w.Code, w.Body.String())
			}
			attempts, e := repo.ListAttempts(ctx, 0, author, false)
			if e != nil {
				t.Fatal(e)
			}
			for _, a := range attempts {
				if a.ID == employeeAttempt {
					t.Fatal("employee result included in personal history")
				}
			}
		})
		t.Run("question loading releases pool connection", func(t *testing.T) {
			db.SetMaxOpenConns(1)
			defer db.SetMaxOpenConns(16)
			bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			if _, e := repo.Get(bounded, test.ID, true); e != nil {
				t.Fatal(e)
			}
			if _, e := repo.GetAttempt(bounded, a.ID); e != nil {
				t.Fatal(e)
			}
		})
		if err = svc.ForkDraft(ctx, test.ID, author); err != nil {
			t.Fatal(err)
		}
		if err = repo.UpdateQuestion(ctx, qid, author, dto.CreateQuestion{Question: "mutated", QuestionType: "text", Points: 1}); err == nil {
			t.Fatal("historical question changed")
		}
		var historicalAnswer int64
		if err := db.QueryRowContext(ctx, `SELECT id FROM test_answers WHERE question_id=$1 ORDER BY id LIMIT 1`, qid).Scan(&historicalAnswer); err != nil {
			t.Fatal(err)
		}
		if err := repo.DeleteQuestion(ctx, qid, author); err == nil {
			t.Fatal("historical question deleted")
		}
		if _, err := repo.AddAnswer(ctx, qid, author, dto.AnswerInput{Answer: "injected"}); err == nil {
			t.Fatal("historical answer added")
		}
		if err := repo.UpdateAnswer(ctx, historicalAnswer, author, dto.AnswerInput{Answer: "mutated"}); err == nil {
			t.Fatal("historical answer changed")
		}
		if err := repo.DeleteAnswer(ctx, historicalAnswer, author); err == nil {
			t.Fatal("historical answer deleted")
		}
		assertCount(t, `SELECT count(*) FROM test_answers WHERE id=$1 AND answer='secret answer' AND is_correct`, 1, historicalAnswer)
		if _, err = db.ExecContext(ctx, "UPDATE tests SET status='published',visibility='private' WHERE id=$1", test.ID); err != nil {
			t.Fatal(err)
		}
		if _, err = svc.Get(ctx, test.ID, reader, false); err == nil {
			t.Fatal("private test disclosed")
		}
	})
	t.Run("publication privacy cache and child access", func(t *testing.T) {
		var id int64
		if err = db.QueryRowContext(ctx, `INSERT INTO publications(author_id,title,slug,status) VALUES($1,'Draft fixture','draft-fixture','draft') RETURNING id`, author).Scan(&id); err != nil {
			t.Fatal(err)
		}
		for _, action := range []string{"comments", "versions", "recommendations"} {
			w := run(publicationActionAPI, "GET", fmt.Sprintf("/api/publications/%d/%s", id, action), "", false)
			if w.Code != 404 {
				t.Fatalf("private %s: %d %s", action, w.Code, w.Body.String())
			}
		}
		var d publicationDetail
		raw, _ := json.Marshal(d)
		if strings.Contains(string(raw), "author_email") {
			t.Fatal("login email in public DTO")
		}
		var ownSeries, foreignSeries int64
		for i, uid := range []int64{author, reader} {
			var series int64
			if err := db.QueryRowContext(ctx, `INSERT INTO publication_series(author_id,title,slug) VALUES($1,'Series fixture',$2) RETURNING id`, uid, fmt.Sprintf("release-series-%d", i)).Scan(&series); err != nil {
				t.Fatal(err)
			}
			if i == 0 {
				ownSeries = series
			} else {
				foreignSeries = series
			}
		}
		for _, series := range []int64{ownSeries, foreignSeries} {
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			err = savePublicationLinks(ctx, tx, id, publicationInput{SeriesID: series})
			if series == ownSeries {
				if err != nil {
					tx.Rollback()
					t.Fatal(err)
				}
				if err := tx.Commit(); err != nil {
					t.Fatal(err)
				}
			} else {
				tx.Rollback()
				if err == nil {
					t.Fatal("foreign series accepted")
				}
			}
		}
		assertCount(t, `SELECT count(*) FROM publication_series_items WHERE publication_id=$1 AND series_id=$2`, 1, id, ownSeries)
	})
	t.Run("password reset concurrent verify single use and error budget", func(t *testing.T) {
		t.Setenv("PASSWORD_RESET_SECRET", "isolated-test-secret-at-least-thirty-two-characters")
		secret, e := passwordResetSecret()
		if e != nil {
			t.Fatal(e)
		}
		email := "fixture0@example.invalid"
		newRequest := func(expired bool) int64 {
			t.Helper()
			if _, e := db.ExecContext(ctx, "UPDATE password_reset_requests SET used_at=NOW() WHERE user_id=$1 AND used_at IS NULL", author); e != nil {
				t.Fatal(e)
			}
			var id int64
			if e := db.QueryRowContext(ctx, `INSERT INTO password_reset_requests(user_id,email_hash,code_hash,request_ip,expires_at) VALUES($1,$2,$3,'192.0.2.1',NOW()+INTERVAL '10 minutes') RETURNING id`, author, resetHMAC(secret, "email", email), resetHMAC(secret, "code", email+":123456")).Scan(&id); e != nil {
				t.Fatal(e)
			}
			if expired {
				if _, e := db.ExecContext(ctx, "UPDATE password_reset_requests SET expires_at=NOW()-INTERVAL '1 second' WHERE id=$1", id); e != nil {
					t.Fatal(e)
				}
			}
			return id
		}
		verify := func(code string) *httptest.ResponseRecorder {
			return run(verifyPasswordReset, "POST", "/api/password-reset/verify", fmt.Sprintf(`{"email":%q,"code":%q}`, email, code), false)
		}
		newRequest(false)
		var count atomic.Int32
		var wg sync.WaitGroup
		tokens := make(chan string, 8)
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				w := verify("123456")
				if w.Code == 200 {
					count.Add(1)
					var data map[string]string
					if e := json.Unmarshal(w.Body.Bytes(), &data); e != nil {
						t.Error(e)
					}
					tokens <- data["reset_token"]
				} else if w.Code != 400 {
					t.Errorf("concurrent verify: %d", w.Code)
				}
			}()
		}
		wg.Wait()
		close(tokens)
		if count.Load() != 1 {
			t.Fatalf("code verified %d times", count.Load())
		}
		token := <-tokens
		payload := fmt.Sprintf(`{"reset_token":%q,"password":"fixture-password-new","password_confirmation":"fixture-password-new"}`, token)
		if w := run(completePasswordReset, "POST", "/api/password-reset/complete", payload, false); w.Code != 200 {
			t.Fatalf("complete reset: %d %s", w.Code, w.Body.String())
		}
		if w := run(completePasswordReset, "POST", "/api/password-reset/complete", payload, false); w.Code != 400 {
			t.Fatalf("reset reused: %d", w.Code)
		}
		id := newRequest(false)
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if w := verify("654321"); w.Code != 400 {
					t.Errorf("wrong code: %d", w.Code)
				}
			}()
		}
		wg.Wait()
		var failures int
		if e := db.QueryRowContext(ctx, "SELECT failed_attempts FROM password_reset_requests WHERE id=$1", id).Scan(&failures); e != nil {
			t.Fatal(e)
		}
		if failures != passwordResetMaxAttempts {
			t.Fatalf("error budget: %d", failures)
		}
		if w := verify("123456"); w.Code != 400 {
			t.Fatalf("exhausted code accepted: %d", w.Code)
		}
		newRequest(true)
		if w := verify("123456"); w.Code != 400 {
			t.Fatalf("expired code: %d", w.Code)
		}
	})
	t.Run("private employment fields stay private in public help", func(t *testing.T) {
		var resumeID, topicID int64
		if e := db.QueryRowContext(ctx, `INSERT INTO resumes(user_id,status,visibility,desired_salary,work_preferences) VALUES($1,'published','private',777777,'PRIVATE_WORK_FIXTURE') RETURNING id`, author).Scan(&resumeID); e != nil {
			t.Fatal(e)
		}
		if e := db.QueryRowContext(ctx, `INSERT INTO help_topics(name) VALUES('Audit help topic') RETURNING id`).Scan(&topicID); e != nil {
			t.Fatal(e)
		}
		if _, e := db.ExecContext(ctx, `INSERT INTO resume_help_topics(resume_id,topic_id) VALUES($1,$2)`, resumeID, topicID); e != nil {
			t.Fatal(e)
		}
		for _, filter := range []struct {
			query string
			count int
		}{{fmt.Sprintf("&help_topic=%d&profile_mode=professional", topicID), 1}, {fmt.Sprintf("&help_topic=%d&profile_mode=job_search", topicID), 0}, {"", 0}} {
			w := run(publicCatalogHandler, "GET", "/api/public/catalog?kind=resumes"+filter.query, "", false)
			if w.Code != 200 {
				t.Fatalf("catalog: %d %s", w.Code, w.Body.String())
			}
			if strings.Contains(w.Body.String(), "777777") || strings.Contains(w.Body.String(), "PRIVATE_WORK_FIXTURE") {
				t.Fatal("employment data disclosed in catalog")
			}
			var result struct {
				Items []json.RawMessage `json:"items"`
			}
			if e := json.Unmarshal(w.Body.Bytes(), &result); e != nil {
				t.Fatal(e)
			}
			if len(result.Items) != filter.count {
				t.Fatalf("private filter %s: %s", filter.query, w.Body.String())
			}
		}
		view, e := loadPublicResume(httptest.NewRequest("GET", "/", nil), resumeID)
		if e != nil {
			t.Fatal(e)
		}
		raw, e := json.Marshal(view)
		if e != nil {
			t.Fatal(e)
		}
		if strings.Contains(string(raw), "777777") || strings.Contains(string(raw), "PRIVATE_WORK_FIXTURE") {
			t.Fatal("employment data disclosed through direct profile")
		}
	})
	t.Run("notification queue bounded admission", func(t *testing.T) {
		tx, e := db.BeginTx(ctx, nil)
		if e != nil {
			t.Fatal(e)
		}
		defer tx.Rollback()
		if _, e = tx.ExecContext(ctx, `INSERT INTO notification_outbox(kind,recipient_email) SELECT 'event','fixture@example.invalid' FROM generate_series(1,10000)`); e != nil {
			t.Fatal(e)
		}
		if e = enqueueNotification(ctx, tx, "event", "Fixture", "fixture@example.invalid", "Fixture", map[string]string{}, ""); e == nil {
			t.Fatal("full queue accepted a message")
		}
	})
	t.Run("request reset uniform responses and concurrent admission", func(t *testing.T) {
		t.Setenv("PASSWORD_RESET_SECRET", "isolated-test-secret-at-least-thirty-two-characters")
		for _, email := range []string{"resetknown@example.invalid", "resetfull@example.invalid"} {
			if _, e := db.ExecContext(ctx, `INSERT INTO users(full_name,email,password_hash) VALUES('Reset fixture',$1,repeat('*',60))`, email); e != nil {
				t.Fatal(e)
			}
		}
		request := func(email string) *httptest.ResponseRecorder {
			r := httptest.NewRequest("POST", "/api/password-reset/request", strings.NewReader(fmt.Sprintf(`{"email":%q}`, email)))
			r.RemoteAddr = "203.0.113.55:4321"
			w := httptest.NewRecorder()
			requestPasswordReset(w, r)
			return w
		}
		var wg sync.WaitGroup
		for range 3 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if w := request("resetknown@example.invalid"); w.Code != 200 {
					t.Errorf("request reset: %d", w.Code)
				}
			}()
		}
		wg.Wait()
		assertCount(t, `SELECT count(*) FROM notification_outbox WHERE kind='reset' AND recipient_email=$1`, 1, "resetknown@example.invalid")
		unknown := request("resetunknown@example.invalid")
		known := request("resetknown@example.invalid")
		if unknown.Code != 200 || unknown.Body.String() != known.Body.String() {
			t.Fatalf("unknown account response differs: %d %s", unknown.Code, unknown.Body.String())
		}
		if _, e := db.ExecContext(ctx, `INSERT INTO notification_outbox(kind,recipient_email) SELECT 'capacity-fixture','fixture@example.invalid' FROM generate_series(1,10000)`); e != nil {
			t.Fatal(e)
		}
		full := request("resetfull@example.invalid")
		missing := request("resetmissing@example.invalid")
		if full.Code != 200 || missing.Code != 200 || full.Body.String() != missing.Body.String() {
			t.Fatalf("full queue discloses existence: %d/%d", full.Code, missing.Code)
		}
		assertCount(t, `SELECT count(*) FROM password_reset_requests r JOIN users u ON u.id=r.user_id WHERE u.email=$1`, 0, "resetfull@example.invalid")
	})
	t.Run("repeated seeds retain moderated content and legacy city ids", func(t *testing.T) {
		var countryID, cityID int64
		if e := db.QueryRowContext(ctx, "SELECT id FROM countries WHERE code='RU'").Scan(&countryID); e != nil {
			t.Fatal(e)
		}
		if e := db.QueryRowContext(ctx, `INSERT INTO cities(country_id,name) VALUES($1,'Audit legacy referenced city') RETURNING id`, countryID).Scan(&cityID); e != nil {
			t.Fatal(e)
		}
		if _, e := db.ExecContext(ctx, `INSERT INTO cities(country_id,name,external_id) SELECT $1,'Audit city '||g,'audit-city-'||g FROM generate_series(1,1000) g`, countryID); e != nil {
			t.Fatal(e)
		}
		if e := prepareGeographyDatabase(ctx); e != nil {
			t.Fatal(e)
		}
		assertCount(t, "SELECT count(*) FROM cities WHERE id=$1 AND external_id IS NULL", 1, cityID)
		var testID, versionID int64
		if e := db.QueryRowContext(ctx, `SELECT t.id,v.id FROM tests t JOIN users u ON u.id=t.author_id JOIN test_versions v ON v.test_id=t.id AND v.version=1 WHERE u.is_system ORDER BY t.id LIMIT 1`).Scan(&testID, &versionID); e != nil {
			t.Fatal(e)
		}
		if _, e := db.ExecContext(ctx, "UPDATE tests SET status='archived',difficulty='hard' WHERE id=$1", testID); e != nil {
			t.Fatal(e)
		}
		if _, e := db.ExecContext(ctx, "UPDATE test_versions SET title='Audit retained title',description='Audit retained description' WHERE id=$1", versionID); e != nil {
			t.Fatal(e)
		}
		if e := prepareMarketplaceDatabase(ctx); e != nil {
			t.Fatal(e)
		}
		assertCount(t, "SELECT count(*) FROM tests WHERE id=$1 AND status='archived' AND difficulty='hard'", 1, testID)
		assertCount(t, "SELECT count(*) FROM test_versions WHERE id=$1 AND title='Audit retained title' AND description='Audit retained description'", 1, versionID)
	})
}
