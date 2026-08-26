package accountingcompany

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type dictionaryInput struct {
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Icon        string `json:"icon"`
	Description string `json:"description"`
	Category    string `json:"category"`
	ImageURL    string `json:"image_url"`
	ColorKey    string `json:"color_key"`
	ColorValue  string `json:"color_value"`
	SortOrder   int    `json:"sort_order"`
	Active      bool   `json:"active"`
}

func (h *Handler) adminDictionaries(w http.ResponseWriter, r *http.Request) {
	if !h.admin(r) {
		failure(w, 403, "Недостаточно прав")
		return
	}
	switch r.Method {
	case http.MethodGet:
		result := map[string]any{}
		queries := map[string]string{
			"directions": `SELECT jsonb_build_object('id',id,'name',name,'slug',slug,'icon',icon,'description',description,'sort_order',sort_order,'active',active,'used_count',(SELECT count(*) FROM accounting_company_direction_links WHERE direction_id=d.id)) FROM accounting_company_directions d WHERE deleted_at IS NULL ORDER BY sort_order,name`,
			"services":   `SELECT jsonb_build_object('id',id,'name',name,'slug',slug,'icon',icon,'category',category,'description',description,'sort_order',sort_order,'active',active,'used_count',(SELECT count(*) FROM accounting_company_services WHERE service_id=s.id)) FROM accounting_company_service_catalog s WHERE deleted_at IS NULL ORDER BY sort_order,name`,
			"headers":    `SELECT jsonb_build_object('id',id,'name',name,'slug',slug,'image_url',image_url,'category',category,'sort_order',sort_order,'active',active,'used_count',(SELECT count(*) FROM accounting_companies WHERE header_template_id=h.id AND deleted_at IS NULL)) FROM accounting_company_header_templates h WHERE deleted_at IS NULL ORDER BY sort_order,name`,
			"accents":    `SELECT jsonb_build_object('id',id,'name',name,'color_key',color_key,'color_value',color_value,'sort_order',sort_order,'active',active,'used_count',(SELECT count(*) FROM accounting_companies WHERE accent_style_id=a.id AND deleted_at IS NULL)) FROM accounting_company_accent_styles a WHERE deleted_at IS NULL ORDER BY sort_order,name`,
		}
		for kind, q := range queries {
			rows, err := h.db.QueryContext(r.Context(), q)
			if err != nil {
				failure(w, 500, "Не удалось загрузить справочники")
				return
			}
			items := []json.RawMessage{}
			for rows.Next() {
				var raw []byte
				if rows.Scan(&raw) == nil {
					items = append(items, raw)
				}
			}
			rows.Close()
			result[kind] = items
		}
		response(w, 200, result)
	case http.MethodPost:
		var in dictionaryInput
		if !decode(w, r, &in) {
			return
		}
		id, err := h.insertDictionary(r, in)
		if err != nil {
			failure(w, 400, err.Error())
			return
		}
		response(w, 201, map[string]any{"id": id})
	default:
		failure(w, 405, "Метод не поддерживается")
	}
}

func (h *Handler) adminDictionary(w http.ResponseWriter, r *http.Request) {
	if !h.admin(r) {
		failure(w, 403, "Недостаточно прав")
		return
	}
	tail := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/accounting-companies/dictionaries/"), "/")
	p := strings.Split(tail, "/")
	if len(p) != 2 {
		failure(w, 404, "Элемент не найден")
		return
	}
	id, err := strconv.ParseInt(p[1], 10, 64)
	if err != nil {
		failure(w, 404, "Элемент не найден")
		return
	}
	kind := p[0]
	switch r.Method {
	case http.MethodPut:
		var in dictionaryInput
		if !decode(w, r, &in) {
			return
		}
		in.Kind = kind
		if err = h.updateDictionary(r, id, in); err != nil {
			failure(w, 400, err.Error())
			return
		}
		response(w, 200, map[string]bool{"ok": true})
	case http.MethodDelete:
		if err = h.deleteDictionary(r, kind, id); err != nil {
			failure(w, 400, err.Error())
			return
		}
		response(w, 200, map[string]bool{"ok": true})
	default:
		failure(w, 405, "Метод не поддерживается")
	}
}

func dictionaryTable(kind string) (string, string, error) {
	switch kind {
	case "directions":
		return "accounting_company_directions", "accounting_company_direction_links", nil
	case "services":
		return "accounting_company_service_catalog", "accounting_company_services", nil
	case "headers":
		return "accounting_company_header_templates", "accounting_companies", nil
	case "accents":
		return "accounting_company_accent_styles", "accounting_companies", nil
	default:
		return "", "", fmt.Errorf("неизвестный справочник")
	}
}

