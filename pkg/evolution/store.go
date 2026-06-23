package evolution

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const entryDelimiter = "\n§\n"

// Target identifies a curated evolution store file.
type Target string

const (
	TargetMemory Target = "memory"
	TargetUser   Target = "user"
	TargetSoul   Target = "soul"
	TargetPrompt Target = "prompt"
)

// Config wires the on-disk evolution directory and character budgets.
type Config struct {
	Dir             string
	MemoryCharLimit int
	UserCharLimit   int
	SoulCharLimit   int
	PromptCharLimit int
}

// Snapshot is the frozen system-prompt view captured at session start.
type Snapshot struct {
	Soul   string
	Prompt string
	Memory string
	User   string
}

// Store implements L4 curated evolution: bounded markdown files with a frozen
// snapshot per session (Hermes-style). Live mutations persist immediately but
// do not alter the in-session system prompt until the next session snapshot.
type Store struct {
	dir string

	memoryCharLimit int
	userCharLimit   int
	soulCharLimit   int
	promptCharLimit int

	mu sync.Mutex

	memoryEntries []string
	userEntries   []string
	soulEntries   []string
	promptEntries []string

	sessionSnapshots map[string]Snapshot
}

// Open loads evolution files from dir and prepares an empty snapshot map.
func Open(cfg Config) (*Store, error) {
	dir := strings.TrimSpace(cfg.Dir)
	if dir == "" {
		return nil, fmt.Errorf("evolution: dir is required")
	}
	if cfg.MemoryCharLimit <= 0 {
		cfg.MemoryCharLimit = 2200
	}
	if cfg.UserCharLimit <= 0 {
		cfg.UserCharLimit = 1375
	}
	if cfg.SoulCharLimit <= 0 {
		cfg.SoulCharLimit = 4000
	}
	if cfg.PromptCharLimit <= 0 {
		cfg.PromptCharLimit = 2000
	}
	s := &Store{
		dir:              filepath.Clean(dir),
		memoryCharLimit:  cfg.MemoryCharLimit,
		userCharLimit:    cfg.UserCharLimit,
		soulCharLimit:    cfg.SoulCharLimit,
		promptCharLimit:  cfg.PromptCharLimit,
		sessionSnapshots: make(map[string]Snapshot),
	}
	if err := s.reloadAll(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) reloadAll() error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("evolution: mkdir: %w", err)
	}
	var err error
	if s.memoryEntries, err = s.readTargetFile(TargetMemory); err != nil {
		return err
	}
	if s.userEntries, err = s.readTargetFile(TargetUser); err != nil {
		return err
	}
	if s.soulEntries, err = s.readTargetFile(TargetSoul); err != nil {
		return err
	}
	if s.promptEntries, err = s.readTargetFile(TargetPrompt); err != nil {
		return err
	}
	s.memoryEntries = dedupeEntries(s.memoryEntries)
	s.userEntries = dedupeEntries(s.userEntries)
	s.soulEntries = dedupeEntries(s.soulEntries)
	s.promptEntries = dedupeEntries(s.promptEntries)
	return nil
}

