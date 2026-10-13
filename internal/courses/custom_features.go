package courses

import (
	"context"
	"database/sql"
	"errors"
)

var ErrCustomFeature = errors.New("Собственная возможность уже есть среди выбранных вариантов справочника")

func saveCustomFeatures(ctx context.Context, tx *sql.Tx, courseID int64, v *Input) error {
	// Preserve additions when an older client omits this field.
	if v.CustomFeatures == nil {
		return nil
	}
	rows, err := tx.QueryContext(ctx, `SELECT i.value FROM course_feature_selections s JOIN dictionary_items i ON i.id=s.dictionary_item_id WHERE s.course_id=$1`, courseID)
	if err != nil {
		return err
	}
	selected := map[string]bool{}
	for rows.Next() {
		var title string
		if err = rows.Scan(&title); err != nil {
			rows.Close()
			return err
		}
		selected[featureTitleKey(title)] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	ids := []int64{}
	seen := map[int64]bool{}
	for i := range v.CustomFeatures {
		feature := &v.CustomFeatures[i]
		if selected[featureTitleKey(feature.Title)] {
			return ErrCustomFeature
		}
		if err = syncRow(ctx, tx, "course_custom_features", "course_id", courseID, &feature.ID, []string{"title", "sort_order"}, []any{feature.Title, i}); err != nil {
			return err
		}
		if seen[feature.ID] {
			return ErrChild
		}
		seen[feature.ID] = true
		ids = append(ids, feature.ID)
	}
	return prune(ctx, tx, "course_custom_features", "course_id", courseID, ids)
}
