package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type publicResumeOption struct {
	Value string `json:"value"`
	Icon  string `json:"icon"`
}

type publicResumeDictionary struct {
	Name  string               `json:"name"`
	Alias string               `json:"alias"`
	Icon  string               `json:"icon"`
	Items []publicResumeOption `json:"items"`
}

type publicResumeBlock struct {
	Name         string                   `json:"name"`
	Dictionaries []publicResumeDictionary `json:"dictionaries"`
}

type publicResumeDutyGroup struct {
	Name   string   `json:"name"`
	Icon   string   `json:"icon"`
	Duties []string `json:"duties"`
}

type publicResumeExperience struct {
	Company          string   `json:"company"`
	Position         string   `json:"position"`
	City             string   `json:"city"`
	Industry         string   `json:"industry"`
	StartMonth       int      `json:"start_month"`
	StartYear        int      `json:"start_year"`
	EndMonth         *int     `json:"end_month"`
	EndYear          *int     `json:"end_year"`
	IsCurrent        bool     `json:"is_current"`
	Responsibilities string   `json:"responsibilities"`
	Achievements     string   `json:"achievements"`
	Duties           []string `json:"duties"`
}

type publicResumeEducation struct {
	Type           string `json:"type"`
	Institution    string `json:"institution"`
	Specialization string `json:"specialization"`
	EndYear        *int   `json:"end_year"`
	IsCurrent      bool   `json:"is_current"`
	Description    string `json:"description"`
	Certificate    string `json:"certificate"`
}

type publicResumeLanguage struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type publicResumeView struct {
	ID                   int64                    `json:"id"`
	OwnerID              int64                    `json:"-"`
	IsOwner              bool                     `json:"is_owner"`
	Name                 string                   `json:"name"`
	Avatar               string                   `json:"avatar"`
	DesiredSalary        float64                  `json:"desired_salary"`
	AvailableImmediately bool                     `json:"available_immediately"`
	SearchStatus         string                   `json:"search_status"`
	WorkPreferences      string                   `json:"work_preferences"`
	PublishedAt          time.Time                `json:"published_at"`
	Blocks               []publicResumeBlock      `json:"blocks"`
	Duties               []publicResumeDutyGroup  `json:"duties"`
	Experiences          []publicResumeExperience `json:"experiences"`
	Education            []publicResumeEducation  `json:"education"`
	Languages            []publicResumeLanguage   `json:"languages"`
	Cities               []resumeFinanceCity      `json:"cities"`
	WorkFormats          []resumeFinanceOption    `json:"work_formats"`
}

func publicResumeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
		return
	}
	id, err := strconv.ParseInt(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/public/resumes/"), "/"), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, "Некорректное резюме")
		return
	}
	view, err := loadPublicResume(r, id)
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusNotFound, "Резюме не найдено или не опубликовано")
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить резюме")
		return
	}
	if currentUser, authErr := userFromRequest(r); authErr == nil {
		view.IsOwner = currentUser.ID == view.OwnerID
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(view)
}

func loadPublicResume(r *http.Request, id int64) (*publicResumeView, error) {
	view := &publicResumeView{
		ID: id, Blocks: []publicResumeBlock{}, Duties: []publicResumeDutyGroup{},
		Experiences: []publicResumeExperience{}, Education: []publicResumeEducation{},
		Languages: []publicResumeLanguage{},
		Cities:    []resumeFinanceCity{}, WorkFormats: []resumeFinanceOption{},
	}
	var salary sql.NullFloat64
	var published sql.NullTime
	err := db.QueryRowContext(r.Context(), `
		SELECT r.id,u.id,u.full_name,COALESCE(u.avatar_url,''),r.desired_salary,
			r.available_immediately,COALESCE(s.name,''),COALESCE(r.work_preferences,''),r.published_at
		FROM resumes r
		JOIN users u ON u.id=r.user_id
		LEFT JOIN resume_search_statuses s ON s.code=r.search_status_code
		WHERE r.id=$1 AND r.status='published' AND r.deleted_at IS NULL`,
		id,
	).Scan(&view.ID, &view.OwnerID, &view.Name, &view.Avatar, &salary, &view.AvailableImmediately, &view.SearchStatus, &view.WorkPreferences, &published)
	if err != nil {
		return nil, err
	}
	if view.Avatar == "" {
		view.Avatar = "/static/avatar-placeholder.svg"
	}
	if salary.Valid {
		view.DesiredSalary = salary.Float64
	}
	if published.Valid {
		view.PublishedAt = published.Time
	}
	if err = loadPublicResumeBlocks(r, view); err != nil {
		return nil, err
	}
	if err = loadPublicResumeDuties(r, view); err != nil {
		return nil, err
	}
	if err = loadPublicResumeExperience(r, view); err != nil {
		return nil, err
	}
	if err = loadPublicResumeEducation(r, view); err != nil {
		return nil, err
	}
	if err = loadPublicResumeLanguages(r, view); err != nil {
		return nil, err
	}
	if err = loadPublicResumePreferences(r, view); err != nil {
		return nil, err
	}
	return view, nil
}

