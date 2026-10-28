package claude

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// The texts are kept in one JSON file in the state directory, so a restart
// or a deploy shows them at once instead of a minute or two later, and
// doesn't ask Claude again for what it has already written. A saved text
// follows the same rules as a fresh one: it shows only for its day and
// while its input has been seen lately, and is written anew when the input
// has changed.

const (
	stateFile = "claude.json"
	// saveEvery: besides after every change, the file is saved this often,
	// so the time each text's input was last seen is never far behind.
	saveEvery = 15 * time.Minute
)

type state struct {
	Version string    `json:"version"`
	Today   todayText `json:"today"`
	Week    weekText  `json:"week"`
	Titles  Titles    `json:"titles"`
}

// stateVersion names the model and the prompts. Texts saved with another
// are left behind and written anew.
func stateVersion(model string) string {
	return hashOf(model + "\x00" + todaySystem + "\x00" + weekSystem + "\x00" + titlesSystem)[:16]
}

// load reads the saved texts. A missing file is a first start; an
// unreadable one, or one from another model or prompts, is left alone and
// overwritten by the next save.
func (c *Claude) load() {
	b, err := os.ReadFile(c.statePath)
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	var s state
	if err == nil {
		err = json.Unmarshal(b, &s)
	}
	if err != nil {
		slog.Warn("claude: could not read the saved texts; writing them anew", "path", c.statePath, "err", err)
		return
	}
	if s.Version != c.version {
		slog.Info("claude: the saved texts are from another model or prompts; writing them anew", "path", c.statePath)
		return
	}
	c.mu.Lock()
	c.today, c.week, c.titles = s.Today, s.Week, s.Titles
	c.mu.Unlock()
	slog.Info("claude: loaded the saved texts", "path", c.statePath, "titles", len(s.Titles))
}

// save writes the texts to a temporary file and renames it into place, so
// the file is never half written. A failure is logged and tried again at
// the next refresh.
func (c *Claude) save(now time.Time) {
	c.mu.RLock()
	b, err := json.MarshalIndent(state{Version: c.version, Today: c.today, Week: c.week, Titles: c.titles}, "", "  ")
	c.mu.RUnlock()
	if err == nil {
		err = writeFile(c.statePath, b)
	}
	if err != nil {
		slog.Warn("claude: could not save the texts", "path", c.statePath, "err", err)
		return
	}
	c.dirty, c.saved = false, now
}

func writeFile(path string, b []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".claude-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name()) // a no-op once renamed
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
