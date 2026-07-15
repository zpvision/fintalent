package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

type city struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	CountryCode string `json:"country_code"`
	Region      string `json:"region"`
	Featured    bool   `json:"is_featured"`
}

type hhArea struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	ParentID string   `json:"parent_id"`
	Areas    []hhArea `json:"areas"`
}

type hhLocation struct{ ID, Name, Region string }

var russianCities = []string{
	"Москва", "Санкт-Петербург", "Новосибирск", "Екатеринбург", "Казань", "Нижний Новгород",
	"Красноярск", "Челябинск", "Самара", "Уфа", "Ростов-на-Дону", "Краснодар", "Омск", "Воронеж",
	"Пермь", "Волгоград", "Саратов", "Тюмень", "Тольятти", "Барнаул", "Ижевск", "Махачкала",
	"Хабаровск", "Ульяновск", "Иркутск", "Владивосток", "Ярославль", "Севастополь", "Ставрополь",
	"Томск", "Кемерово", "Набережные Челны", "Оренбург", "Новокузнецк", "Рязань", "Балашиха",
	"Пенза", "Чебоксары", "Липецк", "Калининград", "Астрахань", "Тула", "Киров", "Сочи", "Курск",
	"Тверь", "Магнитогорск", "Сургут", "Брянск", "Якутск", "Иваново", "Владимир", "Белгород",
	"Архангельск", "Калуга", "Смоленск", "Вологда", "Саранск", "Череповец", "Курган", "Орёл",
	"Мурманск", "Петрозаводск", "Кострома", "Новороссийск", "Йошкар-Ола", "Тамбов", "Нальчик",
	"Грозный", "Стерлитамак", "Псков", "Абакан", "Южно-Сахалинск", "Великий Новгород", "Майкоп",
}

var additionalFeaturedCities = []string{
	"Подольск", "Химки", "Мытищи", "Королёв", "Энгельс", "Шахты", "Нижневартовск", "Симферополь",
}

func prepareGeographyDatabase(ctx context.Context) error {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS countries (
		id BIGSERIAL PRIMARY KEY, name VARCHAR(150) NOT NULL, code CHAR(2) NOT NULL UNIQUE,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE TABLE IF NOT EXISTS cities (
		id BIGSERIAL PRIMARY KEY, country_id BIGINT NOT NULL REFERENCES countries(id) ON DELETE CASCADE,
		name VARCHAR(150) NOT NULL, region_name VARCHAR(200) NOT NULL DEFAULT '', external_id VARCHAR(30), is_featured BOOLEAN NOT NULL DEFAULT FALSE,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	ALTER TABLE cities ADD COLUMN IF NOT EXISTS region_name VARCHAR(200) NOT NULL DEFAULT '';
	ALTER TABLE cities ADD COLUMN IF NOT EXISTS external_id VARCHAR(30);
	ALTER TABLE cities ADD COLUMN IF NOT EXISTS is_featured BOOLEAN NOT NULL DEFAULT FALSE;
	ALTER TABLE cities DROP CONSTRAINT IF EXISTS cities_country_id_name_key;
	CREATE INDEX IF NOT EXISTS cities_country_id_idx ON cities(country_id);
	CREATE INDEX IF NOT EXISTS cities_name_idx ON cities(name);
	CREATE UNIQUE INDEX IF NOT EXISTS cities_external_id_unique_idx ON cities(external_id) WHERE external_id IS NOT NULL;
	CREATE UNIQUE INDEX IF NOT EXISTS cities_location_unique_idx ON cities(country_id,region_name,name);`)
	if err != nil {
		return err
	}
	var countryID int64
	err = db.QueryRowContext(ctx, `INSERT INTO countries(name,code) VALUES('Россия','RU')
		ON CONFLICT(code) DO UPDATE SET name=EXCLUDED.name RETURNING id`).Scan(&countryID)
	if err != nil {
		return err
	}
	var count int
	if err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cities WHERE country_id=$1 AND external_id IS NOT NULL`, countryID).Scan(&count); err != nil {
		return err
	}
	if count < 1000 {
		for _, name := range russianCities {
			if _, err = db.ExecContext(ctx, `INSERT INTO cities(country_id,name) VALUES($1,$2) ON CONFLICT(country_id,region_name,name) DO NOTHING`, countryID, name); err != nil {
				return err
			}
		}
		syncCtx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		if synced, syncErr := syncRussianLocations(syncCtx, countryID); syncErr != nil {
			log.Printf("Не удалось синхронизировать полный список городов: %v", syncErr)
		} else {
			log.Printf("Справочник городов России синхронизирован: %d записей", synced)
		}
	} else {
		if _, err = db.ExecContext(ctx, `DELETE FROM cities WHERE country_id=$1 AND external_id IS NULL`, countryID); err != nil {
			return err
		}
	}
	if err = seedFeaturedRussianCities(ctx, countryID); err != nil {
		return err
	}
	return nil
}

func seedFeaturedRussianCities(ctx context.Context, countryID int64) error {
	var featuredCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cities WHERE country_id=$1 AND is_featured`, countryID).Scan(&featuredCount); err != nil {
		return err
	}
	if featuredCount >= 75 {
		return nil
	}
	for _, name := range russianCities {
		if _, err := db.ExecContext(ctx, `UPDATE cities SET is_featured=TRUE WHERE id=(
			SELECT id FROM cities WHERE country_id=$1 AND name=$2 ORDER BY CASE WHEN region_name LIKE '%'||$2||'%' THEN 0 ELSE 1 END,id LIMIT 1
		)`, countryID, name); err != nil {
			return err
		}
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cities WHERE country_id=$1 AND is_featured`, countryID).Scan(&featuredCount); err != nil {
		return err
	}
	for _, name := range additionalFeaturedCities {
		if featuredCount >= 75 {
			break
		}
		result, err := db.ExecContext(ctx, `UPDATE cities SET is_featured=TRUE WHERE id=(
			SELECT id FROM cities WHERE country_id=$1 AND name=$2 AND NOT is_featured ORDER BY id LIMIT 1
		)`, countryID, name)
		if err != nil {
			return err
		}
		if affected, _ := result.RowsAffected(); affected > 0 {
			featuredCount++
		}
	}
	return nil
}

