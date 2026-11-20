package main

import (
	"FinTalent/internal/academy"
	"context"
	_ "embed"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

//go:embed migrations/078_academy.sql
var academySchema string

//go:embed migrations/079_academy_learning.sql
var academyLearningSchema string

//go:embed migrations/080_academy_assessments.sql
var academyAssessmentSchema string

//go:embed migrations/081_academy_cohorts.sql
var academyCohortSchema string

//go:embed migrations/082_academy_communications.sql
var academyCommunicationSchema string

//go:embed migrations/083_academy_analytics.sql
var academyAnalyticsSchema string

//go:embed migrations/084_academy_admin.sql
var academyAdminSchema string

//go:embed migrations/089_academy_storage_owners.sql
var academyStorageOwnersSchema string

//go:embed migrations/088_academy_object_storage.sql
var academyObjectStorageSchema string

//go:embed migrations/085_academy_publication_defaults.sql
var academyPublicationDefaultsSchema string

//go:embed migrations/092_course_manual_payments.sql
var academyManualPaymentsSchema string

//go:embed migrations/093_academy_program_revisions.sql
var academyProgramRevisionsSchema string

//go:embed migrations/094_academy_default_automations.sql
var academyDefaultAutomationsSchema string

func prepareAcademyDatabase(ctx context.Context) error {
	defer startupTimer("database.prepareAcademyDatabase")()
	doneScript := startupTimer("database.academy.schema")
	_, err := db.ExecContext(ctx, academySchema)
	doneScript()
	if err != nil {
		return err
	}
	doneScript = startupTimer("database.academy.learning")
	_, err = db.ExecContext(ctx, academyLearningSchema)
	doneScript()
	if err != nil {
		return err
	}
	doneScript = startupTimer("database.academy.assessment")
	_, err = db.ExecContext(ctx, academyAssessmentSchema)
	doneScript()
	if err != nil {
		return err
	}
	doneScript = startupTimer("database.academy.cohorts")
	_, err = db.ExecContext(ctx, academyCohortSchema)
	doneScript()
	if err != nil {
		return err
	}
	doneScript = startupTimer("database.academy.communications")
	_, err = db.ExecContext(ctx, academyCommunicationSchema)
	doneScript()
	if err != nil {
		return err
	}
	doneScript = startupTimer("database.academy.analytics")
	_, err = db.ExecContext(ctx, academyAnalyticsSchema)
	doneScript()
	if err != nil {
		return err
	}
	doneScript = startupTimer("database.academy.admin")
	_, err = db.ExecContext(ctx, academyAdminSchema)
	doneScript()
	if err != nil {
		return err
	}
	doneScript = startupTimer("database.academy.publication_defaults")
	_, err = db.ExecContext(ctx, academyPublicationDefaultsSchema)
	doneScript()
	if err != nil {
		return err
	}
	doneScript = startupTimer("database.academy.object_storage")
	_, err = db.ExecContext(ctx, academyObjectStorageSchema)
	doneScript()
	if err != nil {
		return err
	}
	doneScript = startupTimer("database.academy.storage_owners")
	_, err = db.ExecContext(ctx, academyStorageOwnersSchema)
	doneScript()
	if err != nil {
		return err
	}
	doneScript = startupTimer("database.academy.manual_payments")
	_, err = db.ExecContext(ctx, academyManualPaymentsSchema)
	doneScript()
	if err != nil {
		return err
	}
	doneScript = startupTimer("database.academy.program_revisions")
	_, err = db.ExecContext(ctx, academyProgramRevisionsSchema)
	doneScript()
	if err != nil {
		return err
	}
	doneScript = startupTimer("database.academy.default_automations")
	_, err = db.ExecContext(ctx, academyDefaultAutomationsSchema)
	doneScript()
	return err
}
func academyStorageRoot() string {
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
	return absolute
}
func registerAcademyRoutes(gate func(http.HandlerFunc) http.HandlerFunc) func(http.HandlerFunc) http.HandlerFunc {
	absolute := academyStorageRoot()
	h := academy.New(db, func(r *http.Request) (int64, error) {
		u, e := userFromRequest(r)
		if e != nil {
			return 0, e
		}
		return u.ID, nil
	}, gate, absolute)
	h.SetStorage(academyStorageOptions())
	h.SetCommunicationBase(applicationBaseURL())
	h.SetAdminResolver(resolveAcademyAdministrator)
	h.SetPaidInvitationEnqueuer(enqueueAcademyPaidInvitation)
	h.Register(http.DefaultServeMux)
	http.HandleFunc("/academy/unsubscribe/", serveFrontendPage("static/courses-unavailable.html"))
	http.HandleFunc("/profile/academy/notifications", gate(serveFrontendPage("static/courses-unavailable.html")))
	http.HandleFunc("/profile/academy", gate(serveFrontendPage("static/courses-unavailable.html")))
	http.HandleFunc("/profile/learning", gate(serveFrontendPage("static/courses-unavailable.html")))
	http.HandleFunc("/profile/learning/", gate(serveFrontendPage("static/courses-unavailable.html")))
	return h.CoursePage
}

func startAcademyWorker(ctx context.Context, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				jobCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
				err := academy.DeliverCohortNotifications(jobCtx, db)
				cancel()
				if err != nil && ctx.Err() == nil {
					log.Print("academy cohort notification delivery failed")
				}
			}
		}
	}()
}
