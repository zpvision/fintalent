package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"FinTalent/internal/accountingcompany"
	"FinTalent/internal/testmodule/domain"
	"FinTalent/internal/testmodule/dto"
)

func testReleaseEmployeeRaces(t *testing.T, ctx context.Context, owner int64, test *domain.Test, version int64) {
	// Deliberately collide employee id and user id to check their namespaces.
	if _, err := db.ExecContext(ctx, `INSERT INTO company_test_employees(id,owner_user_id,full_name,email) VALUES($1,$1,'Race employee','race-employee@example.invalid')`, owner); err != nil {
		t.Fatal(err)
	}
	call := func(token, action string, body any) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest("POST", "/api/employee-test/"+token+"/"+action, strings.NewReader(string(raw))).WithContext(ctx)
		w := httptest.NewRecorder()
		publicEmployeeTest(w, r)
		return w
	}
	for i := 0; i < 3; i++ {
		token := fmt.Sprintf("%064d", 901+i)
		var invitation int64
		if err := db.QueryRowContext(ctx, `INSERT INTO company_test_invitations(owner_user_id,employee_id,test_id,test_version_id,token) VALUES($1,$1,$2,$3,$4) RETURNING id`, owner, test.ID, version, token).Scan(&invitation); err != nil {
			t.Fatal(err)
		}
		if w := call(token, "start", nil); w.Code != 200 {
			t.Fatalf("start: %d %s", w.Code, w.Body.String())
		}
		var target domain.Question
		for _, q := range test.Questions {
			good := dto.SubmitAnswer{QuestionID: q.ID}
			for _, a := range q.Answers {
				if a.IsCorrect {
					if q.Type == domain.QuestionText {
						good.TextAnswer = a.Answer
					} else {
						good.SelectedAnswerIDs = append(good.SelectedAnswerIDs, a.ID)
					}
				}
			}
			if w := call(token, "answer", good); w.Code != 200 {
				t.Fatalf("employee %s: %d %s", q.Type, w.Code, w.Body.String())
			}
			if q.Type == domain.QuestionSingle {
				target = q
			}
		}
		var wrong int64
		for _, a := range target.Answers {
			if !a.IsCorrect {
				wrong = a.ID
				break
			}
		}
		if wrong == 0 {
			t.Fatal("missing wrong-answer fixture")
		}
		var wg sync.WaitGroup
		start := make(chan struct{})
		codes := make([]int, 3)
		for worker := 0; worker < 3; worker++ {
			wg.Add(1)
			go func(worker int) {
				defer wg.Done()
				<-start
				if worker == 0 {
					codes[worker] = call(token, "answer", dto.SubmitAnswer{QuestionID: target.ID, SelectedAnswerIDs: []int64{wrong}}).Code
				} else {
					codes[worker] = call(token, "finish", nil).Code
				}
			}(worker)
		}
		close(start)
		wg.Wait()
		if (codes[1] == 200) == (codes[2] == 200) {
			t.Fatalf("finish must commit once: %v", codes)
		}
		for _, code := range codes {
			if code >= 500 {
				t.Fatalf("employee race server error: %v", codes)
			}
		}
		var selected int64
		var percent float64
		var status string
		if err := db.QueryRowContext(ctx, `SELECT aa.selected_answer_id,a.percent,i.status FROM company_test_invitations i JOIN test_attempts a ON a.id=i.attempt_id JOIN test_attempt_answers aa ON aa.attempt_id=a.id WHERE i.id=$1 AND aa.question_id=$2`, invitation, target.ID).Scan(&selected, &percent, &status); err != nil {
			t.Fatal(err)
		}
		if status != "finished" || (selected == wrong) != (percent == 75) || (selected != wrong && percent != 100) {
			t.Fatalf("inconsistent grade: selected=%d percent=%v status=%s", selected, percent, status)
		}
		if (codes[0] == 200) != (selected == wrong) {
			t.Fatalf("answer response disagrees with commit: %v", codes)
		}
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO test_attempts(test_id,test_version_id,user_id,status,finished_at,percent) VALUES($1,$2,$3,'finished',NOW(),100)`, test.ID, version, owner); err != nil {
		t.Fatal(err)
	}
	var company int64
	if err := db.QueryRowContext(ctx, `INSERT INTO accounting_companies(owner_user_id,name,slug,status) VALUES($1,'Race company','race-passport','published') RETURNING id`, owner).Scan(&company); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	accountingcompany.New(db, func(*http.Request) (accountingcompany.User, error) { return accountingcompany.User{}, nil }, func(*http.Request) bool { return false }).Register(mux)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", fmt.Sprintf("/api/accounting-companies/%d/passport", company), nil).WithContext(ctx))
	if w.Code != 200 {
		t.Fatalf("passport: %d %s", w.Code, w.Body.String())
	}
	var passport struct {
		Tests       int               `json:"tests_count"`
		Specialists int               `json:"specialists_count"`
		History     []json.RawMessage `json:"history"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &passport); err != nil {
		t.Fatal(err)
	}
	if passport.Tests != 4 || passport.Specialists != 2 || len(passport.History) != 3 {
		t.Fatalf("duplicate attempts or merged people: %+v", passport)
	}
	// A different competency with a third person must increase the overall
	// specialist count, even though neither individual group has three people.
	var secondVersion, invitation, extraAttempt int64
	if err := db.QueryRowContext(ctx, `INSERT INTO test_versions(test_id,version,title,created_by) VALUES($1,2,'Separate competency',$2) RETURNING id`, test.ID, owner).Scan(&secondVersion); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO company_test_employees(id,owner_user_id,full_name,email) VALUES($1,$2,'Another employee','another-race@example.invalid')`, owner+100000, owner); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO company_test_invitations(owner_user_id,employee_id,test_id,test_version_id,token) VALUES($1,$2,$3,$4,$5) RETURNING id`, owner, owner+100000, test.ID, secondVersion, fmt.Sprintf("%064d", 907)).Scan(&invitation); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO test_attempts(test_id,test_version_id,user_id,status,finished_at,percent,context) VALUES($1,$2,$3,'finished',NOW(),100,jsonb_build_object('employee_invitation_id',$4::bigint)) RETURNING id`, test.ID, secondVersion, owner, invitation).Scan(&extraAttempt); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE company_test_invitations SET attempt_id=$2,status='finished',finished_at=NOW() WHERE id=$1`, invitation, extraAttempt); err != nil {
		t.Fatal(err)
	}
	// An extra reference to the same historical attempt must not inflate either
	// the aggregate or the list of attempts.
	if _, err := db.ExecContext(ctx, `INSERT INTO company_test_invitations(owner_user_id,employee_id,test_id,test_version_id,token,status,finished_at,attempt_id) VALUES($1,$2,$3,$4,$5,'finished',NOW(),$6)`, owner, owner+100000, test.ID, secondVersion, fmt.Sprintf("%064d", 908), extraAttempt); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", fmt.Sprintf("/api/accounting-companies/%d/passport", company), nil).WithContext(ctx))
	if w.Code != 200 {
		t.Fatalf("multi-competency passport: %d %s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &passport); err != nil {
		t.Fatal(err)
	}
	if passport.Tests != 5 || passport.Specialists != 3 || len(passport.History) != 4 {
		t.Fatalf("cross-competency or duplicate count: %+v", passport)
	}
}
