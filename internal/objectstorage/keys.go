package objectstorage

import (
	"crypto/rand"
	"encoding/hex"
	"path"
	"strconv"
	"strings"
)

// PublicUploadRef preserves managed legacy URLs without exposing hidden files,
// traversal, Windows alternate streams, or private storage namespaces.
func PublicUploadRef(ref string) bool {
	if !strings.HasPrefix(ref, "/static/uploads/") || path.Clean(ref) != ref || strings.ContainsAny(ref, "\\:\x00\r\n") {
		return false
	}
	for _, part := range strings.Split(strings.TrimPrefix(ref, "/static/uploads/"), "/") {
		if part == "" || strings.HasPrefix(part, ".") {
			return false
		}
	}
	return true
}

// UserKey records the original storage owner, independently of later business ownership.
// Callers must persist this key and continue to authorize access through their domain.
func UserKey(user int64, category string) (string, error) {
	if user <= 0 || (category != "academy" && category != "tests" && category != "files") {
		return "", ErrInvalid
	}
	token, e := randomToken()
	if e != nil {
		return "", e
	}
	return "users/" + strconv.FormatInt(user, 10) + "/" + category + "/" + token, nil
}
func randomToken() (string, error) {
	token := make([]byte, 24)
	if _, err := rand.Read(token); err != nil {
		return "", ErrUnavailable
	}
	return hex.EncodeToString(token), nil
}
