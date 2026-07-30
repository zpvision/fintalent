package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

func prepareVacancySurveyDatabase(ctx context.Context) error {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS vacancy_survey_blocks (
		id BIGSERIAL PRIMARY KEY,
		name VARCHAR(200) NOT NULL,
		sort_order INTEGER NOT NULL DEFAULT 0,
		show_dictionaries_together BOOLEAN NOT NULL DEFAULT FALSE,
		show_dictionary_icon BOOLEAN NOT NULL DEFAULT FALSE,
		plain_answer_text BOOLEAN NOT NULL DEFAULT FALSE,
		columns_per_row INTEGER NOT NULL DEFAULT 4,
		selection_color VARCHAR(20) NOT NULL DEFAULT 'blue',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	ALTER TABLE vacancy_survey_blocks ADD COLUMN IF NOT EXISTS columns_per_row INTEGER NOT NULL DEFAULT 4;
	ALTER TABLE vacancy_survey_blocks ADD COLUMN IF NOT EXISTS show_dictionary_icon BOOLEAN NOT NULL DEFAULT FALSE;
	ALTER TABLE vacancy_survey_blocks ADD COLUMN IF NOT EXISTS plain_answer_text BOOLEAN NOT NULL DEFAULT FALSE;
	ALTER TABLE vacancy_survey_blocks ADD COLUMN IF NOT EXISTS selection_color VARCHAR(20) NOT NULL DEFAULT 'blue';
	CREATE TABLE IF NOT EXISTS vacancy_survey_block_dictionaries (
		block_id BIGINT NOT NULL REFERENCES vacancy_survey_blocks(id) ON DELETE CASCADE,
		dictionary_id BIGINT NOT NULL REFERENCES dictionaries(id) ON DELETE RESTRICT,
		sort_order INTEGER NOT NULL DEFAULT 0,
		selection_color VARCHAR(20) NOT NULL DEFAULT 'blue',
		PRIMARY KEY(block_id, dictionary_id)
	);
	ALTER TABLE vacancy_survey_block_dictionaries ADD COLUMN IF NOT EXISTS selection_color VARCHAR(20) NOT NULL DEFAULT 'blue';`)
	return err
}

func registerVacancySurveyRoutes() {
	http.HandleFunc("/api/admin/vacancy-survey-blocks", adminVacancySurveyBlocks)
	http.HandleFunc("/api/admin/vacancy-survey-blocks/", adminVacancySurveyBlock)
	http.HandleFunc("/api/public/vacancy-survey", publicVacancySurvey)
}

func adminVacancySurveyBlocks(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		blocks, err := loadVacancySurveyBlocks(r.Context(), false)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить блоки создания вакансии")
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
		id, err := createVacancySurveyBlock(r.Context(), input)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, "Не удалось создать блок")
			return
		}
		writeAdminJSON(w, http.StatusCreated, map[string]int64{"id": id})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
	}
}

func adminVacancySurveyBlock(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	tail := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/vacancy-survey-blocks/"), "/")
	if tail == "reorder" {
		reorderVacancySurveyBlocks(w, r)
		return
	}
	id, err := strconv.ParseInt(tail, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, "Некорректный блок")
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
		if err := updateVacancySurveyBlock(r.Context(), id, input); err != nil {
			writeJSON(w, http.StatusBadRequest, "Не удалось сохранить блок")
			return
		}
		writeJSON(w, http.StatusOK, "Блок сохранён")
	case http.MethodDelete:
		if _, err := db.ExecContext(r.Context(), `DELETE FROM vacancy_survey_blocks WHERE id=$1`, id); err != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось удалить блок")
			return
		}
		writeJSON(w, http.StatusOK, "Блок удалён")
	default:
		writeJSON(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
	}
}

func reorderVacancySurveyBlocks(w http.ResponseWriter, r *http.Request) {
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
	if err := db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM vacancy_survey_blocks`).Scan(&count); err != nil || len(input.IDs) != count || hasDuplicateIDs(input.IDs) {
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
		result, execErr := tx.ExecContext(r.Context(), `UPDATE vacancy_survey_blocks SET sort_order=$1,updated_at=NOW() WHERE id=$2`, order, id)
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

func publicVacancySurvey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
		return
	}
	blocks, err := loadVacancySurveyBlocks(r.Context(), true)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить настройку вакансии")
		return
	}
	writeAdminJSON(w, http.StatusOK, blocks)
}

