package config

import "time"

// Manifest captures metadata for a config backup snapshot.
type Manifest struct {
    Type      string    `json:"type"` // always "config"
    CreatedAt time.Time `json:"createdAt"`
    Includes  []string  `json:"includes"` // e.g., ["projects"] for now
}

