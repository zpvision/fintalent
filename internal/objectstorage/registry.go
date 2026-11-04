package objectstorage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"io"
	"strconv"
	"strings"
	"time"
)

// Owner denotes the actual business entity, rather than a randomly selected employee.
type Owner struct {
	Kind string `json:"kind"`
	ID   int64  `json:"id"`
}
type RegisteredFile struct {
	Ref       string `json:"legacy_ref"`
	Key       string `json:"object_key,omitempty"`
	Name      string `json:"original_name"`
	MIME      string `json:"mime"`
	SHA256    string `json:"sha256"`
	Status    string `json:"status,omitempty"`
	Owner     Owner  `json:"owner"`
	Category  string `json:"category"`
	Size      int64  `json:"size"`
	CreatedBy int64  `json:"created_by,omitempty"`
	Storage   string `json:"storage_id,omitempty"`
}
type Registry struct {
	DB    *sql.DB
	Store Store
}

func OwnerKey(owner Owner, category string) (string, error) {
	if owner.Kind == "user" {
		return UserKey(owner.ID, category)
	}
	if owner.Kind != "accounting_company" && owner.Kind != "platform" {
		return "", ErrInvalid
	}
	if owner.Kind != "platform" && owner.ID <= 0 || owner.Kind == "platform" && owner.ID != 0 {
		return "", ErrInvalid
	}
	if category != "academy" && category != "tests" && category != "files" {
		return "", ErrInvalid
	}
	token, e := randomToken()
	if e != nil {
		return "", e
	}
	if owner.Kind == "platform" {
		return "platform/" + category + "/" + token, nil
	}
	return "organizations/accounting-companies/" + strconv.FormatInt(owner.ID, 10) + "/" + category + "/" + token, nil
}

func (r Registry) Lookup(ctx context.Context, ref string) (RegisteredFile, error) {
	var f RegisteredFile
	if r.DB == nil {
		return f, ErrUnavailable
	}
	e := r.DB.QueryRowContext(ctx, `SELECT legacy_ref,owner_kind,owner_id,category,object_key,original_name,mime,size,sha256,status,COALESCE(created_by,0),storage_id FROM object_files WHERE legacy_ref=$1`, ref).Scan(&f.Ref, &f.Owner.Kind, &f.Owner.ID, &f.Category, &f.Key, &f.Name, &f.MIME, &f.Size, &f.SHA256, &f.Status, &f.CreatedBy, &f.Storage)
	if e == nil && f.Storage != "primary" {
		return f, ErrUnavailable
	}
	return f, e
}

// CopyVerified never deletes or modifies the source. Ready is committed only after
// a complete read-back checksum. A retry keeps the same immutable object key.
// The caller must serialize operations for the same source and authorize reads.
func (r Registry) CopyVerified(ctx context.Context, f RegisteredFile, source io.ReadSeeker) error {
	if r.Store == nil || r.DB == nil {
		return ErrUnavailable
	}
	if f.Ref == "" || f.Name == "" || f.Size < 0 {
		return ErrInvalid
	}
	if strings.HasPrefix(f.Ref, "/static/uploads/") && !PublicUploadRef(f.Ref) {
		return ErrInvalid
	}
	key, e := OwnerKey(f.Owner, f.Category)
	if e != nil {
		return e
	}
	var ownerExists bool
	switch f.Owner.Kind {
	case "user":
		e = r.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1)`, f.Owner.ID).Scan(&ownerExists)
	case "accounting_company":
		e = r.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM accounting_companies WHERE id=$1)`, f.Owner.ID).Scan(&ownerExists)
	case "platform":
		ownerExists = true
	}
	if e != nil {
		return e
	}
	if !ownerExists {
		return ErrInvalid
	}
	_, e = r.DB.ExecContext(ctx, `INSERT INTO object_files(legacy_ref,owner_kind,owner_id,category,object_key,original_name,mime,size,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9,0)) ON CONFLICT(legacy_ref) DO NOTHING`, f.Ref, f.Owner.Kind, f.Owner.ID, f.Category, key, f.Name, f.MIME, f.Size, f.CreatedBy)
	if e != nil {
		return e
	}
	saved, e := r.Lookup(ctx, f.Ref)
	if e != nil {
		return e
	}
	if saved.Owner != f.Owner || saved.Category != f.Category || saved.Size != f.Size || saved.MIME != f.MIME {
		return ErrInvalid
	}
	digest := sha256.New()
	if _, e = source.Seek(0, 0); e != nil {
		return e
	}
	n, e := io.Copy(digest, io.LimitReader(source, f.Size+1))
	if e != nil || n != f.Size {
		return ErrInvalid
	}
	sum := hex.EncodeToString(digest.Sum(nil))
	if f.SHA256 != "" && f.SHA256 != sum {
		return ErrInvalid
	}
	if saved.Status == "ready" {
		if saved.SHA256 != sum {
			return ErrInvalid
		}
		body, info, err := r.Store.Open(ctx, saved.Key, "")
		if err != nil {
			return err
		}
		digest.Reset()
		n, err := io.Copy(digest, io.LimitReader(body, f.Size+1))
		body.Close()
		if err != nil || n != f.Size || info.Size != f.Size || hex.EncodeToString(digest.Sum(nil)) != sum {
			return ErrUnavailable
		}
		return nil
	}
	claim, e := r.DB.ExecContext(ctx, `UPDATE object_files SET lease_until=now()+interval '10 minutes' WHERE legacy_ref=$1 AND status<>'ready' AND (lease_until IS NULL OR lease_until<now())`, f.Ref)
	if e != nil {
		return e
	}
	affected, e := claim.RowsAffected()
	if e != nil {
		return e
	}
	if affected != 1 {
		return ErrUnavailable
	}
	defer func() {
		clean, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_, _ = r.DB.ExecContext(clean, `UPDATE object_files SET lease_until=NULL WHERE legacy_ref=$1 AND status<>'ready'`, f.Ref)
	}()
	if _, e = source.Seek(0, 0); e != nil {
		return e
	}
	if e = r.Store.Put(ctx, saved.Key, source, f.Size, f.MIME); e != nil {
		return e
	}
	body, info, e := r.Store.Open(ctx, saved.Key, "")
	if e != nil {
		return e
	}
	digest.Reset()
	n, e = io.Copy(digest, io.LimitReader(body, f.Size+1))
	body.Close()
	if e != nil || n != f.Size || info.Size != f.Size || hex.EncodeToString(digest.Sum(nil)) != sum {
		return ErrUnavailable
	}
	_, e = r.DB.ExecContext(ctx, `UPDATE object_files SET status='ready',sha256=$2,verified_at=now(),lease_until=NULL WHERE legacy_ref=$1 AND status<>'ready'`, f.Ref, sum)
	return e
}
