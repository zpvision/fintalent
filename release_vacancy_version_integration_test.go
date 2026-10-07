package main

import (
	"context"
	"net/http/httptest"
	"testing"

	"FinTalent/internal/testmodule/dto"
	"FinTalent/internal/testmodule/repository"
	"FinTalent/internal/testmodule/service"
	vacancyrepo "FinTalent/internal/vacancymodule/repository"
)

func testReleaseVacancyVersion(t *testing.T, ctx context.Context) {
	insert := func(q string, args ...any) int64 {
		t.Helper()
		var id int64
		if err := db.QueryRowContext(ctx, q, args...).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.ExecContext(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	users := []int64{}
	for _, email := range []string{"version-author@example.invalid", "version-employer@example.invalid", "version-candidate@example.invalid"} {
		users = append(users, insert(`INSERT INTO users(full_name,email,password_hash) VALUES('Version fixture',$1,repeat('*',60)) RETURNING id`, email))
	}
	author, employer, candidate := users[0], users[1], users[2]
	svc := service.New(repository.New(db))
	vr := vacancyrepo.New(db)
	test, err := svc.Create(ctx, author, dto.CreateTest{Title: "Pinned version one", Visibility: "marketplace", IsFree: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddQuestion(ctx, test.ID, author, dto.CreateQuestion{Question: "Original question", QuestionType: "text", Points: 1, Answers: []dto.AnswerInput{{Answer: "Original reference", IsCorrect: true}}}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Publish(ctx, test.ID, author); err != nil {
		t.Fatal(err)
	}
	listing := insert(`INSERT INTO vacancies(user_id,title,status) VALUES($1,'Version vacancy','published') RETURNING id`, employer)
	v, err := vr.Get(ctx, listing, employer)
	if err != nil {
		t.Fatal(err)
	}
	v.SelectedTestIDs = []int64{test.ID}
	if err := vr.Save(ctx, v, nil, ""); err != nil {
		t.Fatal(err)
	}
	var pinned int64
	if err := db.QueryRowContext(ctx, `SELECT test_version_id FROM vacancy_tests WHERE vacancy_external_id=$1 AND test_id=$2`, listing, test.ID).Scan(&pinned); err != nil {
		t.Fatal(err)
	}
	if err := svc.ForkDraft(ctx, test.ID, author); err != nil {
		t.Fatal(err)
	}
	exec(`UPDATE test_versions SET title='Current version two' WHERE test_id=$1 AND id<>$2`, test.ID, pinned)
	if err := svc.Publish(ctx, test.ID, author); err != nil {
		t.Fatal(err)
	}
	if err := vr.Save(ctx, v, nil, ""); err != nil {
		t.Fatal(err)
	} // unrelated edit must preserve the assigned version
	view := &publicVacancyView{ID: listing}
	if err := loadPublicVacancyTests(httptest.NewRequest("GET", "/", nil).WithContext(ctx), view); err != nil {
		t.Fatal(err)
	}
	if len(view.Tests) != 1 || view.Tests[0].Title != "Pinned version one" {
		t.Fatalf("public version: %+v", view.Tests)
	}
	attempt, err := svc.Start(ctx, test.ID, candidate, listing)
	if err != nil {
		t.Fatal(err)
	}
	if attempt.TestVersionID != pinned || attempt.TestTitle != "Pinned version one" {
		t.Fatalf("assigned version: %+v", attempt)
	}
	ordinary, err := svc.Start(ctx, test.ID, candidate, 0)
	if err != nil {
		t.Fatal(err)
	}
	if ordinary.TestVersionID == pinned || ordinary.TestTitle != "Current version two" {
		t.Fatal("ordinary attempt did not use current version")
	}
	newListing := insert(`INSERT INTO vacancies(user_id,title,status) VALUES($1,'Unchanged title','draft') RETURNING id`, employer)
	fresh, err := vr.Get(ctx, newListing, employer)
	if err != nil {
		t.Fatal(err)
	}
	fresh.SelectedTestIDs = []int64{test.ID}
	fresh.Title = "Must roll back"
	for _, state := range []string{"blocked author", "deleted test", "private test"} {
		t.Run(state, func(t *testing.T) {
			exec(`UPDATE users SET is_blocked=$2 WHERE id=$1`, author, state == "blocked author")
			exec(`UPDATE tests SET deleted_at=CASE WHEN $2 THEN NOW() ELSE NULL END,visibility=CASE WHEN $3 THEN 'private' ELSE 'marketplace' END WHERE id=$1`, test.ID, state == "deleted test", state == "private test")
			if err := vr.Save(ctx, fresh, nil, ""); err == nil {
				t.Fatal("unavailable test attached")
			}
			assertCount(t, `SELECT count(*) FROM vacancies WHERE id=$1 AND title='Unchanged title'`, 1, newListing)
			assertCount(t, `SELECT count(*) FROM vacancy_tests WHERE vacancy_external_id=$1`, 0, newListing)
			if _, err := svc.Start(ctx, test.ID, candidate, listing); err == nil {
				t.Fatal("unavailable attached test started")
			}
			view := &publicVacancyView{ID: listing}
			if err := loadPublicVacancyTests(httptest.NewRequest("GET", "/", nil).WithContext(ctx), view); err != nil || len(view.Tests) != 0 {
				t.Fatalf("unavailable test disclosed: %v", err)
			}
		})
	}
	exec(`UPDATE tests SET deleted_at=NULL,visibility='marketplace' WHERE id=$1`, test.ID)
	exec(`UPDATE users SET is_blocked=true,is_system=true WHERE id=$1`, author)
	if err := vr.Save(ctx, fresh, nil, ""); err != nil {
		t.Fatalf("system test rejected: %v", err)
	}
	if _, err := svc.Start(ctx, test.ID, candidate, listing); err != nil {
		t.Fatalf("system test start: %v", err)
	}
	exec(`UPDATE users SET is_blocked=true WHERE id=$1`, employer)
	if _, err := svc.Start(ctx, test.ID, candidate, listing); err == nil {
		t.Fatal("blocked vacancy owner start")
	}
	exec(`UPDATE users SET is_blocked=false WHERE id=$1`, employer)
	exec(`UPDATE vacancies SET status='archived' WHERE id=$1`, listing)
	if _, err := svc.Start(ctx, test.ID, candidate, listing); err == nil {
		t.Fatal("archived vacancy start")
	}
}