func (h *Handler) insertDictionary(r *http.Request, in dictionaryInput) (int64, error) {
	if strings.TrimSpace(in.Name) == "" {
		return 0, fmt.Errorf("укажите название")
	}
	if in.Slug == "" {
		in.Slug = slugify(in.Name)
	}
	var id int64
	var err error
	switch in.Kind {
	case "directions":
		err = h.db.QueryRowContext(r.Context(), `INSERT INTO accounting_company_directions(name,slug,icon,description,sort_order,active) VALUES($1,$2,$3,$4,$5,$6) RETURNING id`, clean(in.Name, 180), clean(in.Slug, 180), clean(in.Icon, 40), clean(in.Description, 3000), in.SortOrder, in.Active).Scan(&id)
	case "services":
		err = h.db.QueryRowContext(r.Context(), `INSERT INTO accounting_company_service_catalog(name,slug,icon,category,description,sort_order,active) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id`, clean(in.Name, 220), clean(in.Slug, 180), clean(in.Icon, 40), clean(in.Category, 140), clean(in.Description, 3000), in.SortOrder, in.Active).Scan(&id)
	case "headers":
		err = h.db.QueryRowContext(r.Context(), `INSERT INTO accounting_company_header_templates(name,slug,image_url,category,sort_order,active) VALUES($1,$2,$3,$4,$5,$6) RETURNING id`, clean(in.Name, 180), clean(in.Slug, 180), clean(in.ImageURL, 500), clean(in.Category, 100), in.SortOrder, in.Active).Scan(&id)
	case "accents":
		err = h.db.QueryRowContext(r.Context(), `INSERT INTO accounting_company_accent_styles(name,color_key,color_value,sort_order,active) VALUES($1,$2,$3,$4,$5) RETURNING id`, clean(in.Name, 80), clean(in.ColorKey, 40), clean(in.ColorValue, 20), in.SortOrder, in.Active).Scan(&id)
	default:
		return 0, fmt.Errorf("неизвестный справочник")
	}
	if err != nil {
		return 0, fmt.Errorf("не удалось сохранить: проверьте уникальность кода")
	}
	return id, nil
}

func (h *Handler) updateDictionary(r *http.Request, id int64, in dictionaryInput) error {
	if strings.TrimSpace(in.Name) == "" {
		return fmt.Errorf("укажите название")
	}
	var res sql.Result
	var err error
	switch in.Kind {
	case "directions":
		res, err = h.db.ExecContext(r.Context(), `UPDATE accounting_company_directions SET name=$2,slug=$3,icon=$4,description=$5,sort_order=$6,active=$7,updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, id, clean(in.Name, 180), clean(in.Slug, 180), clean(in.Icon, 40), clean(in.Description, 3000), in.SortOrder, in.Active)
	case "services":
		res, err = h.db.ExecContext(r.Context(), `UPDATE accounting_company_service_catalog SET name=$2,slug=$3,icon=$4,category=$5,description=$6,sort_order=$7,active=$8,updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, id, clean(in.Name, 220), clean(in.Slug, 180), clean(in.Icon, 40), clean(in.Category, 140), clean(in.Description, 3000), in.SortOrder, in.Active)
	case "headers":
		res, err = h.db.ExecContext(r.Context(), `UPDATE accounting_company_header_templates SET name=$2,slug=$3,image_url=$4,category=$5,sort_order=$6,active=$7,updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, id, clean(in.Name, 180), clean(in.Slug, 180), clean(in.ImageURL, 500), clean(in.Category, 100), in.SortOrder, in.Active)
	case "accents":
		res, err = h.db.ExecContext(r.Context(), `UPDATE accounting_company_accent_styles SET name=$2,color_key=$3,color_value=$4,sort_order=$5,active=$6,updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, id, clean(in.Name, 80), clean(in.ColorKey, 40), clean(in.ColorValue, 20), in.SortOrder, in.Active)
	default:
		return fmt.Errorf("неизвестный справочник")
	}
	if err != nil {
		return fmt.Errorf("не удалось сохранить: проверьте код")
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("элемент не найден")
	}
	return nil
}

func (h *Handler) deleteDictionary(r *http.Request, kind string, id int64) error {
	table, usedTable, err := dictionaryTable(kind)
	if err != nil {
		return err
	}
	var count int
	switch kind {
	case "directions":
		err = h.db.QueryRowContext(r.Context(), `SELECT count(*) FROM accounting_company_direction_links WHERE direction_id=$1`, id).Scan(&count)
	case "services":
		err = h.db.QueryRowContext(r.Context(), `SELECT count(*) FROM accounting_company_services WHERE service_id=$1`, id).Scan(&count)
	case "headers":
		err = h.db.QueryRowContext(r.Context(), `SELECT count(*) FROM accounting_companies WHERE header_template_id=$1 AND deleted_at IS NULL`, id).Scan(&count)
	case "accents":
		err = h.db.QueryRowContext(r.Context(), `SELECT count(*) FROM accounting_companies WHERE accent_style_id=$1 AND deleted_at IS NULL`, id).Scan(&count)
	}
	_ = usedTable
	if err != nil {
		return fmt.Errorf("не удалось проверить использование")
	}
	if count > 0 {
		return fmt.Errorf("значение используется компаниями — сначала отключите его")
	}
	query := fmt.Sprintf("UPDATE %s SET deleted_at=NOW(),active=FALSE,updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL", table)
	res, err := h.db.ExecContext(r.Context(), query, id)
	if err != nil {
		return fmt.Errorf("не удалось удалить")
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("элемент не найден")
	}
	return nil
}