func loadPublicResumeBlocks(r *http.Request, view *publicResumeView) error {
	rows, err := db.QueryContext(r.Context(), `
		SELECT b.id,b.name,d.id,COALESCE(NULLIF(d.resume_title,''),d.name),COALESCE(d.alias,''),COALESCE(d.icon,''),
			i.value,COALESCE(i.icon,'')
		FROM resume_categories rc
		JOIN applicant_survey_blocks b ON b.id=rc.block_id
		JOIN dictionary_items i ON i.id=rc.category_id
		JOIN dictionaries d ON d.id=i.dictionary_id
		LEFT JOIN applicant_survey_block_dictionaries bd ON bd.block_id=b.id AND bd.dictionary_id=d.id
		WHERE rc.resume_id=$1
		ORDER BY b.sort_order,b.id,bd.sort_order,d.id,rc.sort_order,rc.id`,
		view.ID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	blockIndexes := map[int64]int{}
	dictionaryIndexes := map[[2]int64]int{}
	for rows.Next() {
		var blockID, dictionaryID int64
		var blockName, dictionaryName, alias, dictionaryIcon string
		var item publicResumeOption
		if err = rows.Scan(&blockID, &blockName, &dictionaryID, &dictionaryName, &alias, &dictionaryIcon, &item.Value, &item.Icon); err != nil {
			return err
		}
		blockIndex, exists := blockIndexes[blockID]
		if !exists {
			blockIndex = len(view.Blocks)
			blockIndexes[blockID] = blockIndex
			view.Blocks = append(view.Blocks, publicResumeBlock{Name: blockName, Dictionaries: []publicResumeDictionary{}})
		}
		key := [2]int64{blockID, dictionaryID}
		dictionaryIndex, exists := dictionaryIndexes[key]
		if !exists {
			dictionaryIndex = len(view.Blocks[blockIndex].Dictionaries)
			dictionaryIndexes[key] = dictionaryIndex
			view.Blocks[blockIndex].Dictionaries = append(view.Blocks[blockIndex].Dictionaries, publicResumeDictionary{
				Name: dictionaryName, Alias: alias, Icon: dictionaryIcon, Items: []publicResumeOption{},
			})
		}
		view.Blocks[blockIndex].Dictionaries[dictionaryIndex].Items = append(view.Blocks[blockIndex].Dictionaries[dictionaryIndex].Items, item)
	}
	return rows.Err()
}

func loadPublicResumeDuties(r *http.Request, view *publicResumeView) error {
	rows, err := db.QueryContext(r.Context(), `
		SELECT c.id,c.name,COALESCE(c.icon,''),d.name
		FROM resume_duties rd
		JOIN duties d ON d.id=rd.duty_id
		JOIN duty_categories c ON c.id=d.category_id
		WHERE rd.resume_id=$1
		ORDER BY c.sort_order,c.id,d.sort_order,d.id`,
		view.ID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	indexes := map[int64]int{}
	for rows.Next() {
		var categoryID int64
		var categoryName, icon, duty string
		if err = rows.Scan(&categoryID, &categoryName, &icon, &duty); err != nil {
			return err
		}
		index, exists := indexes[categoryID]
		if !exists {
			index = len(view.Duties)
			indexes[categoryID] = index
			view.Duties = append(view.Duties, publicResumeDutyGroup{Name: categoryName, Icon: icon, Duties: []string{}})
		}
		view.Duties[index].Duties = append(view.Duties[index].Duties, duty)
	}
	return rows.Err()
}

func loadPublicResumeExperience(r *http.Request, view *publicResumeView) error {
	rows, err := db.QueryContext(r.Context(), `
		SELECT e.id,e.company_name,e.position,e.city,COALESCE(i.value,''),
			e.start_month,e.start_year,e.end_month,e.end_year,e.is_current,
			e.responsibilities,e.achievements
		FROM resume_work_experiences e
		LEFT JOIN dictionary_items i ON i.id=e.industry_item_id
		WHERE e.resume_id=$1 ORDER BY e.sort_order,e.id`,
		view.ID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var id int64
		var item publicResumeExperience
		var endMonth, endYear sql.NullInt64
		if err = rows.Scan(&id, &item.Company, &item.Position, &item.City, &item.Industry, &item.StartMonth, &item.StartYear, &endMonth, &endYear, &item.IsCurrent, &item.Responsibilities, &item.Achievements); err != nil {
			return err
		}
		if endMonth.Valid {
			value := int(endMonth.Int64)
			item.EndMonth = &value
		}
		if endYear.Valid {
			value := int(endYear.Int64)
			item.EndYear = &value
		}
		item.Duties = []string{}
		ids = append(ids, id)
		view.Experiences = append(view.Experiences, item)
	}
	for index, id := range ids {
		dutyRows, dutyErr := db.QueryContext(r.Context(), `
			SELECT d.name FROM resume_work_experience_duties ed
			JOIN duties d ON d.id=ed.duty_id
			WHERE ed.experience_id=$1 ORDER BY ed.sort_order,d.id`, id)
		if dutyErr != nil {
			return dutyErr
		}
		for dutyRows.Next() {
			var name string
			if dutyErr = dutyRows.Scan(&name); dutyErr != nil {
				dutyRows.Close()
				return dutyErr
			}
			view.Experiences[index].Duties = append(view.Experiences[index].Duties, name)
		}
		dutyRows.Close()
	}
	return rows.Err()
}

func loadPublicResumeEducation(r *http.Request, view *publicResumeView) error {
	rows, err := db.QueryContext(r.Context(), `
		SELECT e.education_type,e.institution,e.specialization,e.end_year,e.is_current,
			e.description,COALESCE(c.original_name,'')
		FROM resume_educations e
		LEFT JOIN resume_education_certificates c ON c.id=e.certificate_id
		WHERE e.resume_id=$1 ORDER BY e.sort_order,e.id`,
		view.ID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var item publicResumeEducation
		var endYear sql.NullInt64
		if err = rows.Scan(&item.Type, &item.Institution, &item.Specialization, &endYear, &item.IsCurrent, &item.Description, &item.Certificate); err != nil {
			return err
		}
		if endYear.Valid {
			value := int(endYear.Int64)
			item.EndYear = &value
		}
		view.Education = append(view.Education, item)
	}
	return rows.Err()
}

func loadPublicResumeLanguages(r *http.Request, view *publicResumeView) error {
	rows, err := db.QueryContext(r.Context(), `
		SELECT l.code,l.name FROM resume_languages rl
		JOIN languages l ON l.id=rl.language_id
		WHERE rl.resume_id=$1 ORDER BY rl.sort_order,l.id`,
		view.ID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var item publicResumeLanguage
		if err = rows.Scan(&item.Code, &item.Name); err != nil {
			return err
		}
		view.Languages = append(view.Languages, item)
	}
	return rows.Err()
}

func loadPublicResumePreferences(r *http.Request, view *publicResumeView) error {
	cityRows, err := db.QueryContext(r.Context(), `
		SELECT c.id,c.name,c.region_name
		FROM resume_preferred_cities rpc
		JOIN cities c ON c.id=rpc.city_id
		WHERE rpc.resume_id=$1
		ORDER BY rpc.sort_order,c.id`,
		view.ID,
	)
	if err != nil {
		return err
	}
	defer cityRows.Close()
	for cityRows.Next() {
		var city resumeFinanceCity
		if err = cityRows.Scan(&city.ID, &city.Name, &city.Region); err != nil {
			return err
		}
		view.Cities = append(view.Cities, city)
	}
	if err = cityRows.Err(); err != nil {
		return err
	}

	formatRows, err := db.QueryContext(r.Context(), `
		SELECT i.id,i.value,COALESCE(i.icon,'')
		FROM resume_work_formats rwf
		JOIN dictionary_items i ON i.id=rwf.dictionary_item_id
		WHERE rwf.resume_id=$1
		ORDER BY rwf.sort_order,i.id`,
		view.ID,
	)
	if err != nil {
		return err
	}
	defer formatRows.Close()
	for formatRows.Next() {
		var option resumeFinanceOption
		if err = formatRows.Scan(&option.ID, &option.Value, &option.Icon); err != nil {
			return err
		}
		view.WorkFormats = append(view.WorkFormats, option)
	}
	return formatRows.Err()
}
