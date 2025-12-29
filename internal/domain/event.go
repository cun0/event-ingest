package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"
)

type EventPayload struct {
	EventName  string          `json:"event_name"`
	Channel    string          `json:"channel"`
	CampaignID string          `json:"campaign_id"`
	UserID     string          `json:"user_id"`
	Timestamp  int64           `json:"timestamp"`
	Tags       []string        `json:"tags"`
	Metadata   json.RawMessage `json:"metadata"`
}

type Event struct {
	DedupKey   string
	EventName  string
	Channel    string
	CampaignID string
	UserID     string
	Timestamp  time.Time
	Tags       []string
	Metadata   json.RawMessage
}

func (p *EventPayload) Validate(now time.Time) error {
	now = now.UTC()

	if strings.TrimSpace(p.EventName) == "" {
		return errors.New("event_name is required")
	}
	if strings.TrimSpace(p.Channel) == "" {
		return errors.New("channel is required")
	}
	if strings.TrimSpace(p.UserID) == "" {
		return errors.New("user_id is required")
	}
	if p.Timestamp <= 0 {
		return errors.New("timestamp must be a positive unix timestamp")
	}

	ts, _, err := parseTimestamp(p.Timestamp, now)
	if err != nil {
		return err
	}

	const maxFutureSkew = 2 * time.Minute
	if ts.After(now.Add(maxFutureSkew)) {
		return errors.New("timestamp must not be in the future")
	}

	if len(p.Metadata) != 0 && !json.Valid(p.Metadata) {
		return errors.New("metadata must be valid JSON")
	}

	return nil
}

func (p *EventPayload) ToEvent(now time.Time) (Event, error) {
	ts, tsKey, err := parseTimestamp(p.Timestamp, now)
	if err != nil {
		return Event{}, err
	}

	normalizedTags := normalizeTags(p.Tags)
	normalizedMetadata := normalizeMetadata(p.Metadata)

	dk := BuildDedupKey(
		strings.TrimSpace(p.EventName),
		strings.TrimSpace(p.Channel),
		strings.TrimSpace(p.CampaignID),
		strings.TrimSpace(p.UserID),
		tsKey,
		normalizedTags,
		normalizedMetadata,
	)

	return Event{
		DedupKey:   dk,
		EventName:  strings.TrimSpace(p.EventName),
		Channel:    strings.TrimSpace(p.Channel),
		CampaignID: strings.TrimSpace(p.CampaignID),
		UserID:     strings.TrimSpace(p.UserID),
		Timestamp:  ts,
		Tags:       normalizedTags,
		Metadata:   normalizedMetadata,
	}, nil
}

// parseTimestamsp accepts seconds or milliseconds
// TODO: Add validation for the timestamp

func parseTimestamp(raw int64, now time.Time) (ts time.Time, tsKey int64, err error) {
	if raw <= 0 {
		return time.Time{}, 0, errors.New("timestamp must be positive")
	}

	// Heuristic: millis is usually >= 1e12 (13 digits). Seconds is ~ 1e9..1e10.
	// Also compare against "now" to avoid edge weirdness.
	if raw >= 1_000_000_000_000 {
		ts = time.UnixMilli(raw).UTC()
		return ts, ts.UnixMilli(), nil
	}

	ts = time.Unix(raw, 0).UTC()
	return ts, ts.UnixMilli(), nil
}

func normalizeTags(in []string) []string {
	if len(in) == 0 {
		return []string{}
	}

	tmp := make([]string, 0, len(in))
	for _, t := range in {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		tmp = append(tmp, t)
	}

	if len(tmp) == 0 {
		return []string{}
	}

	sort.Strings(tmp)

	out := tmp[:0]
	var prev string
	for i, t := range tmp {
		if i == 0 || t != prev {
			out = append(out, t)
			prev = t
		}
	}
	return out
}

func BuildDedupKey(eventName string, channel string, campaignID string, userID string, tsKeyUnixMilli int64, tags []string, metadata json.RawMessage,
) string {
	var b strings.Builder

	b.WriteString(eventName)
	b.WriteString("|")
	b.WriteString(channel)
	b.WriteString("|")
	b.WriteString(campaignID)
	b.WriteString("|")
	b.WriteString(userID)
	b.WriteString("|")
	b.WriteString(strconv.FormatInt(tsKeyUnixMilli, 10))
	b.WriteString("|")

	// tags
	for i, t := range tags {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(t)
	}
	b.WriteString("|")

	// metadata (canonical JSON text)
	if len(metadata) == 0 {
		b.WriteString("{}")
	} else {
		b.Write(metadata)
	}

	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}
func normalizeMetadata(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}
