package main

import (
	"os"
	"strings"
	"testing"
)

func TestMarketplaceSystemAuthorMigrationIsIndependentAndIdempotent(t *testing.T) {
	sql := marketplaceSystemAuthorMigration
	for _, forbidden := range []string{"3@3.ru", "WHERE id=1", "WHERE id = 1", "DELETE FROM tests", "TRUNCATE"} {
		if strings.Contains(sql, forbidden) {
			t.Fatalf("system author migration contains unsafe dependency or destructive statement %q", forbidden)
		}
	}
	for _, required := range []string{
		"system@fintalent.local",
		"ON CONFLICT(email) DO UPDATE",
		"is_blocked=TRUE",
		"is_system=TRUE",
		"UPDATE tests",
		"UPDATE test_versions",
		"slug LIKE 'position-skill-%'",
		"'accounting-topic-vat'",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("system author migration is missing %q", required)
		}
	}
}

func TestMarketplaceSystemSeedInventoryIsUnique(t *testing.T) {
	seen := make(map[string]struct{}, len(accountingTopicTestSeeds))
	for _, seed := range accountingTopicTestSeeds {
		if !strings.HasPrefix(seed.Slug, "accounting-topic-") {
			t.Fatalf("unexpected system test slug %q", seed.Slug)
		}
		if _, exists := seen[seed.Slug]; exists {
			t.Fatalf("duplicate system test slug %q", seed.Slug)
		}
		seen[seed.Slug] = struct{}{}
		if !strings.Contains(marketplaceSystemAuthorMigration, "'"+seed.Slug+"'") {
			t.Fatalf("system author migration does not cover seed slug %q", seed.Slug)
		}
	}
	if len(seen) == 0 {
		t.Fatal("system accounting tests must not be empty")
	}
}

func TestMarketplaceDemoFlag(t *testing.T) {
	original, existed := os.LookupEnv("SEED_DEMO_DATA")
	originalAppEnv, appEnvExisted := os.LookupEnv("APP_ENV")
	t.Cleanup(func() {
		if existed {
			_ = os.Setenv("SEED_DEMO_DATA", original)
		} else {
			_ = os.Unsetenv("SEED_DEMO_DATA")
		}
		if appEnvExisted {
			_ = os.Setenv("APP_ENV", originalAppEnv)
		} else {
			_ = os.Unsetenv("APP_ENV")
		}
	})
	for _, test := range []struct {
		value  string
		appEnv string
		want   bool
	}{{"false", "production", false}, {"FALSE", "development", false}, {" true ", "production", true}, {"", "development", true}, {"", "production", false}} {
		if err := os.Setenv("SEED_DEMO_DATA", test.value); err != nil {
			t.Fatal(err)
		}
		if err := os.Setenv("APP_ENV", test.appEnv); err != nil {
			t.Fatal(err)
		}
		if got := seedMarketplaceDemoData(); got != test.want {
			t.Fatalf("SEED_DEMO_DATA=%q APP_ENV=%q: got %v, want %v", test.value, test.appEnv, got, test.want)
		}
	}
}

func TestMarketplaceTechnicalAccountsCannotShareARealPassword(t *testing.T) {
	if len(disabledSystemPasswordHash) != 60 {
		t.Fatalf("disabled password sentinel length is %d, want 60", len(disabledSystemPasswordHash))
	}
	if strings.HasPrefix(disabledSystemPasswordHash, "$2") {
		t.Fatal("disabled password sentinel must not be a usable bcrypt hash")
	}
}
