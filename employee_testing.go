package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const employeeTestingMigration = `
CREATE TABLE IF NOT EXISTS company_test_employees (
 id BIGSERIAL PRIMARY KEY, owner_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 full_name VARCHAR(240) NOT NULL, email VARCHAR(254) NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 UNIQUE(owner_user_id,email)
);
CREATE TABLE IF NOT EXISTS company_test_invitations (
 id BIGSERIAL PRIMARY KEY, owner_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 employee_id BIGINT NOT NULL REFERENCES company_test_employees(id) ON DELETE CASCADE,
 test_id BIGINT NOT NULL REFERENCES tests(id), test_version_id BIGINT NOT NULL REFERENCES test_versions(id),
 token VARCHAR(96) NOT NULL UNIQUE, attempt_id BIGINT REFERENCES test_attempts(id) ON DELETE SET NULL,
 status VARCHAR(20) NOT NULL DEFAULT 'sent' CHECK(status IN('sent','started','finished','revoked')),
 sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), started_at TIMESTAMPTZ, finished_at TIMESTAMPTZ,
 UNIQUE(employee_id,test_id,token)
);
CREATE INDEX IF NOT EXISTS company_test_invitation_owner_idx ON company_test_invitations(owner_user_id,sent_at DESC);
`

type employeeInput struct {
	FullName string `json:"full_name"`
	Email    string `json:"email"`
}
type employeeBulkInput struct {
	Employees []employeeInput `json:"employees"`
}
type finKoperImportInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
type finKoperSignInResponse struct {
	Data struct {
		AccessToken string `json:"accesstoken"`
	} `json:"data"`
	ErrorMessage string `json:"errormessage"`
}
type finKoperCompaniesResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
	ErrorMessage string `json:"errormessage"`
}
type finKoperCompanyUsersResponse struct {
	Data struct {
		Users []struct {
			Email     string `json:"email"`
			FirstName string `json:"firstname"`
			LastName  string `json:"lastname"`
			DeleteAt  int64  `json:"deleteat"`
		} `json:"users"`
	} `json:"data"`
	ErrorMessage string `json:"errormessage"`
}
type invitationInput struct {
	TestID      int64   `json:"test_id"`
	EmployeeIDs []int64 `json:"employee_ids"`
}
type employeeAnswerInput struct {
	QuestionID        int64   `json:"question_id"`
	SelectedAnswerIDs []int64 `json:"selected_answer_ids"`
	TextAnswer        string  `json:"text_answer"`
}

func registerEmployeeTestingRoutes() {
	if _, err := db.Exec(employeeTestingMigration); err != nil {
		panic(err)
	}
	http.HandleFunc("/employee-test", serveFrontendPage("static/employee-test.html"))
	http.HandleFunc("/api/employee-testing/employees", employeeTestingEmployees)
	http.HandleFunc("/api/employee-testing/employees/", employeeTestingEmployee)
	http.HandleFunc("/api/employee-testing/import/finkoper", employeeTestingImportFinKoper)
	http.HandleFunc("/api/employee-testing/tests", employeeTestingTests)
	http.HandleFunc("/api/employee-testing/invitations", employeeTestingInvitations)
	http.HandleFunc("/api/employee-testing/results", employeeTestingResults)
	http.HandleFunc("/api/employee-test/", publicEmployeeTest)
}

