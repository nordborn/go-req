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
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nordborn/go-errow"
	"github.com/nordborn/golog"
)

var (
	// simple shortcut
	HeaderAppJSON = val{"Content-Type", "application/json"}
)

// Req is a structure for requests.
// Preferred usage: req.New() to create a new Req,
// then modify necessary fields.
// Builder funcs With... can be used.
// It's safe to modify fields before Send()
// (final raw request is builing at that moment)
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

	// POST/PATCH/PUT parameters as urlencoded Vals
	Form Vals

	// Body is a HTTP request body that contains urlencoded string
	// (useful for JSON data or encoded POST/PUT/PATCH parameters).
	// If provided, then Body will be used in request instead of Form
	Body string

	// Middleware is the slice of functions to be processed
	// before each request and each retry attempt;
	// they can modify Req fields.
	// Useful for example, for Headers and ProxyURL if they should be
	// updated before each retry attempt
	// (see TestReqGetJSON_MiddlewareVals)
	Middleware []func()

	// RetryOnTextMarkers will trigger retry attempt if found any of
	// text markers from the slice
	// Default is []string{"error", "Error"}
	RetryOnTextMarkers []string // repeat if response text contains a text marker from the list

	// RetryOnStatusCodes will trigger retry attempt if found any of
	// (from <= status code <= to) in the list of {{from, to},...}
	// Default is [][2]int{{400, 600}}
	RetryOnStatusCodes [][2]int

	// Attempts: number of attempts before Req reports failed request
	Attempts int

	// RetryDelayMillis: delay in milliseconds before each retry attempt
	RetryDelayMillis int

	// Timeout: timeout for a request
	Timeout time.Duration

	// Cookies slice (not cookiejar).
	// Each cookie will be added to the request
	Cookies []*http.Cookie

	reqRaw *http.Request
	Client *http.Client
}

// New generates Req with default arguments.
// Note, that using default global `http.DefaultClient` is not suitable
// for different timeouts and proxies (client's transport-level settings).
// In this case set custom client.
// `Client.Transport` is expected to be `*http.Transport` to manage proxies.
func New(url string) *Req {
	req := Req{
		URL:                url,
		Method:             "GET",
		Attempts:           1,
		RetryOnTextMarkers: []string{"error", "Error"},
		RetryOnStatusCodes: [][2]int{{400, 600}},
		RetryDelayMillis:   1,
		Timeout:            30 * time.Second,
		Client:             http.DefaultClient,
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
	// suitable to apply middleware between attempts
	buildReqRaw := func() error {
		r.Client.Timeout = r.Timeout

		if r.ProxyURL != "" {
			proxyURL, err := url.Parse(r.ProxyURL)
			if err != nil {
				return errow.Wrap(err, "bad proxy url")
			} else {
				t := r.Client.Transport.(*http.Transport)
				t.Proxy = http.ProxyURL(proxyURL)
			}
		}

		fullURL, err = buildFullURL(r.URL, r.Path, r.Params)
		if err != nil {
			return errow.Wrap(err, "bad full url")
		}

		// set reqBody from Data if provided or directly from Body
		var reqBody string
		if r.Form != nil {
			reqBody = r.Form.URLEncode()
		} else {
			reqBody = r.Body
		}

		r.reqRaw, err = http.NewRequest(r.Method, fullURL, strings.NewReader(reqBody))
		if err != nil {
			return errow.Wrap(err, "bad req raw")
		}
		setCookies(r.reqRaw, r.Cookies)
		setHeaders(r.reqRaw, r.Headers)
		return nil
	}

	for attempt = 1; attempt <= r.Attempts; attempt++ {
		shouldRetry := false

		for _, f := range r.Middleware {
			f()
		}

		// first time or after middleware
		if r.reqRaw == nil || len(r.Middleware) > 0 {
			if buildReqRaw() != nil {
				return nil, err // already wrapped err
			}
		}

		// applied closure to close resp Body in the loop even if err occur
		func() {
			golog.Tracef("do request: %v %v\n", r.Method, fullURL)
			respRaw, err = r.Client.Do(r.reqRaw)
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
			defer respRaw.Body.Close()
			content, err = io.ReadAll(respRaw.Body)
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

// Get is shortcut to send GET method
func (r *Req) Get() (*Resp, error) {
	r.Method = "GET"
	return r.Send()
}

// Post is a shortcut to send POST method
func (r *Req) Post() (*Resp, error) {
	r.Method = "POST"
	return r.Send()
}

// Post is a shortcut to send POST method
func (r *Req) Put() (*Resp, error) {
	r.Method = "PUT"
	return r.Send()
}

// Delete is a shortcut to send DELETE method
func (r *Req) Delete() (*Resp, error) {
	r.Method = "DELETE"
	return r.Send()
}

// Patch is a shortcut to send PATCH method
func (r *Req) Patch() (*Resp, error) {
	r.Method = "PATCH"
	return r.Send()
}

// Options is a shortcut to send OPTIONS method
func (r *Req) Options() (*Resp, error) {
	r.Method = "OPTIONS"
	return r.Send()
}

// WithForm is a build func for Form field
func (r *Req) WithForm(form Vals) *Req {
	r.Form = form
	return r
}

// WithBody is a build func for Body field
func (r *Req) WithBody(body string) *Req {
	r.Body = body
	return r
}

// WithPath is a build func for Path field
func (r *Req) WithPath(path string) *Req {
	r.Path = path
	return r
}

// WithParams is a build func for Params field
func (r *Req) WithParams(params Vals) *Req {
	r.Params = params
	return r
}

// WithHeaders is a build func for Headers field
func (r *Req) WithHeaders(headers Vals) *Req {
	r.Headers = headers
	return r
}

// WithProxyURL is a build func for ProxyURL field
func (r *Req) WithProxyURL(proxyURL string) *Req {
	r.ProxyURL = proxyURL
	return r
}

// WithRetryOnTextMarkers is a build func for RetryOnTextMarkers field
func (r *Req) WithMiddleware(funcs []func()) *Req {
	r.Middleware = funcs
	return r
}

// WithRetryOnTextMarkers is a build func for RetryOnTextMarkers field
func (r *Req) WithRetryOnTextMarkers(markers []string) *Req {
	r.RetryOnTextMarkers = markers
	return r
}

// WithRetryOnStatusCodes is a build func for RetryOnStatusCodes field
func (r *Req) WithRetryOnStatusCodes(codeRanges [][2]int) *Req {
	r.RetryOnStatusCodes = codeRanges
	return r
}

// WithAttempts is a build func for Attempts field
func (r *Req) WithAttempts(attempts int) *Req {
	r.Attempts = attempts
	return r
}

// WithRetryDelayMillis is a build func for RetryDelayMillis field
func (r *Req) WithRetryDelayMillis(millis int) *Req {
	r.RetryDelayMillis = millis
	return r
}

// WithTimeout is a build func for Timeout field
func (r *Req) WithTimeout(timeout time.Duration) *Req {
	r.Timeout = timeout
	return r
}

// WithCookies is a build func for Cookies field
func (r *Req) WithCookies(cookies []*http.Cookie) *Req {
	r.Cookies = cookies
	return r
}

// WithClient is a build func for Client field
func (r *Req) WithClient(client *http.Client) *Req {
	r.Client = client
	return r
}
