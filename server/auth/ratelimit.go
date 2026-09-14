package auth

import (
	"container/list"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// LoginRateLimit and PatternRateLimit are ENCRE_04 §7's login rate limits:
// 5 logins per minute per IP, 10 pattern attempts per minute per child.
const (
	LoginRateLimit   = 5
	PatternRateLimit = 10
)

// defaultMaxLimiterKeys bounds a [Limiter]'s memory when [NewLimiter] is used
// instead of [NewLimiterSize]: at up to that many concurrently tracked keys,
// each a small struct plus a [rate.Limiter], total memory stays well under
// what any of ENCRE's deployment targets can spare.
const defaultMaxLimiterKeys = 100_000

// Limiter enforces a per-key requests-per-minute limit with a token bucket
// per key (ENCRE_04 §7). Its zero value is not usable; build one with
// [NewLimiter] or [NewLimiterSize].
//
// The number of distinct keys Limiter tracks is capped: once full, Allow
// evicts the least-recently-used key before adding a new one, so an
// attacker who forges a fresh key on every request (see [ClientIP]'s doc
// comment for why a rate limiter must never key on a client-supplied header
// without checking who supplied it) can churn the map but cannot grow it
// without bound, and cannot make an existing, legitimate key's bucket
// disappear early by flooding new ones — LRU only evicts the single oldest
// entry, one at a time, exactly when a new key needs room.
//
// A Limiter is safe for concurrent use by multiple goroutines.
type Limiter struct {
	limit   rate.Limit
	burst   int
	now     func() time.Time
	maxKeys int

	mu      sync.Mutex
	buckets map[string]*list.Element // value: *bucket
	order   *list.List               // front = most recently used
}

type bucket struct {
	key      string
	lim      *rate.Limiter
	lastSeen time.Time
}

// NewLimiter returns a Limiter that allows up to perMinute events per minute
// for any one key, refilled continuously (a token bucket, not a fixed
// window), tracking up to [defaultMaxLimiterKeys] distinct keys at once. now
// supplies the current time on every [Limiter.Allow] call; pass nil to use
// [time.Now], or an injectable clock in tests that need to move time forward
// without sleeping.
func NewLimiter(perMinute int, now func() time.Time) *Limiter {
	return NewLimiterSize(perMinute, defaultMaxLimiterKeys, now)
}

// NewLimiterSize is [NewLimiter] with an explicit cap on the number of
// distinct keys tracked at once, for callers that need a smaller bound than
// [defaultMaxLimiterKeys] (or a tiny one, in a test that wants to observe
// eviction directly).
func NewLimiterSize(perMinute, maxKeys int, now func() time.Time) *Limiter {
	if now == nil {
		now = time.Now
	}
	return &Limiter{
		limit:   rate.Limit(float64(perMinute) / 60),
		burst:   perMinute,
		now:     now,
		maxKeys: maxKeys,
		buckets: make(map[string]*list.Element),
		order:   list.New(),
	}
}

// Allow reports whether an event for key is allowed right now, consuming one
// token from key's bucket if so.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()

	if elem, ok := l.buckets[key]; ok {
		l.order.MoveToFront(elem)
		b := elem.Value.(*bucket) //nolint:errcheck,forcetypeassert // list.Element.Value always holds *bucket here
		b.lastSeen = now
		return b.lim.AllowN(now, 1)
	}

	l.evictOldestLocked()

	b := &bucket{key: key, lim: rate.NewLimiter(l.limit, l.burst), lastSeen: now}
	l.buckets[key] = l.order.PushFront(b)
	return b.lim.AllowN(now, 1)
}

// evictOldestLocked removes the single least-recently-used key if the map is
// already at capacity, making room for exactly the one key Allow is about to
// insert. Callers must hold l.mu.
func (l *Limiter) evictOldestLocked() {
	if len(l.buckets) < l.maxKeys {
		return
	}
	oldest := l.order.Back()
	if oldest == nil {
		return
	}
	l.order.Remove(oldest)
	delete(l.buckets, oldest.Value.(*bucket).key) //nolint:errcheck,forcetypeassert // list.Element.Value always holds *bucket here
}
