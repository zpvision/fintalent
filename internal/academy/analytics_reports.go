package academy

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var reportKinds = []string{"students", "progress", "tests", "assignments", "cohorts", "staff", "communications", "cohort", "performance", "risk", "summary"}

func reportKind(kind string) bool {
	for _, v := range reportKinds {
		if kind == v {
			return true
		}
	}
	return false
}

type analyticsReportJob struct {
	AdminKey   string            `json:"admin_actor_key"`
	ID         int64             `json:"id"`
	Course     int64             `json:"course_id"`
	Actor      int64             `json:"created_by"`
	Kind       string            `json:"kind"`
	Format     string            `json:"format"`
	Filters    AnalyticsFilters  `json:"filters"`
	Settings   AnalyticsSettings `json:"settings"`
	Status     string            `json:"status"`
	RequestKey string            `json:"request_key"`
	Scope      []int64           `json:"scope_ids"`
	Cohorts    []int64           `json:"scope_cohorts"`
	OwnerScope bool              `json:"owner_scope"`
	Lease      string            `json:"lease_key"`
	Storage    string            `json:"storage_key"`
	Created    time.Time         `json:"created_at"`
	Expires    time.Time         `json:"expires_at"`
	Rows       int               `json:"rows_count"`
	Size       int64             `json:"size_bytes"`
	Error      string            `json:"error_code"`
}

