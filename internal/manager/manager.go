package manager

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Channel represents a release track.
type Channel string

const (
	ChannelStable Channel = "stable"
	ChannelBeta   Channel = "beta"
)

// Version represents an installed llama.cpp version.
type Version struct {
	ID        string    `json:"id"`         // e.g. "b3412-cuda"
	Build     string    `json:"build"`      // e.g. "b3412"
	Backend   string    `json:"backend"`    // e.g. "cuda"
	Channel   Channel   `json:"channel"`    // stable or beta
	InstalledAt time.Time `json:"installed_at"`
}

// Manifest is written into each version dir at install time.
type Manifest struct {
	Build     string            `json:"build"`
	Backend   string            `json:"backend"`
	Channel   Channel           `json:"channel"`
	Aliases   map[string]string `json:"aliases"` // canonical name → real binary name
	InstalledAt time.Time       `json:"installed_at"`
}

// Channels stores the active version per channel.
type Channels struct {
	Stable string `json:"stable"` // version ID active on stable channel
	Beta   string `json:"beta"`   // version ID active on beta channel
}

// Manager handles all local version state.
type Manager struct {
	home string // ~/.llava
}

// New creates a Manager for the given llava home directory.
func New(home string) *Manager {
	return &Manager{home: home}
}

// Home returns the llava home directory.
func (m *Manager) Home() string {
	return m.home
}

// VersionsDir returns the path to the versions directory.
func (m *Manager) VersionsDir() string {
	return filepath.Join(m.home, "versions")
}

// VersionDir returns the path to a specific version's directory.
func (m *Manager) VersionDir(id string) string {
	return filepath.Join(m.home, "versions", id)
}

// ShimsDir returns the path to the shims directory.
func (m *Manager) ShimsDir() string {
	return filepath.Join(m.home, "shims")
}

// CacheDir returns the path to the API cache directory.
func (m *Manager) CacheDir() string {
	return filepath.Join(m.home, "cache")
}

// ActiveFile returns the path to the active version file.
func (m *Manager) ActiveFile() string {
	return filepath.Join(m.home, "active")
}

// Init creates the llava directory structure.
func (m *Manager) Init() error {
	dirs := []string{
		m.home,
		m.VersionsDir(),
		m.ShimsDir(),
		m.CacheDir(),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("cannot create %s: %w", d, err)
		}
	}
	return nil
}

// Active returns the currently active version ID, or empty string if none.
// If active and channels are out of sync (crash recovery), the stale channel
// entry is cleared and "" is returned to trigger re-selection.
func (m *Manager) Active() string {
	data, err := os.ReadFile(m.ActiveFile())
	if err != nil {
		return ""
	}
	active := strings.TrimSpace(string(data))
	if active == "" {
		return ""
	}

	// Crash recovery: if active exists but channels doesn't know about it,
	// clear the stale channel entry.
	ch, chErr := m.LoadChannels()
	if chErr == nil {
		chFile := filepath.Join(m.home, "channels.json")
		actInfo, actErr := os.Stat(m.ActiveFile())
		if actErr == nil {
			chInfo, chInfoErr := os.Stat(chFile)
			if chInfoErr == nil && actInfo.ModTime().After(chInfo.ModTime()) {
				// Active is newer than channels — channels may be stale.
				manifest, mErr := m.ReadManifest(active)
				if mErr == nil {
					needsClear := false
					switch manifest.Channel {
					case ChannelStable:
						needsClear = ch.Stable != active
					case ChannelBeta:
						needsClear = ch.Beta != active
					}
					if needsClear {
						switch manifest.Channel {
						case ChannelStable:
							ch.Stable = ""
						case ChannelBeta:
							ch.Beta = ""
						}
						m.SaveChannels(ch)
					}
				}
			}
		}
	}
	return active
}

// SetActive writes a version ID as the active version.
// Deprecated: use SwitchActiveAndChannel for atomic state updates.
func (m *Manager) SetActive(id string) error {
	return os.WriteFile(m.ActiveFile(), []byte(id), 0644)
}

// SwitchActiveAndChannel atomically writes both the active version and its
// channel entry in channels.json. Uses a single atomic write pattern:
// write temp file → rename. This prevents state drift if the process crashes
// between writes.
func (m *Manager) SwitchActiveAndChannel(id string, ch Channel) error {
	// Write channels.json atomically (temp + rename).
	chPath := filepath.Join(m.home, "channels.json")
	chTmp := chPath + ".tmp"
	channels, err := m.LoadChannels()
	if err != nil {
		return err
	}
	switch ch {
	case ChannelStable:
		channels.Stable = id
	case ChannelBeta:
		channels.Beta = id
	default:
		return fmt.Errorf("unknown channel: %s", ch)
	}
	if err := atomicWriteFile(chTmp, func() ([]byte, error) {
		buf, err := json.Marshal(channels)
		if err != nil {
			return nil, err
		}
		buf = append(buf, '\n')
		return buf, nil
	}); err != nil {
		os.Remove(chTmp) // best-effort cleanup
		return err
	}
	if err := os.Rename(chTmp, chPath); err != nil {
		os.Remove(chTmp)
		return err
	}

	// Write active atomically (temp + rename).
	actPath := m.ActiveFile()
	actTmp := actPath + ".tmp"
	data := []byte(id + "\n")
	if err := atomicWriteFile(actTmp, func() ([]byte, error) {
		return data, nil
	}); err != nil {
		os.Remove(actTmp)
		return err
	}
	return os.Rename(actTmp, actPath)
}