func jsonResponse(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func jsonError(w http.ResponseWriter, status int, message string) {
	jsonResponse(w, status, map[string]string{"error": message})
}
func decodeJSONBody(w http.ResponseWriter, r *http.Request, value any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	if json.NewDecoder(r.Body).Decode(value) != nil {
		jsonError(w, 400, "Некорректные данные")
		return false
	}
	return true
}

const (
	finKoperSignInURL    = "https://api.finkoper.com/api/v1/users/signin"
	finKoperCompaniesURL = "https://api.finkoper.com/api/v1/companys"
	finKoperUsersURL     = "https://api.finkoper.com/api/v2/users/company/"
)

var finKoperHTTPClient = &http.Client{Timeout: 20 * time.Second}

type finKoperHTTPError struct {
	status int
}

func (e finKoperHTTPError) Error() string { return "FinKoper вернул ошибку" }

func requestFinKoper(ctx context.Context, method, endpoint, token string, body any, response any) error {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := finKoperHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
		return finKoperHTTPError{status: res.StatusCode}
	}
	decoder := json.NewDecoder(io.LimitReader(res.Body, 5<<20))
	if err = decoder.Decode(response); err != nil {
		return errors.New("некорректный ответ FinKoper")
	}
	return nil
}

func employeeTestingImportFinKoper(w http.ResponseWriter, r *http.Request) {
	u, err := userFromRequest(r)
	if err != nil {
		jsonError(w, http.StatusUnauthorized, "Требуется авторизация")
		return
	}
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
		return
	}
	var in finKoperImportInput
	if !decodeJSONBody(w, r, &in) {
		return
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if _, parseErr := mail.ParseAddress(in.Email); parseErr != nil || in.Password == "" || len(in.Password) > 1024 {
		jsonError(w, http.StatusBadRequest, "Введите корректные логин и пароль FinKoper")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	var signIn finKoperSignInResponse
	err = requestFinKoper(ctx, http.MethodPost, finKoperSignInURL, "", map[string]string{"email": in.Email, "password": in.Password}, &signIn)
	in.Password = ""
	if err != nil || signIn.Data.AccessToken == "" {
		var upstreamErr finKoperHTTPError
		if errors.As(err, &upstreamErr) && (upstreamErr.status == http.StatusUnauthorized || upstreamErr.status == http.StatusForbidden) {
			jsonError(w, http.StatusUnauthorized, "FinKoper не принял логин или пароль")
		} else {
			jsonError(w, http.StatusBadGateway, "Не удалось авторизоваться в FinKoper")
		}
		return
	}
	accessToken := signIn.Data.AccessToken
	defer func() { accessToken = "" }()

	var companies finKoperCompaniesResponse
	if err = requestFinKoper(ctx, http.MethodGet, finKoperCompaniesURL, accessToken, nil, &companies); err != nil {
		jsonError(w, http.StatusBadGateway, "Не удалось получить компании из FinKoper")
		return
	}
	if len(companies.Data) == 0 {
		jsonError(w, http.StatusNotFound, "В FinKoper не найдено доступных компаний")
		return
	}

	employeesByEmail := make(map[string]employeeInput)
	for _, company := range companies.Data {
		companyID := strings.TrimSpace(company.ID)
		if companyID == "" {
			continue
		}
		var companyUsers finKoperCompanyUsersResponse
		endpoint := finKoperUsersURL + url.PathEscape(companyID)
		if err = requestFinKoper(ctx, http.MethodGet, endpoint, accessToken, nil, &companyUsers); err != nil {
			jsonError(w, http.StatusBadGateway, "Не удалось получить сотрудников компании из FinKoper")
			return
		}
		for _, externalUser := range companyUsers.Data.Users {
			email := strings.ToLower(strings.TrimSpace(externalUser.Email))
			name := strings.TrimSpace(strings.TrimSpace(externalUser.FirstName) + " " + strings.TrimSpace(externalUser.LastName))
			if externalUser.DeleteAt != 0 || name == "" {
				continue
			}
			if _, parseErr := mail.ParseAddress(email); parseErr != nil {
				continue
			}
			employeesByEmail[email] = employeeInput{FullName: name, Email: email}
		}
	}
	if len(employeesByEmail) == 0 {
		jsonError(w, http.StatusNotFound, "В компаниях FinKoper не найдено сотрудников с e-mail")
		return
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Не удалось сохранить сотрудников")
		return
	}
	defer tx.Rollback()
	for _, employee := range employeesByEmail {
		if _, err = tx.ExecContext(ctx, `INSERT INTO company_test_employees(owner_user_id,full_name,email) VALUES($1,$2,$3) ON CONFLICT(owner_user_id,email) DO UPDATE SET full_name=EXCLUDED.full_name`, u.ID, employee.FullName, employee.Email); err != nil {
			jsonError(w, http.StatusInternalServerError, "Не удалось сохранить сотрудников")
			return
		}
	}
	if err = tx.Commit(); err != nil {
		jsonError(w, http.StatusInternalServerError, "Не удалось сохранить сотрудников")
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"imported": len(employeesByEmail), "companies": len(companies.Data)})
}

