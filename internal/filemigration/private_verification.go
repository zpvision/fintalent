package filemigration

import (
	"FinTalent/internal/objectstorage"
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var ErrPrivateExposure = errors.New("private endpoint did not deny anonymous access")

func IsPrivateSource(s Source) bool {
	return s.ID > 0 && s.File.Owner.Kind == "user" && s.File.Owner.ID > 0 &&
		(s.Type == "test_material" && s.File.Ref == "test-material:"+strconv.FormatInt(s.ID, 10) ||
			s.Type == "file" && strings.HasPrefix(s.File.Ref, "certificate:"))
}

// VerifyPrivateAnonymous uses no cookie, invitation token, or administrator
// impersonation. This checks denial, not successful access by a real owner.
func VerifyPrivateAnonymous(ctx context.Context, base string, s Source) error {
	if !IsPrivateSource(s) || ValidateApplicationBase(base) != nil {
		return objectstorage.ErrInvalid
	}
	path := "/api/test-materials/" + strconv.FormatInt(s.ID, 10)
	if s.Type == "file" {
		path = "/api/v1/resumes/education-certificate?id=" + strconv.FormatInt(s.ID, 10)
	}
	req, err := http.NewRequestWithContext(ctx, "GET", strings.TrimRight(base, "/")+path, nil)
	if err != nil {
		return objectstorage.ErrInvalid
	}
	req.Header.Set("Cache-Control", "no-cache")
	client := http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return objectstorage.ErrUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode != 401 && resp.StatusCode != 403 && resp.StatusCode != 404 {
		return ErrPrivateExposure
	}
	return nil
}

// Called only after Apply has validated the original owner/source and performed
// a full streaming read-back SHA-256 check. The explicit CLI journal records this
// alternative administrative verification; it never claims an owner API login.
func VerifyPrivateAdministration(ctx context.Context, r objectstorage.Registry, base string, s Source) error {
	if !IsPrivateSource(s) {
		return objectstorage.ErrInvalid
	}
	f, err := r.Lookup(ctx, s.File.Ref)
	if err != nil {
		return err
	}
	if f.Status != "ready" || f.Owner != s.File.Owner || f.Size != s.File.Size || f.SHA256 != s.File.SHA256 || f.Category != s.File.Category {
		return objectstorage.ErrInvalid
	}
	return VerifyPrivateAnonymous(ctx, base, s)
}
