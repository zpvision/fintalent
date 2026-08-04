package main

import "testing"

func TestAccountingTopicTestSeeds(t *testing.T) {
	if len(accountingTopicTestSeeds) != 13 {
		t.Fatalf("expected 13 accounting topic tests, got %d", len(accountingTopicTestSeeds))
	}
	seen := map[string]bool{}
	for _, item := range accountingTopicTestSeeds {
		if item.Slug == "" || item.Title == "" || item.Category == "" || item.Description == "" {
			t.Fatalf("incomplete seed: %#v", item)
		}
		if seen[item.Slug] {
			t.Fatalf("duplicate slug %q", item.Slug)
		}
		seen[item.Slug] = true
	}
}