func employeeTestingEmployees(w http.ResponseWriter, r *http.Request) {
	u, err := userFromRequest(r)
	if err != nil {
		jsonError(w, 401, "Требуется авторизация")
		return
	}
	switch r.Method {
	case http.MethodGet:
		rows, err := db.QueryContext(r.Context(), `SELECT e.id,e.full_name,e.email,e.created_at,COUNT(i.id),COUNT(i.id) FILTER(WHERE i.status='finished') FROM company_test_employees e LEFT JOIN company_test_invitations i ON i.employee_id=e.id WHERE e.owner_user_id=$1 GROUP BY e.id ORDER BY e.full_name,e.id`, u.ID)
		if err != nil {
			jsonError(w, 500, "Не удалось загрузить сотрудников")
			return
		}
		defer rows.Close()
		items := []map[string]any{}
		for rows.Next() {
			var id, total, finished int64
			var name, email string
			var created any
			if rows.Scan(&id, &name, &email, &created, &total, &finished) == nil {
				items = append(items, map[string]any{"id": id, "full_name": name, "email": email, "created_at": created, "assignments": total, "completed": finished})
			}
		}
		jsonResponse(w, 200, map[string]any{"items": items})
	case http.MethodPost:
		var in employeeBulkInput
		if !decodeJSONBody(w, r, &in) {
			return
		}
		if len(in.Employees) == 0 || len(in.Employees) > 1000 {
			jsonError(w, 400, "Добавьте от 1 до 1000 сотрудников")
			return
		}
		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			jsonError(w, 500, "Не удалось сохранить сотрудников")
			return
		}
		defer tx.Rollback()
		added := 0
		for _, item := range in.Employees {
			name := strings.TrimSpace(item.FullName)
			email := strings.ToLower(strings.TrimSpace(item.Email))
			if name == "" {
				continue
			}
			if _, e := mail.ParseAddress(email); e != nil {
				jsonError(w, 400, "Некорректный e-mail: "+email)
				return
			}
			res, e := tx.ExecContext(r.Context(), `INSERT INTO company_test_employees(owner_user_id,full_name,email) VALUES($1,$2,$3) ON CONFLICT(owner_user_id,email) DO UPDATE SET full_name=EXCLUDED.full_name`, u.ID, name, email)
			if e != nil {
				jsonError(w, 500, "Не удалось сохранить сотрудников")
				return
			}
			if n, _ := res.RowsAffected(); n > 0 {
				added++
			}
		}
		if err = tx.Commit(); err != nil {
			jsonError(w, 500, "Не удалось сохранить сотрудников")
			return
		}
		jsonResponse(w, 200, map[string]any{"saved": added})
	default:
		jsonError(w, 405, "Метод не поддерживается")
	}
}

