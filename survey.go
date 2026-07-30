package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

type surveyBlock struct {
	ID                       int64              `json:"id"`
	Name                     string             `json:"name"`
	Order                    int                `json:"order"`
	ShowDictionariesTogether bool               `json:"show_dictionaries_together"`
	ShowDictionaryIcon       bool               `json:"show_dictionary_icon"`
	PlainAnswerText          bool               `json:"plain_answer_text"`
	ColumnsPerRow            int                `json:"columns_per_row,omitempty"`
	SelectionColor           string             `json:"selection_color"`
	Dictionaries             []surveyDictionary `json:"dictionaries"`
}

type surveyDictionary struct {
	ID             int64                  `json:"id"`
	Name           string                 `json:"name"`
	Alias          string                 `json:"alias"`
	Icon           string                 `json:"icon"`
	SingleChoice   bool                   `json:"single_choice"`
	SelectionColor string                 `json:"selection_color"`
	Items          []publicDictionaryItem `json:"items,omitempty"`
}

type surveyBlockInput struct {
	Name                     string           `json:"name"`
	DictionaryIDs            []int64          `json:"dictionary_ids"`
	ShowDictionariesTogether bool             `json:"show_dictionaries_together"`
	ShowDictionaryIcon       bool             `json:"show_dictionary_icon"`
	PlainAnswerText          bool             `json:"plain_answer_text"`
	ColumnsPerRow            int              `json:"columns_per_row"`
	SelectionColor           string           `json:"selection_color"`
	DictionaryColors         map[int64]string `json:"dictionary_colors"`
}

func prepareApplicantSurveyDatabase(ctx context.Context) error {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS applicant_survey_blocks (
		id BIGSERIAL PRIMARY KEY,
		name VARCHAR(200) NOT NULL,
		sort_order INTEGER NOT NULL DEFAULT 0,
		columns_per_row INTEGER NOT NULL DEFAULT 4,
		selection_color VARCHAR(20) NOT NULL DEFAULT 'blue',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	ALTER TABLE applicant_survey_blocks ADD COLUMN IF NOT EXISTS columns_per_row INTEGER NOT NULL DEFAULT 4;
	ALTER TABLE applicant_survey_blocks ADD COLUMN IF NOT EXISTS selection_color VARCHAR(20) NOT NULL DEFAULT 'blue';
	ALTER TABLE applicant_survey_blocks ADD COLUMN IF NOT EXISTS show_dictionaries_together BOOLEAN NOT NULL DEFAULT FALSE;
	ALTER TABLE applicant_survey_blocks ADD COLUMN IF NOT EXISTS show_dictionary_icon BOOLEAN NOT NULL DEFAULT FALSE;
	ALTER TABLE applicant_survey_blocks ADD COLUMN IF NOT EXISTS plain_answer_text BOOLEAN NOT NULL DEFAULT FALSE;
	CREATE TABLE IF NOT EXISTS applicant_survey_block_dictionaries (
		block_id BIGINT NOT NULL REFERENCES applicant_survey_blocks(id) ON DELETE CASCADE,
		dictionary_id BIGINT NOT NULL REFERENCES dictionaries(id) ON DELETE RESTRICT,
		sort_order INTEGER NOT NULL DEFAULT 0,
		selection_color VARCHAR(20) NOT NULL DEFAULT 'blue',
		PRIMARY KEY(block_id, dictionary_id)
	);
	ALTER TABLE applicant_survey_block_dictionaries ADD COLUMN IF NOT EXISTS selection_color VARCHAR(20) NOT NULL DEFAULT 'blue';`)
	if err != nil {
		return err
	}
	var count int
	if err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM applicant_survey_blocks`).Scan(&count); err != nil || count > 0 {
		return err
	}
	_, err = db.ExecContext(ctx, `WITH first_block AS (
		INSERT INTO applicant_survey_blocks(name,sort_order) VALUES('Желаемая должность',0) RETURNING id
	), second_block AS (
		INSERT INTO applicant_survey_blocks(name,sort_order) VALUES('Профессиональные навыки',1) RETURNING id
	)
	INSERT INTO applicant_survey_block_dictionaries(block_id,dictionary_id,sort_order)
	SELECT first_block.id,d.id,0 FROM first_block JOIN dictionaries d ON d.alias='position'
	UNION ALL
	SELECT second_block.id,d.id,0 FROM second_block JOIN dictionaries d ON d.alias='accounting_areas'`)
	return err
}

func registerApplicantSurveyRoutes() {
	http.HandleFunc("/api/admin/survey-blocks", adminSurveyBlocks)
	http.HandleFunc("/api/admin/survey-blocks/", adminSurveyBlock)
	http.HandleFunc("/api/public/resume-survey", publicResumeSurvey)
}