// SnapshotForSession returns the frozen prompt blocks for sessionID, capturing
// the current on-disk state the first time each session is seen.
func (s *Store) SnapshotForSession(sessionID string) Snapshot {
	if s == nil {
		return Snapshot{}
	}
	key := strings.TrimSpace(sessionID)
	if key == "" {
		key = "_default"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if snap, ok := s.sessionSnapshots[key]; ok {
		return snap
	}
	_ = s.reloadAllLocked()
	snap := Snapshot{
		Soul:   s.renderBlock(TargetSoul, s.soulEntries),
		Prompt: s.renderBlock(TargetPrompt, s.promptEntries),
		Memory: s.renderBlock(TargetMemory, s.memoryEntries),
		User:   s.renderBlock(TargetUser, s.userEntries),
	}
	s.sessionSnapshots[key] = snap
	return snap
}

// Add appends a curated entry to the target store.
func (s *Store) Add(target Target, content string) (map[string]any, error) {
	content = normalizeEntry(content)
	if content == "" {
		return nil, fmt.Errorf("content cannot be empty")
	}
	if msg := scanEvolutionContent(content); msg != "" {
		return nil, errors.New(msg)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.reloadTargetLocked(target); err != nil {
		return nil, err
	}
	entries := s.entriesFor(target)
	for _, e := range entries {
		if e == content {
			return s.successResponse(target, entries, "entry already exists (no duplicate added)"), nil
		}
	}
	limit := s.limitFor(target)
	newEntries := append(append([]string(nil), entries...), content)
	if charCount(newEntries) > limit {
		current := charCount(entries)
		return nil, fmt.Errorf("store at %d/%d chars; adding this entry would exceed the limit", current, limit)
	}
	s.setEntries(target, newEntries)
	if err := s.saveTargetLocked(target); err != nil {
		return nil, err
	}
	return s.successResponse(target, newEntries, "entry added"), nil
}

// Replace updates the entry containing oldText with newContent.
func (s *Store) Replace(target Target, oldText, newContent string) (map[string]any, error) {
	oldText = normalizeEntry(oldText)
	newContent = normalizeEntry(newContent)
	if oldText == "" {
		return nil, fmt.Errorf("old_text cannot be empty")
	}
	if newContent == "" {
		return nil, fmt.Errorf("new_content cannot be empty; use remove to delete entries")
	}
	if msg := scanEvolutionContent(newContent); msg != "" {
		return nil, errors.New(msg)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.reloadTargetLocked(target); err != nil {
		return nil, err
	}
	entries := s.entriesFor(target)
	idx, err := matchEntryIndex(entries, oldText)
	if err != nil {
		return nil, err
	}
	test := append([]string(nil), entries...)
	test[idx] = newContent
	if charCount(test) > s.limitFor(target) {
		return nil, fmt.Errorf("replacement would exceed the char limit for %s", target)
	}
	entries[idx] = newContent
	s.setEntries(target, entries)
	if err := s.saveTargetLocked(target); err != nil {
		return nil, err
	}
	return s.successResponse(target, entries, "entry replaced"), nil
}

// Remove deletes the entry containing oldText.
func (s *Store) Remove(target Target, oldText string) (map[string]any, error) {
	oldText = normalizeEntry(oldText)
	if oldText == "" {
		return nil, fmt.Errorf("old_text cannot be empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.reloadTargetLocked(target); err != nil {
		return nil, err
	}
	entries := s.entriesFor(target)
	idx, err := matchEntryIndex(entries, oldText)
	if err != nil {
		return nil, err
	}
	entries = append(entries[:idx], entries[idx+1:]...)
	s.setEntries(target, entries)
	if err := s.saveTargetLocked(target); err != nil {
		return nil, err
	}
	return s.successResponse(target, entries, "entry removed"), nil
}

func matchEntryIndex(entries []string, oldText string) (int, error) {
	var matches []int
	for i, e := range entries {
		if strings.Contains(e, oldText) {
			matches = append(matches, i)
		}
	}
	switch len(matches) {
	case 0:
		return 0, fmt.Errorf("no entry matched %q", oldText)
	case 1:
		return matches[0], nil
	default:
		unique := make(map[string]struct{})
		for _, i := range matches {
			unique[entries[i]] = struct{}{}
		}
		if len(unique) > 1 {
			return 0, fmt.Errorf("multiple entries matched %q; be more specific", oldText)
		}
		return matches[0], nil
	}
}

func dedupeEntries(entries []string) []string {
	if len(entries) == 0 {
		return entries
	}
	seen := make(map[string]struct{}, len(entries))
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e == "" {
			continue
		}
		if _, ok := seen[e]; ok {
			continue
		}
		seen[e] = struct{}{}
		out = append(out, e)
	}
	return out
}

func (s *Store) pathFor(target Target) string {
	name := "MEMORY.md"
	switch target {
	case TargetUser:
		name = "USER.md"
	case TargetSoul:
		name = "SOUL.md"
	case TargetPrompt:
		name = "PROMPT.md"
	}
	return filepath.Join(s.dir, name)
}

func (s *Store) readTargetFile(target Target) ([]string, error) {
	path := s.pathFor(target)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("evolution: read %s: %w", path, err)
	}
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return nil, nil
	}
	parts := strings.Split(text, entryDelimiter)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out, nil
}

func (s *Store) reloadTargetLocked(target Target) error {
	entries, err := s.readTargetFile(target)
	if err != nil {
		return err
	}
	s.setEntries(target, dedupeEntries(entries))
	return nil
}

func (s *Store) reloadAllLocked() error {
	return s.reloadAll()
}

func (s *Store) saveTargetLocked(target Target) error {
	entries := s.entriesFor(target)
	content := strings.Join(entries, entryDelimiter)
	path := s.pathFor(target)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("evolution: mkdir: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".evo-*.tmp")
	if err != nil {
		return fmt.Errorf("evolution: temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("evolution: write %s: %w", path, err)
	}
	return nil
}

func (s *Store) entriesFor(target Target) []string {
	switch target {
	case TargetUser:
		return s.userEntries
	case TargetSoul:
		return s.soulEntries
	case TargetPrompt:
		return s.promptEntries
	default:
		return s.memoryEntries
	}
}

func (s *Store) setEntries(target Target, entries []string) {
	switch target {
	case TargetUser:
		s.userEntries = entries
	case TargetSoul:
		s.soulEntries = entries
	case TargetPrompt:
		s.promptEntries = entries
	default:
		s.memoryEntries = entries
	}
}

func (s *Store) limitFor(target Target) int {
	switch target {
	case TargetUser:
		return s.userCharLimit
	case TargetSoul:
		return s.soulCharLimit
	case TargetPrompt:
		return s.promptCharLimit
	default:
		return s.memoryCharLimit
	}
}

func (s *Store) renderBlock(target Target, entries []string) string {
	if len(entries) == 0 {
		return ""
	}
	limit := s.limitFor(target)
	content := strings.Join(entries, entryDelimiter)
	current := charCount(entries)
	pct := 0
	if limit > 0 {
		pct = int(float64(current) / float64(limit) * 100)
	}
	sep := strings.Repeat("═", 46)
	var header string
	switch target {
	case TargetUser:
		header = fmt.Sprintf("USER PROFILE (who the user is) [%d%% — %d/%d chars]", pct, current, limit)
	case TargetSoul:
		header = fmt.Sprintf("AGENT IDENTITY (SOUL) [%d%% — %d/%d chars]", pct, current, limit)
	case TargetPrompt:
		header = fmt.Sprintf("EVOLVED SYSTEM PROMPT [%d%% — %d/%d chars]", pct, current, limit)
	default:
		header = fmt.Sprintf("MEMORY (your personal notes) [%d%% — %d/%d chars]", pct, current, limit)
	}
	return sep + "\n" + header + "\n" + sep + "\n" + content
}

func (s *Store) successResponse(target Target, entries []string, message string) map[string]any {
	current := charCount(entries)
	limit := s.limitFor(target)
	pct := 0
	if limit > 0 {
		pct = int(float64(current) / float64(limit) * 100)
	}
	return map[string]any{
		"success":     true,
		"target":      string(target),
		"entries":     entries,
		"usage":       fmt.Sprintf("%d%% — %d/%d chars", pct, current, limit),
		"entry_count": len(entries),
		"message":     message,
	}
}

// ListEntries returns all curated entries for target (reloads from disk).
func (s *Store) ListEntries(target Target) ([]string, error) {
	if s == nil {
		return nil, fmt.Errorf("evolution: store is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.reloadTargetLocked(target); err != nil {
		return nil, err
	}
	entries := s.entriesFor(target)
	out := append([]string(nil), entries...)
	return out, nil
}

// SearchEntries returns entries whose text contains query (case-insensitive).
// limit <= 0 means no limit.
func (s *Store) SearchEntries(target Target, query string, limit int) ([]string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}
	entries, err := s.ListEntries(target)
	if err != nil {
		return nil, err
	}
	needle := strings.ToLower(query)
	var matches []string
	for _, e := range entries {
		if strings.Contains(strings.ToLower(e), needle) {
			matches = append(matches, e)
			if limit > 0 && len(matches) >= limit {
				break
			}
		}
	}
	return matches, nil
}

// ParseTarget validates a tool target string.
func ParseTarget(raw string) (Target, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "memory", "":
		return TargetMemory, nil
	case "user":
		return TargetUser, nil
	case "soul":
		return TargetSoul, nil
	case "prompt", "system", "system_prompt":
		return TargetPrompt, nil
	default:
		return "", fmt.Errorf("invalid target %q; use memory, user, soul, or prompt", raw)
	}
}
