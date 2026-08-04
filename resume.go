package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type publicDictionary struct {
	Name  string                 `json:"name"`
	Alias string                 `json:"alias"`
	Items []publicDictionaryItem `json:"items"`
}

type publicDictionaryItem struct {
	ID      int64  `json:"id"`
	Value   string `json:"value"`
	Comment string `json:"comment"`
	Icon    string `json:"icon"`
}

func registerResumeRoutes() {
	http.HandleFunc("/resume/create", servePage("static/resume-create.html"))
	http.HandleFunc("/resume/view/", servePage("static/resume-view.html"))
	http.HandleFunc("/api/public/resumes/", publicResumeHandler)
	http.HandleFunc("/api/resumes/", resumeKnowledgeActionHandler)
	http.HandleFunc("/api/public/dictionaries/", publicDictionaryHandler)
	http.HandleFunc("/api/assets/position-icon/", positionIconHandler)
	http.HandleFunc("/api/assets/accounting-area-icon/", accountingAreaIconHandler)
	http.HandleFunc("/api/v1/resumes/experience", resumeExperienceHandler)
	http.HandleFunc("/api/v1/resumes/education", resumeEducationHandler)
	http.HandleFunc("/api/v1/resumes/education-certificate", resumeEducationCertificateHandler)
	http.HandleFunc("/api/v1/resumes/languages", resumeLanguagesHandler)
	http.HandleFunc("/api/v1/resumes/finance", resumeFinanceHandler)
	http.HandleFunc("/api/v1/resumes/publish", resumePublishHandler)
	http.HandleFunc("/api/v1/resumes/status", resumeStatusHandler)
}

