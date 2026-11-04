// Package objectstorage is the provider-neutral boundary for Academy files.
package objectstorage

import (
	"context"
	"errors"
	"io"
	"time"
)

var ErrNotFound = errors.New("object not found")
var ErrUnavailable = errors.New("object storage unavailable")
var ErrInvalid = errors.New("invalid object storage request")

type Info struct {
	Size              int64
	ContentType, ETag string
}
type Part struct {
	Number int    `json:"number"`
	Size   int64  `json:"size"`
	ETag   string `json:"etag"`
}
type Signed struct {
	URL       string            `json:"url"`
	Headers   map[string]string `json:"headers"`
	ExpiresAt time.Time         `json:"expires_at"`
}
type Store interface {
	Put(context.Context, string, io.Reader, int64, string) error
	Open(context.Context, string, string) (io.ReadCloser, Info, error)
	Stat(context.Context, string) (Info, error)
	Delete(context.Context, string) error
}
type Multipart interface {
	Store
	Begin(context.Context, string, string) (string, error)
	SignPart(context.Context, string, string, int, int64, time.Duration) (Signed, error)
	Parts(context.Context, string, string) ([]Part, error)
	Complete(context.Context, string, string, []Part) error
	Abort(context.Context, string, string) error
	SignGet(context.Context, string, string, string, time.Duration) (Signed, error)
	Identity() string
}
