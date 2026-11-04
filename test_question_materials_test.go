package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"FinTalent/internal/testmodule/dto"
	"FinTalent/internal/testmodule/handler"
	"FinTalent/internal/testmodule/repository"
	"FinTalent/internal/testmodule/service"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

func materialPNG() []byte {
	var out bytes.Buffer
	_ = png.Encode(&out, image.NewRGBA(image.Rect(0, 0, 10, 10)))
	return out.Bytes()
}
func TestQuestionMaterialTypes(t *testing.T) {
	if questionMaterialType(materialPNG()) != "image/png" {
		t.Fatal("PNG rejected")
	}
	if questionMaterialType([]byte("%PDF-1.7\nfixture")) != "application/pdf" {
		t.Fatal("PDF rejected")
	}
	for _, raw := range [][]byte{nil, []byte("<svg onload=alert(1)></svg>"), []byte("<html>file.pdf</html>"), []byte("\x89PNG\r\n\x1a\ninvalid")} {
		if questionMaterialType(raw) != "" {
			t.Fatal("unsafe material accepted")
		}
	}
}

// Opt-in; all fixtures live in a newly created schema on the configured cloud PostgreSQL.
func TestQuestionMaterialsIsolatedPostgres(t *testing.T) {
	if os.Getenv("RUN_TEST_MATERIALS_DB_TESTS") != "1" {
		t.Skip("set RUN_TEST_MATERIALS_DB_TESTS=1 with cloud DATABASE_URL")
	}
	loadLocalEnv(".env")
	config, err := pgx.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil || config.Host == "localhost" || config.Host == "127.0.0.1" || os.Getenv("DATABASE_URL") == "" {
		t.Fatal("cloud DATABASE_URL required")
	}
	admin := stdlib.OpenDB(*config)
	defer admin.Close()
	schema := fmt.Sprintf("test_materials_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = admin.Exec("CREATE SCHEMA " + quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if !strings.HasPrefix(schema, "test_materials_") {
			panic("unsafe schema")
		}
		if _, err := admin.Exec("DROP SCHEMA " + quoted + " CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	config.RuntimeParams["search_path"] = schema
	isolated := stdlib.OpenDB(*config)
	defer isolated.Close()
	previous := db
	db = isolated
	defer func() { db = previous }()
	var actual string
	if err = db.QueryRow("SELECT current_schema()").Scan(&actual); err != nil || actual != schema {
		t.Fatal("schema isolation failed")
	}
	t.Setenv("APP_ENV", "production")
	t.Setenv("SEED_DEMO_DATA", "false")
	t.Setenv("SYNC_GEOGRAPHY", "false")
	if err = prepareDatabase(); err != nil {
		t.Fatal(err)
	}
	if err = prepareTestingDatabase(context.Background()); err != nil {
		t.Fatal("migration replay", err)
	}
	users := make([]int64, 3)
	cookies := map[int64]*http.Cookie{}
	for i := range users {
		if err = db.QueryRow(`INSERT INTO users(full_name,email,password_hash) VALUES($1,$2,repeat('*',60)) RETURNING id`, fmt.Sprintf("Materials user %d", i), fmt.Sprintf("materials-%d@example.invalid", i)).Scan(&users[i]); err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		if err = createSession(w, users[i]); err != nil {
			t.Fatal(err)
		}
		cookies[users[i]] = w.Result().Cookies()[0]
	}
	owner, student, other := users[0], users[1], users[2]
	ctx := context.Background()
	repo := repository.New(db)
	test, err := repo.Create(ctx, owner, dto.CreateTest{Title: "Материалы вопроса", Category: "Финансы", Difficulty: "easy", Visibility: "private", IsFree: true, PassingPercent: 60}, "materials-fixture")
	if err != nil {
		t.Fatal(err)
	}
	second, err := repo.Create(ctx, other, dto.CreateTest{Title: "Чужой тест", Category: "Финансы", Difficulty: "easy", Visibility: "private", IsFree: true, PassingPercent: 60}, "other-materials")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	handler.New(service.New(repo), func(r *http.Request) (int64, error) {
		u, e := userFromRequest(r)
		if e != nil {
			return 0, e
		}
		return u.ID, nil
	}, isAdmin).Register(mux)
	mux.HandleFunc("/api/test-materials", uploadQuestionMaterial)
	mux.HandleFunc("/api/test-materials/", serveQuestionMaterial)
	upload := func(uid, testID int64, name string, content []byte, status int) repository.Material {
		t.Helper()
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		_ = mw.WriteField("test_id", fmt.Sprint(testID))
		f, _ := mw.CreateFormFile("file", name)
		_, _ = f.Write(content)
		_ = mw.Close()
		r := httptest.NewRequest("POST", "/api/test-materials", &body)
		r.Header.Set("Content-Type", mw.FormDataContentType())
		if uid > 0 {
			r.AddCookie(cookies[uid])
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("upload got %d want %d: %s", w.Code, status, w.Body.String())
		}
		var asset repository.Material
		_ = json.Unmarshal(w.Body.Bytes(), &asset)
		return asset
	}
	upload(0, test.ID, "image.png", materialPNG(), 401)
	upload(other, test.ID, "image.png", materialPNG(), 403)
	upload(owner, test.ID, "image.svg", []byte("<svg/>"), 400)
	upload(owner, test.ID, "fake.png", []byte("<html>"), 400)
	upload(owner, test.ID, "fake.html", []byte("%PDF-1.7\n"), 400)
	upload(owner, test.ID, "large.pdf", append([]byte("%PDF-1.7\n"), make([]byte, questionMaterialLimit)...), 400)
	imageAsset := upload(owner, test.ID, "диаграмма.png", materialPNG(), 201)
	pdfAsset := upload(owner, test.ID, "данные.pdf", []byte("%PDF-1.7\nfixture\n%%EOF"), 201)
	question := dto.CreateQuestion{Question: "Какой вывод верен?", QuestionType: "single_choice", Points: 1, Settings: map[string]any{"materials": []repository.Material{{ID: imageAsset.ID, Name: "forged"}, {ID: pdfAsset.ID}}}, Answers: []dto.AnswerInput{{Answer: "Да", IsCorrect: true}, {Answer: "Нет"}}}
	qid, err := repo.AddQuestion(ctx, test.ID, owner, question)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.AddQuestion(ctx, second.ID, other, question); err == nil {
		t.Fatal("foreign asset attached")
	}
	get := func(uid, q, asset int64, header, token string, status int) {
		t.Helper()
		r := httptest.NewRequest("GET", fmt.Sprintf("/api/test-materials/%d?question=%d", asset, q), nil)
		if uid > 0 {
			r.AddCookie(cookies[uid])
		}
		if header != "" {
			r.Header.Set(header, token)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("file user=%d question=%d got %d want %d: %s", uid, q, w.Code, status, w.Body.String())
		}
		if status == 200 && (w.Header().Get("Cache-Control") != "private, no-store" || w.Header().Get("X-Content-Type-Options") != "nosniff") {
			t.Fatal("private response headers missing")
		}
	}
	get(owner, 0, imageAsset.ID, "", "", 200)
	get(0, qid, imageAsset.ID, "", "", 404)
	get(student, qid, imageAsset.ID, "", "", 404)
	current, err := repo.Get(ctx, test.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(current.Questions[0].Settings["materials"])
	if !bytes.Contains(raw, []byte("диаграмма.png")) || bytes.Contains(raw, []byte("forged")) {
		t.Fatal("metadata was not canonicalized")
	}
	// Older editors omit materials; retain them. Explicit [] detaches only the current version.
	question.Settings = map[string]any{"shuffle_answers": false}
	if err = repo.UpdateQuestion(ctx, qid, owner, question); err != nil {
		t.Fatal(err)
	}
	current, _ = repo.Get(ctx, test.ID, true)
	if current.Questions[0].Settings["materials"] == nil {
		t.Fatal("legacy update lost materials")
	}
	versionID := current.Questions[0].TestVersionID
	var employee, attempt, invitation int64
	token := strings.Repeat("a", 64)
	resultToken := strings.Repeat("b", 64)
	hash := sha256.Sum256([]byte(resultToken))
	resultHash := hex.EncodeToString(hash[:])
	if err = db.QueryRow(`INSERT INTO company_test_employees(owner_user_id,full_name,email) VALUES($1,'Employee','materials-1@example.invalid') RETURNING id`, owner).Scan(&employee); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`INSERT INTO test_attempts(test_id,test_version_id,user_id,max_score) VALUES($1,$2,$3,1) RETURNING id`, test.ID, versionID, owner).Scan(&attempt); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`INSERT INTO company_test_invitations(owner_user_id,employee_id,test_id,test_version_id,token,attempt_id,status) VALUES($1,$2,$3,$4,$5,$6,'started') RETURNING id`, owner, employee, test.ID, versionID, token, attempt).Scan(&invitation); err != nil {
		t.Fatal(err)
	}
	get(0, qid, imageAsset.ID, "X-Test-Invitation", token, 200)
	get(0, qid+999, imageAsset.ID, "X-Test-Invitation", token, 404)
	get(0, qid, pdfAsset.ID, "X-Test-Result", resultToken, 404)
	if _, err = db.Exec(`UPDATE test_attempts SET status='finished',finished_at=NOW() WHERE id=$1`, attempt); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE company_test_invitations SET status='finished',result_token_hash=$2,result_token_expires_at=NOW()+INTERVAL '1 hour' WHERE id=$1`, invitation, resultHash); err != nil {
		t.Fatal(err)
	}
	get(0, qid, pdfAsset.ID, "X-Test-Result", resultToken, 200)
	reportResult, err := repo.GetAttempt(ctx, attempt)
	if err != nil {
		t.Fatal(err)
	}
	reportMaterials, err := loadEmployeePDFMaterials(ctx, reportResult)
	if err != nil {
		t.Fatal(err)
	}
	if len(reportMaterials[qid]) != 2 || reportMaterials[qid][0].Image == "" || reportMaterials[qid][1].Image != "" || reportMaterials[qid][1].Name != pdfAsset.Name {
		t.Fatal("report must embed images and list PDF names only")
	}
	get(student, qid, pdfAsset.ID, "", "", 200)
	if _, err = db.Exec(`UPDATE company_test_invitations SET result_token_expires_at=NOW()-INTERVAL '1 hour' WHERE id=$1`, invitation); err != nil {
		t.Fatal(err)
	}
	get(0, qid, pdfAsset.ID, "X-Test-Result", resultToken, 404)
	if _, err = db.Exec(`UPDATE company_test_invitations SET status='revoked' WHERE id=$1`, invitation); err != nil {
		t.Fatal(err)
	}
	get(0, qid, imageAsset.ID, "X-Test-Invitation", token, 404)
	// Existing attempt freezes version 1; an edited fork cannot alter its material references.
	if err = repo.ForkDraft(ctx, test.ID, owner); err != nil {
		t.Fatal(err)
	}
	fork, err := repo.Get(ctx, test.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	newQ := fork.Questions[0].ID
	question.Settings = map[string]any{"materials": []repository.Material{}}
	if err = repo.UpdateQuestion(ctx, newQ, owner, question); err != nil {
		t.Fatal(err)
	}
	old, err := repo.GetVersion(ctx, test.ID, versionID, true)
	if err != nil || len(old.Questions) == 0 || old.Questions[0].Settings["materials"] == nil {
		t.Fatal("historical materials lost", err)
	}
	get(0, qid, pdfAsset.ID, "X-Test-Invitation", token, 404)
	get(student, newQ, pdfAsset.ID, "", "", 404)
	get(owner, qid, pdfAsset.ID, "", "", 200)
	if _, err = db.Exec(`INSERT INTO test_attempts(test_id,test_version_id,user_id,max_score) VALUES($1,$2,$3,1)`, test.ID, versionID, student); err != nil {
		t.Fatal(err)
	}
	get(student, qid, imageAsset.ID, "", "", 200)
	t.Log("upload, MIME/size checks, ownership, invitation/result expiry, recipient access and immutable fork passed")
}
