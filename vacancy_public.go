package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type publicVacancyItem struct {
	ID         int64  `json:"id"`
	Value      string `json:"value"`
	Comment    string `json:"comment"`
	Icon       string `json:"icon"`
	Importance string `json:"importance"`
}

type publicVacancyDictionary struct {
	ID             int64               `json:"id"`
	Name           string              `json:"name"`
	Alias          string              `json:"alias"`
	Icon           string              `json:"icon"`
	SelectionColor string              `json:"selection_color"`
	Items          []publicVacancyItem `json:"items"`
}

type publicVacancyBlock struct {
	ID           int64                     `json:"id"`
	Name         string                    `json:"name"`
	SortOrder    int                       `json:"sort_order"`
	Dictionaries []publicVacancyDictionary `json:"dictionaries"`
}

type publicVacancyDutyGroup struct {
	ID     int64    `json:"id"`
	Name   string   `json:"name"`
	Icon   string   `json:"icon"`
	Duties []string `json:"duties"`
}

type publicVacancyTest struct {
	ID               int64   `json:"id"`
	Title            string  `json:"title"`
	Description      string  `json:"description"`
	TimeLimitSeconds int     `json:"time_limit_seconds"`
	QuestionCount    int     `json:"question_count"`
	Rating           float64 `json:"rating"`
	ReviewCount      int     `json:"review_count"`
	IsFree           bool    `json:"is_free"`
	Price            float64 `json:"price"`
	Author           string  `json:"author"`
}

type publicVacancyApplicationStats struct {
	Total     int `json:"total"`
	Passed    int `json:"passed"`
	NotPassed int `json:"not_passed"`
}

type publicVacancyView struct {
	ID            int64                         `json:"id"`
	Description   string                        `json:"description"`
	SalaryFrom    *float64                      `json:"salary_from"`
	SalaryTo      *float64                      `json:"salary_to"`
	SalaryTaxMode string                        `json:"salary_tax_mode"`
	Currency      string                        `json:"currency"`
	City          string                        `json:"city"`
	Address       string                        `json:"address"`
	OwnerName     string                        `json:"owner_name"`
	PublishedAt   time.Time                     `json:"published_at"`
	Blocks        []publicVacancyBlock          `json:"blocks"`
	Duties        []publicVacancyDutyGroup      `json:"duties"`
	Tests         []publicVacancyTest           `json:"tests"`
	Applications  publicVacancyApplicationStats `json:"applications"`
}

func registerPublicVacancyRoutes() {
	http.HandleFunc("/api/public/vacancies/", publicVacancy)
}

func publicVacancy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
		return
	}
	id, err := strconv.ParseInt(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/public/vacancies/"), "/"), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, "Некорректная вакансия")
		return
	}
	view, err := loadPublicVacancy(r, id)
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusNotFound, "Вакансия не найдена или снята с публикации")
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить вакансию")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(view)
}

func loadPublicVacancy(r *http.Request, id int64) (*publicVacancyView, error) {
	view := &publicVacancyView{Blocks: []publicVacancyBlock{}, Duties: []publicVacancyDutyGroup{}, Tests: []publicVacancyTest{}}
	var salaryFrom, salaryTo sql.NullFloat64
	err := db.QueryRowContext(r.Context(), `SELECT v.id,v.description,v.salary_from,v.salary_to,v.salary_tax_mode,v.currency,v.city,v.address,u.full_name,v.published_at
		FROM vacancies v JOIN users u ON u.id=v.user_id
		WHERE v.id=$1 AND v.status='published' AND v.deleted_at IS NULL`, id).
		Scan(&view.ID, &view.Description, &salaryFrom, &salaryTo, &view.SalaryTaxMode, &view.Currency, &view.City, &view.Address, &view.OwnerName, &view.PublishedAt)
	if err != nil {
		return nil, err
	}
	if salaryFrom.Valid {
		view.SalaryFrom = &salaryFrom.Float64
	}
	if salaryTo.Valid {
		view.SalaryTo = &salaryTo.Float64
	}
	if err = loadPublicVacancyBlocks(r, view); err != nil {
		return nil, err
	}
	if err = loadPublicVacancyDuties(r, view); err != nil {
		return nil, err
	}
	if err = loadPublicVacancyTests(r, view); err != nil {
		return nil, err
	}
	if err = loadPublicVacancyApplicationStats(r, view); err != nil {
		return nil, err
	}
	return view, nil
}