func employeeTestingEmployee(w http.ResponseWriter, r *http.Request) {
	u, err := userFromRequest(r)
	if err != nil {
		jsonError(w, http.StatusUnauthorized, "Требуется авторизация")
		return
	}
	if r.Method != http.MethodDelete {
		jsonError(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
		return
	}
	idText := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/employee-testing/employees/"), "/")
	employeeID, err := strconv.ParseInt(idText, 10, 64)
	if err != nil || employeeID < 1 {
		jsonError(w, http.StatusNotFound, "Сотрудник не найден")
		return
	}
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Не удалось удалить сотрудника")
		return
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(r.Context(), `SELECT attempt_id FROM company_test_invitations WHERE employee_id=$1 AND owner_user_id=$2 AND attempt_id IS NOT NULL FOR UPDATE`, employeeID, u.ID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Не удалось удалить результаты сотрудника")
		return
	}
	attemptIDs := make([]int64, 0)
	for rows.Next() {
		var attemptID int64
		if rows.Scan(&attemptID) == nil {
			attemptIDs = append(attemptIDs, attemptID)
		}
	}
	rows.Close()
	result, err := tx.ExecContext(r.Context(), `DELETE FROM company_test_employees WHERE id=$1 AND owner_user_id=$2`, employeeID, u.ID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Не удалось удалить сотрудника")
		return
	}
	deleted, _ := result.RowsAffected()
	if deleted == 0 {
		jsonError(w, http.StatusNotFound, "Сотрудник не найден")
		return
	}
	for _, attemptID := range attemptIDs {
		if _, err = tx.ExecContext(r.Context(), `DELETE FROM test_attempts WHERE id=$1`, attemptID); err != nil {
			jsonError(w, http.StatusInternalServerError, "Не удалось удалить результаты сотрудника")
			return
		}
	}
	if err = tx.Commit(); err != nil {
		jsonError(w, http.StatusInternalServerError, "Не удалось удалить сотрудника")
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"deleted": true, "deleted_results": len(attemptIDs)})
}

func employeeTestingTests(w http.ResponseWriter, r *http.Request) {
	u, err := userFromRequest(r)
	if err != nil {
		jsonError(w, 401, "Требуется авторизация")
		return
	}
	rows, err := db.QueryContext(r.Context(), `SELECT t.id,v.title,(SELECT COUNT(*) FROM test_questions q WHERE q.test_version_id=v.id) FROM tests t JOIN test_versions v ON v.test_id=t.id AND v.version=t.current_version WHERE t.author_id=$1 AND t.status='published' ORDER BY v.title`, u.ID)
	if err != nil {
		jsonError(w, 500, "Не удалось загрузить тесты")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, count int64
		var title string
		if rows.Scan(&id, &title, &count) == nil {
			items = append(items, map[string]any{"id": id, "title": title, "question_count": count})
		}
	}
	jsonResponse(w, 200, map[string]any{"items": items})
}

func invitationToken() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
func employeeTestingInvitations(w http.ResponseWriter, r *http.Request) {
	u, err := userFromRequest(r)
	if err != nil {
		jsonError(w, 401, "Требуется авторизация")
		return
	}
	if r.Method != http.MethodPost {
		jsonError(w, 405, "Метод не поддерживается")
		return
	}
	var in invitationInput
	if !decodeJSONBody(w, r, &in) {
		return
	}
	if in.TestID < 1 || len(in.EmployeeIDs) == 0 {
		jsonError(w, 400, "Выберите тест и сотрудников")
		return
	}
	var versionID int64
	if db.QueryRowContext(r.Context(), `SELECT v.id FROM tests t JOIN test_versions v ON v.test_id=t.id AND v.version=t.current_version WHERE t.id=$1 AND t.author_id=$2 AND t.status='published'`, in.TestID, u.ID).Scan(&versionID) != nil {
		jsonError(w, 400, "Можно назначать только свой опубликованный тест")
		return
	}
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		jsonError(w, 500, "Не удалось создать приглашения")
		return
	}
	defer tx.Rollback()
	links := []map[string]any{}
	for _, employeeID := range in.EmployeeIDs {
		var name, email string
		if tx.QueryRowContext(r.Context(), `SELECT full_name,email FROM company_test_employees WHERE id=$1 AND owner_user_id=$2`, employeeID, u.ID).Scan(&name, &email) != nil {
			continue
		}
		token := invitationToken()
		var id int64
		err = tx.QueryRowContext(r.Context(), `INSERT INTO company_test_invitations(owner_user_id,employee_id,test_id,test_version_id,token) VALUES($1,$2,$3,$4,$5) RETURNING id`, u.ID, employeeID, in.TestID, versionID, token).Scan(&id)
		if err != nil {
			jsonError(w, 500, "Не удалось создать приглашения")
			return
		}
		links = append(links, map[string]any{"id": id, "employee_id": employeeID, "full_name": name, "email": email, "url": "/employee-test?token=" + token})
	}
	if err = tx.Commit(); err != nil {
		jsonError(w, 500, "Не удалось создать приглашения")
		return
	}
	jsonResponse(w, 201, map[string]any{"items": links, "email_sent": false, "message": "Персональные ссылки созданы. Почтовый сервер пока не настроен — скопируйте ссылки сотрудникам."})
}

