Command rated implements standalone rate-limiting service with http
interface.

Its http endpoint accepts requests with non-empty query string used as a key
to check against limited set of buckets to do token-based rate limiting.

It responds either with 204 is request should be allowed, or 429 if it
should be limited.

Note that because of limited amount of buckets in use collisions are
expected.

    Usage of rated:
    -addr string
            address to listen (default "localhost:8080")
    -buckets int
            number of buckets to use (default 100000)
    -burst int
            burst amount (default 10)
    -refill duration
            time to refill bucket by 1 token (default 1s)

Usage example:

    rated -burst=3

Execute curl few times in a row with the same query:

    ~ ¶ for i in {1..3} ; do curl -sD- 'http://localhost:8080/?foo=bar' ;done
    HTTP/1.1 204 No Content
    Cache-Control: no-store
    Date: Sat, 28 Dec 2019 10:38:29 GMT

    HTTP/1.1 204 No Content
    Cache-Control: no-store
    Date: Sat, 28 Dec 2019 10:38:29 GMT

    HTTP/1.1 429 Too Many Requests
    Cache-Control: no-store
    Content-Type: text/plain; charset=utf-8
    X-Content-Type-Options: nosniff
    Date: Sat, 28 Dec 2019 10:38:29 GMT
    Content-Length: 18

    Too Many Requests