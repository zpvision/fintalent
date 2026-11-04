package objectstorage

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var keyPattern = regexp.MustCompile(`^[a-zA-Z0-9_/-]+(?:\.[a-zA-Z0-9]+)?$`)

func ValidKey(key string) bool {
	return len(key) > 0 && len(key) <= 512 && keyPattern.MatchString(key) && !strings.HasPrefix(key, "/") && !strings.Contains(key, "..") && !strings.Contains(key, "//")
}

type Local struct{ Root string }

func (s Local) path(key string) (string, error) {
	if !ValidKey(key) {
		return "", ErrInvalid
	}
	return filepath.Join(s.Root, filepath.FromSlash(key)), nil
}
func (s Local) Put(ctx context.Context, key string, r io.Reader, size int64, typ string) error {
	p, e := s.path(key)
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		return ErrUnavailable
	}
	f, e := os.CreateTemp(filepath.Dir(p), ".upload-")
	if e != nil {
		return ErrUnavailable
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	n, e := io.Copy(f, io.LimitReader(&contextReader{ctx, r}, size+1))
	ce := f.Close()
	if e != nil || ce != nil || n != size {
		return ErrUnavailable
	}
	if e = os.Rename(tmp, p); e != nil {
		return ErrUnavailable
	}
	return nil
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r *contextReader) Read(b []byte) (int, error) {
	if e := r.ctx.Err(); e != nil {
		return 0, e
	}
	return r.r.Read(b)
}
func (s Local) Open(ctx context.Context, key, byteRange string) (io.ReadCloser, Info, error) {
	p, e := s.path(key)
	if e != nil {
		return nil, Info{}, e
	}
	f, e := os.Open(p)
	if os.IsNotExist(e) {
		return nil, Info{}, ErrNotFound
	}
	if e != nil {
		return nil, Info{}, ErrUnavailable
	}
	st, e := f.Stat()
	if e != nil {
		f.Close()
		return nil, Info{}, ErrUnavailable
	}
	if byteRange != "" {
		f.Close()
		return nil, Info{}, ErrInvalid
	}
	return f, Info{Size: st.Size()}, nil
}
func (s Local) Stat(ctx context.Context, key string) (Info, error) {
	r, i, e := s.Open(ctx, key, "")
	if r != nil {
		r.Close()
	}
	return i, e
}
func (s Local) Delete(ctx context.Context, key string) error {
	p, e := s.path(key)
	if e != nil {
		return e
	}
	e = os.Remove(p)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return ErrUnavailable
	}
	return nil
}