func syncRussianLocations(ctx context.Context, countryID int64) (int, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.hh.ru/areas/113", nil)
	if err != nil {
		return 0, err
	}
	request.Header.Set("HH-User-Agent", "FinTalent/1.0 (dev@fintalent.ru)")
	response, err := (&http.Client{Timeout: 45 * time.Second}).Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HeadHunter API вернул %d", response.StatusCode)
	}
	var root hhArea
	if err = json.NewDecoder(response.Body).Decode(&root); err != nil {
		return 0, err
	}
	locations := []hhLocation{}
	for _, region := range root.Areas {
		collectHHLeaves(region, region.Name, &locations)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `DELETE FROM cities WHERE country_id=$1 AND external_id IS NULL`, countryID); err != nil {
		return 0, err
	}
	statement, err := tx.PrepareContext(ctx, `INSERT INTO cities(country_id,name,region_name,external_id) VALUES($1,$2,$3,$4)
		ON CONFLICT(external_id) WHERE external_id IS NOT NULL DO UPDATE SET name=EXCLUDED.name,region_name=EXCLUDED.region_name,country_id=EXCLUDED.country_id`)
	if err != nil {
		return 0, err
	}
	defer statement.Close()
	for _, location := range locations {
		if _, err = statement.ExecContext(ctx, countryID, location.Name, location.Region, location.ID); err != nil {
			return 0, err
		}
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return len(locations), nil
}

func collectHHLeaves(area hhArea, region string, result *[]hhLocation) {
	if len(area.Areas) == 0 {
		*result = append(*result, hhLocation{ID: area.ID, Name: area.Name, Region: region})
		return
	}
	for _, child := range area.Areas {
		collectHHLeaves(child, region, result)
	}
}

func registerGeographyRoutes() { http.HandleFunc("/api/public/cities", publicCities) }

func publicCities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
		return
	}
	code := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("country")))
	if code == "" {
		code = "RU"
	}
	ctx, cancel := contextWithTimeout()
	defer cancel()
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(query)) > 100 {
		writeJSON(w, http.StatusBadRequest, "Слишком длинный поисковый запрос")
		return
	}
	pattern := "%" + query + "%"
	rows, err := db.QueryContext(ctx, `SELECT c.id,c.name,co.code,c.region_name,c.is_featured FROM cities c JOIN countries co ON co.id=c.country_id
		WHERE co.code=$1 AND (($2='' AND c.is_featured) OR ($2<>'' AND c.name ILIKE $3))
		ORDER BY CASE WHEN LOWER(c.name)=LOWER($2) THEN 0 WHEN LOWER(c.name) LIKE LOWER($2)||'%' THEN 1 ELSE 2 END,c.name,c.region_name LIMIT 75`, code, query, pattern)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить города")
		return
	}
	defer rows.Close()
	result := []city{}
	for rows.Next() {
		var item city
		if err := rows.Scan(&item.ID, &item.Name, &item.CountryCode, &item.Region, &item.Featured); err != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить города")
			return
		}
		result = append(result, item)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(result)
}
