package courses

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

func validateTariffs(v *Input, publishing bool) error {
	if v.PricingMode != "" && v.PricingMode != "single" && v.PricingMode != "tariffs" {
		return fmt.Errorf("Выберите одну цену или несколько тарифов")
	}
	if len(v.Tariffs) > 4 {
		return fmt.Errorf("Добавьте не больше 4 тарифов")
	}
	if v.PricingMode == "tariffs" && len(v.Tariffs) < 2 {
		return fmt.Errorf("Добавьте от 2 до 4 тарифов или выберите одну цену")
	}
	recommended := 0
	names := map[string]bool{}
	ids := map[int64]bool{}
	for i := range v.Tariffs {
		t := &v.Tariffs[i]
		t.Title = strings.TrimSpace(t.Title)
		t.Description = strings.TrimSpace(t.Description)
		if t.ID < 0 || (t.ID > 0 && ids[t.ID]) {
			return ErrChild
		}
		ids[t.ID] = true
		if t.Price < 0 || t.Price > 10000000000 || utf8.RuneCountInString(t.Title) > 80 || utf8.RuneCountInString(t.Description) > 300 {
			return fmt.Errorf("Тариф: название до 80, описание до 300 символов, цена от 0 до 100 млн ₽")
		}
		if publishing && v.PricingMode == "tariffs" && (t.Title == "" || t.Description == "") {
			return fmt.Errorf("Заполните название и описание каждого тарифа")
		}
		if t.OldPrice != nil && (*t.OldPrice <= t.Price || *t.OldPrice > 10000000000) {
			return fmt.Errorf("Старая цена тарифа должна быть выше текущей и не больше 100 млн ₽")
		}
		if math.IsNaN(t.Hours) || math.IsInf(t.Hours, 0) || t.Hours < 0 || t.Hours > 100000 || (publishing && v.PricingMode == "tariffs" && t.Hours <= 0) {
			return fmt.Errorf("Укажите часы обучения для каждого тарифа: больше 0 и до 100000")
		}
		key := featureTitleKey(t.Title)
		if key != "" && names[key] {
			return fmt.Errorf("Названия тарифов не должны повторяться")
		}
		names[key] = true
		if t.Recommended {
			recommended++
		}
	}
	if recommended > 1 {
		return fmt.Errorf("Рекомендуемым может быть только один тариф")
	}
	return nil
}

func saveTariffs(ctx context.Context, tx *sql.Tx, courseID int64, v *Input) error {
	// Older clients omit the mode; preserve the pricing configuration in that case.
	if v.PricingMode == "" {
		return nil
	}
	if v.Tariffs != nil {
		if _, err := tx.ExecContext(ctx, "UPDATE course_tariffs SET recommended=FALSE WHERE course_id=$1", courseID); err != nil {
			return err
		}
		ids := []int64{}
		for i := range v.Tariffs {
			t := &v.Tariffs[i]
			if err := syncRow(ctx, tx, "course_tariffs", "course_id", courseID, &t.ID, []string{"title", "description", "price", "recommended", "sort_order", "old_price", "hours"}, []any{t.Title, t.Description, t.Price, t.Recommended, i, t.OldPrice, t.Hours}); err != nil {
				return err
			}
			ids = append(ids, t.ID)
		}
		if err := prune(ctx, tx, "course_tariffs", "course_id", courseID, ids); err != nil {
			return err
		}
	}
	_, err := tx.ExecContext(ctx, "UPDATE courses SET pricing_mode=$2 WHERE id=$1", courseID, v.PricingMode)
	return err
}