type resumeLanguageItem struct {
	ID   int64  `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

func resumeLanguagesHandler(w http.ResponseWriter, r *http.Request) {
	u, err := userFromRequest(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, "Требуется авторизация")
		return
	}

	switch r.Method {
	case http.MethodGet:
		rows, queryErr := db.QueryContext(r.Context(), `
			SELECT id, code, name
			FROM languages
			WHERE is_active = TRUE
			ORDER BY sort_order, name`)
		if queryErr != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить языки")
			return
		}
		defer rows.Close()
		items := []resumeLanguageItem{}
		for rows.Next() {
			var item resumeLanguageItem
			if queryErr = rows.Scan(&item.ID, &item.Code, &item.Name); queryErr != nil {
				writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить языки")
				return
			}
			items = append(items, item)
		}

		selected := []int64{}
		selectedRows, queryErr := db.QueryContext(r.Context(), `
			SELECT rl.language_id
			FROM resume_languages rl
			JOIN resumes r ON r.id = rl.resume_id
			WHERE r.user_id = $1 AND r.deleted_at IS NULL
			ORDER BY rl.sort_order, rl.language_id`,
			u.ID,
		)
		if queryErr != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить выбранные языки")
			return
		}
		defer selectedRows.Close()
		for selectedRows.Next() {
			var id int64
			if queryErr = selectedRows.Scan(&id); queryErr != nil {
				writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить выбранные языки")
				return
			}
			selected = append(selected, id)
		}
		writeAdminJSON(w, http.StatusOK, map[string]any{"items": items, "selected_ids": selected})

	case http.MethodPut:
		var payload struct {
			LanguageIDs []int64 `json:"language_ids"`
		}
		if json.NewDecoder(r.Body).Decode(&payload) != nil || len(payload.LanguageIDs) > 20 || duplicateIDs(payload.LanguageIDs) {
			writeJSON(w, http.StatusBadRequest, "Проверьте выбранные языки")
			return
		}
		tx, txErr := db.BeginTx(r.Context(), nil)
		if txErr != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось сохранить языки")
			return
		}
		defer tx.Rollback()
		var resumeID int64
		if txErr = tx.QueryRowContext(r.Context(), `
			INSERT INTO resumes(user_id,current_step)
			VALUES($1,1)
			ON CONFLICT(user_id) DO UPDATE SET updated_at=NOW()
			RETURNING id`,
			u.ID,
		).Scan(&resumeID); txErr != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось сохранить языки")
			return
		}
		if _, txErr = tx.ExecContext(r.Context(), `DELETE FROM resume_languages WHERE resume_id=$1`, resumeID); txErr != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось сохранить языки")
			return
		}
		for order, languageID := range payload.LanguageIDs {
			result, insertErr := tx.ExecContext(r.Context(), `
				INSERT INTO resume_languages(resume_id,language_id,sort_order)
				SELECT $1,id,$3 FROM languages WHERE id=$2 AND is_active=TRUE`,
				resumeID, languageID, order,
			)
			if insertErr != nil {
				writeJSON(w, http.StatusInternalServerError, "Не удалось сохранить языки")
				return
			}
			if affected, _ := result.RowsAffected(); affected == 0 {
				writeJSON(w, http.StatusBadRequest, "Выбран недоступный язык")
				return
			}
		}
		if txErr = tx.Commit(); txErr != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось сохранить языки")
			return
		}
		writeJSON(w, http.StatusOK, "Языки сохранены")

	default:
		writeJSON(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
	}
}

func resumeStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
		return
	}
	u, err := userFromRequest(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, "Требуется авторизация")
		return
	}
	var id int64
	var status string
	err = db.QueryRowContext(r.Context(), `
		SELECT id, status
		FROM resumes
		WHERE user_id = $1 AND deleted_at IS NULL`,
		u.ID,
	).Scan(&id, &status)
	if err == sql.ErrNoRows {
		writeAdminJSON(w, http.StatusOK, map[string]any{
			"exists":    false,
			"published": false,
		})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, "Не удалось проверить резюме")
		return
	}
	writeAdminJSON(w, http.StatusOK, map[string]any{
		"exists":    true,
		"published": status == "published",
		"id":        id,
		"status":    status,
	})
}

type resumeExperience struct {
	ID               int64   `json:"id,omitempty"`
	CompanyName      string  `json:"company_name"`
	Position         string  `json:"position"`
	City             string  `json:"city"`
	IndustryItemID   *int64  `json:"industry_item_id,omitempty"`
	IndustryName     string  `json:"industry_name,omitempty"`
	StartMonth       int     `json:"start_month"`
	StartYear        int     `json:"start_year"`
	EndMonth         *int    `json:"end_month,omitempty"`
	EndYear          *int    `json:"end_year,omitempty"`
	IsCurrent        bool    `json:"is_current"`
	Responsibilities string  `json:"responsibilities"`
	Achievements     string  `json:"achievements"`
	DutyIDs          []int64 `json:"duty_ids"`
}

func resumeExperienceHandler(w http.ResponseWriter, r *http.Request) {
	u, err := userFromRequest(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, "Требуется авторизация")
		return
	}
	switch r.Method {
	case http.MethodGet:
		rows, queryErr := db.QueryContext(r.Context(), `SELECT e.id,e.company_name,e.position,e.city,e.industry_item_id,COALESCE(i.value,''),e.start_month,e.start_year,e.end_month,e.end_year,e.is_current,e.responsibilities,e.achievements
			FROM resumes x JOIN resume_work_experiences e ON e.resume_id=x.id
			LEFT JOIN dictionary_items i ON i.id=e.industry_item_id
			WHERE x.user_id=$1 AND x.deleted_at IS NULL ORDER BY e.sort_order,e.id`, u.ID)
		if queryErr != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить опыт работы")
			return
		}
		defer rows.Close()
		items := []resumeExperience{}
		for rows.Next() {
			var item resumeExperience
			var industry sql.NullInt64
			var endMonth, endYear sql.NullInt64
			if queryErr = rows.Scan(&item.ID, &item.CompanyName, &item.Position, &item.City, &industry, &item.IndustryName, &item.StartMonth, &item.StartYear, &endMonth, &endYear, &item.IsCurrent, &item.Responsibilities, &item.Achievements); queryErr != nil {
				writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить опыт работы")
				return
			}
			if industry.Valid {
				item.IndustryItemID = &industry.Int64
			}
			if endMonth.Valid {
				value := int(endMonth.Int64)
				item.EndMonth = &value
			}
			if endYear.Valid {
				value := int(endYear.Int64)
				item.EndYear = &value
			}
			item.DutyIDs = []int64{}
			items = append(items, item)
		}
		_ = rows.Close()
		if len(items) > 0 {
			byID := make(map[int64]*resumeExperience, len(items))
			for index := range items {
				byID[items[index].ID] = &items[index]
			}
			dutyRows, dutyErr := db.QueryContext(r.Context(), `SELECT ed.experience_id,ed.duty_id
				FROM resumes x JOIN resume_work_experiences e ON e.resume_id=x.id
				JOIN resume_work_experience_duties ed ON ed.experience_id=e.id
				WHERE x.user_id=$1 AND x.deleted_at IS NULL ORDER BY e.sort_order,ed.sort_order,ed.duty_id`, u.ID)
			if dutyErr != nil {
				writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить обязанности по местам работы")
				return
			}
			defer dutyRows.Close()
			for dutyRows.Next() {
				var experienceID, dutyID int64
				if dutyErr = dutyRows.Scan(&experienceID, &dutyID); dutyErr != nil {
					writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить обязанности по местам работы")
					return
				}
				if item := byID[experienceID]; item != nil {
					item.DutyIDs = append(item.DutyIDs, dutyID)
				}
			}
		}
		writeAdminJSON(w, http.StatusOK, items)
	case http.MethodPut:
		var items []resumeExperience
		if json.NewDecoder(r.Body).Decode(&items) != nil || len(items) > 20 {
			writeJSON(w, http.StatusBadRequest, "Проверьте список мест работы")
			return
		}
		for index := range items {
			if message := validateResumeExperience(&items[index]); message != "" {
				writeJSON(w, http.StatusBadRequest, message)
				return
			}
			if duplicateIDs(items[index].DutyIDs) {
				writeJSON(w, http.StatusBadRequest, "В обязанностях места работы есть повторяющиеся варианты")
				return
			}
		}
		tx, txErr := db.BeginTx(r.Context(), nil)
		if txErr != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось сохранить опыт работы")
			return
		}
		defer tx.Rollback()
		var resumeID int64
		if txErr = tx.QueryRowContext(r.Context(), `INSERT INTO resumes(user_id,current_step) VALUES($1,1) ON CONFLICT(user_id) DO UPDATE SET updated_at=NOW() RETURNING id`, u.ID).Scan(&resumeID); txErr != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось сохранить опыт работы")
			return
		}
		if _, txErr = tx.ExecContext(r.Context(), `DELETE FROM resume_work_experiences WHERE resume_id=$1`, resumeID); txErr != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось сохранить опыт работы")
			return
		}
		for order, item := range items {
			var validIndustry *int64
			if item.IndustryItemID != nil {
				var id int64
				if txErr = tx.QueryRowContext(r.Context(), `SELECT i.id FROM dictionary_items i JOIN dictionaries d ON d.id=i.dictionary_id WHERE i.id=$1 AND d.alias='business_sector' AND i.active=TRUE AND i.deleted_at IS NULL`, *item.IndustryItemID).Scan(&id); txErr != nil {
					writeJSON(w, http.StatusBadRequest, "Выбрано недоступное направление компании")
					return
				}
				validIndustry = &id
			}
			var experienceID int64
			if txErr = tx.QueryRowContext(r.Context(), `INSERT INTO resume_work_experiences(resume_id,company_name,position,city,industry_item_id,start_month,start_year,end_month,end_year,is_current,responsibilities,achievements,sort_order)
				VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) RETURNING id`, resumeID, item.CompanyName, item.Position, item.City, validIndustry, item.StartMonth, item.StartYear, item.EndMonth, item.EndYear, item.IsCurrent, item.Responsibilities, item.Achievements, order).Scan(&experienceID); txErr != nil {
				writeJSON(w, http.StatusInternalServerError, "Не удалось сохранить опыт работы")
				return
			}
			for dutyOrder, dutyID := range item.DutyIDs {
				result, dutyErr := tx.ExecContext(r.Context(), `INSERT INTO resume_work_experience_duties(experience_id,duty_id,sort_order)
					SELECT $1,d.id,$3 FROM duties d JOIN duty_categories c ON c.id=d.category_id
					WHERE d.id=$2 AND d.is_active=TRUE AND c.is_active=TRUE`, experienceID, dutyID, dutyOrder)
				if dutyErr != nil {
					writeJSON(w, http.StatusInternalServerError, "Не удалось сохранить обязанности места работы")
					return
				}
				if count, _ := result.RowsAffected(); count != 1 {
					writeJSON(w, http.StatusBadRequest, "Выбрана недоступная обязанность")
					return
				}
				if _, dutyErr = tx.ExecContext(r.Context(), `INSERT INTO resume_duties(resume_id,duty_id) VALUES($1,$2) ON CONFLICT(resume_id,duty_id) DO NOTHING`, resumeID, dutyID); dutyErr != nil {
					writeJSON(w, http.StatusInternalServerError, "Не удалось обновить обязанности резюме")
					return
				}
			}
		}
		if txErr = tx.Commit(); txErr != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось сохранить опыт работы")
			return
		}
		writeJSON(w, http.StatusOK, "Опыт работы сохранён")
	default:
		writeJSON(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
	}
}

func validateResumeExperience(item *resumeExperience) string {
	item.CompanyName = strings.TrimSpace(item.CompanyName)
	item.Position = strings.TrimSpace(item.Position)
	item.City = strings.TrimSpace(item.City)
	item.Responsibilities = strings.TrimSpace(item.Responsibilities)
	item.Achievements = strings.TrimSpace(item.Achievements)
	if item.CompanyName == "" || len([]rune(item.CompanyName)) > 240 {
		return "Укажите название компании"
	}
	if item.Position == "" || len([]rune(item.Position)) > 240 {
		return "Укажите должность"
	}
	if len([]rune(item.City)) > 200 || len([]rune(item.Responsibilities)) > 4000 || len([]rune(item.Achievements)) > 4000 {
		return "Одно из полей опыта работы заполнено некорректно"
	}
	maxYear := time.Now().Year() + 1
	if item.StartMonth < 1 || item.StartMonth > 12 || item.StartYear < 1950 || item.StartYear > maxYear {
		return "Проверьте дату начала работы"
	}
	if item.IsCurrent {
		item.EndMonth, item.EndYear = nil, nil
		return ""
	}
	if item.EndMonth == nil || item.EndYear == nil || *item.EndMonth < 1 || *item.EndMonth > 12 || *item.EndYear < 1950 || *item.EndYear > maxYear {
		return "Проверьте дату окончания работы"
	}
	if *item.EndYear < item.StartYear || (*item.EndYear == item.StartYear && *item.EndMonth < item.StartMonth) {
		return "Дата окончания работы не может быть раньше даты начала"
	}
	return ""
}

type resumeEducation struct {
	ID              int64  `json:"id,omitempty"`
	EducationType   string `json:"education_type"`
	Institution     string `json:"institution"`
	Specialization  string `json:"specialization"`
	City            string `json:"city"`
	StartYear       int    `json:"start_year"`
	EndYear         *int   `json:"end_year,omitempty"`
	IsCurrent       bool   `json:"is_current"`
	Description     string `json:"description"`
	CertificateID   *int64 `json:"certificate_id,omitempty"`
	CertificateName string `json:"certificate_name,omitempty"`
}

var resumeEducationTypes = map[string]bool{
	"higher": true, "incomplete_higher": true, "secondary_vocational": true,
	"secondary": true, "professional_retraining": true, "course": true,
	"certificate": true, "other": true,
}

func resumeEducationHandler(w http.ResponseWriter, r *http.Request) {
	u, err := userFromRequest(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, "Требуется авторизация")
		return
	}
	switch r.Method {
	case http.MethodGet:
		rows, queryErr := db.QueryContext(r.Context(), `SELECT e.id,e.education_type,e.institution,e.specialization,e.city,COALESCE(e.start_year,0),e.end_year,e.is_current,e.description,e.certificate_id,COALESCE(c.original_name,'')
			FROM resumes x JOIN resume_educations e ON e.resume_id=x.id
			LEFT JOIN resume_education_certificates c ON c.id=e.certificate_id
			WHERE x.user_id=$1 AND x.deleted_at IS NULL ORDER BY e.sort_order,e.id`, u.ID)
		if queryErr != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить образование")
			return
		}
		defer rows.Close()
		items := []resumeEducation{}
		for rows.Next() {
			var item resumeEducation
			var endYear, certificateID sql.NullInt64
			if queryErr = rows.Scan(&item.ID, &item.EducationType, &item.Institution, &item.Specialization, &item.City, &item.StartYear, &endYear, &item.IsCurrent, &item.Description, &certificateID, &item.CertificateName); queryErr != nil {
				writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить образование")
				return
			}
			if endYear.Valid {
				value := int(endYear.Int64)
				item.EndYear = &value
			}
			if certificateID.Valid {
				item.CertificateID = &certificateID.Int64
			}
			items = append(items, item)
		}
		writeAdminJSON(w, http.StatusOK, items)
	case http.MethodPut:
		var items []resumeEducation
		if json.NewDecoder(r.Body).Decode(&items) != nil || len(items) > 20 {
			writeJSON(w, http.StatusBadRequest, "Проверьте список образования")
			return
		}
		for index := range items {
			if message := validateResumeEducation(&items[index]); message != "" {
				writeJSON(w, http.StatusBadRequest, message)
				return
			}
		}
		tx, txErr := db.BeginTx(r.Context(), nil)
		if txErr != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось сохранить образование")
			return
		}
		defer tx.Rollback()
		var resumeID int64
		if txErr = tx.QueryRowContext(r.Context(), `INSERT INTO resumes(user_id,current_step) VALUES($1,1) ON CONFLICT(user_id) DO UPDATE SET updated_at=NOW() RETURNING id`, u.ID).Scan(&resumeID); txErr != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось сохранить образование")
			return
		}
		if _, txErr = tx.ExecContext(r.Context(), `DELETE FROM resume_educations WHERE resume_id=$1`, resumeID); txErr != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось сохранить образование")
			return
		}
		for order, item := range items {
			if item.CertificateID != nil {
				var exists bool
				if txErr = tx.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM resume_education_certificates WHERE id=$1 AND user_id=$2)`, *item.CertificateID, u.ID).Scan(&exists); txErr != nil || !exists {
					writeJSON(w, http.StatusBadRequest, "Выбран недоступный файл сертификата")
					return
				}
			}
			if _, txErr = tx.ExecContext(r.Context(), `INSERT INTO resume_educations(resume_id,education_type,institution,specialization,city,start_year,end_year,is_current,description,sort_order,certificate_id)
				VALUES($1,$2,$3,$4,$5,NULL,$6,$7,$8,$9,$10)`, resumeID, item.EducationType, item.Institution, item.Specialization, item.City, item.EndYear, item.IsCurrent, item.Description, order, item.CertificateID); txErr != nil {
				writeJSON(w, http.StatusInternalServerError, "Не удалось сохранить образование")
				return
			}
		}
		if txErr = tx.Commit(); txErr != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось сохранить образование")
			return
		}
		writeJSON(w, http.StatusOK, "Образование сохранено")
	default:
		writeJSON(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
	}
}

