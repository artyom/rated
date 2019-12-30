// Command rated implements standalone rate-limiting service with an HTTP
// interface.
//
// Its HTTP endpoint accepts requests with non-empty query string used as a key
// to check against a limited set of buckets to do token-based rate limiting.
// It treats the whole query as-is as a key, so queries "foo=1&bar=2" and
// "bar=2&foo=1" represent two distinct keys.
//
// It responds either with 204 if a request should be allowed, or 429 if it
// should be limited.
//
// Note that because of a limited amount of buckets in use collisions are
// expected.
//
// It is possible to apply custom per-key rate limits with "Burst" and "Refill"
// HTTP headers:
//
//	curl -sD- -H "Burst: 3" -H "Refill: 1s" 'http://localhost:8080/?key'
package main

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

func main() {
	args := runArgs{
		Burst:  10,
		Size:   100000,
		Refill: time.Second,
		Addr:   "localhost:8080",
	}
	flag.IntVar(&args.Burst, "burst", args.Burst, "burst amount")
	flag.IntVar(&args.Size, "buckets", args.Size, "number of buckets to use")
	flag.DurationVar(&args.Refill, "refill", args.Refill, "time to refill bucket by 1 token")
	flag.StringVar(&args.Addr, "addr", args.Addr, "address to listen")
	flag.Parse()
	if err := run(args); err != nil {
		os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}

type runArgs struct {
	Refill time.Duration
	Burst  int
	Size   int
	Addr   string
}

func run(args runArgs) error {
	if args.Refill <= 0 {
		return errors.New("-refill argument must be positive")
	}
	if args.Burst <= 0 {
		return errors.New("-burst argument must be positive")
	}
	if args.Size <= 0 {
		return errors.New("-buckets argument must be positive")
	}
	if args.Addr == "" {
		return errors.New("-addr is empty")
	}
	srv := &http.Server{
		Addr:         args.Addr,
		Handler:      newLimiter(args.Size, args.Burst, args.Refill),
		ReadTimeout:  time.Second,
		WriteTimeout: time.Second,
		IdleTimeout:  30 * time.Second,
	}
	return srv.ListenAndServe()
}

func newLimiter(size, burst int, refillEvery time.Duration) *limiter {
	if refillEvery <= 0 {
		panic("newLimiter called with non-positive refillEvery")
	}
	if burst < 0 {
		panic("newLimiter called with negative burst")
	}
	if size <= 0 {
		panic("newLimiter called with non-positive size")
	}
	return &limiter{
		burst:       float64(burst),
		refillEvery: float64(refillEvery),
		buckets:     make([]bucket, size),
		hashFunc:    newHasher(),
	}
}

type limiter struct {
	burst       float64
	refillEvery float64 // float64(time.Duration) — how often to refill buckets by 1 token
	hashFunc    func(string) uint64
	mu          sync.Mutex
	buckets     []bucket
}

func (l *limiter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.RawQuery == "" {
		fmt.Fprintf(w, helpText, len(l.buckets), int(l.burst), time.Duration(l.refillEvery))
		return
	}
	var burst int
	var refill time.Duration
	const burstHeader = "Burst"
	const refillHeader = "Refill"
	if _, ok := r.Header[burstHeader]; ok {
		n, err := strconv.ParseUint(r.Header.Get(burstHeader), 10, 32)
		if err != nil {
			http.Error(w, fmt.Sprintf("%s header: %v", burstHeader, err), http.StatusBadRequest)
			return
		}
		burst = int(n)
	}
	if _, ok := r.Header[refillHeader]; ok {
		d, err := time.ParseDuration(r.Header.Get(refillHeader))
		if err != nil {
			http.Error(w, fmt.Sprintf("%s header: %v", refillHeader, err), http.StatusBadRequest)
			return
		}
		refill = d
	}
	w.Header().Set("Cache-Control", "no-store")
	if l.allow(r.URL.RawQuery, burst, refill) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
}

// allow reports true if given key does not exceed allowed rate. If burst
// and/or refill values are positive, they are used instead of default burst
// and refill values set on the limiter.
func (l *limiter) allow(key string, burst int, refill time.Duration) bool {
	now := time.Now()
	idx := int(l.hashFunc(key) % uint64(len(l.buckets)))
	b, r := l.burst, l.refillEvery
	if burst > 0 {
		b = float64(burst)
	}
	if refill > 0 {
		r = float64(refill)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buckets[idx].allow(now, b, r)
}

type bucket struct {
	left  float64 // tokens left
	mtime int64   // last access time as nanoseconds since Unix epoch
}

func (b *bucket) allow(now time.Time, burst, refillEvery float64) bool {
	if b.mtime != 0 { // refill bucket
		spent := now.Sub(time.Unix(0, b.mtime))
		if refillBy := float64(spent) / refillEvery; refillBy > 0 {
			b.left += refillBy
			if b.left > burst {
				b.left = burst
			}
		}
	} else if b.left == 0 { // previously unused bucket
		b.left = burst
	}
	b.mtime = now.UnixNano()
	if b.left >= 1 {
		b.left--
		return true
	}
	return false
}

const helpText = `Accepts requests with non-empty query string used as a key to check
against a limited set of buckets to do token-based rate limiting.

Expected responses:

* 204 — request should be allowed;
* 429 — request exceeded rate and should be limited.

Note that because of a limited amount of buckets in use collisions are expected.

Current settings are: %d buckets, each holds up to %d tokens
and refills by one token every %v.
`
