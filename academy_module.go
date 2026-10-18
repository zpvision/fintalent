package main

import (
	"FinTalent/internal/academy"
	"context"
	_ "embed"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

//go:embed migrations/078_academy.sql
var academySchema string

//go:embed migrations/079_academy_learning.sql
var academyLearningSchema string

//go:embed migrations/080_academy_assessments.sql
var academyAssessmentSchema string

func prepareAcademyDatabase(ctx context.Context) error {
	_, err := db.ExecContext(ctx, academySchema)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, academyLearningSchema)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, academyAssessmentSchema)
	return err
}
func registerAcademyRoutes(gate func(http.HandlerFunc) http.HandlerFunc) {
	root := os.Getenv("ACADEMY_STORAGE_DIR")
	if root == "" {
		root = filepath.Join(".local-data", "academy")
	}
	// Never permit a storage path under the publicly served tree.
	absolute, err := filepath.Abs(root)
	if err != nil {
		panic("invalid Academy storage directory")
	}
	public, _ := filepath.Abs("static")
	rel, relErr := filepath.Rel(public, absolute)
	if relErr == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		panic("Academy storage must be outside static")
	}
	h := academy.New(db, func(r *http.Request) (int64, error) {
		u, e := userFromRequest(r)
		if e != nil {
			return 0, e
		}
		return u.ID, nil
	}, gate, absolute)
	h.Register(http.DefaultServeMux)
	http.HandleFunc("/profile/learning", serveFrontendPage("static/courses-unavailable.html"))
	http.HandleFunc("/profile/learning/", serveFrontendPage("static/courses-unavailable.html"))
}