func validateResumeEducation(item *resumeEducation) string {
	item.EducationType = strings.TrimSpace(item.EducationType)
	item.Institution = strings.TrimSpace(item.Institution)
	item.Specialization = strings.TrimSpace(item.Specialization)
	item.City = strings.TrimSpace(item.City)
	item.Description = strings.TrimSpace(item.Description)
	if !resumeEducationTypes[item.EducationType] {
		return "Выберите тип образования"
	}
	if item.Institution == "" || len([]rune(item.Institution)) > 300 {
		return "Укажите учебное заведение"
	}
	if len([]rune(item.Specialization)) > 300 || len([]rune(item.City)) > 200 || len([]rune(item.Description)) > 3000 {
		return "Одно из полей образования заполнено некорректно"
	}
	maxYear := time.Now().Year() + 10
	item.StartYear = 0
	item.City = ""
	if item.IsCurrent {
		item.EndYear = nil
		return ""
	}
	if item.EndYear == nil || *item.EndYear < 1950 || *item.EndYear > maxYear {
		return "Проверьте год окончания обучения"
	}
	return ""
}

func resumeEducationCertificateHandler(w http.ResponseWriter, r *http.Request) {
	u, err := userFromRequest(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, "Требуется авторизация")
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 9<<20)
	if err = r.ParseMultipartForm(8 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, "Файл слишком большой")
		return
	}
	file, header, err := r.FormFile("certificate")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, "Выберите файл сертификата")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, (8<<20)+1))
	if err != nil || len(data) == 0 || len(data) > 8<<20 {
		writeJSON(w, http.StatusBadRequest, "Размер файла должен быть не больше 8 МБ")
		return
	}
	contentType := http.DetectContentType(data)
	extensions := map[string]string{"application/pdf": ".pdf", "image/jpeg": ".jpg", "image/png": ".png"}
	extension, allowed := extensions[contentType]
	if !allowed {
		writeJSON(w, http.StatusBadRequest, "Разрешены PDF, JPG и PNG")
		return
	}
	originalName := strings.TrimSpace(filepath.Base(header.Filename))
	if originalName == "" {
		originalName = "certificate" + extension
	}
	if runes := []rune(originalName); len(runes) > 300 {
		originalName = string(runes[:300])
	}
	randomBytes := make([]byte, 16)
	if _, err = rand.Read(randomBytes); err != nil {
		writeJSON(w, http.StatusInternalServerError, "Не удалось сохранить файл")
		return
	}
	storageName := strconv.FormatInt(u.ID, 10) + "-" + hex.EncodeToString(randomBytes) + extension
	directory := filepath.Join("uploads", "resume-certificates")
	if err = os.MkdirAll(directory, 0o750); err != nil {
		writeJSON(w, http.StatusInternalServerError, "Не удалось сохранить файл")
		return
	}
	fullPath := filepath.Join(directory, storageName)
	if err = os.WriteFile(fullPath, data, 0o640); err != nil {
		writeJSON(w, http.StatusInternalServerError, "Не удалось сохранить файл")
		return
	}
	var id int64
	err = db.QueryRowContext(r.Context(), `INSERT INTO resume_education_certificates(user_id,storage_name,original_name,content_type,file_size) VALUES($1,$2,$3,$4,$5) RETURNING id`, u.ID, storageName, originalName, contentType, len(data)).Scan(&id)
	if err != nil {
		_ = os.Remove(fullPath)
		writeJSON(w, http.StatusInternalServerError, "Не удалось сохранить файл")
		return
	}
	writeAdminJSON(w, http.StatusCreated, map[string]any{"id": id, "name": originalName})
}