func employeeTestingResults(w http.ResponseWriter, r *http.Request) {
	u, err := userFromRequest(r)
	if err != nil {
		jsonError(w, 401, "Требуется авторизация")
		return
	}
	rows, err := db.QueryContext(r.Context(), `SELECT i.id,e.full_name,e.email,v.title,i.status,i.sent_at,i.started_at,i.finished_at,COALESCE(a.percent,0),COALESCE(a.score,0),COALESCE(a.max_score,0),COALESCE(a.duration_seconds,0),COALESCE(a.passed,FALSE),i.token,COALESCE(i.attempt_id,0) FROM company_test_invitations i JOIN company_test_employees e ON e.id=i.employee_id JOIN test_versions v ON v.id=i.test_version_id LEFT JOIN test_attempts a ON a.id=i.attempt_id WHERE i.owner_user_id=$1 ORDER BY i.sent_at DESC,i.id DESC`, u.ID)
	if err != nil {
		jsonError(w, 500, "Не удалось загрузить результаты")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, attemptID int64
		var name, email, title, status, token string
		var sent any
		var started, finished sql.NullTime
		var percent, score, max float64
		var duration int
		var passed bool
		if rows.Scan(&id, &name, &email, &title, &status, &sent, &started, &finished, &percent, &score, &max, &duration, &passed, &token, &attemptID) == nil {
			var startedAt, finishedAt any
			if started.Valid {
				startedAt = started.Time
			}
			if finished.Valid {
				finishedAt = finished.Time
			}
			items = append(items, map[string]any{"id": id, "attempt_id": attemptID, "full_name": name, "email": email, "test_title": title, "status": status, "sent_at": sent, "started_at": startedAt, "finished_at": finishedAt, "percent": percent, "score": score, "max_score": max, "duration_seconds": duration, "passed": passed, "url": "/employee-test?token=" + token})
		}
	}
	jsonResponse(w, 200, map[string]any{"items": items})
}

func publicEmployeeTest(w http.ResponseWriter, r *http.Request) {
	tail := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/employee-test/"), "/")
	parts := strings.Split(tail, "/")
	token := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}
	if len(token) < 20 {
		jsonError(w, 404, "Ссылка недействительна")
		return
	}
	switch {
	case action == "start" && r.Method == http.MethodPost:
		employeeTestStart(w, r, token)
	case action == "answer" && r.Method == http.MethodPost:
		employeeTestAnswer(w, r, token)
	case action == "finish" && r.Method == http.MethodPost:
		employeeTestFinish(w, r, token)
	case action == "" && r.Method == http.MethodGet:
		employeeTestInfo(w, r, token)
	default:
		jsonError(w, 405, "Метод не поддерживается")
	}
}

