package courses

import (
	"context"
	"database/sql"
	"errors"
)

var ErrLessonType = errors.New("Выберите активный тип урока из справочника")

func validateLessonTypes(ctx context.Context, tx *sql.Tx, courseID int64, v *Input) error {
	for _, m := range v.Modules {
		for _, l := range m.Lessons {
			var valid bool
			err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM course_lesson_types t WHERE t.key=$1 AND ((t.active AND t.deleted_at IS NULL) OR EXISTS(SELECT 1 FROM course_lessons l JOIN course_modules m ON m.id=l.module_id WHERE l.id=$2 AND l.type=t.key AND m.course_id=$3)))`, l.Type, l.ID, courseID).Scan(&valid)
			if err != nil {
				return err
			}
			if !valid {
				return ErrLessonType
			}
		}
	}
	return nil
}
