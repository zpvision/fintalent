package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type marketplaceTest struct {
	ID                int64      `json:"id"`
	Title             string     `json:"title"`
	Description       string     `json:"description"`
	PositionID        int64      `json:"position_id"`
	Position          string     `json:"position"`
	Category          string     `json:"category"`
	Difficulty        string     `json:"difficulty"`
	IsFree            bool       `json:"is_free"`
	Price             float64    `json:"price"`
	TimeLimitSeconds  int        `json:"time_limit_seconds"`
	QuestionCount     int        `json:"question_count"`
	Rating            float64    `json:"rating"`
	ReviewCount       int        `json:"review_count"`
	AuthorID          int64      `json:"author_id"`
	Author            string     `json:"author"`
	AuthorRating      float64    `json:"author_rating"`
	LastReviewAuthor  string     `json:"last_review_author"`
	LastReviewComment string     `json:"last_review_comment"`
	LastReviewRating  int        `json:"last_review_rating"`
	LastReviewAt      *time.Time `json:"last_review_at,omitempty"`
}

type marketplaceReview struct {
	Rating    int       `json:"rating"`
	Comment   string    `json:"comment"`
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"created_at"`
}

func prepareMarketplaceDatabase(ctx context.Context) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS test_positions (
		test_id BIGINT NOT NULL REFERENCES tests(id) ON DELETE CASCADE,
		position_id BIGINT NOT NULL REFERENCES dictionary_items(id) ON DELETE RESTRICT,
		PRIMARY KEY(test_id,position_id)
	); CREATE INDEX IF NOT EXISTS test_positions_position_idx ON test_positions(position_id,test_id);`); err != nil {
		return err
	}
	if err := seedPositionTests(ctx); err != nil {
		return err
	}
	return seedAccountingTopicTests(ctx)
}

type accountingTopicTestSeed struct {
	Slug        string
	Title       string
	Category    string
	Difficulty  string
	Description string
}

var accountingTopicTestSeeds = []accountingTopicTestSeed{
	{"accounting-topic-vat", "НДС: расчёт, вычеты и отчётность", "Налоговый учёт", "hard", "Практическая проверка знаний по учёту НДС, налоговым вычетам, документам и подготовке декларации."},
	{"accounting-topic-profit-tax", "Налог на прибыль организаций", "Налоговый учёт", "hard", "Проверка знаний о доходах, расходах, налоговой базе, регистрах и контроле расчёта налога на прибыль."},
	{"accounting-topic-payroll", "Расчёт заработной платы", "ТМЦ и зарплата", "medium", "Практический тест по начислению зарплаты, удержаниям, кадровым документам и контролю расчётов с сотрудниками."},
	{"accounting-topic-accounting-basics", "Бухгалтерский учёт: основы и проводки", "Бухгалтерский учёт", "medium", "Проверка понимания двойной записи, первичных документов, счетов учёта и закрытия периода."},
	{"accounting-topic-usn", "УСН: учёт доходов и расходов", "Налоговый учёт", "medium", "Тест по организации налогового учёта при УСН, первичным документам и подготовке отчётности."},
	{"accounting-topic-fixed-assets", "Основные средства и амортизация", "Бухгалтерский учёт", "medium", "Проверка знаний о принятии основных средств к учёту, амортизации, модернизации и выбытии."},
	{"accounting-topic-inventory", "Товарно-материальные ценности", "ТМЦ и зарплата", "medium", "Практические вопросы по поступлению, движению, инвентаризации и списанию товарно-материальных ценностей."},
	{"accounting-topic-cash", "Кассовые операции и подотчёт", "Бухгалтерский учёт", "easy", "Тест по кассовой дисциплине, подотчётным суммам, подтверждающим документам и внутреннему контролю."},
	{"accounting-topic-reporting", "Бухгалтерская отчётность", "Право и отчётность", "hard", "Проверка навыков подготовки, сверки и анализа бухгалтерской отчётности перед сдачей."},
	{"accounting-topic-financial-analysis", "Финансовый анализ компании", "Финансовый анализ", "hard", "Тест по чтению отчётности, оценке ликвидности, рентабельности, денежного потока и финансовых рисков."},
	{"accounting-topic-one-c", "1С:Бухгалтерия — практическая работа", "1С и программы", "medium", "Проверка практических навыков ведения учёта, контроля документов и закрытия периода в 1С:Бухгалтерии."},
	{"accounting-topic-audit", "Внутренний контроль и аудит", "Право и отчётность", "hard", "Проверка знаний о контрольных процедурах, аудиторских доказательствах, рисках и исправлении ошибок."},
	{"accounting-topic-management-accounting", "Управленческий учёт и бюджетирование", "Финансовый анализ", "medium", "Тест по бюджетам, план-факт анализу, центрам ответственности и подготовке управленческой отчётности."},
}

