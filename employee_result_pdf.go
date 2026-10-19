package main

import (
	"bytes"
	"context"
	"database/sql"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"log"
	"math"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"FinTalent/internal/testmodule/domain"
	"FinTalent/internal/testmodule/repository"
)

//go:embed templates/employee-result-pdf.html
var employeePDFTemplate string

//go:embed static/test-answer-review.css
var employeePDFReviewCSS string

//go:embed static/employee-result-pdf.css
var employeePDFCSS string

//go:embed static/logo.png
var employeePDFLogo []byte

var employeePDFSlots = make(chan struct{}, 2)

type employeePDFOption struct {
	Text, Class, Marker string
	Selected            bool
}

type employeePDFQuestion struct {
	Number                        int
	Text, Explanation, TextAnswer string
	Correct, Textual              bool
	Options                       []employeePDFOption
}

type employeePDFReport struct {
	Name, Email, Title, Date, Duration, Score, MaxScore string
	Percent, Correct                                    int
	Questions                                           []employeePDFQuestion
	CSS                                                 template.CSS
	Logo                                                template.URL
}

func buildEmployeePDFHTML(name, email string, result *domain.Attempt) ([]byte, error) {
	duration := max(0, result.DurationSeconds)
	report := employeePDFReport{
		Name: name, Email: email, Title: result.TestTitle,
		Duration: fmt.Sprintf("%d мин. %d сек.", duration/60, duration%60),
		Score:    strconv.FormatFloat(result.Score, 'f', -1, 64), MaxScore: strconv.FormatFloat(result.MaxScore, 'f', -1, 64),
		Percent: int(math.Round(result.Percent)),
		// Only bundled styles and image bytes are trusted; all result text is escaped by html/template.
		CSS:  template.CSS(employeePDFReviewCSS + "\n" + employeePDFCSS),
		Logo: template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(employeePDFLogo)),
	}
	if result.FinishedAt != nil {
		report.Date = result.FinishedAt.UTC().Format("02.01.2006 15:04 UTC")
	}
	groups := make(map[int64][]domain.AttemptAnswer)
	for _, answer := range result.Answers {
		groups[answer.QuestionID] = append(groups[answer.QuestionID], answer)
	}
	for index, question := range result.Questions {
		answers := groups[question.ID]
		item := employeePDFQuestion{Number: index + 1, Text: question.Question, Explanation: question.Explanation, Textual: question.Type == domain.QuestionText, TextAnswer: "Ответ не дан"}
		selected := make(map[int64]bool)
		for i, answer := range answers {
			if i == 0 {
				item.Correct = answer.IsCorrect != nil && *answer.IsCorrect
				if answer.TextAnswer != "" {
					item.TextAnswer = answer.TextAnswer
				}
			}
			if answer.SelectedAnswerID != nil {
				selected[*answer.SelectedAnswerID] = true
			}
		}
		if item.Correct {
			report.Correct++
		}
		for _, option := range question.Answers {
			value := employeePDFOption{Text: option.Answer, Selected: selected[option.ID]}
			if option.IsCorrect {
				value.Class, value.Marker = "right-answer", "✓"
			} else if value.Selected {
				value.Class, value.Marker = "wrong-answer", "×"
			}
			item.Options = append(item.Options, value)
		}
		report.Questions = append(report.Questions, item)
	}
	tmpl, err := template.New("employee-result").Parse(employeePDFTemplate)
	if err != nil {
		return nil, err
	}
	var html bytes.Buffer
	if err := tmpl.Execute(&html, report); err != nil {
		return nil, err
	}
	return html.Bytes(), nil
}

func loadEmployeePDFInvitation(ctx context.Context, ownerID, invitationID int64) (name, email string, attemptID int64, err error) {
	err = db.QueryRowContext(ctx, `SELECT e.full_name,e.email,i.attempt_id FROM company_test_invitations i
		JOIN company_test_employees e ON e.id=i.employee_id JOIN test_attempts a ON a.id=i.attempt_id
		WHERE i.id=$1 AND i.owner_user_id=$2 AND i.status='finished' AND a.status='finished'`, invitationID, ownerID).Scan(&name, &email, &attemptID)
	return
}

