package repository

import (
	"FinTalent/internal/testmodule/dto"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

// Material content and metadata are immutable; detaching never removes historical files.
type Material struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	MIME string `json:"mime"`
	Size int    `json:"size"`
}

func validateMaterials(ctx context.Context, tx *sql.Tx, testID, owner int64, in *dto.CreateQuestion) error {
	raw, present := in.Settings["materials"]
	if !present {
		return nil
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return errors.New("некорректные материалы вопроса")
	}
	var requested []Material
	if err = json.Unmarshal(encoded, &requested); err != nil || len(requested) > 6 {
		return errors.New("добавьте не больше 6 материалов к вопросу")
	}
	canonical := make([]Material, 0, len(requested))
	seen := map[int64]bool{}
	for _, item := range requested {
		if item.ID <= 0 || seen[item.ID] {
			return errors.New("некорректные материалы вопроса")
		}
		seen[item.ID] = true
		var asset Material
		if err = tx.QueryRowContext(ctx, `SELECT id,name,mime,size FROM test_question_materials WHERE id=$1 AND test_id=$2 AND owner_id=$3`, item.ID, testID, owner).Scan(&asset.ID, &asset.Name, &asset.MIME, &asset.Size); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrForbidden
			}
			return err
		}
		canonical = append(canonical, asset)
	}
	in.Settings["materials"] = canonical
	return nil
}
