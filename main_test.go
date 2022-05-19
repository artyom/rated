package main

import (
	"context"
	"fmt"
	"hash/maphash"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"
)

func TestLimiter(t *testing.T) {
	l := newLimiter(10, 2, time.Second)
	results := make([]bool, 3)
	for i := 0; i < len(results); i++ {
		results[i] = l.allow("foo", 0, 0)
	}
	want := []bool{true, true, false}
	for i := 0; i < len(results); i++ {
		if results[i] != want[i] {
			t.Fatalf("got:\n%v\nwant:\n%v", results, want)
		}
	}
}

func TestBucket(t *testing.T) {
	var b bucket
	results := make([]bool, 3)
	for i := 0; i < len(results); i++ {
		results[i] = b.allow(time.Now(), 2, float64(time.Second))
	}
	want := []bool{true, true, false}
	for i := 0; i < len(results); i++ {
		if results[i] != want[i] {
			t.Logf("bucket: %+v", b)
			t.Fatalf("got:\n%v\nwant:\n%v", results, want)
		}
	}
}

func TestLimiter_ServeHTTP(t *testing.T) {
	l := newLimiter(10, 2, time.Second)
	srv := httptest.NewServer(l)
	defer srv.Close()
	client := srv.Client()
	want := []bool{true, true, false}
	results := make([]bool, 3)
	for i := 0; i < len(results); i++ {
		deny, err := rateLimitKey(client, time.Second, srv.URL+"/", "foo", 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		results[i] = !deny
	}
	for i := 0; i < len(results); i++ {
		if results[i] != want[i] {
			t.Fatalf("default limiter settings: got:\n%v\nwant:\n%v", results, want)
		}
	}

	// reset buckets to make sure "bar" key doesn't collide with "foo" key to
	// the same bucket
	for i := range l.buckets {
		l.buckets[i] = bucket{}
	}

	// per-key custom settings
	want = []bool{true, false, false}
	for i := 0; i < len(results); i++ {
		deny, err := rateLimitKey(client, time.Second, srv.URL+"/", "bar", 1, 500*time.Millisecond)
		if err != nil {
			t.Fatal(err)
		}
		results[i] = !deny
	}
	for i := 0; i < len(results); i++ {
		if results[i] != want[i] {
			t.Fatalf("custom per-key settings: got:\n%v\nwant:\n%v", results, want)
		}
	}
}

func rateLimitKey(client *http.Client, timeout time.Duration, ratedURL, key string, burst int, refill time.Duration) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ratedURL = ratedURL + "?" + url.QueryEscape(key)
	rreq, err := http.NewRequestWithContext(ctx, http.MethodPost, ratedURL, nil)
	if err != nil {
		return false, err
	}
	if burst > 0 {
		rreq.Header.Set("Burst", strconv.Itoa(burst))
	}
	if refill > 0 {
		rreq.Header.Set("Refill", refill.String())
	}
	rresp, err := client.Do(rreq)
	if err != nil {
		return false, err
	}
	defer rresp.Body.Close()
	switch rresp.StatusCode {
	case http.StatusTooManyRequests:
		return true, nil
	case http.StatusNoContent:
		return false, nil
	}
	return false, fmt.Errorf("unexpected response status from rated: %q", rresp.Status)
}

var sink uint64

const testKey = "hello, world"

func BenchmarkMaphash(b *testing.B) {
	var h maphash.Hash
	for i := 0; i < b.N; i++ {
		h.Reset()
		h.WriteString(testKey)
		sink = h.Sum64()
	}
}
