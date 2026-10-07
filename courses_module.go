package main

import (
	"FinTalent/internal/courses"
	"context"
	_ "embed"
	"net/http"
)

//go:embed migrations/070_courses.sql
var coursesSchema string

func prepareCoursesDatabase(ctx context.Context) error {
	_, err := db.ExecContext(ctx, coursesSchema)
	return err
}
func registerCoursesRoutes() {
	h := courses.New(db, func(r *http.Request) (int64, error) {
		u, e := userFromRequest(r)
		if e != nil {
			return 0, e
		}
		return u.ID, nil
	}, isAdmin)
	h.Register(http.DefaultServeMux)
	http.HandleFunc("/courses", serveFrontendPage("static/courses-unavailable.html"))
	http.HandleFunc("/courses/", serveFrontendPage("static/courses-unavailable.html"))
	http.HandleFunc("/profile/courses", serveFrontendPage("static/courses-unavailable.html"))
	http.HandleFunc("/profile/courses/", serveFrontendPage("static/courses-unavailable.html"))
	http.HandleFunc("/api/courses/upload", profiMarketUploadAPI)
}