func employeeTestingDownloadPDF(w http.ResponseWriter, r *http.Request, owner *user, invitationID int64) {
	w.Header().Set("Cache-Control", "private, no-store")
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	name, email, attemptID, err := loadEmployeePDFInvitation(ctx, owner.ID, invitationID)
	if errors.Is(err, sql.ErrNoRows) {
		jsonError(w, 404, "Завершённый результат не найден")
		return
	}
	if err != nil {
		jsonError(w, 500, "Не удалось загрузить результат")
		return
	}
	select {
	case employeePDFSlots <- struct{}{}:
		defer func() { <-employeePDFSlots }()
	default:
		w.Header().Set("Retry-After", "5")
		jsonError(w, http.StatusServiceUnavailable, "Генератор PDF занят. Повторите через несколько секунд")
		return
	}
	result, err := repository.New(db).GetAttempt(ctx, attemptID)
	if err != nil {
		jsonError(w, 500, "Не удалось загрузить результат")
		return
	}
	html, err := buildEmployeePDFHTML(name, email, result)
	if err != nil {
		jsonError(w, 500, "Не удалось подготовить PDF")
		return
	}
	pdf, err := renderEmployeeHTMLPDF(ctx, html)
	if err != nil {
		log.Printf("employee result PDF: %v", err)
		jsonError(w, http.StatusServiceUnavailable, "Не удалось сформировать PDF. Попробуйте позже или обратитесь к администратору")
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": fmt.Sprintf("FinTalent-result-%d.pdf", invitationID)}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", strconv.Itoa(len(pdf)))
	w.Write(pdf)
}

func employeePDFBrowser() (string, error) {
	if configured := strings.TrimSpace(os.Getenv("PDF_CHROMIUM_PATH")); configured != "" {
		return exec.LookPath(configured)
	}
	for _, candidate := range []string{"chromium", "chromium-browser", "google-chrome", "google-chrome-stable", "msedge"} {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", errors.New("Chromium is not installed; set PDF_CHROMIUM_PATH")
}

func renderEmployeeHTMLPDF(ctx context.Context, html []byte) ([]byte, error) {
	browser, err := employeePDFBrowser()
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "fintalent-result-pdf-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	input, output := filepath.Join(dir, "report.html"), filepath.Join(dir, "report.pdf")
	if err := os.WriteFile(input, html, 0600); err != nil {
		return nil, err
	}
	inputURL := url.URL{Scheme: "file", Path: filepath.ToSlash(input)}
	if !strings.HasPrefix(inputURL.Path, "/") {
		inputURL.Path = "/" + inputURL.Path
	}
	cmd := exec.CommandContext(ctx, browser, "--headless", "--disable-gpu", "--no-first-run", "--no-default-browser-check", "--disable-background-networking", "--disable-extensions", "--disable-sync", "--disable-dev-shm-usage", "--host-resolver-rules=MAP * ~NOTFOUND", "--no-proxy-server", "--no-pdf-header-footer", "--timeout=15000", "--user-data-dir="+filepath.Join(dir, "profile"), "--print-to-pdf="+output, inputURL.String())
	// Do not pass the service environment (database/SMTP credentials) to Chromium.
	for _, key := range []string{"PATH", "HOME", "TMPDIR", "TEMP", "TMP", "SYSTEMROOT", "WINDIR", "LOCALAPPDATA", "LANG", "XDG_RUNTIME_DIR"} {
		if value, ok := os.LookupEnv(key); ok {
			cmd.Env = append(cmd.Env, key+"="+value)
		}
	}
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("Chromium conversion: %w", err)
	}
	info, err := os.Stat(output)
	if err != nil {
		return nil, err
	}
	if info.Size() > 30<<20 {
		return nil, errors.New("generated PDF exceeds size limit")
	}
	pdf, err := os.ReadFile(output)
	if err != nil {
		return nil, err
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		return nil, errors.New("Chromium returned an invalid PDF")
	}
	return pdf, nil
}
