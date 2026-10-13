package main

import (
	"FinTalent/internal/courses"
	"context"
	_ "embed"
	"net/http"
)

//go:embed migrations/070_courses.sql
var coursesSchema string

//go:embed migrations/071_course_benefit_colors.sql
var courseBenefitColorsSchema string

//go:embed migrations/072_course_settings.sql
var courseSettingsSchema string

//go:embed migrations/073_course_lesson_types.sql
var courseLessonTypesSchema string

//go:embed migrations/074_course_access.sql
var courseAccessSchema string

//go:embed migrations/075_course_custom_features.sql
var courseCustomFeaturesSchema string

func prepareCoursesDatabase(ctx context.Context) error {
	_, err := db.ExecContext(ctx, coursesSchema)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, courseBenefitColorsSchema)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, courseSettingsSchema)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, courseLessonTypesSchema)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, courseAccessSchema)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, courseCustomFeaturesSchema)
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
	http.HandleFunc("/api/admin/course-access", adminCourseAccess)
	http.HandleFunc("/courses", h.Gate(serveFrontendPage("static/courses-unavailable.html")))
	http.HandleFunc("/courses/", h.Gate(serveFrontendPage("static/courses-unavailable.html")))
	http.HandleFunc("/profile/courses", h.Gate(serveFrontendPage("static/courses-unavailable.html")))
	http.HandleFunc("/profile/courses/", h.Gate(serveFrontendPage("static/courses-unavailable.html")))
	http.HandleFunc("/api/courses/upload", h.Gate(profiMarketUploadAPI))
}
