package store

import "time"

// schemaVersion is bumped whenever a file's shape changes incompatibly.
const schemaVersion = 1

// Subscription is a cached channel the user is subscribed to.
type Subscription struct {
	ChannelID           string    `json:"channel_id"`
	Title               string    `json:"title"`
	UploadsLFPlaylistID string    `json:"uploads_lf_playlist_id"` // UULF-prefixed, Shorts-free
	Muted               bool      `json:"muted"`
	LastSeen            time.Time `json:"last_seen"`
}

// SubscriptionsFile is the on-disk shape of subscriptions.json.
type SubscriptionsFile struct {
	SchemaVersion int            `json:"schema_version"`
	Subscriptions []Subscription `json:"subscriptions"`
}

// Video is cached metadata for a single video.
type Video struct {
	VideoID                string    `json:"video_id"`
	ChannelID              string    `json:"channel_id"`
	Title                  string    `json:"title"`
	Description            string    `json:"description,omitempty"`
	PublishedAt            time.Time `json:"published_at"`
	DurationSeconds        int       `json:"duration_seconds"`
	Portrait               bool      `json:"portrait,omitempty"`
	ContainsSyntheticMedia bool      `json:"contains_synthetic_media,omitempty"`
}

// VideosFile is the on-disk shape of videos/<channel_id>.json.
type VideosFile struct {
	SchemaVersion int     `json:"schema_version"`
	Videos        []Video `json:"videos"`
}

// VideoState is the per-video mutable feed state.
type VideoState struct {
	Seen          bool    `json:"seen"`
	Hidden        bool    `json:"hidden"`
	AIScore       float64 `json:"ai_score,omitempty"`
	SyntheticFlag bool    `json:"synthetic_flag,omitempty"`
}

// FeedStateFile is the on-disk shape of feed_state.json: video_id -> state.
type FeedStateFile struct {
	SchemaVersion int                   `json:"schema_version"`
	State         map[string]VideoState `json:"state"`
}

// QueueFile is the on-disk shape of queue.json.
type QueueFile struct {
	SchemaVersion  int      `json:"schema_version"`
	VideoIDs       []string `json:"video_ids"` // ordered; local order is source of truth
	MirrorPlaylist string   `json:"mirror_playlist_id,omitempty"`
}

// WatchLogEntry records a single mpv playback launch.
type WatchLogEntry struct {
	VideoID   string    `json:"video_id"`
	Title     string    `json:"title"`
	Channel   string    `json:"channel"`
	StartedAt time.Time `json:"started_at"`
	Completed bool      `json:"completed"`
}

// WatchLogFile is the on-disk shape of watch_log.json (append-mostly).
type WatchLogFile struct {
	SchemaVersion int             `json:"schema_version"`
	Entries       []WatchLogEntry `json:"entries"`
}

// MutesFile is the on-disk shape of mutes.json.
type MutesFile struct {
	SchemaVersion int      `json:"schema_version"`
	ChannelIDs    []string `json:"channel_ids"`
}

// Playlist is a cached snapshot of a user playlist.
type Playlist struct {
	PlaylistID string `json:"playlist_id"`
	Title      string `json:"title"`
	ItemCount  int    `json:"item_count"`
	IsQueue    bool   `json:"is_queue,omitempty"`
}

// PlaylistsCacheFile is the on-disk shape of playlists_cache.json.
type PlaylistsCacheFile struct {
	SchemaVersion int        `json:"schema_version"`
	Playlists     []Playlist `json:"playlists"`
}

// Verdict is a cached LLM inspect result for a video (PRD §5.2 tier 2).
type Verdict struct {
	VideoID        string    `json:"video_id"`
	LikelyAI       bool      `json:"likely_ai"`
	Reasoning      string    `json:"reasoning,omitempty"`
	SuspectedTools []string  `json:"suspected_tools,omitempty"`
	CheckedAt      time.Time `json:"checked_at"`
}

// VerdictsFile is the on-disk shape of verdicts.json: video_id -> verdict.
type VerdictsFile struct {
	SchemaVersion int                `json:"schema_version"`
	Verdicts      map[string]Verdict `json:"verdicts"`
}