func employeeTestInfo(w http.ResponseWriter, r *http.Request, token string) {
	var id, testID, versionID int64
	var employee, title, status string
	var attempt sql.NullInt64
	var attemptStarted sql.NullTime
	var limit, questionCount int
	if db.QueryRowContext(r.Context(), `SELECT i.id,i.test_id,i.test_version_id,e.full_name,v.title,i.status,i.attempt_id,i.started_at,COALESCE(t.time_limit_seconds,0),(SELECT COUNT(*) FROM test_questions q WHERE q.test_version_id=i.test_version_id) FROM company_test_invitations i JOIN company_test_employees e ON e.id=i.employee_id JOIN test_versions v ON v.id=i.test_version_id JOIN tests t ON t.id=i.test_id WHERE i.token=$1 AND i.status<>'revoked'`, token).Scan(&id, &testID, &versionID, &employee, &title, &status, &attempt, &attemptStarted, &limit, &questionCount) != nil {
		jsonError(w, 404, "Ссылка недействительна")
		return
	}
	questions := []map[string]any{}
	answeredQuestionIDs := []int64{}
	if attempt.Valid {
		rows, _ := db.QueryContext(r.Context(), `SELECT q.id,q.question,q.question_type,q.points FROM test_questions q WHERE q.test_version_id=$1 ORDER BY q.sort_order,q.id`, versionID)
		if rows != nil {
			defer rows.Close()
			for rows.Next() {
				var qid int64
				var question, typ string
				var points float64
				if rows.Scan(&qid, &question, &typ, &points) == nil {
					answers := []map[string]any{}
					arows, _ := db.QueryContext(r.Context(), `SELECT id,answer FROM test_answers WHERE question_id=$1 ORDER BY sort_order,id`, qid)
					if arows != nil {
						for arows.Next() {
							var aid int64
							var answer string
							if arows.Scan(&aid, &answer) == nil {
								answers = append(answers, map[string]any{"id": aid, "answer": answer})
							}
						}
						arows.Close()
					}
					questions = append(questions, map[string]any{"id": qid, "question": question, "question_type": typ, "points": points, "answers": answers})
				}
			}
		}
		answeredRows, answeredErr := db.QueryContext(r.Context(), `SELECT DISTINCT question_id FROM test_attempt_answers WHERE attempt_id=$1 ORDER BY question_id`, attempt.Int64)
		if answeredErr == nil {
			defer answeredRows.Close()
			for answeredRows.Next() {
				var questionID int64
				if answeredRows.Scan(&questionID) == nil {
					answeredQuestionIDs = append(answeredQuestionIDs, questionID)
				}
			}
		}
	}
	var startedAt any
	remaining := limit
	if attemptStarted.Valid {
		startedAt = attemptStarted.Time
		if limit > 0 {
			remaining = max(0, limit-int(time.Since(attemptStarted.Time).Seconds()))
		}
	}
	jsonResponse(w, 200, map[string]any{"employee_name": employee, "test_title": title, "status": status, "attempt_id": attempt.Int64, "attempt_started_at": startedAt, "time_limit_seconds": limit, "remaining_seconds": remaining, "question_count": questionCount, "answered_question_ids": answeredQuestionIDs, "questions": questions})
}

func employeeTestStart(w http.ResponseWriter, r *http.Request, token string) {
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		jsonError(w, 500, "Не удалось начать тест")
		return
	}
	defer tx.Rollback()
	var invitationID, testID, versionID, ownerID int64
	var attempt sql.NullInt64
	var status string
	if tx.QueryRowContext(r.Context(), `SELECT id,test_id,test_version_id,owner_user_id,attempt_id,status FROM company_test_invitations WHERE token=$1 FOR UPDATE`, token).Scan(&invitationID, &testID, &versionID, &ownerID, &attempt, &status) != nil || status == "revoked" {
		jsonError(w, 404, "Ссылка недействительна")
		return
	}
	if status == "finished" {
		jsonError(w, 409, "Тест уже завершён")
		return
	}
	attemptID := attempt.Int64
	if !attempt.Valid {
		err = tx.QueryRowContext(r.Context(), `INSERT INTO test_attempts(test_id,test_version_id,user_id,max_score,context) SELECT $1,$2,$3,COALESCE(SUM(points),0),jsonb_build_object('employee_invitation_id',$4::bigint) FROM test_questions WHERE test_version_id=$2 RETURNING id`, testID, versionID, ownerID, invitationID).Scan(&attemptID)
		if err != nil {
			jsonError(w, 500, "Не удалось начать тест")
			return
		}
		_, err = tx.ExecContext(r.Context(), `UPDATE company_test_invitations SET attempt_id=$1,status='started',started_at=NOW() WHERE id=$2`, attemptID, invitationID)
		if err != nil {
			jsonError(w, 500, "Не удалось начать тест")
			return
		}
	}
	if tx.Commit() != nil {
		jsonError(w, 500, "Не удалось начать тест")
		return
	}
	jsonResponse(w, 200, map[string]any{"attempt_id": attemptID})
}