func seedAccountingTopicTests(ctx context.Context) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var authorID int64
	if err = tx.QueryRowContext(ctx, `SELECT id FROM users WHERE email='3@3.ru'`).Scan(&authorID); err != nil {
		return err
	}
	for _, item := range accountingTopicTestSeeds {
		var testID int64
		err = tx.QueryRowContext(ctx, `INSERT INTO tests(author_id,slug,category,category_id,difficulty,status,visibility,is_free,passing_percent,time_limit_seconds)
			VALUES($1,$2,$3::varchar,(SELECT id FROM test_categories WHERE name=$3::text),$4,'published','marketplace',TRUE,70,1200)
			ON CONFLICT(slug) DO UPDATE SET author_id=EXCLUDED.author_id,category=EXCLUDED.category,category_id=EXCLUDED.category_id,difficulty=EXCLUDED.difficulty,status='published',visibility='marketplace',updated_at=NOW()
			RETURNING id`, authorID, item.Slug, item.Category, item.Difficulty).Scan(&testID)
		if err != nil {
			return err
		}
		var versionID int64
		err = tx.QueryRowContext(ctx, `INSERT INTO test_versions(test_id,version,title,description,created_by,published_at)
			VALUES($1,1,$2,$3,$4,NOW()) ON CONFLICT(test_id,version) DO UPDATE SET title=EXCLUDED.title,description=EXCLUDED.description,published_at=COALESCE(test_versions.published_at,NOW()),updated_at=NOW() RETURNING id`, testID, item.Title, item.Description, authorID).Scan(&versionID)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO test_statistics(test_id) VALUES($1) ON CONFLICT DO NOTHING`, testID); err != nil {
			return err
		}
		var count int
		if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM test_questions WHERE test_version_id=$1`, versionID).Scan(&count); err != nil {
			return err
		}
		if count == 0 {
			for number := 1; number <= 12; number++ {
				if err = seedAccountingTopicQuestion(ctx, tx, versionID, item.Title, number); err != nil {
					return err
				}
			}
		}
	}
	if err = seedAccountingTopicAttempts(ctx, tx, authorID); err != nil {
		return err
	}
	return tx.Commit()
}

func seedAccountingTopicAttempts(ctx context.Context, tx *sql.Tx, userID int64) error {
	results := []struct {
		slug          string
		percent       float64
		daysAgo       int
		confirmations int
	}{
		{"accounting-topic-vat", 94, 3, 6},
		{"accounting-topic-usn", 92, 1, 6},
		{"accounting-topic-payroll", 89, 8, 3},
		{"accounting-topic-reporting", 91, 12, 2},
		{"accounting-topic-one-c", 87, 17, 3},
	}
	for _, result := range results {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO test_attempts(test_id,test_version_id,user_id,score,max_score,percent,passed,started_at,finished_at,duration_seconds,status,context)
			SELECT t.id,v.id,$1,$2,100,$2,TRUE,
				NOW()-($3::int||' days')::interval-INTERVAL '18 minutes',
				NOW()-($3::int||' days')::interval,1080,'finished',
				jsonb_build_object('demo_accounting_result',t.slug)
			FROM tests t
			JOIN test_versions v ON v.test_id=t.id AND v.version=t.current_version
			WHERE t.slug=$4 AND NOT EXISTS(
				SELECT 1 FROM test_attempts a
				WHERE a.user_id=$1 AND a.test_id=t.id
				  AND a.context->>'demo_accounting_result'=t.slug
			)`, userID, result.percent, result.daysAgo, result.slug)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO resume_test_confirmations(resume_id,test_id,confirmer_id,created_at)
			SELECT r.id,t.id,confirmers.id,
				NOW()-((confirmers.position*7+t.id%5)||' hours')::interval
			FROM resumes r
			JOIN tests t ON t.slug=$2
			JOIN LATERAL (
				SELECT selected.id,ROW_NUMBER() OVER (ORDER BY selected.id)::int AS position
				FROM (
					SELECT u.id
					FROM users u
					WHERE u.id<>r.user_id
					  AND (u.email LIKE 'market-review-%@fintalent.local' OR u.email LIKE '%@%.ru')
					ORDER BY CASE WHEN u.email LIKE 'market-review-%@fintalent.local' THEN 0 ELSE 1 END,u.id
					LIMIT $3
				) selected
			) confirmers ON TRUE
			WHERE r.user_id=$1 AND r.deleted_at IS NULL
			ON CONFLICT(resume_id,test_id,confirmer_id) DO NOTHING`, userID, result.slug, result.confirmations)
		if err != nil {
			return err
		}
	}
	return nil
}