func loadPublicVacancyBlocks(r *http.Request, view *publicVacancyView) error {
	rows, err := db.QueryContext(r.Context(), `SELECT b.id,b.name,b.sort_order,d.id,COALESCE(NULLIF(d.vacancy_title,''),d.name),COALESCE(d.alias,''),COALESCE(d.icon,''),COALESCE(bd.selection_color,'blue'),i.id,i.value,i.comment,COALESCE(i.icon,''),vc.importance
		FROM vacancy_categories vc
		JOIN vacancy_survey_blocks b ON b.id=vc.block_id
		JOIN dictionary_items i ON i.id=vc.category_id
		JOIN dictionaries d ON d.id=i.dictionary_id
		LEFT JOIN vacancy_survey_block_dictionaries bd ON bd.block_id=b.id AND bd.dictionary_id=d.id
		WHERE vc.vacancy_id=$1
		ORDER BY b.sort_order,b.id,bd.sort_order,d.id,vc.sort_order,vc.id`, view.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	blockIndex := map[int64]int{}
	dictionaryIndex := map[[2]int64]int{}
	for rows.Next() {
		var blockID, dictionaryID int64
		var blockName, dictionaryName, alias, dictionaryIcon, color string
		var blockOrder int
		var item publicVacancyItem
		if err = rows.Scan(&blockID, &blockName, &blockOrder, &dictionaryID, &dictionaryName, &alias, &dictionaryIcon, &color, &item.ID, &item.Value, &item.Comment, &item.Icon, &item.Importance); err != nil {
			return err
		}
		bi, ok := blockIndex[blockID]
		if !ok {
			bi = len(view.Blocks)
			blockIndex[blockID] = bi
			view.Blocks = append(view.Blocks, publicVacancyBlock{ID: blockID, Name: blockName, SortOrder: blockOrder, Dictionaries: []publicVacancyDictionary{}})
		}
		key := [2]int64{blockID, dictionaryID}
		di, ok := dictionaryIndex[key]
		if !ok {
			di = len(view.Blocks[bi].Dictionaries)
			dictionaryIndex[key] = di
			view.Blocks[bi].Dictionaries = append(view.Blocks[bi].Dictionaries, publicVacancyDictionary{ID: dictionaryID, Name: dictionaryName, Alias: alias, Icon: dictionaryIcon, SelectionColor: color, Items: []publicVacancyItem{}})
		}
		view.Blocks[bi].Dictionaries[di].Items = append(view.Blocks[bi].Dictionaries[di].Items, item)
	}
	return rows.Err()
}

func loadPublicVacancyDuties(r *http.Request, view *publicVacancyView) error {
	rows, err := db.QueryContext(r.Context(), `SELECT c.id,c.name,c.icon,d.name
		FROM vacancy_duties vd JOIN duties d ON d.id=vd.duty_id JOIN duty_categories c ON c.id=d.category_id
		WHERE vd.vacancy_id=$1 ORDER BY c.sort_order,c.id,d.sort_order,d.id`, view.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	index := map[int64]int{}
	for rows.Next() {
		var categoryID int64
		var categoryName, icon, dutyName string
		if err = rows.Scan(&categoryID, &categoryName, &icon, &dutyName); err != nil {
			return err
		}
		position, ok := index[categoryID]
		if !ok {
			position = len(view.Duties)
			index[categoryID] = position
			view.Duties = append(view.Duties, publicVacancyDutyGroup{ID: categoryID, Name: categoryName, Icon: icon, Duties: []string{}})
		}
		view.Duties[position].Duties = append(view.Duties[position].Duties, dutyName)
	}
	return rows.Err()
}

func loadPublicVacancyTests(r *http.Request, view *publicVacancyView) error {
	rows, err := db.QueryContext(r.Context(), `SELECT t.id,tv.title,tv.description,COALESCE(t.time_limit_seconds,0),
		(SELECT COUNT(*) FROM test_questions q WHERE q.test_version_id=tv.id),
		COALESCE((SELECT AVG(tr.rating) FROM test_reviews tr WHERE tr.test_id=t.id),0),
		COALESCE((SELECT COUNT(*) FROM test_reviews tr WHERE tr.test_id=t.id),0),t.is_free,t.price,u.full_name
		FROM vacancy_tests vt JOIN tests t ON t.id=vt.test_id JOIN test_versions tv ON tv.id=vt.test_version_id JOIN users u ON u.id=t.author_id
		WHERE vt.vacancy_external_id=$1 ORDER BY vt.sort_order,vt.id`, view.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var test publicVacancyTest
		if err = rows.Scan(&test.ID, &test.Title, &test.Description, &test.TimeLimitSeconds, &test.QuestionCount, &test.Rating, &test.ReviewCount, &test.IsFree, &test.Price, &test.Author); err != nil {
			return err
		}
		view.Tests = append(view.Tests, test)
	}
	return rows.Err()
}

func loadPublicVacancyApplicationStats(r *http.Request, view *publicVacancyView) error {
	return db.QueryRowContext(r.Context(), `WITH required_tests AS (
			SELECT DISTINCT test_id FROM vacancy_tests WHERE vacancy_external_id=$1
		), candidates AS (
			SELECT a.user_id,COUNT(DISTINCT a.test_id) FILTER(WHERE a.status='finished' AND a.passed=TRUE) passed_count
			FROM test_attempts a JOIN required_tests rt ON rt.test_id=a.test_id GROUP BY a.user_id
		), totals AS (
			SELECT COUNT(*)::int total,
				COUNT(*) FILTER(WHERE passed_count=(SELECT COUNT(*) FROM required_tests))::int passed
			FROM candidates
		)
		SELECT total,passed,total-passed FROM totals`, view.ID).
		Scan(&view.Applications.Total, &view.Applications.Passed, &view.Applications.NotPassed)
}
