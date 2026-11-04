// Explicit offline administrator tool. No schema changes, source deletions,
// authentication changes, service restarts or implicit production migration.
package main

import (
	"FinTalent/internal/filemigration"
	"FinTalent/internal/objectstorage"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	_ "github.com/jackc/pgx/v5/stdlib"
	"net/url"
	"os"
	"strings"
	"time"
)

func main() {
	root := flag.String("root", ".", "installation root containing static/uploads and uploads/resume-certificates")
	inventory := flag.String("inventory", "", "write a private inventory manifest; never copy files")
	manifest := flag.String("manifest", "", "reviewed inventory manifest")
	journal := flag.String("journal", "", "private append-only JSONL result journal (required with --apply)")
	apply := flag.Bool("apply", false, "copy and verify sources, then enable S3 reads")
	privateAdmin := flag.Bool("private-admin-verification", false, "private files only: explicit administrative verification, without owner impersonation")
	rollback := flag.Bool("rollback", false, "restore legacy reads for the selected manifest entries; retain all objects")
	batch := flag.Int("batch-size", 100, "maximum number of selected entries")
	after := flag.String("after", "", "select legacy refs lexicographically after this cursor")
	appURL := flag.String("app-url", "", "verify application reads after copying (required with --apply)")
	flag.Parse()
	if *privateAdmin && (!*apply || *rollback || *inventory != "" || *appURL == "") {
		fatal("private admin verification requires apply, manifest, journal and app-url")
	}
	if *batch < 1 || *batch > 1000 || (*apply && *rollback) || (*inventory != "" && (*apply || *rollback || *manifest != "")) {
		fatal("invalid options")
	}
	db, e := sql.Open("pgx", os.Getenv("DATABASE_URL"))
	if e != nil {
		fatal("database configuration invalid")
	}
	defer db.Close()
	db.SetMaxOpenConns(2)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if e = db.PingContext(ctx); e != nil {
		fatal("database unavailable")
	}
	if *inventory != "" {
		sources, e := filemigration.Inventory(ctx, db, *root)
		if e != nil {
			fatal("inventory failed; check schema and installation root")
		}
		out, e := os.OpenFile(*inventory, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			fatal("cannot create private manifest; existing file will not be overwritten")
		}
		e = json.NewEncoder(out).Encode(sources)
		closeErr := out.Close()
		if e != nil || closeErr != nil {
			fatal("manifest write failed")
		}
		unresolved := 0
		for _, s := range sources {
			if s.Problem != "" {
				unresolved++
			}
		}
		fmt.Printf("Inventory: %d entries; unresolved: %d; no files copied or records changed.\n", len(sources), unresolved)
		return
	}
	if *manifest == "" {
		fatal("specify --inventory or --manifest")
	}
	in, e := os.Open(*manifest)
	if e != nil {
		fatal("cannot open manifest")
	}
	sources, e := filemigration.ReadManifest(in)
	in.Close()
	if e != nil {
		fatal("invalid manifest")
	}
	if *privateAdmin {
		if os.Getenv("MIGRATION_APP_COOKIE") != "" {
			fatal("private admin verification must not use a user cookie")
		}
		for _, source := range sources {
			if !filemigration.IsPrivateSource(source) {
				fatal("private admin verification refuses public or unsupported sources")
			}
		}
	}
	registry := objectstorage.Registry{DB: db}
	if *apply {
		if *appURL == "" {
			fatal("--app-url is required to verify stable application URLs")
		}
		if filemigration.ValidateApplicationBase(*appURL) != nil {
			fatal("--app-url must be an HTTPS application origin, or HTTP loopback origin, without credentials, path or query")
		}
		if os.Getenv("MIGRATION_APP_COOKIE") != "" {
			expected, err := url.Parse(os.Getenv("APP_BASE_URL"))
			target, targetErr := url.Parse(*appURL)
			if err != nil || targetErr != nil || expected.Hostname() == "" || expected.User != nil || target.User != nil || expected.Scheme != target.Scheme || !strings.EqualFold(expected.Host, target.Host) {
				fatal("with MIGRATION_APP_COOKIE, --app-url must match the trusted APP_BASE_URL origin")
			}
		}
		cfg, e := objectstorage.ConfigFrom(os.Getenv, false)
		if e != nil || cfg.Driver != "s3" {
			fatal("valid S3 configuration required")
		}
		s := objectstorage.NewS3(cfg)
		if e = s.Check(ctx); e != nil {
			fatal("S3 unavailable or access denied")
		}
		registry.Store = s
	}
	var out *os.File
	if *apply || *rollback {
		if *journal == "" {
			fatal("--journal is required for mutations")
		}
		out, e = os.OpenFile(*journal, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if e != nil {
			fatal("cannot open private journal")
		}
		defer out.Close()
	}
	processed, failed := 0, 0
	last := ""
	for _, source := range sources {
		if source.File.Ref <= *after {
			continue
		}
		if processed >= *batch {
			break
		}
		processed++
		last = source.File.Ref
		status := "dry_run"
		var operationError error
		job, done := context.WithTimeout(context.Background(), 5*time.Minute)
		if *rollback {
			operationError = filemigration.Rollback(job, registry, *root, source)
			status = "legacy_reads_restored"
		} else if *apply {
			if *privateAdmin {
				operationError = filemigration.VerifyPrivateAnonymous(job, *appURL, source)
			}
			if operationError == nil {
				operationError = filemigration.Apply(job, registry, *root, source)
			}
			if operationError == nil {
				if *privateAdmin {
					operationError = filemigration.VerifyPrivateAdministration(job, registry, *appURL, source)
				} else {
					operationError = filemigration.VerifyApplication(job, registry, *appURL, os.Getenv("MIGRATION_APP_COOKIE"), source)
				}
				if operationError != nil {
					fix, cancelFix := context.WithTimeout(context.WithoutCancel(job), time.Minute)
					rollbackError := filemigration.Rollback(fix, registry, *root, source)
					cancelFix()
					if rollbackError != nil {
						status = "app_check_failed_rollback_requires_attention"
					}
				}
			}
			if status != "app_check_failed_rollback_requires_attention" {
				status = "copied_verified_app_checked"
				if *privateAdmin {
					status = "copied_verified_private_admin_checked"
				}
			}
		} else if source.Problem != "" {
			status = source.Problem
		}
		done()
		if operationError != nil {
			failed++
			if status != "app_check_failed_rollback_requires_attention" {
				status = "failed_source_retained"
			}
		}
		if out != nil {
			code := ""
			if operationError != nil {
				code = "operation_failed"
				if source.Problem != "" {
					code = source.Problem
				} else if errors.Is(operationError, filemigration.ErrPrivateExposure) {
					code = "private_endpoint_did_not_deny_anonymous_access"
				} else if errors.Is(operationError, objectstorage.ErrInvalid) {
					code = "source_or_metadata_changed"
				} else if errors.Is(operationError, objectstorage.ErrUnavailable) {
					code = "storage_or_application_unavailable"
				} else if errors.Is(operationError, filemigration.ErrApplicationAccess) {
					code = "application_authentication_or_access_denied"
				} else if errors.Is(operationError, filemigration.ErrApplicationResolver) {
					code = "application_bypassed_s3_resolver"
				}
			}
			e = json.NewEncoder(out).Encode(map[string]any{"time": time.Now().UTC(), "ref": source.File.Ref, "status": status, "error_code": code, "source_sha256": source.File.SHA256, "owner": source.File.Owner, "category": source.File.Category, "private_admin_verification": *privateAdmin, "owner_authenticated_read_tested": !*privateAdmin && *apply && os.Getenv("MIGRATION_APP_COOKIE") != "" && operationError == nil})
			if e != nil || out.Sync() != nil {
				fatal("journal write failed; stop and reconcile before retry")
			}
		}
		if status == "app_check_failed_rollback_requires_attention" || errors.Is(operationError, objectstorage.ErrUnavailable) || errors.Is(operationError, filemigration.ErrApplicationAccess) || errors.Is(operationError, filemigration.ErrApplicationResolver) || errors.Is(operationError, filemigration.ErrPrivateExposure) {
			fatal("storage/application check failed; batch stopped, inspect the journal before retry")
		}
	}
	fmt.Printf("Processed: %d; failed: %d; dry-run: %t; originals retained.\n", processed, failed, !*apply && !*rollback)
	// Cursor is a legacy file reference, never an endpoint or credential.
	fmt.Printf("Last cursor: %q\n", last)
	if failed > 0 {
		os.Exit(1)
	}
}
func fatal(message string) { fmt.Fprintln(os.Stderr, message); os.Exit(1) }
