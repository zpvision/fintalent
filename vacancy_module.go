package main

import (
	"context"
	"embed"
	"net/http"

	vacancyhandler "FinTalent/internal/vacancymodule/handler"
	vacancyrepository "FinTalent/internal/vacancymodule/repository"
	vacancyservice "FinTalent/internal/vacancymodule/service"
)

//go:embed migrations/005_vacancy_module.sql migrations/006_vacancy_per_answer_importance.sql migrations/007_dictionary_vacancy_importance.sql migrations/008_dictionary_single_choice_and_compact_blocks.sql migrations/009_dictionary_vacancy_title.sql migrations/010_duties.sql migrations/011_vacancy_finance.sql migrations/012_resume_experience.sql migrations/019_resume_education.sql migrations/020_kpi_dictionary_icons.sql migrations/021_resume_work_preferences.sql migrations/038_vacancy_contractor_types.sql
var vacancyMigrationFS embed.FS

func prepareVacancyModuleDatabase(ctx context.Context) error {
	schema, err := vacancyMigrationFS.ReadFile("migrations/005_vacancy_module.sql")
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, string(schema))
	if err != nil {
		return err
	}
	schema, err = vacancyMigrationFS.ReadFile("migrations/006_vacancy_per_answer_importance.sql")
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, string(schema))
	if err != nil {
		return err
	}
	schema, err = vacancyMigrationFS.ReadFile("migrations/007_dictionary_vacancy_importance.sql")
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, string(schema))
	if err != nil {
		return err
	}
	schema, err = vacancyMigrationFS.ReadFile("migrations/008_dictionary_single_choice_and_compact_blocks.sql")
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, string(schema))
	if err != nil {
		return err
	}
	schema, err = vacancyMigrationFS.ReadFile("migrations/009_dictionary_vacancy_title.sql")
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, string(schema))
	if err != nil {
		return err
	}
	schema, err = vacancyMigrationFS.ReadFile("migrations/010_duties.sql")
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, string(schema))
	if err != nil {
		return err
	}
	schema, err = vacancyMigrationFS.ReadFile("migrations/011_vacancy_finance.sql")
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, string(schema))
	if err != nil {
		return err
	}
	schema, err = vacancyMigrationFS.ReadFile("migrations/012_resume_experience.sql")
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, string(schema))
	if err != nil {
		return err
	}
	schema, err = vacancyMigrationFS.ReadFile("migrations/019_resume_education.sql")
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, string(schema))
	if err != nil {
		return err
	}
	schema, err = vacancyMigrationFS.ReadFile("migrations/020_kpi_dictionary_icons.sql")
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, string(schema))
	if err != nil {
		return err
	}
	schema, err = vacancyMigrationFS.ReadFile("migrations/021_resume_work_preferences.sql")
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, string(schema))
	if err != nil {
		return err
	}
	schema, err = vacancyMigrationFS.ReadFile("migrations/038_vacancy_contractor_types.sql")
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, string(schema))
	return err
}

func registerVacancyModuleRoutes() {
	repository := vacancyrepository.New(db)
	service := vacancyservice.New(repository)
	handler := vacancyhandler.New(service, func(r *http.Request) (int64, error) {
		u, err := userFromRequest(r)
		if err != nil {
			return 0, err
		}
		return u.ID, nil
	})
	handler.Register(http.DefaultServeMux)
}
