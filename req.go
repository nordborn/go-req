// Package req provides simple high-level API
// mostly suitable to build robust REST API clients.
// It was created by ideas of Python's 'requests' package.
//
// Important features:
// - RetryOnStatusCodes parameter
// - RetryOnTextMarkers parameter
// - Middleware (slice of functions executing before each
//               request attempt)
// - Vals - ordered HTTP parameters (instead of url.Values which is a map)
// - for now, it doesn't support sessions (you should pass cookies directly)
//   and was developed mostly as a client for REST APIs
//
// ---
// Example1: Path, Params, Data, resp.JSON
//
// r := req.New("http://httpbin.org")
// r.Path = "post" // => http://httpbin.org/post
// r.Params = req.Vals{{"a", "b"}, {"c": "d"}} // => http://httpbin.org/post?a=b&c=d
// r.Data = req.Vals{{"n1", "v1"}, {"n2", "v2"}} // => r.Body="n1=v1&n2=v2"
// resp, err := r.Post()
// ...
// respData := struct{
//   Data string `json:"data"`
// }{}
// err = resp.JSON(&respData) // unmarshal to the struct
//
// ---
// Example2: JSON Body, Headers, the power of Middleware
//
// r := req.New("http://httpbin.org/get")
// r.Body = req.Vals{{"n1", "v1"}, {"n2", "v2"}}.JSON() // => {"n1":"v1", "n2":"v2"}
// mw := func() {
//   // New headers for each attempt
//   r.Headers = Vals{
//     req.HeaderAppJSON,
//     {"Now", fmt.Sprint(time.Now().Unix())}
//   }
// }
// r.Middleware = []func(){mw}
// resp, err := r.Get()

package req

import (
	"fmt"
	"github.com/nordborn/go-errow"
	"github.com/nordborn/golog"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	// simple shortcut
	HeaderAppJSON = &val{"Content-Type", "application/json"}
)

// Req is a structure for requests.
// Preferred usage: req.New() to create a new Req,
// then modify necessary fields
type Req struct {
	// Method is one of the allowed HTTP methods:
	// "GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS" used by Send().
	// Usually, you should use Get(), Post().. methods of the struct,
	// so, basically, you don't need to set it directly.
	Method string
	// URL is a basic URL ("http://example.com")
	URL string
	// Path is URL path after domain name ("/test/me/") without get parameters
	Path string
	// Params: get parameters as Vals:
	// ({{"par1", "val1"}, {"par2", "val2"}} => ?par1=val1&par2=val2)
	Params Vals
	// Headers: HTTP headers as Vals: req.Vals{{"Content-Type", "application/json"}}
	Headers Vals
	// ProxyURL should be string in format "http://user:name@ip:port"
	ProxyURL string
	// POST/PATCH/PUT parameters as Vals
	Data Vals
	// Body is a HTTP request body that contains urlencoded string
	// (useful for JSON data or encoded POST/PUT/PATCH parameters).
	// If provided, then Body will be used in request instead of Data
	Body string
	// Middleware is the slice of functions to be processed
	// before each request and each retry attempt;
	// they can modify Req fields.
	// Useful for Headers and ProxyURL which should be
	// updated before each retry attempt
	Middleware []func()
	// RetryOnTextMarkers will trigger retry attempt if found any of
	// text markers from the slice
	RetryOnTextMarkers []string // repeat if response text contains a text marker from the list
	// RetryOnStatusCodes will trigger retry attempt if found any of
	// (from <= status code <= to) in the list of {{from, to},...}
	RetryOnStatusCodes [][2]int
	// RetryTimes: number of retry attempts before Req reports failed request
	RetryTimes int
	// RetryDelayMillis: delay in milliseconds before each retry attempt
	RetryDelayMillis int
	// Timeout: timeout for a request
	Timeout time.Duration
	// Cookies slice (not cookiejar).
	// Each cookie will be added to the request
	Cookies []*http.Cookie
	// Transport allows to set custom transport
	// (default transport is set in New())
	Transport *http.Transport
	reqRaw    *http.Request
	client    *http.Client
}

// New generates Req with default arguments.
func New(url string) *Req {
	t := http.Transport{}
	// Disable 'keep alive' is a way to disable idle connections
	// or they will use a huge vol of memory on frequent requests.
	// You may tune the transport yourself
	// (set number of idle connections etc.)
	t.DisableKeepAlives = true

	req := Req{
		URL:                url,
		Method:             "GET",
		RetryTimes:         3,
		RetryOnTextMarkers: []string{"error", "Error"},
		RetryOnStatusCodes: [][2]int{{400, 600}},
		RetryDelayMillis:   1,
		Timeout:            10 * time.Second,
		Transport:          &t,
	}

	return &req
}

// ReqRaw provides read access to underlying http.Request
// _after_ http request.
// You can't set reqRaw directly because it have to be
// built before each request attempt
func (r *Req) ReqRaw() *http.Request {
	return r.reqRaw
}