func seedAccountingTopicQuestion(ctx context.Context, tx *sql.Tx, versionID int64, topic string, number int) error {
	templates := []string{
		"Какое действие нужно выполнить первым при проверке участка «%s»?",
		"Что лучше всего подтверждает корректность операции по теме «%s»?",
		"Как следует действовать при обнаружении расхождения в разделе «%s»?",
		"Какая процедура снижает риск ошибки при работе с темой «%s»?",
		"Что необходимо сделать перед формированием итоговой отчётности по теме «%s»?",
		"Какие данные следует сопоставить при контрольной проверке раздела «%s»?",
	}
	question := fmt.Sprintf(templates[(number-1)%len(templates)], topic)
	kind := "single_choice"
	if number%4 == 0 {
		kind = "multiple_choice"
	}
	var questionID int64
	if err := tx.QueryRowContext(ctx, `INSERT INTO test_questions(test_version_id,sort_order,question,question_type,explanation,points,settings)
		VALUES($1,$2,$3,$4,'Правильное решение опирается на первичные документы, регламент, сверку данных и фиксацию результата проверки.',1,'{"shuffle_answers":true}'::jsonb) RETURNING id`, versionID, number-1, question, kind).Scan(&questionID); err != nil {
		return err
	}
	answers := []struct {
		text    string
		correct bool
	}{
		{"Проверить первичные документы и сопоставить данные учёта", true},
		{"Зафиксировать результат проверки и основание корректировки", kind == "multiple_choice"},
		{"Изменить показатели без подтверждающих документов", false},
		{"Игнорировать расхождение до следующего отчётного периода", false},
	}
	for order, answer := range answers {
		if _, err := tx.ExecContext(ctx, `INSERT INTO test_answers(question_id,answer,is_correct,sort_order) VALUES($1,$2,$3,$4)`, questionID, answer.text, answer.correct, order); err != nil {
			return err
		}
	}
	return nil
}

