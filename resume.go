package main

import (
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"os"
	"strconv"
	"strings"
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
	http.HandleFunc("/api/public/dictionaries/", publicDictionaryHandler)
	http.HandleFunc("/api/assets/position-icon/", positionIconHandler)
	http.HandleFunc("/api/assets/accounting-area-icon/", accountingAreaIconHandler)
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
