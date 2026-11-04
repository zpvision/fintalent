package objectstorage

import (
	"strings"
	"testing"
)

func TestUserKeysAreScopedAndUnique(t *testing.T) {
	for _, category := range []string{"academy", "tests", "files"} {
		a, e := UserKey(123, category)
		if e != nil || !ValidKey(a) || !strings.HasPrefix(a, "users/123/"+category+"/") {
			t.Fatal("invalid scoped key")
		}
		b, e := UserKey(123, category)
		if e != nil || a == b {
			t.Fatal("key reused")
		}
	}
	for _, category := range []string{"../academy", "academy/../../files", "unknown", ""} {
		if _, e := UserKey(1, category); e == nil {
			t.Fatal("unsafe category accepted")
		}
	}
	if _, e := UserKey(0, "academy"); e == nil {
		t.Fatal("invalid owner accepted")
	}
}

func TestOrganizationAndPlatformOwnership(t *testing.T) {
	key, e := OwnerKey(Owner{Kind: "accounting_company", ID: 77}, "files")
	if e != nil || !strings.HasPrefix(key, "organizations/accounting-companies/77/files/") {
		t.Fatal("organization ownership lost", e)
	}
	key, e = OwnerKey(Owner{Kind: "platform"}, "files")
	if e != nil || !strings.HasPrefix(key, "platform/files/") {
		t.Fatal("platform ownership lost", e)
	}
	for _, owner := range []Owner{{Kind: "accounting_company"}, {Kind: "platform", ID: 77}, {Kind: "unknown", ID: 1}} {
		if _, e := OwnerKey(owner, "files"); e == nil {
			t.Fatal("invalid owner accepted")
		}
	}
}

func TestPublicUploadReferencesCannotExposeHiddenFiles(t *testing.T) {
	if !PublicUploadRef("/static/uploads/avatars/fixture.png") {
		t.Fatal("valid legacy URL rejected")
	}
	for _, ref := range []string{"/static/uploads/", "/static/uploads/.env", "/static/uploads/path/.git/config", "/static/uploads/../secret", "/static/uploads/path/file.png:$DATA", "/static/uploads/path\\secret", "/private/certificates/file.pdf"} {
		if PublicUploadRef(ref) {
			t.Fatal("unsafe public URL accepted")
		}
	}
}

func TestBrowserOriginsAreExactAndProviderNeutral(t *testing.T) {
	c := Config{Driver: "s3", Endpoint: "https://storage.yandexcloud.net", Bucket: "fixture-bucket", PathStyle: true}
	origins := BrowserOrigins(c)
	if len(origins) != 1 || origins[0] != c.Endpoint {
		t.Fatal(origins)
	}
	c.PathStyle = false
	origins = BrowserOrigins(c)
	if len(origins) != 2 || origins[1] != "https://fixture-bucket.storage.yandexcloud.net" {
		t.Fatal(origins)
	}
	c.Endpoint = "https://another-provider.invalid"
	if BrowserOrigins(c)[0] != c.Endpoint {
		t.Fatal("provider domain hardcoded")
	}
	c.Endpoint = "https://user:fixture@another-provider.invalid"
	if len(BrowserOrigins(c)) != 0 {
		t.Fatal("credential-bearing origin admitted")
	}
}