func employeeTestAnswer(w http.ResponseWriter, r *http.Request, token string) {
	var in employeeAnswerInput
	if !decodeJSONBody(w, r, &in) {
		return
	}
	var attemptID int64
	if db.QueryRowContext(r.Context(), `SELECT i.attempt_id FROM company_test_invitations i JOIN tests t ON t.id=i.test_id WHERE i.token=$1 AND i.status='started' AND (COALESCE(t.time_limit_seconds,0)=0 OR i.started_at+(t.time_limit_seconds*INTERVAL '1 second')>NOW())`, token).Scan(&attemptID) != nil {
		jsonError(w, 403, "Попытка недоступна")
		return
	}
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		jsonError(w, 500, "Не удалось сохранить ответ")
		return
	}
	defer tx.Rollback()
	var responseSeconds int
	if err = tx.QueryRowContext(r.Context(), `SELECT GREATEST(0,EXTRACT(EPOCH FROM NOW()-COALESCE((SELECT MAX(answered_at) FROM test_attempt_answers WHERE attempt_id=$1),(SELECT started_at FROM test_attempts WHERE id=$1)))::int)`, attemptID).Scan(&responseSeconds); err != nil {
		jsonError(w, 500, "Не удалось сохранить время ответа")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `DELETE FROM test_attempt_answers WHERE attempt_id=$1 AND question_id=$2`, attemptID, in.QuestionID); err != nil {
		jsonError(w, 500, "Не удалось сохранить ответ")
		return
	}
	if len(in.SelectedAnswerIDs) == 0 {
		_, err = tx.ExecContext(r.Context(), `INSERT INTO test_attempt_answers(attempt_id,question_id,text_answer,response_seconds) SELECT a.id,q.id,$3,$4 FROM test_attempts a JOIN test_questions q ON q.test_version_id=a.test_version_id WHERE a.id=$1 AND q.id=$2 AND a.status='started'`, attemptID, in.QuestionID, strings.TrimSpace(in.TextAnswer), responseSeconds)
	} else {
		for _, answerID := range in.SelectedAnswerIDs {
			_, err = tx.ExecContext(r.Context(), `INSERT INTO test_attempt_answers(attempt_id,question_id,selected_answer_id,response_seconds) SELECT a.id,q.id,ta.id,$4 FROM test_attempts a JOIN test_questions q ON q.test_version_id=a.test_version_id JOIN test_answers ta ON ta.question_id=q.id WHERE a.id=$1 AND q.id=$2 AND ta.id=$3 AND a.status='started'`, attemptID, in.QuestionID, answerID, responseSeconds)
			if err != nil {
				break
			}
		}
	}
	if err != nil || tx.Commit() != nil {
		jsonError(w, 500, "Не удалось сохранить ответ")
		return
	}
	jsonResponse(w, 200, map[string]bool{"saved": true})
}