type resumeSearchStatus struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Icon string `json:"icon"`
}

type resumeFinanceOption struct {
	ID    int64  `json:"id"`
	Value string `json:"value"`
	Icon  string `json:"icon"`
}

type resumeFinanceCity struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Region string `json:"region"`
}

type resumeFinancePayload struct {
	DesiredSalary        *float64              `json:"desired_salary"`
	AvailableImmediately bool                  `json:"available_immediately"`
	SearchStatusCode     string                `json:"search_status_code"`
	WorkPreferences      string                `json:"work_preferences"`
	Statuses             []resumeSearchStatus  `json:"statuses,omitempty"`
	WorkFormats          []resumeFinanceOption `json:"work_formats,omitempty"`
	WorkFormatIDs        []int64               `json:"work_format_ids"`
	Cities               []resumeFinanceCity   `json:"cities"`
	CityIDs              []int64               `json:"city_ids"`
}

func resumeFinanceHandler(w http.ResponseWriter, r *http.Request) {
	u, err := userFromRequest(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, "Требуется авторизация")
		return
	}
	userID := u.ID

	switch r.Method {
	case http.MethodGet:
		rows, err := db.Query(`
			SELECT code, name, icon
			FROM resume_search_statuses
			WHERE is_active = TRUE
			ORDER BY sort_order, name`)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить финансовые данные")
			return
		}
		defer rows.Close()

		payload := resumeFinancePayload{SearchStatusCode: "open", Statuses: []resumeSearchStatus{}, WorkFormats: []resumeFinanceOption{}, WorkFormatIDs: []int64{}, Cities: []resumeFinanceCity{}, CityIDs: []int64{}}
		for rows.Next() {
			var status resumeSearchStatus
			if err := rows.Scan(&status.Code, &status.Name, &status.Icon); err != nil {
				writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить статусы резюме")
				return
			}
			payload.Statuses = append(payload.Statuses, status)
		}
		if err := rows.Err(); err != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить статусы резюме")
			return
		}

		formatRows, formatErr := db.Query(`
			SELECT i.id,i.value,COALESCE(i.icon,'')
			FROM dictionary_items i
			JOIN dictionaries d ON d.id=i.dictionary_id
			WHERE d.alias='work_format' AND i.active=TRUE AND i.deleted_at IS NULL
			ORDER BY i.sort_order,i.id`)
		if formatErr != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить форматы работы")
			return
		}
		defer formatRows.Close()
		for formatRows.Next() {
			var option resumeFinanceOption
			if formatErr = formatRows.Scan(&option.ID, &option.Value, &option.Icon); formatErr != nil {
				writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить форматы работы")
				return
			}
			payload.WorkFormats = append(payload.WorkFormats, option)
		}

		var salary sql.NullFloat64
		var status sql.NullString
		var resumeID int64
		err = db.QueryRow(`
			SELECT r.id,r.desired_salary,r.available_immediately,r.search_status_code,
				COALESCE(r.work_preferences,'')
			FROM resumes r
			WHERE user_id = $1`,
			userID,
		).Scan(&resumeID, &salary, &payload.AvailableImmediately, &status, &payload.WorkPreferences)
		if err != nil && err != sql.ErrNoRows {
			writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить финансовые данные")
			return
		}
		if salary.Valid {
			payload.DesiredSalary = &salary.Float64
		}
		if status.Valid && status.String != "" {
			payload.SearchStatusCode = status.String
		}
		if resumeID > 0 {
			selectedRows, selectedErr := db.Query(`
				SELECT dictionary_item_id
				FROM resume_work_formats
				WHERE resume_id=$1
				ORDER BY sort_order,dictionary_item_id`,
				resumeID,
			)
			if selectedErr != nil {
				writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить форматы работы")
				return
			}
			defer selectedRows.Close()
			for selectedRows.Next() {
				var id int64
				if selectedErr = selectedRows.Scan(&id); selectedErr != nil {
					writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить форматы работы")
					return
				}
				payload.WorkFormatIDs = append(payload.WorkFormatIDs, id)
			}
			cityRows, cityErr := db.Query(`
				SELECT c.id,c.name,c.region_name
				FROM resume_preferred_cities rpc
				JOIN cities c ON c.id=rpc.city_id
				WHERE rpc.resume_id=$1
				ORDER BY rpc.sort_order,c.id`,
				resumeID,
			)
			if cityErr != nil {
				writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить города")
				return
			}
			defer cityRows.Close()
			for cityRows.Next() {
				var city resumeFinanceCity
				if cityErr = cityRows.Scan(&city.ID, &city.Name, &city.Region); cityErr != nil {
					writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить города")
					return
				}
				payload.Cities = append(payload.Cities, city)
				payload.CityIDs = append(payload.CityIDs, city.ID)
			}
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(payload)

	case http.MethodPut:
		var payload resumeFinancePayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeJSON(w, http.StatusBadRequest, "Некорректные данные")
			return
		}
		payload.SearchStatusCode = strings.TrimSpace(payload.SearchStatusCode)
		payload.WorkPreferences = strings.TrimSpace(payload.WorkPreferences)
		if len([]rune(payload.WorkPreferences)) > 2000 {
			writeJSON(w, http.StatusBadRequest, "Пожелания к работе не должны превышать 2000 символов")
			return
		}
		if payload.DesiredSalary == nil || *payload.DesiredSalary <= 0 || *payload.DesiredSalary > 100000000 {
			writeJSON(w, http.StatusBadRequest, "Укажите желаемую зарплату")
			return
		}
		if len(payload.WorkFormatIDs) == 0 || len(payload.WorkFormatIDs) > 3 || duplicateIDs(payload.WorkFormatIDs) {
			writeJSON(w, http.StatusBadRequest, "Выберите формат работы")
			return
		}
		if len(payload.CityIDs) == 0 || len(payload.CityIDs) > 5 || duplicateIDs(payload.CityIDs) {
			writeJSON(w, http.StatusBadRequest, "Выберите от одного до пяти городов")
			return
		}

		var statusExists bool
		if err := db.QueryRow(`
			SELECT EXISTS(
				SELECT 1 FROM resume_search_statuses
				WHERE code = $1 AND is_active = TRUE
			)`,
			payload.SearchStatusCode,
		).Scan(&statusExists); err != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось проверить статус резюме")
			return
		}
		if !statusExists {
			writeJSON(w, http.StatusBadRequest, "Выберите статус резюме")
			return
		}

		var cityCount int
		if err := db.QueryRow(`
			SELECT COUNT(*)
			FROM cities c JOIN countries co ON co.id=c.country_id
			WHERE c.id=ANY($1) AND co.code='RU'`,
			payload.CityIDs,
		).Scan(&cityCount); err != nil || cityCount != len(payload.CityIDs) {
			writeJSON(w, http.StatusBadRequest, "Выберите города из справочника")
			return
		}

		tx, txErr := db.BeginTx(r.Context(), nil)
		if txErr != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось сохранить финансовые данные")
			return
		}
		defer tx.Rollback()
		var resumeID int64
		txErr = tx.QueryRow(`
			INSERT INTO resumes (
				user_id, desired_salary, available_immediately,
				search_status_code, preferred_city_id, work_preferences, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, NOW())
			ON CONFLICT (user_id) DO UPDATE SET
				desired_salary = EXCLUDED.desired_salary,
				available_immediately = EXCLUDED.available_immediately,
				search_status_code = EXCLUDED.search_status_code,
				preferred_city_id = EXCLUDED.preferred_city_id,
				work_preferences = EXCLUDED.work_preferences,
				updated_at = NOW()
			RETURNING id`,
			userID, *payload.DesiredSalary, payload.AvailableImmediately, payload.SearchStatusCode, payload.CityIDs[0], payload.WorkPreferences,
		).Scan(&resumeID)
		if txErr != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось сохранить финансовые данные")
			return
		}
		if _, txErr = tx.Exec(`DELETE FROM resume_work_formats WHERE resume_id=$1`, resumeID); txErr != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось сохранить форматы работы")
			return
		}
		for order, formatID := range payload.WorkFormatIDs {
			result, insertErr := tx.Exec(`
				INSERT INTO resume_work_formats(resume_id,dictionary_item_id,sort_order)
				SELECT $1,i.id,$3
				FROM dictionary_items i JOIN dictionaries d ON d.id=i.dictionary_id
				WHERE i.id=$2 AND d.alias='work_format' AND i.active=TRUE AND i.deleted_at IS NULL`,
				resumeID, formatID, order,
			)
			if insertErr != nil {
				writeJSON(w, http.StatusInternalServerError, "Не удалось сохранить форматы работы")
				return
			}
			if affected, _ := result.RowsAffected(); affected == 0 {
				writeJSON(w, http.StatusBadRequest, "Выбран недоступный формат работы")
				return
			}
		}
		if _, txErr = tx.Exec(`DELETE FROM resume_preferred_cities WHERE resume_id=$1`, resumeID); txErr != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось сохранить города")
			return
		}
		for order, cityID := range payload.CityIDs {
			if _, txErr = tx.Exec(`
				INSERT INTO resume_preferred_cities(resume_id,city_id,sort_order)
				VALUES($1,$2,$3)`,
				resumeID, cityID, order,
			); txErr != nil {
				writeJSON(w, http.StatusInternalServerError, "Не удалось сохранить города")
				return
			}
		}
		if txErr = tx.Commit(); txErr != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось сохранить финансовые данные")
			return
		}
		writeJSON(w, http.StatusOK, "Финансовые данные сохранены")

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func resumePublishHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	u, err := userFromRequest(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, "Требуется авторизация")
		return
	}
	userID := u.ID

	var salary sql.NullFloat64
	var status sql.NullString
	var hasCity bool
	var hasWorkFormat bool
	if err := db.QueryRow(`
		SELECT r.desired_salary,r.search_status_code,
			EXISTS(SELECT 1 FROM resume_preferred_cities rpc WHERE rpc.resume_id=r.id),
			EXISTS(SELECT 1 FROM resume_work_formats rwf WHERE rwf.resume_id=r.id)
		FROM resumes r
		WHERE r.user_id = $1`,
		userID,
	).Scan(&salary, &status, &hasCity, &hasWorkFormat); err != nil {
		if err == sql.ErrNoRows {
			writeJSON(w, http.StatusBadRequest, "Сначала заполните резюме")
			return
		}
		writeJSON(w, http.StatusInternalServerError, "Не удалось проверить резюме")
		return
	}
	if !salary.Valid || salary.Float64 <= 0 || !status.Valid || status.String == "" {
		writeJSON(w, http.StatusBadRequest, "Заполните шаг «Финансы»")
		return
	}
	if !hasCity || !hasWorkFormat {
		writeJSON(w, http.StatusBadRequest, "Выберите город и формат работы на шаге «Финансы»")
		return
	}

	var resumeID int64
	err = db.QueryRow(`
		UPDATE resumes
		SET status = 'published',
			visibility = CASE WHEN search_status_code = 'hidden' THEN 'private' ELSE 'public' END,
			published_at = NOW(),
			updated_at = NOW()
		WHERE user_id = $1
		RETURNING id`,
		userID,
	).Scan(&resumeID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, "Не удалось опубликовать резюме")
		return
	}
	writeAdminJSON(w, http.StatusOK, map[string]any{
		"published": true,
		"id":        resumeID,
	})
}