func seedPositionTests(ctx context.Context) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var authorID int64
	if err = tx.QueryRowContext(ctx, `SELECT id FROM users WHERE email='3@3.ru'`).Scan(&authorID); err != nil {
		return err
	}
	names := []string{"Анна Смирнова", "Михаил Орлов", "Елена Волкова", "Сергей Петров", "Ольга Кузнецова", "Алексей Морозов", "Наталья Соколова", "Марина Фёдорова", "Дмитрий Лебедев", "Юлия Новикова", "Андрей Попов", "Татьяна Васильева"}
	for index, name := range names {
		email := fmt.Sprintf("market-review-%d@fintalent.local", index+1)
		_, err = tx.ExecContext(ctx, `INSERT INTO users(full_name,email,password_hash) SELECT $1,$2,password_hash FROM users WHERE id=$3 ON CONFLICT(email) DO NOTHING`, name, email, authorID)
		if err != nil {
			return err
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT i.id,i.value FROM dictionary_items i JOIN dictionaries d ON d.id=i.dictionary_id WHERE d.alias='position' AND i.active=TRUE AND i.deleted_at IS NULL ORDER BY i.sort_order,i.id`)
	if err != nil {
		return err
	}
	type position struct {
		id   int64
		name string
	}
	positions := []position{}
	for rows.Next() {
		var p position
		if err = rows.Scan(&p.id, &p.name); err != nil {
			rows.Close()
			return err
		}
		positions = append(positions, p)
	}
	rows.Close()
	for positionIndex, p := range positions {
		slug := fmt.Sprintf("position-skill-%d", p.id)
		category := initialTestCategories[positionIndex%len(initialTestCategories)]
		var testID int64
		err = tx.QueryRowContext(ctx, `INSERT INTO tests(author_id,slug,category,category_id,difficulty,status,visibility,is_free,passing_percent,time_limit_seconds)
			VALUES($1,$2,$3,(SELECT id FROM test_categories WHERE name=$4),'medium','published','marketplace',TRUE,70,2400)
			ON CONFLICT(slug) DO UPDATE SET status='published',visibility='marketplace' RETURNING id`, authorID, slug, category, category).Scan(&testID)
		if err != nil {
			return err
		}
		var versionID int64
		title := p.name
		description := "Комплексная проверка практических знаний, внимательности и профессиональных навыков для должности «" + p.name + "»."
		err = tx.QueryRowContext(ctx, `INSERT INTO test_versions(test_id,version,title,description,created_by,published_at) VALUES($1,1,$2,$3,$4,NOW()) ON CONFLICT(test_id,version) DO UPDATE SET title=EXCLUDED.title,description=EXCLUDED.description,published_at=NOW() RETURNING id`, testID, title, description, authorID).Scan(&versionID)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO test_statistics(test_id) VALUES($1) ON CONFLICT DO NOTHING`, testID); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO test_positions(test_id,position_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, testID, p.id); err != nil {
			return err
		}
		var count int
		if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM test_questions WHERE test_version_id=$1`, versionID).Scan(&count); err != nil {
			return err
		}
		if count == 0 {
			for q := 1; q <= 24; q++ {
				if err = seedQuestion(ctx, tx, versionID, p.name, q); err != nil {
					return err
				}
			}
		}
		comments := []string{"Отлично проверяет практические знания.", "Вопросы близки к реальным рабочим ситуациям.", "Удобный и содержательный тест.", "Помог быстро оценить уровень кандидата.", "Хороший баланс теории и практики.", "Понятные формулировки и полезные кейсы.", "Будем использовать при подборе специалистов.", "Понравились практические задания и понятный интерфейс.", "Тест хорошо показывает сильные и слабые стороны.", "Полезная проверка перед собеседованием.", "Содержание соответствует реальным задачам специалиста.", "Прохождение заняло разумное время, вопросы качественные."}
		for i, comment := range comments {
			rating := 5
			if i == 2 || i == 5 || i == 9 {
				rating = 4
			}
			email := fmt.Sprintf("market-review-%d@fintalent.local", i+1)
			_, err = tx.ExecContext(ctx, `INSERT INTO test_reviews(test_id,employer_id,rating,comment) SELECT $1,id,$3,$4 FROM users WHERE email=$2 ON CONFLICT(test_id,employer_id) DO UPDATE SET rating=EXCLUDED.rating,comment=EXCLUDED.comment`, testID, email, rating, comment)
			if err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func seedQuestion(ctx context.Context, tx *sql.Tx, versionID int64, position string, number int) error {
	types := []string{"single_choice", "multiple_choice", "boolean", "case", "text"}
	kind := types[(number-1)%len(types)]
	question := fmt.Sprintf("%d. Рабочая ситуация для должности «%s»: выберите наиболее профессиональное решение.", number, position)
	if kind == "multiple_choice" {
		question = fmt.Sprintf("%d. Какие действия важны в работе специалиста «%s»?", number, position)
	}
	if kind == "boolean" {
		question = fmt.Sprintf("%d. Верно ли, что документы необходимо проверять до отражения в учете?", number)
	}
	if kind == "case" {
		question = fmt.Sprintf("%d. Обнаружено расхождение в документах. Как должен действовать специалист «%s»?", number, position)
	}
	if kind == "text" {
		question = fmt.Sprintf("%d. Кратко опишите порядок самопроверки результата работы для должности «%s».", number, position)
	}
	var questionID int64
	if err := tx.QueryRowContext(ctx, `INSERT INTO test_questions(test_version_id,sort_order,question,question_type,explanation,points) VALUES($1,$2,$3,$4,'Оцениваются корректность, последовательность и документальное подтверждение решения.',1) RETURNING id`, versionID, number-1, question, kind).Scan(&questionID); err != nil {
		return err
	}
	if kind == "text" {
		return nil
	}
	answers := []struct {
		text    string
		correct bool
	}{{"Проверить первичные данные и регламент, зафиксировать результат", true}, {"Игнорировать расхождение", false}, {"Изменить данные без подтверждения", false}, {"Отложить вопрос без уведомления", false}}
	if kind == "boolean" {
		answers = []struct {
			text    string
			correct bool
		}{{"Да", true}, {"Нет", false}}
	}
	if kind == "multiple_choice" {
		answers = []struct {
			text    string
			correct bool
		}{{"Проверить исходные документы", true}, {"Соблюсти сроки", true}, {"Зафиксировать результат", true}, {"Скрыть обнаруженную ошибку", false}}
	}
	for i, a := range answers {
		if _, err := tx.ExecContext(ctx, `INSERT INTO test_answers(question_id,answer,is_correct,sort_order) VALUES($1,$2,$3,$4)`, questionID, a.text, a.correct, i); err != nil {
			return err
		}
	}
	return nil
}

func registerMarketplaceRoutes() {
	http.HandleFunc("/marketplace", serveFrontendPage("static/marketplace.html"))
	http.HandleFunc("/marketplace/create-test", serveFrontendPage("static/marketplace-create-test.html"))
	http.HandleFunc("/api/marketplace/tests", marketplaceTests)
	http.HandleFunc("/api/marketplace/test-reviews", marketplaceTestReviews)
}

func marketplaceTestReviews(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
		return
	}
	testID, err := strconv.ParseInt(r.URL.Query().Get("test_id"), 10, 64)
	if err != nil || testID <= 0 {
		writeJSON(w, http.StatusBadRequest, "Некорректный тест")
		return
	}
	rows, err := db.QueryContext(r.Context(), `SELECT r.rating,r.comment,u.full_name,r.created_at FROM test_reviews r JOIN users u ON u.id=r.employer_id WHERE r.test_id=$1 ORDER BY r.created_at DESC,r.id DESC LIMIT 20`, testID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить отзывы")
		return
	}
	defer rows.Close()
	reviews := []marketplaceReview{}
	for rows.Next() {
		var review marketplaceReview
		if err = rows.Scan(&review.Rating, &review.Comment, &review.Author, &review.CreatedAt); err != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить отзывы")
			return
		}
		reviews = append(reviews, review)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(reviews)
}
func marketplaceTests(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, "Метод не поддерживается")
		return
	}
	query := `SELECT t.id,v.title,v.description,COALESCE(p.position_id,0),COALESCE(i.value,'Для всех позиций'),t.category,t.difficulty,t.is_free,t.price,COALESCE(t.time_limit_seconds,0),(SELECT COUNT(*) FROM test_questions q WHERE q.test_version_id=v.id),COALESCE((SELECT AVG(r.rating) FROM test_reviews r WHERE r.test_id=t.id),0),COALESCE((SELECT COUNT(*) FROM test_reviews r WHERE r.test_id=t.id),0),u.id,u.full_name,COALESCE((SELECT AVG(ar.rating) FROM test_reviews ar JOIN tests author_test ON author_test.id=ar.test_id WHERE author_test.author_id=t.author_id),0),COALESCE(lr.author,''),COALESCE(lr.comment,''),COALESCE(lr.rating,0),lr.created_at
		FROM tests t
		JOIN test_versions v ON v.test_id=t.id AND v.version=t.current_version
		LEFT JOIN test_positions p ON p.test_id=t.id
		LEFT JOIN dictionary_items i ON i.id=p.position_id
		JOIN users u ON u.id=t.author_id
		LEFT JOIN LATERAL (
			SELECT ru.full_name author,r.comment,r.rating,r.created_at
			FROM test_reviews r JOIN users ru ON ru.id=r.employer_id
			WHERE r.test_id=t.id ORDER BY r.created_at DESC,r.id DESC LIMIT 1
		) lr ON TRUE
		WHERE t.status='published' AND t.visibility='marketplace'`
	args := []any{}
	if id := r.URL.Query().Get("position_id"); id != "" {
		args = append(args, id)
		query += " AND p.position_id=$1"
	}
	query += " ORDER BY v.title,t.id"
	rows, err := db.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeJSON(w, 500, "Не удалось загрузить тесты")
		return
	}
	defer rows.Close()
	out := []marketplaceTest{}
	for rows.Next() {
		var item marketplaceTest
		if err = rows.Scan(&item.ID, &item.Title, &item.Description, &item.PositionID, &item.Position, &item.Category, &item.Difficulty, &item.IsFree, &item.Price, &item.TimeLimitSeconds, &item.QuestionCount, &item.Rating, &item.ReviewCount, &item.AuthorID, &item.Author, &item.AuthorRating, &item.LastReviewAuthor, &item.LastReviewComment, &item.LastReviewRating, &item.LastReviewAt); err != nil {
			writeJSON(w, 500, "Не удалось загрузить тесты")
			return
		}
		out = append(out, item)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(out)
}

var _ = strings.TrimSpace
