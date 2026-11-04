// Explicit administrator command. It never starts the app, applies schema or removes sources.
package main

import (
	"FinTalent/internal/academy"
	"FinTalent/internal/objectstorage"
	"context"
	"database/sql"
	"flag"
	"fmt"
	_ "github.com/jackc/pgx/v5/stdlib"
	"os"
	"path/filepath"
	"time"
)

func main() {
	apply := flag.Bool("apply", false, "copy and verify objects; default is a dry run")
	root := flag.String("local-root", os.Getenv("ACADEMY_STORAGE_DIR"), "existing private local directory")
	flag.Parse()
	if *root == "" {
		*root = filepath.Join(".local-data", "academy")
	}
	cfg, e := objectstorage.ConfigFrom(os.Getenv, false)
	if e != nil || cfg.Driver != "s3" {
		fatal("S3 configuration is required")
	}
	db, e := sql.Open("pgx", os.Getenv("DATABASE_URL"))
	if e != nil {
		fatal("invalid database configuration")
	}
	defer db.Close()
	db.SetMaxOpenConns(2)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if e = db.PingContext(ctx); e != nil {
		fatal("database unavailable")
	}
	s := objectstorage.NewS3(cfg)
	if *apply {
		if e = s.Check(ctx); e != nil {
			fatal("S3 unavailable or access denied")
		}
	}
	h := academy.New(db, nil, nil, *root)
	h.SetStorage(academy.StorageOptions{Primary: s, Multipart: s, Driver: "s3", TTL: cfg.TTL})
	var after int64
	count := 0
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		rows, e := db.QueryContext(ctx, `SELECT id FROM academy_media_assets WHERE storage_driver='local' AND deleted_at IS NULL AND id>$1 ORDER BY id LIMIT 100`, after)
		if e != nil {
			cancel()
			fatal("cannot list local materials; update the application schema first")
		}
		ids := []int64{}
		for rows.Next() {
			var id int64
			if e = rows.Scan(&id); e != nil {
				rows.Close()
				cancel()
				fatal("cannot read material ID")
			}
			ids = append(ids, id)
		}
		e = rows.Err()
		rows.Close()
		cancel()
		if e != nil {
			fatal("cannot read materials")
		}
		if len(ids) == 0 {
			break
		}
		for _, id := range ids {
			after = id
			if *apply {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				e = h.MigrateLocalAsset(ctx, id)
				cancel()
				if e != nil {
					fmt.Fprintf(os.Stderr, "Material %d failed; source retained. Retry after diagnosis.\n", id)
					os.Exit(1)
				}
			}
			count++
		}
	}
	fmt.Printf("Materials processed: %d; applied: %t; local source files retained.\n", count, *apply)
}
func fatal(message string) { fmt.Fprintln(os.Stderr, message); os.Exit(1) }
