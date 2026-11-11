package main

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestAcademyCRMPageAndChoicesPostgres(t *testing.T) {
	f := newProgramRevisionFixture(t)
	for i := 0; i < 65; i++ {
		var uid int64
		err := f.DB.QueryRow(`INSERT INTO users(email,password_hash,full_name,courses_access) VALUES($1,'test-only',$2,true) RETURNING id`, fmt.Sprintf("crm-page-%d@example.invalid", i), fmt.Sprintf("CRM Student %02d", i)).Scan(&uid)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = f.DB.Exec(`INSERT INTO academy_enrollments(course_id,user_id,program_version_id,request_key,enrolled_by) VALUES($1,$2,$3,$4,$5)`, f.Course, uid, f.Version, fmt.Sprintf("crm-page-%d", i), f.Owner); err != nil {
			t.Fatal(err)
		}
	}
	get := func(path string, actor int64, status int) []byte {
		t.Helper()
		r := httptest.NewRequest("GET", fmt.Sprintf("/api/academy/courses/%d/%s", f.Course, path), nil)
		r.Header.Set("X-Test-User", strconv.FormatInt(actor, 10))
		w := httptest.NewRecorder()
		f.Mux.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
		return w.Body.Bytes()
	}
	var first, second struct {
		Items []struct {
			ID   int64  `json:"enrollment_id"`
			Name string `json:"user_name"`
		}
		Total int
	}
	if err := json.Unmarshal(get("crm?page=1&sort=name", f.Owner, 200), &first); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(get("crm?page=2&sort=name", f.Owner, 200), &second); err != nil {
		t.Fatal(err)
	}
	if first.Total != 65 || len(first.Items) != 50 || len(second.Items) != 15 {
		t.Fatal("pagination", first.Total, len(first.Items), len(second.Items))
	}
	seen := map[int64]bool{}
	for _, row := range first.Items {
		seen[row.ID] = true
	}
	for _, row := range second.Items {
		if seen[row.ID] {
			t.Fatal("duplicate page row")
		}
	}
	var filtered struct {
		Items []json.RawMessage
		Total int
	}
	json.Unmarshal(get("crm?page=1&search=Student%2003", f.Owner, 200), &filtered)
	if filtered.Total != 1 {
		t.Fatal("search total", filtered.Total)
	}
	var choices []map[string]any
	json.Unmarshal(get("crm?compact=1", f.Owner, 200), &choices)
	if len(choices) != 65 || choices[0]["percent"] != nil {
		t.Fatal("recipient choices contain metrics or missing users")
	}
	var legacy []json.RawMessage
	if err := json.Unmarshal(get("crm", f.Owner, 200), &legacy); err != nil || len(legacy) != 65 {
		t.Fatal("legacy contract changed", err)
	}
	get("crm?page=0", f.Owner, 400)
	get("crm?page=1&sort=invalid", f.Owner, 400)
	get("crm?page=1", f.Student, 404)
	get("crm?compact=1", f.Student, 404)
	get("cohorts?compact=1", f.Owner, 200)
}
