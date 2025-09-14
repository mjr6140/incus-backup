package backend

// Entry represents a single backup snapshot entry discovered in a backend.
// It is intentionally generic so the CLI can render a consolidated view.
type Entry struct {
    Type        string // instance|volume|image|config
    Project     string // for instances/volumes
    Pool        string // for volumes
    Name        string // instance or volume name
    Fingerprint string // image fingerprint
    Timestamp   string // YYYYMMDDThhmmssZ
    Path        string // absolute filesystem path to snapshot directory
}

// Kind constants used for filtering.
const (
    KindAll      = "all"
    KindInstance = "instances"
    KindVolume   = "volumes"
    KindImage    = "images"
    KindConfig   = "config"
)

// StorageBackend defines read-only listing for now.
type StorageBackend interface {
    List(kind string) ([]Entry, error)
}