// Send provides HTTP request with given arguments.
// If postParams passed, then usual PostForm method wil be used
// Req builds client and reqRaw once and
// then at each attempt only if len(Middleware) > 0
func (r *Req) Send() (*Resp, error) {
	var (
		respRaw *http.Response
		content []byte
		myResp  Resp
		err     error
		attempt int
		success bool
		reason  string
		fullURL string
	)

	// closure to call from attempt
	build := func() error {
		if r.ProxyURL != "" {
			proxyURL, err := url.Parse(r.ProxyURL)
			if err != nil {
				golog.Errorf("can't parse proxy url %v: %v\n", r.ProxyURL, err)
			} else {
				r.Transport.Proxy = http.ProxyURL(proxyURL)
			}
		}

		r.client = &http.Client{Transport: r.Transport, Timeout: r.Timeout}

		fullURL, err = buildFullURL(r.URL, r.Path, r.Params)
		if err != nil {
			return errow.Wrap(err)
		}

		// set reqBody from Data if provided or directly from Body
		var reqBody string
		if r.Data != nil {
			reqBody = r.Data.URLEncode()
		} else {
			reqBody = r.Body
		}

		r.reqRaw, err = http.NewRequest(r.Method, fullURL, strings.NewReader(reqBody))
		if err != nil {
			return errow.Wrap(err)
		}
		setCookies(r.reqRaw, r.Cookies)
		setHeaders(r.reqRaw, r.Headers)
		return nil
	}

	for attempt = 1; attempt <= r.RetryTimes; attempt++ {
		shouldRetry := false

		for _, f := range r.Middleware {
			f()
		}

		// first time or after middleware
		if r.client == nil || len(r.Middleware) > 0 {
			err = build()
			if err != nil {
				return nil, err // already wrapped
			}
		}

		// applied closure to close resp Body in the loop even if err occur
		func() {
			golog.Tracef("do request: %v %v\n", r.Method, fullURL)
			respRaw, err = r.client.Do(r.reqRaw)
			if respRaw != nil {
				defer respRaw.Body.Close()
			}
			if err != nil {
				shouldRetry = true
				errStr := err.Error()
				if strings.Contains(errStr, fullURL) {
					golog.Warningf("att #%v: resp err: %v. Retry\n", attempt, errStr)
				} else {
					golog.Warningf("att #%v: resp err: %v: %v. Retry\n", attempt, fullURL, errStr)
				}
				reason = errStr
				return
			}
			content, err = ioutil.ReadAll(respRaw.Body)
			if err != nil {
				shouldRetry = true
				golog.Warningf("att #%v: resp read err: %v: %v. Retry\n", attempt, fullURL, err)
				reason = err.Error()
				return
			}
		}()

		if shouldRetry {
			delay(r.RetryDelayMillis)
			continue
		}

		if shouldRetryOnStatusCode(respRaw.StatusCode, r.RetryOnStatusCodes) {
			golog.Warningf(
				"att #%v: %v: got resp with repeatOnCodes. Code='%v', content='%s' . Retry\n",
				attempt, fullURL, respRaw.StatusCode, content)
			reason = fmt.Sprintf(
				"finally got unwanted status code '%v' and content '%s'",
				respRaw.StatusCode, content)
			delay(r.RetryDelayMillis)
			continue
		}

		if shouldRetryOnTextMarker(content, r.RetryOnTextMarkers) {
			golog.Warningf(
				"att #%v: %v: got resp with repeatOnTextMarkers. Code='%v', content='%s'. Retry\n",
				attempt, fullURL, respRaw.StatusCode, content)
			reason = fmt.Sprintf(
				"finally got unwanted text marker in resp with status code '%v' and content '%s'",
				respRaw.StatusCode, content)
			delay(r.RetryDelayMillis)
			continue
		}
		// no errors or retry cases
		success = true
		break
	}

	myResp = Resp{Content: content, RespRaw: respRaw}

	if !success {
		// avoid duplicated url in the msg
		msg := ""
		if strings.Contains(reason, fullURL) {
			msg = reason
		} else {
			msg = fmt.Sprintf("%v %v: %v", r.Method, fullURL, reason)
		}
		return &myResp, errow.New("FAILED: ", msg)
	}
	golog.Traceln("SUCCESS:", r.Method, fullURL)
	return &myResp, nil
}

// Get is shortcut for GET method
func (r *Req) Get() (*Resp, error) {
	r.Method = "GET"
	return r.Send()
}

// Post is a shortcut for POST method
func (r *Req) Post() (*Resp, error) {
	r.Method = "POST"
	return r.Send()
}

// Post is a shortcut for POST method
func (r *Req) Put() (*Resp, error) {
	r.Method = "PUT"
	return r.Send()
}

// Delete is a shortcut for DELETE method
func (r *Req) Delete() (*Resp, error) {
	r.Method = "DELETE"
	return r.Send()
}

// Patch is a shortcut for PATCH method
func (r *Req) Patch() (*Resp, error) {
	r.Method = "PATCH"
	return r.Send()
}

// Options is a shortcut for OPTIONS method
func (r *Req) Options() (*Resp, error) {
	r.Method = "OPTIONS"
	return r.Send()
}
