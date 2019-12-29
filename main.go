// Command rated implements standalone rate-limiting service with http
// interface.
//
// Its http endpoint accepts requests with non-empty query string used as a key
// to check against limited set of buckets to do token-based rate limiting.
//
// It responds either with 204 is request should be allowed, or 429 if it
// should be limited.
//
// Note that because of limited amount of buckets in use collisions are
// expected.
package main

import (
	"errors"
	"flag"
	"net/http"
	"os"
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
		w.Write([]byte(helpText))
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if l.allow(r.URL.RawQuery) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
}

func (l *limiter) allow(key string) bool {
	now := time.Now()
	idx := int(l.hashFunc(key) % uint64(len(l.buckets)))
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buckets[idx].allow(now, l.burst, l.refillEvery)
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
against limited set of buckets to do token-based rate limiting.

Expected responses:

* 204 — request should be allowed;
* 429 — request exceeded rate and should be limited.

Note that because of limited amount of buckets in use collisions are expected.
`
