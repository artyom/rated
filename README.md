Command rated implements standalone rate-limiting service with an HTTP
interface.

Its HTTP endpoint accepts requests with non-empty query string used as a key to
check against a limited set of buckets to do token-based rate limiting. It
treats the whole query as-is as a key, so queries `foo=1&bar=2` and
`bar=2&foo=1` represent two distinct keys.

It responds either with 204 if a request should be allowed, or 429 if it should
be limited.

Note that because of a limited amount of buckets in use collisions are
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

        rated -burst=2

Execute curl three times in a row with the same query:

        ~ ¶ for i in {1..3} ; do curl -sD- 'http://localhost:8080/?somekey' ;done
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

Notice how the third request returns 429 because it exceeds a default burst of
2 requests.

## Custom per-key limits

You can apply custom per-key rate limits with "Burst" and "Refill" HTTP
headers. "Refill" header is parsed with
[time.ParseDuration](https://golang.org/pkg/time/#ParseDuration). If either
header value cannot be parsed, the endpoint responds with 400 Bad Request.

Start rated with default settings and a burst of 2:

        rated -burst=2

Check limits for some key few times in a row as above, but now applying custom
rate limits:

        ~ ¶ for i in {1..3} ; do curl -sD- -H "Burst: 3" -H "Refill: 1s" 'http://localhost:8080/?otherkey' ;done
        HTTP/1.1 204 No Content
        Cache-Control: no-store
        Date: Mon, 30 Dec 2019 05:34:19 GMT

        HTTP/1.1 204 No Content
        Cache-Control: no-store
        Date: Mon, 30 Dec 2019 05:34:19 GMT

        HTTP/1.1 204 No Content
        Cache-Control: no-store
        Date: Mon, 30 Dec 2019 05:34:19 GMT

Notice how all three requests are allowed because of the custom burst of 3.
