package courses

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

var ErrFeature = errors.New("Выберите доступные возможности курса")

// Keep the existing special behaviours in sync with dictionary selections.
func saveFeatures(ctx context.Context, tx *sql.Tx, courseID int64, v *Input) error {
	if v.FeatureIDs == nil { // Compatibility with clients that still send the original flags.
		rows, err := tx.QueryContext(ctx, `SELECT b.dictionary_item_id,b.feature_key FROM course_feature_bindings b JOIN dictionary_items i ON i.id=b.dictionary_item_id WHERE i.deleted_at IS NULL AND i.active`)
		if err != nil {
			return err
		}
		flags := map[string]bool{"certificate": v.Certificate, "chat": v.Chat, "updates": v.Updates, "any_device": v.AnyDevice, "corporate": v.Corporate}
		v.FeatureIDs = []int64{}
		for rows.Next() {
			var id int64
			var key string
			if err = rows.Scan(&id, &key); err != nil {
				rows.Close()
				return err
			}
			if flags[key] {
				v.FeatureIDs = append(v.FeatureIDs, id)
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	b, _ := json.Marshal(v.FeatureIDs)
	var valid bool
	err := tx.QueryRowContext(ctx, `SELECT NOT EXISTS(SELECT 1 FROM jsonb_array_elements_text($2::jsonb) x WHERE NOT EXISTS(SELECT 1 FROM dictionary_items i JOIN dictionaries d ON d.id=i.dictionary_id WHERE i.id=x.value::bigint AND d.alias='course_features' AND ((i.active AND i.deleted_at IS NULL) OR EXISTS(SELECT 1 FROM course_feature_selections s WHERE s.course_id=$1 AND s.dictionary_item_id=i.id))))`, courseID, string(b)).Scan(&valid)
	if err != nil {
		return err
	}
	if !valid {
		return ErrFeature
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM course_feature_selections WHERE course_id=$1`, courseID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO course_feature_selections(course_id,dictionary_item_id) SELECT $1,value::bigint FROM jsonb_array_elements_text($2::jsonb)`, courseID, string(b)); err != nil {
		return err
	}
	v.Certificate, v.Chat, v.Updates, v.AnyDevice, v.Corporate = false, false, false, false, false
	rows, err := tx.QueryContext(ctx, `SELECT b.feature_key FROM course_feature_selections s JOIN course_feature_bindings b ON b.dictionary_item_id=s.dictionary_item_id WHERE s.course_id=$1`, courseID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		if err = rows.Scan(&key); err != nil {
			return err
		}
		switch key {
		case "certificate":
			v.Certificate = true
		case "chat":
			v.Chat = true
		case "updates":
			v.Updates = true
		case "any_device":
			v.AnyDevice = true
		case "corporate":
			v.Corporate = true
		}
	}
	return rows.Err()
}
