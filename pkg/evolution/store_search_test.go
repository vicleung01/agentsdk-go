package evolution

import "testing"

func TestStoreSearchEntries(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(Config{Dir: dir})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := store.Add(TargetUser, "prefers concise answers"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := store.Add(TargetUser, "timezone Asia/Shanghai"); err != nil {
		t.Fatalf("add: %v", err)
	}
	matches, err := store.SearchEntries(TargetUser, "concise", 0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	all, err := store.ListEntries(TargetUser)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(all))
	}
}
