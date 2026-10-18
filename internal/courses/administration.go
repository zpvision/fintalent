package courses

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

var ErrPublicationRestricted = errors.New("Публикация ограничена администрацией FinTalent. Обратитесь в поддержку")

func LockPublication(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended('courses:publication',0))")
	return err
}
func SetPublicationRestriction(ctx context.Context, tx *sql.Tx, id int64, author, blocked bool, reason string) error {
	if err := LockPublication(ctx, tx); err != nil {
		return err
	}
	column := "course_id"
	if author {
		column = "author_id"
	}
	_, err := tx.ExecContext(ctx, "INSERT INTO academy_publication_limits("+column+",blocked,reason) VALUES($1,$2,$3) ON CONFLICT("+column+") DO UPDATE SET blocked=excluded.blocked,reason=excluded.reason,revision=academy_publication_limits.revision+1,updated_at=now()", id, blocked, reason)
	if err != nil {
		return err
	}
	if blocked {
		where := "id=$1"
		if author {
			where = "owner_id=$1"
		}
		_, err = tx.ExecContext(ctx, "UPDATE courses SET status='hidden',revision=revision+1,updated_at=now() WHERE "+where+" AND status IN('published','moderation')", id)
	}
	return err
}
func publicationAllowed(ctx context.Context, db interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, id int64) error {
	var blocked bool
	err := db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM courses c JOIN academy_publication_limits l ON l.course_id=c.id OR l.author_id=c.owner_id WHERE c.id=$1 AND l.blocked)", id).Scan(&blocked)
	if err != nil {
		return err
	}
	if blocked {
		return ErrPublicationRestricted
	}
	return nil
}

// TransitionAdministrative applies the same landing lifecycle as author submission and legacy moderation.
func TransitionAdministrative(ctx context.Context, tx *sql.Tx, id int64, action string) (string, error) {
	return transitionCourse(ctx, tx, id, 0, action, true)
}

type TransitionError struct {
	Status  int
	Message string
}

func (e TransitionError) Error() string { return e.Message }
func transitionCourse(ctx context.Context, tx *sql.Tx, id, uid int64, action string, admin bool) (string, error) {
	if err := LockPublication(ctx, tx); err != nil {
		return "", err
	}
	var owner int64
	if err := tx.QueryRowContext(ctx, "SELECT owner_id FROM courses WHERE id=$1 FOR UPDATE", id).Scan(&owner); err != nil {
		return "", err
	}
	if !admin && owner != uid {
		return "", sql.ErrNoRows
	}
	raw, err := AdministrativeCourse(ctx, tx, id)
	if err != nil {
		return "", err
	}
	var v Input
	if err = json.Unmarshal(raw, &v); err != nil {
		return "", err
	}
	var state struct {
		Status      string
		Slug        string
		PublishedAt *string `json:"published_at"`
	}
	_ = json.Unmarshal(raw, &state)
	target := ""
	switch {
	case action == "submit" && !admin && (state.Status == "draft" || state.Status == "hidden"):
		target = "moderation"
	case action == "hide" && (state.Status == "published" || state.Status == "moderation"):
		target = "hidden"
	case action == "approve" && admin && state.Status == "moderation":
		target = "published"
	case action == "reject" && admin && state.Status == "moderation":
		target = "draft"
	default:
		return "", TransitionError{409, "Это действие недоступно для текущего статуса курса"}
	}
	if target == "moderation" || target == "published" {
		if err = publicationAllowed(ctx, tx, id); err != nil {
			if errors.Is(err, ErrPublicationRestricted) {
				return "", TransitionError{409, err.Error()}
			}
			return "", err
		}
		if err = Validate(&v, true); err != nil {
			return "", TransitionError{400, err.Error()}
		}
		var active bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM dictionary_items i JOIN dictionaries d ON d.id=i.dictionary_id WHERE i.id=$1 AND d.alias='course_categories' AND i.active AND i.deleted_at IS NULL)`, v.CategoryID).Scan(&active); err != nil {
			return "", err
		}
		if !active {
			return "", TransitionError{400, ErrCategory.Error()}
		}
	}
	slug := state.Slug
	if state.PublishedAt == nil {
		slug = fmt.Sprintf("%s-%d", Slug(v.Title), id)
	}
	res, err := tx.ExecContext(ctx, `UPDATE courses SET status=$1::text,slug=$2,revision=revision+1,updated_at=now(),published_at=CASE WHEN $1::text='published' THEN COALESCE(published_at,now()) ELSE published_at END WHERE id=$3 AND revision=$4`, target, slug, id, v.Revision)
	if err != nil {
		return "", err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return "", ErrConflict
	}
	return target, nil
}
