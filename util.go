package req

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/nordborn/go-errow"
	"github.com/nordborn/golog"
)

func buildFullURL(base, path string, getParams Vals) (string, error) {
	baseURL, err := url.Parse(base)
	if err != nil {
		return "", errow.Wrap(err)
	}
	pathURL, err := url.Parse(path)
	if err != nil {
		return "", errow.Wrap(err)
	}

	reqURL := baseURL.ResolveReference(pathURL)
	// redefine get params if provided OR use get params from 'path'
	if getParams != nil {
		reqURL.RawQuery = getParams.URLEncode()
	}

	return reqURL.String(), nil
}

func shouldRetryOnStatusCode(statusCode int, retryOnCodes [][2]int) bool {
	for _, codesPair := range retryOnCodes {
		if statusCode >= codesPair[0] && statusCode <= codesPair[1] {
			golog.Tracef("found matched code %v in %v\n", statusCode, codesPair)
			return true
		}
	}
	return false
}

func shouldRetryOnTextMarker(content []byte, repeatOnTextMarkers []string) bool {
	for _, textMarker := range repeatOnTextMarkers {
		if bytes.Contains(content, []byte(textMarker)) {
			return true
		}
	}
	return false
}

func delay(delayMillis int) {
	if delayMillis > 0 {
		<-time.After(time.Duration(delayMillis) * time.Millisecond)
	}
}

// setHeaders modifies request: it sets headers
func setHeaders(request *http.Request, headers Vals) {
	for _, v := range headers {
		request.Header.Set(v.K, fmt.Sprint(v.V))
	}
}

// setCookies modifies request: it sets cookies
func setCookies(request *http.Request, cookies []*http.Cookie) {
	for _, c := range cookies {
		request.AddCookie(c)
	}
}
