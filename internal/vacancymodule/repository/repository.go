package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"FinTalent/internal/vacancymodule/domain"
)

var (
	ErrNotFound  = errors.New("not found")
	ErrForbidden = errors.New("forbidden")
)

type Postgres struct{ db *sql.DB }

func New(db *sql.DB) *Postgres { return &Postgres{db: db} }

func (p *Postgres) Builder(ctx context.Context) ([]domain.BuilderBlock, error) {
	rows, err := p.db.QueryContext(ctx, `SELECT b.id,b.name,b.sort_order,b.show_dictionaries_together,b.show_dictionary_icon,b.plain_answer_text,b.columns_per_row,d.id,COALESCE(NULLIF(d.vacancy_title,''),d.name),COALESCE(d.alias,''),COALESCE(d.icon,''),d.use_importance_in_vacancy,d.single_choice,COALESCE(bd.selection_color,'blue'),i.id,i.value,i.comment,i.icon
		FROM vacancy_survey_blocks b
		LEFT JOIN vacancy_survey_block_dictionaries bd ON bd.block_id=b.id
		LEFT JOIN dictionaries d ON d.id=bd.dictionary_id
		LEFT JOIN dictionary_items i ON i.dictionary_id=d.id AND i.active=TRUE AND i.deleted_at IS NULL
		ORDER BY b.sort_order,b.id,bd.sort_order,d.id,i.sort_order,i.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	blocks := []domain.BuilderBlock{}
	blockIndexes := map[int64]int{}
	dictionaryIndexes := map[[2]int64]int{}
	for rows.Next() {
		var blockID int64
		var blockName string
		var blockOrder int
		var showTogether bool
		var showDictionaryIcon bool
		var plainAnswerText bool
		var columnsPerRow int
		var dictionaryID, itemID sql.NullInt64
		var dictionaryName, alias, dictionaryIcon, value, comment, icon sql.NullString
		var useImportance sql.NullBool
		var singleChoice sql.NullBool
		var selectionColor string
		if err := rows.Scan(&blockID, &blockName, &blockOrder, &showTogether, &showDictionaryIcon, &plainAnswerText, &columnsPerRow, &dictionaryID, &dictionaryName, &alias, &dictionaryIcon, &useImportance, &singleChoice, &selectionColor, &itemID, &value, &comment, &icon); err != nil {
			return nil, err
		}
		bi, ok := blockIndexes[blockID]
		if !ok {
			bi = len(blocks)
			blockIndexes[blockID] = bi
			blocks = append(blocks, domain.BuilderBlock{ID: blockID, Name: blockName, SortOrder: blockOrder, ShowDictionariesTogether: showTogether, ShowDictionaryIcon: showDictionaryIcon, PlainAnswerText: plainAnswerText, ColumnsPerRow: columnsPerRow, Dictionaries: []domain.BuilderDictionary{}})
		}
		if dictionaryID.Valid {
			key := [2]int64{blockID, dictionaryID.Int64}
			di, exists := dictionaryIndexes[key]
			if !exists {
				di = len(blocks[bi].Dictionaries)
				dictionaryIndexes[key] = di
				blocks[bi].Dictionaries = append(blocks[bi].Dictionaries, domain.BuilderDictionary{ID: dictionaryID.Int64, Name: dictionaryName.String, Alias: alias.String, Icon: dictionaryIcon.String, UseImportanceInVacancy: useImportance.Bool, SingleChoice: singleChoice.Bool, SelectionColor: selectionColor, Items: []domain.BuilderItem{}})
			}
			if itemID.Valid {
				blocks[bi].Dictionaries[di].Items = append(blocks[bi].Dictionaries[di].Items, domain.BuilderItem{ID: itemID.Int64, Value: value.String, Comment: comment.String, Icon: icon.String})
			}
		}
	}
	return blocks, rows.Err()
}

func (p *Postgres) Metadata(ctx context.Context, pairs [][2]int64) (map[[2]int64]domain.CategoryMetadata, error) {
	if len(pairs) == 0 {
		return map[[2]int64]domain.CategoryMetadata{}, nil
	}
	clauses := make([]string, 0, len(pairs))
	args := make([]any, 0, len(pairs)*2)
	for _, pair := range pairs {
		args = append(args, pair[0], pair[1])
		clauses = append(clauses, fmt.Sprintf("(i.id=$%d AND b.id=$%d)", len(args)-1, len(args)))
	}
	query := `SELECT i.id,b.id,d.id,i.value,d.use_importance_in_vacancy,d.single_choice,i.active AND i.deleted_at IS NULL
		FROM dictionary_items i JOIN dictionaries d ON d.id=i.dictionary_id
		JOIN vacancy_survey_block_dictionaries bd ON bd.dictionary_id=d.id
		JOIN vacancy_survey_blocks b ON b.id=bd.block_id WHERE ` + strings.Join(clauses, " OR ")
	rows, err := p.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[[2]int64]domain.CategoryMetadata{}
	for rows.Next() {
		var m domain.CategoryMetadata
		if err := rows.Scan(&m.CategoryID, &m.BlockID, &m.DictionaryID, &m.Name, &m.UseImportanceInVacancy, &m.SingleChoice, &m.Active); err != nil {
			return nil, err
		}
		out[[2]int64{m.CategoryID, m.BlockID}] = m
	}
	return out, rows.Err()
}

func (p *Postgres) Create(ctx context.Context, user int64) (int64, error) {
	var id int64
	err := p.db.QueryRowContext(ctx, `INSERT INTO vacancies(user_id) VALUES($1) RETURNING id`, user).Scan(&id)
	return id, err
}

func (p *Postgres) List(ctx context.Context, user int64) ([]*domain.Vacancy, error) {
	rows, err := p.db.QueryContext(ctx, `SELECT v.id,v.user_id,
		COALESCE((SELECT STRING_AGG(vc.category_name_snapshot, ', ' ORDER BY vc.sort_order,vc.id) FROM vacancy_categories vc WHERE vc.vacancy_id=v.id AND vc.block_id=(SELECT b.id FROM vacancy_survey_blocks b ORDER BY b.sort_order,b.id LIMIT 1)),NULLIF(v.title,''),'Вакансия'),
		v.description,v.status,v.salary_from,v.salary_to,v.salary_tax_mode,v.currency,v.employment_type,v.work_format,v.city,v.address,v.accepts_individual_entrepreneur,v.accepts_self_employed,v.experience_from,v.experience_to,v.current_step,
		(SELECT test_id FROM vacancy_tests WHERE vacancy_external_id=v.id ORDER BY sort_order,id LIMIT 1),v.published_at,v.created_at,v.updated_at
		FROM vacancies v WHERE v.user_id=$1 AND v.deleted_at IS NULL ORDER BY v.updated_at DESC,v.id DESC`, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*domain.Vacancy{}
	for rows.Next() {
		v, scanErr := scanVacancy(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func scanVacancy(scanner interface{ Scan(...any) error }) (*domain.Vacancy, error) {
	var v domain.Vacancy
	var salaryFrom, salaryTo sql.NullFloat64
	var expFrom, expTo sql.NullInt64
	var published sql.NullTime
	var selectedTest sql.NullInt64
	err := scanner.Scan(&v.ID, &v.UserID, &v.Title, &v.Description, &v.Status, &salaryFrom, &salaryTo, &v.SalaryTaxMode, &v.Currency, &v.EmploymentType, &v.WorkFormat, &v.City, &v.Address, &v.AcceptsIndividualEntrepreneur, &v.AcceptsSelfEmployed, &expFrom, &expTo, &v.CurrentStep, &selectedTest, &published, &v.CreatedAt, &v.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if salaryFrom.Valid {
		v.SalaryFrom = &salaryFrom.Float64
	}
	if salaryTo.Valid {
		v.SalaryTo = &salaryTo.Float64
	}
	if expFrom.Valid {
		x := int(expFrom.Int64)
		v.ExperienceFrom = &x
	}
	if selectedTest.Valid {
		v.SelectedTestID = &selectedTest.Int64
	}
	if expTo.Valid {
		x := int(expTo.Int64)
		v.ExperienceTo = &x
	}
	if published.Valid {
		v.PublishedAt = &published.Time
	}
	v.Requirements = []domain.Requirement{}
	v.DutyIDs = []int64{}
	v.SelectedTestIDs = []int64{}
	return &v, nil
}

func (p *Postgres) Get(ctx context.Context, id, user int64) (*domain.Vacancy, error) {
	v, err := scanVacancy(p.db.QueryRowContext(ctx, `SELECT id,user_id,title,description,status,salary_from,salary_to,salary_tax_mode,currency,employment_type,work_format,city,address,accepts_individual_entrepreneur,accepts_self_employed,experience_from,experience_to,current_step,(SELECT test_id FROM vacancy_tests WHERE vacancy_external_id=vacancies.id ORDER BY sort_order,id LIMIT 1),published_at,created_at,updated_at FROM vacancies WHERE id=$1 AND deleted_at IS NULL`, id))
	if err != nil {
		return nil, err
	}
	if v.UserID != user {
		return nil, ErrForbidden
	}
	rows, err := p.db.QueryContext(ctx, `SELECT vc.category_id,COALESCE(vc.block_id,0),i.dictionary_id,vc.importance,vc.sort_order,vc.category_name_snapshot FROM vacancy_categories vc JOIN dictionary_items i ON i.id=vc.category_id WHERE vc.vacancy_id=$1 ORDER BY vc.sort_order,vc.id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var r domain.Requirement
		if err := rows.Scan(&r.CategoryID, &r.BlockID, &r.DictionaryID, &r.Importance, &r.SortOrder, &r.CategoryName); err != nil {
			return nil, err
		}
		v.Requirements = append(v.Requirements, r)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	testRows, err := p.db.QueryContext(ctx, `SELECT test_id FROM vacancy_tests WHERE vacancy_external_id=$1 ORDER BY sort_order,id`, id)
	if err != nil {
		return nil, err
	}
	for testRows.Next() {
		var testID int64
		if err = testRows.Scan(&testID); err != nil {
			testRows.Close()
			return nil, err
		}
		v.SelectedTestIDs = append(v.SelectedTestIDs, testID)
	}
	if err = testRows.Err(); err != nil {
		testRows.Close()
		return nil, err
	}
	testRows.Close()
	if len(v.SelectedTestIDs) > 0 {
		v.SelectedTestID = &v.SelectedTestIDs[0]
	}
	dutyRows, err := p.db.QueryContext(ctx, `SELECT duty_id FROM vacancy_duties WHERE vacancy_id=$1 ORDER BY id`, id)
	if err != nil {
		return nil, err
	}
	defer dutyRows.Close()
	for dutyRows.Next() {
		var dutyID int64
		if err = dutyRows.Scan(&dutyID); err != nil {
			return nil, err
		}
		v.DutyIDs = append(v.DutyIDs, dutyID)
	}
	return v, dutyRows.Err()
}

func (p *Postgres) VacancyDuties(ctx context.Context, id int64) ([]int64, error) {
	rows, err := p.db.QueryContext(ctx, `SELECT duty_id FROM vacancy_duties WHERE vacancy_id=$1 ORDER BY id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var dutyID int64
		if err = rows.Scan(&dutyID); err != nil {
			return nil, err
		}
		ids = append(ids, dutyID)
	}
	return ids, rows.Err()
}

func (p *Postgres) SaveVacancyDuties(ctx context.Context, id int64, ids []int64) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(702000000000000000::bigint + $1::bigint)`, id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM vacancy_duties WHERE vacancy_id=$1`, id); err != nil {
		return err
	}
	for _, dutyID := range ids {
		result, execErr := tx.ExecContext(ctx, `INSERT INTO vacancy_duties(vacancy_id,duty_id) SELECT $1,d.id FROM duties d JOIN duty_categories c ON c.id=d.category_id WHERE d.id=$2 AND d.is_active=TRUE AND c.is_active=TRUE`, id, dutyID)
		if execErr != nil {
			return execErr
		}
		if n, _ := result.RowsAffected(); n != 1 {
			return errors.New("invalid duty")
		}
	}
	return tx.Commit()
}

func (p *Postgres) Save(ctx context.Context, v *domain.Vacancy, requirements []domain.Requirement, hash string) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(701000000000000000::bigint + $1::bigint)`, v.ID); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `UPDATE vacancies SET title=$1,description=$2,salary_from=$3,salary_to=$4,salary_tax_mode=$5,currency=$6,employment_type=$7,work_format=$8,city=$9,address=$10,accepts_individual_entrepreneur=$11,accepts_self_employed=$12,experience_from=$13,experience_to=$14,current_step=$15,requirements_hash=$16,updated_at=NOW() WHERE id=$17 AND user_id=$18 AND status IN ('draft','published') AND deleted_at IS NULL`, v.Title, v.Description, v.SalaryFrom, v.SalaryTo, v.SalaryTaxMode, v.Currency, v.EmploymentType, v.WorkFormat, v.City, v.Address, v.AcceptsIndividualEntrepreneur, v.AcceptsSelfEmployed, v.ExperienceFrom, v.ExperienceTo, v.CurrentStep, hash, v.ID, v.UserID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return ErrForbidden
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM vacancy_categories WHERE vacancy_id=$1`, v.ID); err != nil {
		return err
	}
	for _, r := range requirements {
		_, err = tx.ExecContext(ctx, `INSERT INTO vacancy_categories(vacancy_id,category_id,block_id,importance,sort_order,category_name_snapshot) VALUES($1,$2,$3,$4,$5,$6)`, v.ID, r.CategoryID, r.BlockID, r.Importance, r.SortOrder, r.CategoryName)
		if err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM vacancy_tests WHERE vacancy_external_id=$1`, v.ID); err != nil {
		return err
	}
	testIDs := v.SelectedTestIDs
	if len(testIDs) == 0 && v.SelectedTestID != nil {
		testIDs = []int64{*v.SelectedTestID}
	}
	for order, testID := range testIDs {
		result, testErr := tx.ExecContext(ctx, `INSERT INTO vacancy_tests(vacancy_external_id,test_id,test_version_id,sort_order,is_required) SELECT $1,t.id,tv.id,$3,TRUE FROM tests t JOIN test_versions tv ON tv.test_id=t.id AND tv.version=t.current_version WHERE t.id=$2 AND t.status='published' AND t.visibility='marketplace'`, v.ID, testID, order)
		if testErr != nil {
			return testErr
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			return errors.New("test unavailable")
		}
	}
	return tx.Commit()
}

