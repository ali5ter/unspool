// Package store is unspool's local-first, plain-JSON data store.
//
// Every file lives under a single store directory (config StoreDir) and is
// written atomically (temp file + rename) so an interrupted write can never
// corrupt the on-disk state. Each file carries a schema_version field for
// future migrations. This is also the export format: --export json is close
// to a straight copy of the relevant file.
package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Store reads and writes unspool's JSON files under Dir.
type Store struct {
	Dir string
}

// New returns a Store rooted at dir.
func New(dir string) *Store {
	return &Store{Dir: dir}
}

func (s *Store) path(name string) string {
	return filepath.Join(s.Dir, name)
}

func (s *Store) videoPath(channelID string) string {
	return filepath.Join(s.Dir, "videos", channelID+".json")
}

// loadJSON reads and unmarshals path, returning zero when the file is absent.
func loadJSON[T any](path string) (T, error) {
	var out T
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return out, err
	}
	return out, nil
}

// saveJSON marshals v and writes it atomically to path (temp file + rename).
func saveJSON[T any](path string, v T) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// LoadSubscriptions reads subscriptions.json.
func (s *Store) LoadSubscriptions() (SubscriptionsFile, error) {
	f, err := loadJSON[SubscriptionsFile](s.path("subscriptions.json"))
	if f.SchemaVersion == 0 {
		f.SchemaVersion = schemaVersion
	}
	return f, err
}

// SaveSubscriptions writes subscriptions.json.
func (s *Store) SaveSubscriptions(f SubscriptionsFile) error {
	f.SchemaVersion = schemaVersion
	return saveJSON(s.path("subscriptions.json"), f)
}

// LoadVideos reads videos/<channelID>.json.
func (s *Store) LoadVideos(channelID string) (VideosFile, error) {
	f, err := loadJSON[VideosFile](s.videoPath(channelID))
	if f.SchemaVersion == 0 {
		f.SchemaVersion = schemaVersion
	}
	return f, err
}

// SaveVideos writes videos/<channelID>.json.
func (s *Store) SaveVideos(channelID string, f VideosFile) error {
	f.SchemaVersion = schemaVersion
	return saveJSON(s.videoPath(channelID), f)
}

// LoadFeedState reads feed_state.json.
func (s *Store) LoadFeedState() (FeedStateFile, error) {
	f, err := loadJSON[FeedStateFile](s.path("feed_state.json"))
	if f.State == nil {
		f.State = map[string]VideoState{}
	}
	if f.SchemaVersion == 0 {
		f.SchemaVersion = schemaVersion
	}
	return f, err
}

// SaveFeedState writes feed_state.json.
func (s *Store) SaveFeedState(f FeedStateFile) error {
	f.SchemaVersion = schemaVersion
	return saveJSON(s.path("feed_state.json"), f)
}

// SetVideoLiked updates the liked flag for a single video in feed_state.json.
func (s *Store) SetVideoLiked(videoID string, liked bool) error {
	f, err := s.LoadFeedState()
	if err != nil {
		return err
	}
	state := f.State[videoID]
	state.Liked = liked
	f.State[videoID] = state
	return s.SaveFeedState(f)
}

// MarkVideosSeen marks every ID in videoIDs as seen in feed_state.json, in
// a single read-modify-write. Callers batch IDs rather than calling this
// per video — feed navigation can mark many videos seen in quick
// succession, and one write per keystroke would both hammer disk and risk
// concurrent callers racing this file's read-modify-write and losing each
// other's updates.
func (s *Store) MarkVideosSeen(videoIDs []string) error {
	if len(videoIDs) == 0 {
		return nil
	}
	f, err := s.LoadFeedState()
	if err != nil {
		return err
	}
	for _, id := range videoIDs {
		state := f.State[id]
		state.Seen = true
		f.State[id] = state
	}
	return s.SaveFeedState(f)
}

// LoadQueue reads queue.json.
func (s *Store) LoadQueue() (QueueFile, error) {
	f, err := loadJSON[QueueFile](s.path("queue.json"))
	if f.SchemaVersion == 0 {
		f.SchemaVersion = schemaVersion
	}
	return f, err
}

// SaveQueue writes queue.json.
func (s *Store) SaveQueue(f QueueFile) error {
	f.SchemaVersion = schemaVersion
	return saveJSON(s.path("queue.json"), f)
}

// LoadWatchLog reads watch_log.json.
func (s *Store) LoadWatchLog() (WatchLogFile, error) {
	f, err := loadJSON[WatchLogFile](s.path("watch_log.json"))
	if f.SchemaVersion == 0 {
		f.SchemaVersion = schemaVersion
	}
	return f, err
}

// AppendWatchLog appends entry to watch_log.json.
func (s *Store) AppendWatchLog(entry WatchLogEntry) error {
	f, err := s.LoadWatchLog()
	if err != nil {
		return err
	}
	f.Entries = append(f.Entries, entry)
	f.SchemaVersion = schemaVersion
	return saveJSON(s.path("watch_log.json"), f)
}

// LoadMutes reads mutes.json.
func (s *Store) LoadMutes() (MutesFile, error) {
	f, err := loadJSON[MutesFile](s.path("mutes.json"))
	if f.SchemaVersion == 0 {
		f.SchemaVersion = schemaVersion
	}
	return f, err
}

// SaveMutes writes mutes.json.
func (s *Store) SaveMutes(f MutesFile) error {
	f.SchemaVersion = schemaVersion
	return saveJSON(s.path("mutes.json"), f)
}

// MuteChannel adds channelID to mutes.json if not already present.
func (s *Store) MuteChannel(channelID string) error {
	f, err := s.LoadMutes()
	if err != nil {
		return err
	}
	for _, id := range f.ChannelIDs {
		if id == channelID {
			return nil
		}
	}
	f.ChannelIDs = append(f.ChannelIDs, channelID)
	return s.SaveMutes(f)
}

// UnmuteChannel removes channelID from mutes.json.
func (s *Store) UnmuteChannel(channelID string) error {
	f, err := s.LoadMutes()
	if err != nil {
		return err
	}
	out := f.ChannelIDs[:0]
	for _, id := range f.ChannelIDs {
		if id != channelID {
			out = append(out, id)
		}
	}
	f.ChannelIDs = out
	return s.SaveMutes(f)
}

// LoadPlaylistsCache reads playlists_cache.json.
func (s *Store) LoadPlaylistsCache() (PlaylistsCacheFile, error) {
	f, err := loadJSON[PlaylistsCacheFile](s.path("playlists_cache.json"))
	if f.SchemaVersion == 0 {
		f.SchemaVersion = schemaVersion
	}
	return f, err
}

// SavePlaylistsCache writes playlists_cache.json.
func (s *Store) SavePlaylistsCache(f PlaylistsCacheFile) error {
	f.SchemaVersion = schemaVersion
	return saveJSON(s.path("playlists_cache.json"), f)
}

// LoadVerdicts reads verdicts.json.
func (s *Store) LoadVerdicts() (VerdictsFile, error) {
	f, err := loadJSON[VerdictsFile](s.path("verdicts.json"))
	if f.Verdicts == nil {
		f.Verdicts = map[string]Verdict{}
	}
	if f.SchemaVersion == 0 {
		f.SchemaVersion = schemaVersion
	}
	return f, err
}

// SaveVerdicts writes verdicts.json.
func (s *Store) SaveVerdicts(f VerdictsFile) error {
	f.SchemaVersion = schemaVersion
	return saveJSON(s.path("verdicts.json"), f)
}
