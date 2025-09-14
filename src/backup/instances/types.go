package instances

import "time"

// Manifest captures metadata for an instance snapshot export.
type Manifest struct {
    Type        string            `json:"type"` // instance
    Project     string            `json:"project"`
    Name        string            `json:"name"`
    CreatedAt   time.Time         `json:"createdAt"`
    Options     map[string]string `json:"options,omitempty"` // snapshot, optimized
}

