package progress

import (
    "fmt"
    "io"
    "sync"
    "time"
)

// Reader wraps an io.Reader and periodically writes progress updates to out.
type Reader struct {
    r           io.Reader
    out         io.Writer
    label       string
    total       int64
    read        int64
    mu          sync.Mutex
    lastPrinted time.Time
}

// NewReader creates a new progress Reader. If total is 0, percentage is omitted.
func NewReader(r io.Reader, total int64, label string, out io.Writer) *Reader {
    return &Reader{r: r, out: out, label: label, total: total}
}

func (p *Reader) Read(b []byte) (int, error) {
    n, err := p.r.Read(b)
    if n > 0 {
        p.mu.Lock()
        p.read += int64(n)
        now := time.Now()
        if now.Sub(p.lastPrinted) >= 200*time.Millisecond {
            p.print()
            p.lastPrinted = now
        }
        p.mu.Unlock()
    }
    if err == io.EOF {
        p.mu.Lock()
        p.print() // final
        fmt.Fprint(p.out, "\n")
        p.mu.Unlock()
    }
    return n, err
}

func (p *Reader) print() {
    if p.out == nil {
        return
    }
    if p.total > 0 {
        pct := float64(p.read) / float64(p.total) * 100
        fmt.Fprintf(p.out, "\r[%s] %.1f%% (%d/%d bytes)", p.label, pct, p.read, p.total)
    } else {
        fmt.Fprintf(p.out, "\r[%s] %d bytes", p.label, p.read)
    }
}