func loadVacancySurveyBlocks(ctx context.Context, withItems bool) ([]surveyBlock, error) {
	rows, err := db.QueryContext(ctx, `SELECT b.id,b.name,b.sort_order,b.show_dictionaries_together,b.show_dictionary_icon,b.plain_answer_text,b.columns_per_row,b.selection_color,d.id,COALESCE(NULLIF(d.vacancy_title,''),d.name),COALESCE(d.alias,''),COALESCE(d.icon,''),COALESCE(d.single_choice,FALSE),COALESCE(bd.selection_color,'blue')
		FROM vacancy_survey_blocks b
		LEFT JOIN vacancy_survey_block_dictionaries bd ON bd.block_id=b.id
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
		var showTogether bool
		var showDictionaryIcon bool
		var plainAnswerText bool
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

func createVacancySurveyBlock(ctx context.Context, input surveyBlockInput) (int64, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var id int64
	columnsPerRow := normalizedColumnsPerRow(input)
	selectionColor := normalizedSelectionColor(input.SelectionColor)
	err = tx.QueryRowContext(ctx, `INSERT INTO vacancy_survey_blocks(name,sort_order,show_dictionaries_together,show_dictionary_icon,plain_answer_text,columns_per_row,selection_color) VALUES($1,(SELECT COALESCE(MAX(sort_order),-1)+1 FROM vacancy_survey_blocks),$2,$3,$4,$5,$6) RETURNING id`, strings.TrimSpace(input.Name), input.ShowDictionariesTogether, input.ShowDictionaryIcon, input.PlainAnswerText, columnsPerRow, selectionColor).Scan(&id)
	if err != nil {
		return 0, err
	}
	if err = saveVacancySurveyBlockDictionaries(ctx, tx, id, input.DictionaryIDs, input.DictionaryColors); err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

func updateVacancySurveyBlock(ctx context.Context, id int64, input surveyBlockInput) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	columnsPerRow := normalizedColumnsPerRow(input)
	selectionColor := normalizedSelectionColor(input.SelectionColor)
	result, err := tx.ExecContext(ctx, `UPDATE vacancy_survey_blocks SET name=$1,show_dictionaries_together=$2,show_dictionary_icon=$3,plain_answer_text=$4,columns_per_row=$5,selection_color=$6,updated_at=NOW() WHERE id=$7`, strings.TrimSpace(input.Name), input.ShowDictionariesTogether, input.ShowDictionaryIcon, input.PlainAnswerText, columnsPerRow, selectionColor, id)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return errors.New("vacancy survey block not found")
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM vacancy_survey_block_dictionaries WHERE block_id=$1`, id); err != nil {
		return err
	}
	if err = saveVacancySurveyBlockDictionaries(ctx, tx, id, input.DictionaryIDs, input.DictionaryColors); err != nil {
		return err
	}
	return tx.Commit()
}

func normalizedColumnsPerRow(input surveyBlockInput) int {
	if len(input.DictionaryIDs) != 1 || input.ColumnsPerRow < 1 || input.ColumnsPerRow > 6 {
		return 4
	}
	return input.ColumnsPerRow
}

func normalizedSelectionColor(value string) string {
	switch value {
	case "green", "blue", "violet", "orange", "rose", "teal":
		return value
	default:
		return "blue"
	}
}

func saveVacancySurveyBlockDictionaries(ctx context.Context, tx *sql.Tx, blockID int64, ids []int64, colors map[int64]string) error {
	if hasDuplicateIDs(ids) {
		return errors.New("duplicate dictionary")
	}
	for order, id := range ids {
		result, err := tx.ExecContext(ctx, `INSERT INTO vacancy_survey_block_dictionaries(block_id,dictionary_id,sort_order,selection_color)
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
