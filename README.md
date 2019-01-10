[![Build Status](https://travis-ci.org/nordborn/go-req.svg?branch=master)](https://travis-ci.org/nordborn/go-req)
[![Code Coverage](https://codecov.io/gh/nordborn/go-req/branch/master/graph/badge.svg)](https://codecov.io/gh/nordborn/go-req/branch/master/graph/badge.svg)


Package req provides high level HTTP requests API
with ideas from Python's 'requests' package.
It mostly focused to be a robust REST API client.


**Important features:**
- RetryOnStatusCodes parameter
- RetryOnTextMarkers parameter
- Middleware (slice of functions executing before each
              request attempt)
- Vals - ordered HTTP parameters (instead of url.Values which is a map)
- for now, it doesn't support sessions (and cookies in a request)
  and was developed mostly as a client for REST APIs


**Example1: Path, Params, Data, resp.JSON**
```Go
package main
import "https://github.com/nordborn/go-req"

func main() {
    r := req.New("http://httpbin.org")
    r.Path = "post" // => http://httpbin.org/post
    r.Params = req.Vals{{"a", "b"}, {"c": "d"}} // => http://httpbin.org/post?a=b&c=d
    r.Data = req.Vals{{"n1", "v1"}, {"n2", "v2"}} // => r.Body="n1=v1&n2=v2"
    resp, err := r.Post()
    respData := struct{
    	Data string `json:"data"`
    }{}
    err = resp.JSON(&respData) // unmarshal to the struct
    ...
}
```


**Example2: JSON Body, Headers, the power of Middleware (new headers on each attempt)**
```Go
package main
import "https://github.com/nordborn/go-req"

func main() {
    r := req.New("http://httpbin.org/get")
    r.Body = req.Vals{{"n1", "v1"}, {"n2", "v2"}}.JSON() // => r.Body=`{"n1":"v1", "n2":"v2"}` 
    mw := func() {
    	// new headers at each attempt
        r.Headers = Vals{
            req.HeaderAppJSON, 
            {"nonce", fmt.Sprint(time.Now().Unix())}
        }
    }
    r.Middleware = []func(){mw}
    resp, err := r.Get()
    ...
}
```


**Req contains fields:**
- Method: one of the allowed HTTP methods:
        "GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS" used by Send().
        Usually, you should use Get(), Post().. methods of the struct,
        so, basically, you don't need to set it directly.
- URL: basic URL ("http://example.com")
- Path: URL path after domain name ("/test/me/")
- Params: GET parameters as Vals:
       ({{"par1", "val1"}, {"par2", "val2"}} => ?par1=val1&par2=val2)
- Headers: HTTP headers as Vals: req.Vals{{"Content-Type", "application/json"}}
- ProxyURL: proxy URL ("http://user:name@ip:port")
- Data: POST/PATCH/PUT parameters as Vals
- Body: HTTP request body that contains urlencoded string
      (useful for JSON data or encoded POST/PUT/PATCH parameters).
      If provided, then Body will be used in request instead of Data
- Middleware: functions to be processed before each request
            and each retry attempt;
            they can modify Req fields.
            Useful for Headers and ProxyURL which should be
            updated before each retry attempt
- RetryOnTextMarkers: will trigger retry attempt if found any of
                    text markers from the slice
- RetryOnStatusCodes: will trigger retry attempt if found any of
                    (from <= status code <= to) in the list of {{from, to},...}
- RetryTimes: number of retry attempts before Req reports failed request
- RetryDelayMillis: delay in milliseconds before each retry attempt
- Timeout: timeout for a request

**Default arguments:**
```Go
// New generates Req with default arguments.
func New(url string) *Req {
	req := Req{
		URL:                url,
		Method:             "GET",
		RetryTimes:         3,
		RetryOnTextMarkers: []string{"error", "Error"},
		RetryOnStatusCodes: [][2]int{{400, 600}},
		RetryDelayMillis:   1,
		Timeout:            10 * time.Second,
	}
	return &req
```

**HTTP Methods**

```Go
// Get is shortcut for GET method
func (r *Req) Get() (*Resp, error) {}
func (r *Req) Post() (*Resp, error) {}
func (r *Req) Put() (*Resp, error) {}
func (r *Req) Delete() (*Resp, error) {}
func (r *Req) Patch() (*Resp, error) {}
func (r *Req) Options() (*Resp, error) {}
```