func adminSurveyBlocks(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		blocks, err := loadSurveyBlocks(r.Context(), false)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить блоки опроса")
			return
		}
		writeAdminJSON(w, http.StatusOK, blocks)
	case http.MethodPost:
		var input surveyBlockInput
		if !decodeSurveyJSON(w, r, &input) {
			return
		}
		if !validSurveyBlockInput(input) {
			writeJSON(w, http.StatusBadRequest, "Укажите название и хотя бы один справочник")
			return
		}
		id, err := createSurveyBlock(r.Context(), input)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, "Не удалось создать блок опроса")
			return
		}
		writeAdminJSON(w, http.StatusCreated, map[string]int64{"id": id})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
	}
}

func adminSurveyBlock(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	tail := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/survey-blocks/"), "/")
	if tail == "reorder" {
		reorderSurveyBlocks(w, r)
		return
	}
	id, err := strconv.ParseInt(tail, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, "Некорректный блок опроса")
		return
	}
	switch r.Method {
	case http.MethodPut:
		var input surveyBlockInput
		if !decodeSurveyJSON(w, r, &input) {
			return
		}
		if !validSurveyBlockInput(input) {
			writeJSON(w, http.StatusBadRequest, "Укажите название и хотя бы один справочник")
			return
		}
		if err := updateSurveyBlock(r.Context(), id, input); err != nil {
			writeJSON(w, http.StatusBadRequest, "Не удалось сохранить блок опроса")
			return
		}
		writeJSON(w, http.StatusOK, "Блок опроса сохранён")
	case http.MethodDelete:
		if _, err := db.ExecContext(r.Context(), `DELETE FROM applicant_survey_blocks WHERE id=$1`, id); err != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось удалить блок опроса")
			return
		}
		writeJSON(w, http.StatusOK, "Блок опроса удалён")
	default:
		writeJSON(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
	}
}

func reorderSurveyBlocks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
		return
	}
	var input struct {
		IDs []int64 `json:"ids"`
	}
	if !decodeSurveyJSON(w, r, &input) {
		return
	}
	var count int
	if err := db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM applicant_survey_blocks`).Scan(&count); err != nil || len(input.IDs) != count || hasDuplicateIDs(input.IDs) {
		writeJSON(w, http.StatusBadRequest, "Некорректный порядок блоков")
		return
	}
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, "Не удалось изменить порядок")
		return
	}
	defer tx.Rollback()
	for order, id := range input.IDs {
		result, execErr := tx.ExecContext(r.Context(), `UPDATE applicant_survey_blocks SET sort_order=$1,updated_at=NOW() WHERE id=$2`, order, id)
		if execErr != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось изменить порядок")
			return
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			writeJSON(w, http.StatusBadRequest, "Некорректный порядок блоков")
			return
		}
	}
	if err = tx.Commit(); err != nil {
		writeJSON(w, http.StatusInternalServerError, "Не удалось изменить порядок")
		return
	}
	writeJSON(w, http.StatusOK, "Порядок блоков сохранён")
}

func publicResumeSurvey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
		return
	}
	blocks, err := loadSurveyBlocks(r.Context(), true)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить опрос")
		return
	}
	writeAdminJSON(w, http.StatusOK, blocks)
}

func loadSurveyBlocks(ctx context.Context, withItems bool) ([]surveyBlock, error) {
	rows, err := db.QueryContext(ctx, `SELECT b.id,b.name,b.sort_order,b.show_dictionaries_together,b.show_dictionary_icon,b.plain_answer_text,b.columns_per_row,b.selection_color,d.id,d.name,COALESCE(d.alias,''),COALESCE(d.icon,''),COALESCE(d.single_choice,FALSE),COALESCE(bd.selection_color,'blue')
		FROM applicant_survey_blocks b
		LEFT JOIN applicant_survey_block_dictionaries bd ON bd.block_id=b.id
		LEFT JOIN dictionaries d ON d.id=bd.dictionary_id
		ORDER BY b.sort_order,b.id,bd.sort_order,d.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	blocks := []surveyBlock{}
	indexes := map[int64]int{}
	for rows.Next() {
		var blockID int64
		var name string
		var order int
		var showTogether, showDictionaryIcon, plainAnswerText bool
		var columnsPerRow int
		var selectionColor string
		var dictionaryID sql.NullInt64
		var dictionaryName, alias, dictionaryIcon sql.NullString
		var singleChoice bool
		var dictionaryColor string
		if err := rows.Scan(&blockID, &name, &order, &showTogether, &showDictionaryIcon, &plainAnswerText, &columnsPerRow, &selectionColor, &dictionaryID, &dictionaryName, &alias, &dictionaryIcon, &singleChoice, &dictionaryColor); err != nil {
			return nil, err
		}
		idx, exists := indexes[blockID]
		if !exists {
			idx = len(blocks)
			indexes[blockID] = idx
			blocks = append(blocks, surveyBlock{ID: blockID, Name: name, Order: order, ShowDictionariesTogether: showTogether, ShowDictionaryIcon: showDictionaryIcon, PlainAnswerText: plainAnswerText, ColumnsPerRow: columnsPerRow, SelectionColor: selectionColor, Dictionaries: []surveyDictionary{}})
		}
		if dictionaryID.Valid {
			dictionary := surveyDictionary{ID: dictionaryID.Int64, Name: dictionaryName.String, Alias: alias.String, Icon: dictionaryIcon.String, SingleChoice: singleChoice, SelectionColor: dictionaryColor}
			if withItems {
				items, itemErr := loadPublicDictionaryItems(ctx, dictionary.ID)
				if itemErr != nil {
					return nil, itemErr
				}
				dictionary.Items = items
			}
			blocks[idx].Dictionaries = append(blocks[idx].Dictionaries, dictionary)
		}
	}
	return blocks, rows.Err()
}