func publicDictionaryHandler(w http.ResponseWriter, r *http.Request) {
	alias := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/public/dictionaries/"))
	if r.Method != http.MethodGet || !validAlias(alias) {
		writeJSON(w, http.StatusBadRequest, "Некорректный справочник")
		return
	}
	ctx, cancel := contextWithTimeout()
	defer cancel()
	var result publicDictionary
	if err := db.QueryRowContext(ctx, `SELECT name,alias FROM dictionaries WHERE alias=$1`, alias).Scan(&result.Name, &result.Alias); err != nil {
		writeJSON(w, http.StatusNotFound, "Справочник не найден")
		return
	}
	rows, err := db.QueryContext(ctx, `SELECT id,value,comment,icon FROM dictionary_items WHERE dictionary_id=(SELECT id FROM dictionaries WHERE alias=$1) ORDER BY sort_order,id`, alias)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить варианты")
		return
	}
	defer rows.Close()
	result.Items = []publicDictionaryItem{}
	for rows.Next() {
		var item publicDictionaryItem
		if err := rows.Scan(&item.ID, &item.Value, &item.Comment, &item.Icon); err != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить варианты")
			return
		}
		result.Items = append(result.Items, item)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(result)
}

func positionIconHandler(w http.ResponseWriter, r *http.Request) {
	serveAtlasIcon(w, r, "/api/assets/position-icon/", "static/position-icons-atlas.png")
}

func accountingAreaIconHandler(w http.ResponseWriter, r *http.Request) {
	serveAtlasIcon(w, r, "/api/assets/accounting-area-icon/", "static/accounting-area-icons-atlas.png")
}

func serveAtlasIcon(w http.ResponseWriter, r *http.Request, routePrefix, atlasPath string) {
	filename := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, routePrefix), ".png")
	index, err := strconv.Atoi(filename)
	if err != nil || index < 0 || index > 15 {
		http.NotFound(w, r)
		return
	}
	file, err := os.Open(atlasPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	img, err := png.Decode(file)
	if err != nil {
		http.Error(w, "Не удалось открыть изображение", http.StatusInternalServerError)
		return
	}
	bounds := img.Bounds()
	column, row := index%4, index/4
	rect := image.Rect(bounds.Min.X+column*bounds.Dx()/4, bounds.Min.Y+row*bounds.Dy()/4, bounds.Min.X+(column+1)*bounds.Dx()/4, bounds.Min.Y+(row+1)*bounds.Dy()/4)
	cropped, ok := img.(interface {
		SubImage(image.Rectangle) image.Image
	})
	if !ok {
		http.Error(w, "Неподдерживаемое изображение", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_ = png.Encode(w, cropped.SubImage(rect))
}