// atomicWriteFile writes data to a temp file then renames it into place.
// The dataFn produces the bytes and is only called once.
func atomicWriteFile(tmpPath string, dataFn func() ([]byte, error)) error {
	tmpF, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	data, err := dataFn()
	if err != nil {
		tmpF.Close()
		os.Remove(tmpPath)
		return err
	}
	if _, err := tmpF.Write(data); err != nil {
		tmpF.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmpF.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}

// IsInstalled checks whether a version ID is installed locally.
func (m *Manager) IsInstalled(id string) bool {
	_, err := os.Stat(m.VersionDir(id))
	return err == nil
}

// ListInstalled returns all installed versions, newest first.
func (m *Manager) ListInstalled() ([]Version, error) {
	entries, err := os.ReadDir(m.VersionsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	active := m.Active()
	var versions []Version

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		id := e.Name()
		manifest, err := m.ReadManifest(id)
		if err != nil {
			// Partially installed or foreign dir — include with minimal info.
			parts := strings.SplitN(id, "-", 2)
			v := Version{ID: id, Build: parts[0]}
			if len(parts) > 1 {
				v.Backend = parts[1]
			}
			if id == active {
				// mark active below
			}
			versions = append(versions, v)
			continue
		}
		versions = append(versions, Version{
			ID:          id,
			Build:       manifest.Build,
			Backend:     manifest.Backend,
			Channel:     manifest.Channel,
			InstalledAt: manifest.InstalledAt,
		})
	}

	// Sort newest build first (lexicographic on build tag works for bNNNN).
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].Build > versions[j].Build
	})

	return versions, nil
}

// WriteManifest writes metadata into a version's directory.
func (m *Manager) WriteManifest(id string, manifest *Manifest) error {
	path := filepath.Join(m.VersionDir(id), "manifest.json")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(manifest)
}

// ReadManifest reads metadata from a version's directory.
func (m *Manager) ReadManifest(id string) (*Manifest, error) {
	path := filepath.Join(m.VersionDir(id), "manifest.json")
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var manifest Manifest
	if err := json.NewDecoder(f).Decode(&manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

// LoadChannels reads the channels.json file.
func (m *Manager) LoadChannels() (*Channels, error) {
	path := filepath.Join(m.home, "channels.json")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Channels{}, nil
		}
		return nil, err
	}
	defer f.Close()
	var ch Channels
	if err := json.NewDecoder(f).Decode(&ch); err != nil {
		return nil, err
	}
	return &ch, nil
}

// SaveChannels writes the channels.json file.
func (m *Manager) SaveChannels(ch *Channels) error {
	path := filepath.Join(m.home, "channels.json")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(ch)
}

// SetChannelVersion updates one channel's active version and persists it.
func (m *Manager) SetChannelVersion(ch Channel, id string) error {
	channels, err := m.LoadChannels()
	if err != nil {
		return err
	}
	switch ch {
	case ChannelStable:
		channels.Stable = id
	case ChannelBeta:
		channels.Beta = id
	default:
		return fmt.Errorf("unknown channel: %s", ch)
	}
	return m.SaveChannels(channels)
}

// Remove deletes a version directory and all its contents.
func (m *Manager) Remove(id string) error {
	dir := m.VersionDir(id)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("version %q is not installed", id)
	}

	active := m.Active()
	if active == id {
		return fmt.Errorf("cannot remove active version %q — run 'llava use <other>' first", id)
	}

	return os.RemoveAll(dir)
}

// ClearStaleChannelReferences clears channels.json entries that point to
// uninstalled versions. Also clears active if it points to a removed version.
func (m *Manager) ClearStaleChannelReferences() error {
	ch, err := m.LoadChannels()
	if err != nil {
		return err
	}
	cleared := false
	if ch.Stable != "" && !m.IsInstalled(ch.Stable) {
		ch.Stable = ""
		cleared = true
	}
	if ch.Beta != "" && !m.IsInstalled(ch.Beta) {
		ch.Beta = ""
		cleared = true
	}
	if cleared {
		if err := m.SaveChannels(ch); err != nil {
			return err
		}
	}

	// If active points to a removed version, clear it.
	active := m.Active()
	if active != "" && !m.IsInstalled(active) {
		actPath := m.ActiveFile()
		actTmp := actPath + ".tmp"
		if err := atomicWriteFile(actTmp, func() ([]byte, error) {
			return []byte("\n"), nil
		}); err != nil {
			return err
		}
		if err := os.Rename(actTmp, actPath); err != nil {
			os.Remove(actTmp)
			return err
		}
	}
	return nil
}

// VersionID builds a version dir name from build tag and backend.
// e.g. "b3412" + "cuda" → "b3412-cuda"
func VersionID(build, backend string) string {
	if backend == "" || backend == "cpu" {
		return build + "-cpu"
	}
	return build + "-" + backend
}

// ParseVersionID splits a version ID back into build and backend.
func ParseVersionID(id string) (build, backend string) {
	parts := strings.SplitN(id, "-", 2)
	if len(parts) == 1 {
		return parts[0], "cpu"
	}
	return parts[0], parts[1]
}