func employeeTestFinish(w http.ResponseWriter, r *http.Request, token string) {
	var invitationID, attemptID, versionID int64
	var passing float64
	if db.QueryRowContext(r.Context(), `SELECT i.id,i.attempt_id,i.test_version_id,t.passing_percent FROM company_test_invitations i JOIN tests t ON t.id=i.test_id WHERE i.token=$1 AND i.status='started'`, token).Scan(&invitationID, &attemptID, &versionID, &passing) != nil {
		jsonError(w, 403, "Попытка недоступна")
		return
	}
	rows, err := db.QueryContext(r.Context(), `SELECT id,question_type,points FROM test_questions WHERE test_version_id=$1`, versionID)
	if err != nil {
		jsonError(w, 500, "Не удалось завершить тест")
		return
	}
	defer rows.Close()
	type grade struct {
		id      int64
		correct bool
		points  float64
	}
	grades := []grade{}
	score, max := 0.0, 0.0
	for rows.Next() {
		var qid int64
		var typ string
		var points float64
		if rows.Scan(&qid, &typ, &points) != nil {
			continue
		}
		max += points
		correct := false
		if typ == "text" {
			var expected, actual string
			_ = db.QueryRowContext(r.Context(), `SELECT COALESCE((SELECT lower(trim(answer)) FROM test_answers WHERE question_id=$1 AND is_correct LIMIT 1),'')`, qid).Scan(&expected)
			_ = db.QueryRowContext(r.Context(), `SELECT COALESCE((SELECT lower(trim(text_answer)) FROM test_attempt_answers WHERE attempt_id=$1 AND question_id=$2 LIMIT 1),'')`, attemptID, qid).Scan(&actual)
			correct = expected != "" && expected == actual
		} else {
			var expected, selected int
			var wrong bool
			_ = db.QueryRowContext(r.Context(), `SELECT COUNT(*) FILTER(WHERE is_correct),COUNT(*) FILTER(WHERE NOT is_correct AND id IN(SELECT selected_answer_id FROM test_attempt_answers WHERE attempt_id=$1 AND question_id=$2))>0 FROM test_answers WHERE question_id=$2`, attemptID, qid).Scan(&expected, &wrong)
			_ = db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM test_attempt_answers aa JOIN test_answers a ON a.id=aa.selected_answer_id WHERE aa.attempt_id=$1 AND aa.question_id=$2 AND a.is_correct`, attemptID, qid).Scan(&selected)
			correct = expected > 0 && expected == selected && !wrong
		}
		if correct {
			score += points
		}
		grades = append(grades, grade{qid, correct, points})
	}
	percent := 0.0
	if max > 0 {
		percent = math.Round(score/max*10000) / 100
	}
	passed := percent >= passing
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		jsonError(w, 500, "Не удалось завершить тест")
		return
	}
	defer tx.Rollback()
	for _, g := range grades {
		earned := 0.0
		if g.correct {
			earned = g.points
		}
		_, _ = tx.ExecContext(r.Context(), `UPDATE test_attempt_answers SET is_correct=$1,earned_points=$2 WHERE attempt_id=$3 AND question_id=$4`, g.correct, earned, attemptID, g.id)
	}
	res, err := tx.ExecContext(r.Context(), `UPDATE test_attempts SET score=$1,max_score=$2,percent=$3,passed=$4,finished_at=NOW(),duration_seconds=EXTRACT(EPOCH FROM NOW()-started_at)::int,status='finished' WHERE id=$5 AND status='started'`, score, max, percent, passed, attemptID)
	if err != nil {
		jsonError(w, 500, "Не удалось завершить тест")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		jsonError(w, 409, "Тест уже завершён")
		return
	}
	_, err = tx.ExecContext(r.Context(), `UPDATE company_test_invitations SET status='finished',finished_at=NOW() WHERE id=$1`, invitationID)
	if err != nil || tx.Commit() != nil {
		jsonError(w, 500, "Не удалось завершить тест")
		return
	}
	jsonResponse(w, 200, map[string]any{"score": score, "max_score": max, "percent": percent, "passed": passed})
}
