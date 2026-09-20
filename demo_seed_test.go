package main

import (
	"strings"
	"testing"
)

func TestRecurringSeedsPreserveLifecycleState(t *testing.T) {
	tests := []struct {
		name      string
		read      func() ([]byte, error)
		forbidden []string
	}{
		{
			name: "profiles",
			read: func() ([]byte, error) {
				return demoContentFS.ReadFile("migrations/022_demo_content.sql")
			},
			forbidden: []string{"on conflict(user_id) do update set status=", "on conflict(user_id) do update set visibility="},
		},
		{
			name: "publications",
			read: func() ([]byte, error) {
				return publicationMigrationFS.ReadFile("migrations/025_publications_demo.sql")
			},
			forbidden: []string{"on conflict(slug) do update set status=", "status='published',visibility='public'"},
		},
		{
			name: "profimarket legacy cards",
			read: func() ([]byte, error) {
				return profiMarketMigrationFS.ReadFile("migrations/028_profimarket_demo.sql")
			},
			forbidden: []string{"on conflict(slug) do update set author_user_id=excluded.author_user_id,status="},
		},
		{
			name: "profimarket product cards",
			read: func() ([]byte, error) {
				return profiMarketMigrationFS.ReadFile("migrations/052_profimarket_product_demo.sql")
			},
			forbidden: []string{"on conflict(slug) do update set type=excluded.type,status=", "deleted_at=null"},
		},
		{
			name: "profimarket dictionaries",
			read: func() ([]byte, error) {
				return profiMarketMigrationFS.ReadFile("migrations/027_profimarket.sql")
			},
			forbidden: []string{"on conflict(code) do update set name=excluded.name,active=true"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			schema, err := test.read()
			if err != nil {
				t.Fatal(err)
			}
			normalized := strings.ToLower(strings.Join(strings.Fields(string(schema)), " "))
			for _, forbidden := range test.forbidden {
				if strings.Contains(normalized, forbidden) {
					t.Fatalf("recurring seed overwrites administrator-controlled state: %q", forbidden)
				}
			}
		})
	}
}