func loadPublicDictionaryItems(ctx context.Context, dictionaryID int64) ([]publicDictionaryItem, error) {
	rows, err := db.QueryContext(ctx, `SELECT id,value,comment,icon FROM dictionary_items WHERE dictionary_id=$1 AND active=TRUE AND deleted_at IS NULL ORDER BY sort_order,id`, dictionaryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []publicDictionaryItem{}
	for rows.Next() {
		var item publicDictionaryItem
		if err := rows.Scan(&item.ID, &item.Value, &item.Comment, &item.Icon); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func createSurveyBlock(ctx context.Context, input surveyBlockInput) (int64, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var id int64
	columnsPerRow := normalizedColumnsPerRow(input)
	selectionColor := normalizedSelectionColor(input.SelectionColor)
	err = tx.QueryRowContext(ctx, `INSERT INTO applicant_survey_blocks(name,sort_order,show_dictionaries_together,show_dictionary_icon,plain_answer_text,columns_per_row,selection_color) VALUES($1,(SELECT COALESCE(MAX(sort_order),-1)+1 FROM applicant_survey_blocks),$2,$3,$4,$5,$6) RETURNING id`, strings.TrimSpace(input.Name), input.ShowDictionariesTogether, input.ShowDictionaryIcon, input.PlainAnswerText, columnsPerRow, selectionColor).Scan(&id)
	if err != nil {
		return 0, err
	}
	if err = saveSurveyBlockDictionaries(ctx, tx, id, input.DictionaryIDs, input.DictionaryColors); err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

func updateSurveyBlock(ctx context.Context, id int64, input surveyBlockInput) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	columnsPerRow := normalizedColumnsPerRow(input)
	selectionColor := normalizedSelectionColor(input.SelectionColor)
	result, err := tx.ExecContext(ctx, `UPDATE applicant_survey_blocks SET name=$1,show_dictionaries_together=$2,show_dictionary_icon=$3,plain_answer_text=$4,columns_per_row=$5,selection_color=$6,updated_at=NOW() WHERE id=$7`, strings.TrimSpace(input.Name), input.ShowDictionariesTogether, input.ShowDictionaryIcon, input.PlainAnswerText, columnsPerRow, selectionColor, id)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return errors.New("survey block not found")
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM applicant_survey_block_dictionaries WHERE block_id=$1`, id); err != nil {
		return err
	}
	if err = saveSurveyBlockDictionaries(ctx, tx, id, input.DictionaryIDs, input.DictionaryColors); err != nil {
		return err
	}
	return tx.Commit()
}

func saveSurveyBlockDictionaries(ctx context.Context, tx *sql.Tx, blockID int64, ids []int64, colors map[int64]string) error {
	if hasDuplicateIDs(ids) {
		return errors.New("duplicate dictionary")
	}
	for order, id := range ids {
		result, err := tx.ExecContext(ctx, `INSERT INTO applicant_survey_block_dictionaries(block_id,dictionary_id,sort_order,selection_color)
			SELECT $1,id,$3,$4 FROM dictionaries WHERE id=$2`, blockID, id, order, normalizedSelectionColor(colors[id]))
		if err != nil {
			return err
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			return errors.New("dictionary not found")
		}
	}
	return nil
}

func decodeSurveyJSON(w http.ResponseWriter, r *http.Request, value any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		writeJSON(w, http.StatusBadRequest, "Некорректные данные")
		return false
	}
	return true
}

func validSurveyBlockInput(input surveyBlockInput) bool {
	name := strings.TrimSpace(input.Name)
	return name != "" && len([]rune(name)) <= 200 && len(input.DictionaryIDs) > 0 && !hasDuplicateIDs(input.DictionaryIDs)
}

func hasDuplicateIDs(ids []int64) bool {
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return true
		}
		if _, exists := seen[id]; exists {
			return true
		}
		seen[id] = struct{}{}
	}
	return false
}