func (p *Postgres) SetStatus(ctx context.Context, id, user int64, status string) error {
	res, err := p.db.ExecContext(ctx, `UPDATE vacancies SET status=$1,published_at=CASE WHEN $4='published' THEN COALESCE(published_at,NOW()) ELSE published_at END,updated_at=NOW() WHERE id=$2 AND user_id=$3 AND deleted_at IS NULL`, status, id, user, status)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return ErrForbidden
	}
	return nil
}
func (p *Postgres) Delete(ctx context.Context, id, user int64) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `DELETE FROM vacancy_tests WHERE vacancy_external_id=$1 AND EXISTS(SELECT 1 FROM vacancies WHERE id=$1 AND user_id=$2)`, id, user); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM vacancies WHERE id=$1 AND user_id=$2`, id, user)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return ErrForbidden
	}
	return tx.Commit()
}

func (p *Postgres) SaveResume(ctx context.Context, user int64, step int, categoryIDs []int64) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var resumeID int64
	err = tx.QueryRowContext(ctx, `INSERT INTO resumes(user_id,current_step) VALUES($1,$2) ON CONFLICT(user_id) DO UPDATE SET current_step=EXCLUDED.current_step,updated_at=NOW() RETURNING id`, user, step).Scan(&resumeID)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(703000000000000000::bigint + $1::bigint)`, resumeID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM resume_categories WHERE resume_id=$1`, resumeID); err != nil {
		return err
	}
	for order, id := range categoryIDs {
		_, execErr := tx.ExecContext(ctx, `INSERT INTO resume_categories(resume_id,category_id,block_id,sort_order)
			SELECT $1,i.id,(SELECT MIN(bd.block_id) FROM applicant_survey_block_dictionaries bd WHERE bd.dictionary_id=i.dictionary_id),$3
			FROM dictionary_items i
			WHERE i.id=$2 AND i.active=TRUE AND i.deleted_at IS NULL
				AND EXISTS(SELECT 1 FROM applicant_survey_block_dictionaries bd WHERE bd.dictionary_id=i.dictionary_id)
			ON CONFLICT(resume_id,category_id) DO NOTHING`, resumeID, id, order)
		if execErr != nil {
			return execErr
		}
	}
	return tx.Commit()
}

func (p *Postgres) LoadResume(ctx context.Context, user int64) (int, []int64, error) {
	var id int64
	var step int
	err := p.db.QueryRowContext(ctx, `SELECT id,current_step FROM resumes WHERE user_id=$1 AND deleted_at IS NULL`, user).Scan(&id, &step)
	if err == sql.ErrNoRows {
		return 1, []int64{}, nil
	}
	if err != nil {
		return 0, nil, err
	}
	rows, err := p.db.QueryContext(ctx, `SELECT rc.category_id
		FROM resume_categories rc
		JOIN dictionary_items i ON i.id=rc.category_id
		WHERE rc.resume_id=$1 AND i.active=TRUE AND i.deleted_at IS NULL
			AND EXISTS(SELECT 1 FROM applicant_survey_block_dictionaries bd WHERE bd.dictionary_id=i.dictionary_id)
		ORDER BY rc.sort_order,rc.id`, id)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var x int64
		if err := rows.Scan(&x); err != nil {
			return 0, nil, err
		}
		ids = append(ids, x)
	}
	return step, ids, rows.Err()
}

func (p *Postgres) Preview(ctx context.Context, requirements []domain.Requirement) (*domain.PreviewResult, error) {
	zero := 0.0
	result := &domain.PreviewResult{AverageScore: &zero, ScoreRanges: map[string]int64{"90_100": 0, "80_89": 0, "60_79": 0, "40_59": 0, "0_39": 0}}
	if len(requirements) == 0 {
		if err := p.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM resumes WHERE status='published' AND visibility='public' AND deleted_at IS NULL`).Scan(&result.TotalResumes); err != nil {
			return nil, err
		}
		return result, nil
	}
	values := make([]string, 0, len(requirements))
	args := []any{}
	for _, r := range requirements {
		args = append(args, r.CategoryID, r.BlockID, r.Importance, domain.ImportanceCoefficients[r.Importance])
		values = append(values, fmt.Sprintf("($%d::bigint,$%d::bigint,$%d::text,$%d::int)", len(args)-3, len(args)-2, len(args)-1, len(args)))
	}
	query := `WITH req(category_id,block_id,importance,weight) AS (VALUES ` + strings.Join(values, ",") + `), active_resumes AS (
		SELECT id FROM resumes WHERE status='published' AND visibility='public' AND deleted_at IS NULL), block_scores AS (
		SELECT r.id,req.block_id,COALESCE(SUM(req.weight) FILTER(WHERE rc.category_id IS NOT NULL),0)::numeric/NULLIF(SUM(req.weight),0)*100 score
		FROM active_resumes r CROSS JOIN req LEFT JOIN resume_categories rc ON rc.resume_id=r.id AND rc.category_id=req.category_id
		GROUP BY r.id,req.block_id), scores AS (
		SELECT r.id,NOT EXISTS(SELECT 1 FROM req q WHERE q.importance='required' AND NOT EXISTS(SELECT 1 FROM resume_categories rc WHERE rc.resume_id=r.id AND rc.category_id=q.category_id)) mandatory,AVG(bs.score) score
		FROM active_resumes r JOIN block_scores bs ON bs.id=r.id GROUP BY r.id)
		SELECT COUNT(*),COUNT(*) FILTER(WHERE mandatory),COUNT(*) FILTER(WHERE score>0),AVG(score),
		COUNT(*) FILTER(WHERE score>=90),COUNT(*) FILTER(WHERE score>=80 AND score<90),COUNT(*) FILTER(WHERE score>=60 AND score<80),COUNT(*) FILTER(WHERE score>=40 AND score<60),COUNT(*) FILTER(WHERE score<40) FROM scores`
	var avg sql.NullFloat64
	var range90, range80, range60, range40, range0 int64
	err := p.db.QueryRowContext(ctx, query, args...).Scan(&result.TotalResumes, &result.MandatoryMatched, &result.PartiallyMatched, &avg, &range90, &range80, &range60, &range40, &range0)
	if err != nil {
		return nil, err
	}
	if avg.Valid {
		result.AverageScore = &avg.Float64
	}
	result.ScoreRanges["90_100"], result.ScoreRanges["80_89"] = range90, range80
	result.ScoreRanges["60_79"], result.ScoreRanges["40_59"], result.ScoreRanges["0_39"] = range60, range40, range0
	return result, nil
}
