package httptransport

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type rateEntry struct {
	count   int
	expires time.Time
}
type rateLimits struct {
	sync.Mutex
	entries map[string]rateEntry
}

func (l *rateLimits) allow(key string, limit int, now time.Time) bool {
	l.Lock()
	defer l.Unlock()
	entry := l.entries[key]
	if !now.Before(entry.expires) {
		if !l.makeRoom(now) {
			return false
		}

		entry = rateEntry{expires: now.Add(time.Minute)}
	}
	if entry.count >= limit {
		return false
	}
	entry.count++
	l.entries[key] = entry
	return true
}

func (l *rateLimits) check(w http.ResponseWriter, key string, limit int) bool {
	if l.allow(key, limit, time.Now()) {
		return true
	}
	w.Header().Set("Retry-After", "60")
	writeError(w, http.StatusTooManyRequests, "Too many requests. Try again in a minute.")
	return false
}

func clientIP(r *http.Request, trusted []*net.IPNet) string {
	peer, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "unknown"
	}
	// Only use a proxy-supplied chain when the socket peer is explicitly trusted.
	chain := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(chain) - 1; i >= 0 && trustedIP(peer, trusted); i-- {
		ip := net.ParseIP(strings.TrimSpace(chain[i]))
		if ip == nil {
			break
		}
		peer = ip.String()
	}
	return peer
}
func trustedIP(value string, ranges []*net.IPNet) bool {
	ip := net.ParseIP(value)
	for _, network := range ranges {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func (l *rateLimits) makeRoom(now time.Time) bool {
	if len(l.entries) < 10000 {
		return true
	}
	for k, v := range l.entries {
		if !now.Before(v.expires) {
			delete(l.entries, k)
		}
	}
	return len(l.entries) < 10000
}