func (h *Handler) analyticsReports(w http.ResponseWriter, r *http.Request, course, actor int64, path []string) {
	owner := h.courseOwner(r, course, actor)
	if len(path) == 0 && r.Method == "GET" {
		var raw json.RawMessage
		err := h.db.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY x.id DESC),'[]') FROM(SELECT id,kind,format,filters,settings,created_at,finished_at,expires_at,rows_count,size_bytes,error_code,CASE WHEN expires_at<=now() THEN 'expired' ELSE status END status FROM academy_report_jobs WHERE course_id=$1 AND created_by IS NOT DISTINCT FROM NULLIF($2,0) AND admin_actor_key=$3 AND deleted_at IS NULL ORDER BY id DESC LIMIT 100)x`, course, actor, adminKey(r.Context())).Scan(&raw)
		if err != nil {
			analyticsProblem(w, err)
			return
		}
		respond(w, 200, raw)
		return
	}
	if len(path) == 0 && r.Method == "POST" {
		var b struct {
			Kind    string           `json:"kind"`
			Format  string           `json:"format"`
			Filters AnalyticsFilters `json:"filters"`
			Key     string           `json:"request_key"`
		}
		if !decode(w, r, &b) {
			return
		}
		if !reportKind(b.Kind) || !oneOf(b.Format, "csv", "xlsx") || !keyPattern.MatchString(b.Key) {
			analyticsProblem(w, ErrInvalid)
			return
		}
		f, e := parseAnalyticsFilters(b.Filters.values(), time.Now())
		if e != nil {
			analyticsProblem(w, e)
			return
		}
		f.Page = 1
		f.Size = 100
		if reportCommunicationKind(b.Kind) && !h.communicationAccess(r, course, actor) {
			analyticsProblem(w, ErrAccess)
			return
		}
		tx, e := h.db.BeginTx(r.Context(), nil)
		if e != nil {
			analyticsProblem(w, e)
			return
		}
		defer tx.Rollback()
		_, e = tx.ExecContext(r.Context(), `SELECT pg_advisory_xact_lock(hashtext('academy:reports'),$1::integer)`, actor)
		if e != nil {
			analyticsProblem(w, e)
			return
		}
		var id int64
		var same bool
		e = tx.QueryRowContext(r.Context(), `SELECT id,kind=$4 AND format=$5 AND filters=$6::jsonb AND deleted_at IS NULL FROM academy_report_jobs WHERE request_key=$1 AND course_id=$2 AND created_by IS NOT DISTINCT FROM NULLIF($3,0) AND admin_actor_key=$7`, b.Key, course, actor, b.Kind, b.Format, jsonValue(f), adminKey(r.Context())).Scan(&id, &same)
		if e == nil {
			if !same {
				fail(w, 409, "Этот ключ уже использован для другого отчёта")
				return
			}
			respond(w, 202, map[string]any{"id": id, "queued": true})
			return
		}
		if e != sql.ErrNoRows {
			analyticsProblem(w, e)
			return
		}
		var pending, hour int
		e = tx.QueryRowContext(r.Context(), `SELECT count(*) FILTER(WHERE status IN('queued','running')),count(*) FILTER(WHERE created_at>now()-INTERVAL '1 hour') FROM academy_report_jobs WHERE created_by IS NOT DISTINCT FROM NULLIF($1,0) AND admin_actor_key=$2`, actor, adminKey(r.Context())).Scan(&pending, &hour)
		if e != nil {
			analyticsProblem(w, e)
			return
		}
		if pending >= 3 || hour >= 10 {
			fail(w, 429, "Не более трёх ожидающих отчётов и десяти формирований в час")
			return
		}
		retention, e := academySettings(r.Context(), tx)
		if e != nil {
			analyticsProblem(w, e)
			return
		}
		settings, e := analyticsSettings(r.Context(), tx, course)
		if e != nil {
			analyticsProblem(w, e)
			return
		}
		e = tx.QueryRowContext(r.Context(), `INSERT INTO academy_report_jobs(course_id,created_by,kind,format,filters,settings,owner_scope,request_key,admin_actor_key,expires_at) VALUES($1,NULLIF($2,0),$3,$4,$5,$6,$7,$8,$9,now()+make_interval(days=>$10)) RETURNING id`, course, actor, b.Kind, b.Format, jsonValue(f), jsonValue(settings), owner, b.Key, adminKey(r.Context()), retention.ReportDays).Scan(&id)
		if e == nil {
			e = communicationAudit(r.Context(), tx, course, actor, "report", id, "created", map[string]any{"kind": b.Kind, "format": b.Format, "filters": f})
		}
		if e == nil {
			e = tx.Commit()
		}
		if e != nil {
			analyticsProblem(w, e)
			return
		}
		respond(w, 202, map[string]any{"id": id, "status": "queued"})
		return
	}
	if len(path) == 0 {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	id, e := strconv.ParseInt(path[0], 10, 64)
	if e != nil || id <= 0 {
		analyticsProblem(w, sql.ErrNoRows)
		return
	}
	var job analyticsReportJob
	var raw []byte
	e = h.db.QueryRowContext(r.Context(), `SELECT to_jsonb(j) FROM academy_report_jobs j WHERE id=$1 AND course_id=$2 AND created_by IS NOT DISTINCT FROM NULLIF($3,0) AND admin_actor_key=$4 AND deleted_at IS NULL`, id, course, actor, adminKey(r.Context())).Scan(&raw)
	if e != nil {
		analyticsProblem(w, e)
		return
	}
	if e = json.Unmarshal(raw, &job); e != nil {
		analyticsProblem(w, e)
		return
	}
	if len(path) == 1 && r.Method == "GET" {
		if !job.Expires.After(time.Now()) {
			job.Status = "expired"
		}
		respond(w, 200, map[string]any{"id": job.ID, "kind": job.Kind, "format": job.Format, "filters": job.Filters, "status": job.Status, "expires_at": job.Expires, "rows_count": job.Rows, "error_code": job.Error})
		return
	}
	if len(path) == 1 && r.Method == "DELETE" {
		tx, e := h.db.BeginTx(r.Context(), nil)
		if e != nil {
			analyticsProblem(w, e)
			return
		}
		defer tx.Rollback()
		_, e = tx.ExecContext(r.Context(), `UPDATE academy_report_jobs SET status='cancelled',deleted_at=now(),lease_key=NULL WHERE id=$1`, id)
		if e == nil {
			e = communicationAudit(r.Context(), tx, course, actor, "report", id, "deleted", nil)
		}
		if e == nil {
			e = tx.Commit()
		}
		if e != nil {
			analyticsProblem(w, e)
			return
		}
		if job.Storage != "" {
			if p, e := reportPath(h.root, job.Storage); e == nil {
				_ = os.Remove(p)
			}
		}
		respond(w, 200, map[string]bool{"deleted": true})
		return
	}
	if len(path) != 2 || path[1] != "file" || r.Method != "GET" {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	if job.Status != "ready" || job.Expires.Before(time.Now()) {
		fail(w, 409, "Отчёт ещё не готов или срок хранения закончился")
		return
	}
	if job.OwnerScope && !owner {
		analyticsProblem(w, ErrAccess)
		return
	}
	var authorized bool
	e = h.db.QueryRowContext(r.Context(), `SELECT $6 OR (NOT EXISTS(SELECT 1 FROM jsonb_array_elements_text($1::jsonb) x WHERE NOT academy_can_manage_enrollment(x.value::bigint,$2,false) OR($3 AND NOT academy_can_communicate(x.value::bigint,$2))) AND($4 OR NOT EXISTS(SELECT 1 FROM jsonb_array_elements_text($5::jsonb) x WHERE NOT EXISTS(SELECT 1 FROM academy_cohort_staff s WHERE s.cohort_id=x.value::bigint AND s.user_id=$2))))`, jsonValue(job.Scope), actor, reportCommunicationKind(job.Kind), owner, jsonValue(job.Cohorts), administrative(r.Context())).Scan(&authorized)
	if e != nil {
		analyticsProblem(w, e)
		return
	}
	if !authorized {
		fail(w, 403, "Область доступа изменилась. Сформируйте новый отчёт по доступным ученикам")
		return
	}
	p, e := reportPath(h.root, job.Storage)
	if e != nil {
		analyticsProblem(w, sql.ErrNoRows)
		return
	}
	file, e := os.Open(p)
	if e != nil {
		analyticsProblem(w, sql.ErrNoRows)
		return
	}
	defer file.Close()
	stat, e := file.Stat()
	if e != nil {
		analyticsProblem(w, e)
		return
	}
	tx, e := h.db.BeginTx(r.Context(), nil)
	if e != nil {
		analyticsProblem(w, e)
		return
	}
	defer tx.Rollback()
	e = communicationAudit(r.Context(), tx, course, actor, "report", id, "downloaded", nil)
	if e == nil {
		e = tx.Commit()
	}
	if e != nil {
		analyticsProblem(w, e)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", `attachment; filename="fintalent-academy-`+strconv.FormatInt(id, 10)+`.`+job.Format+`"`)
	if job.Format == "xlsx" {
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	} else {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	}
	http.ServeContent(w, r, stat.Name(), stat.ModTime(), file)
}

// One bounded job per call; the existing application lifecycle invokes it. PostgreSQL owns scheduling/leases.
func ProcessAnalyticsReports(ctx context.Context, db *sql.DB, root string) error {
	// Only this writer's expired temporary files are eligible; never traverse directories or symlinks.
	if entries, err := os.ReadDir(filepath.Join(root, "reports")); err == nil {
		for _, entry := range entries {
			if entry.Type().IsRegular() && strings.HasPrefix(entry.Name(), ".academy-report-") {
				if info, err := entry.Info(); err == nil && time.Since(info.ModTime()) > 24*time.Hour {
					_ = os.Remove(filepath.Join(root, "reports", entry.Name()))
				}
			}
		}
	}
	var expired []struct {
		ID  int64  `json:"id"`
		Key string `json:"storage_key"`
	}
	if e := analyticsJSON(ctx, db, `SELECT COALESCE(jsonb_agg(to_jsonb(x)),'[]') FROM(SELECT id,storage_key FROM academy_report_jobs WHERE (expires_at<=now() OR deleted_at IS NOT NULL) AND storage_key IS NOT NULL LIMIT 50)x`, &expired); e != nil {
		return e
	}
	for _, j := range expired {
		p, e := reportPath(root, j.Key)
		if e != nil {
			return e
		}
		e = os.Remove(p)
		if e != nil && !errors.Is(e, os.ErrNotExist) {
			continue
		}
		if _, e = db.ExecContext(ctx, `UPDATE academy_report_jobs SET status=CASE WHEN deleted_at IS NULL THEN 'expired' ELSE 'cancelled' END,storage_key=NULL WHERE id=$1`, j.ID); e != nil {
			return e
		}
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	_, e = tx.ExecContext(ctx, `UPDATE academy_report_jobs SET status='expired',lease_key=NULL WHERE expires_at<=now() AND status<>'expired' AND deleted_at IS NULL AND storage_key IS NULL`)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, `UPDATE academy_report_jobs SET status='queued',lease_key=NULL,error_code='worker_restarted' WHERE status='running' AND started_at<now()-INTERVAL '10 minutes' AND deleted_at IS NULL`)
	if e != nil {
		return e
	}
	var raw []byte
	e = tx.QueryRowContext(ctx, `SELECT to_jsonb(j) FROM academy_report_jobs j WHERE status='queued' AND expires_at>now() AND deleted_at IS NULL ORDER BY created_at,id FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&raw)
	if e == sql.ErrNoRows {
		return tx.Commit()
	}
	if e != nil {
		return e
	}
	var job analyticsReportJob
	if e = json.Unmarshal(raw, &job); e != nil {
		return e
	}
	oldStorage := job.Storage
	job.Lease = Key()
	job.Storage = Key() + "." + job.Format
	_, e = tx.ExecContext(ctx, `UPDATE academy_report_jobs SET status='running',started_at=now(),lease_key=$2,storage_key=$3,error_code='' WHERE id=$1`, job.ID, job.Lease, job.Storage)
	if e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	if oldStorage != "" {
		if p, er := reportPath(root, oldStorage); er == nil {
			_ = os.Remove(p)
		}
	}
	if job.AdminKey != "" {
		var principal AdminPrincipal
		if job.AdminKey == "platform_admin" {
			principal = AdminPrincipal{Key: job.AdminKey, Role: "superadmin", Name: "Администратор FinTalent"}
		} else {
			principal, e = ResolveAdminGrant(ctx, db, job.Actor)
		}
		if e != nil || !principal.Can("analytics") {
			_, er := db.ExecContext(ctx, "UPDATE academy_report_jobs SET status='failed',error_code='permission_changed',lease_key=NULL WHERE id=$1 AND lease_key=$2", job.ID, job.Lease)
			return er
		}
		ctx = adminScope(ctx, principal)
	}
	h := New(db, nil, nil, root)
	f, e := parseAnalyticsFilters(job.Filters.values(), job.Created)
	var d *analyticsDataset
	var rows [][]string
	var size int64
	if e == nil {
		d, e = h.loadAnalytics(ctx, job.Course, job.Actor, f, reportCommunicationKind(job.Kind), &job.Settings)
	}
	if e == nil && job.OwnerScope && !d.Owner {
		e = ErrAccess
	}
	if e == nil {
		rows, e = d.exportRows(job.Kind)
	}
	if e == nil {
		e = ctx.Err()
	}
	if e == nil {
		size, e = writeReportFile(root, job.Storage, job.Format, rows)
	}
	status, code := "ready", ""
	if e != nil {
		status = "failed"
		code = "generation_failed"
		if errors.Is(e, ErrAccess) || errors.Is(e, sql.ErrNoRows) {
			code = "permission_changed"
		}
		var in inputError
		if errors.As(e, &in) {
			code = "selection_too_large_or_invalid"
		}
		if errors.Is(e, context.DeadlineExceeded) {
			code = "calculation_timeout"
		}
	}
	scope, cohorts := []int64{}, []int64{}
	if d != nil {
		for _, s := range d.Students {
			scope = append(scope, s.ID)
		}
		for id := range d.Cohorts {
			cohorts = append(cohorts, id)
		}
	}
	finish, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	res, e := db.ExecContext(finish, `UPDATE academy_report_jobs SET status=$3,error_code=$4,scope_ids=$5,scope_cohorts=$6,rows_count=$7,size_bytes=$8,finished_at=now(),lease_key=NULL WHERE id=$1 AND lease_key=$2 AND status='running' AND deleted_at IS NULL AND expires_at>now()`, job.ID, job.Lease, status, code, jsonValue(scope), jsonValue(cohorts), max(0, len(rows)-1), size)
	if e != nil {
		return e
	}
	n, e := res.RowsAffected()
	if e != nil {
		return e
	}
	if n == 0 || status != "ready" {
		if p, er := reportPath(root, job.Storage); er == nil {
			_ = os.Remove(p)
		}
	}
	return nil
}
