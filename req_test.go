package req

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestReqGet(t *testing.T) {
	r := New("http://ip-api.com")
	r.Path = "json"

	resp, err := r.Send()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(resp.Text())
}

func TestReqPost(t *testing.T) {
	r := New("http://httpbin.org")
	r.Path = "post"

	resp, err := r.Post()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(resp.Text())
}

func TestReqGet_cookies(t *testing.T) {
	r := New("http://httpbin.org")
	r.Path = "get"
	cookies := []*http.Cookie{
		{Name: "CookieName1", Value: "CookieVal1"},
		{Name: "CookieName2", Value: "CookieVal2"},
	}
	r.Cookies = cookies

	resp, err := r.Get()
	if err != nil {
		t.Fatal(err)
	}
	respCookieStr := `"Cookie": "CookieName1=CookieVal1; CookieName2=CookieVal2"`
	if !strings.Contains(resp.Text(), respCookieStr) {
		t.Fatalf("Expected, but not found '%v', resp text: %v\n",
			respCookieStr,
			resp.Text())
	}
	t.Log(resp.Text())
}

func TestReqGet_shouldRetryTextMarkers(t *testing.T) {
	r := New("http://httpbin.org")
	r.Path = "post"
	r.Data = Vals{
		{"error", "error"},
	}

	resp, err := r.Post()
	if err == nil {
		t.Fatal(err)
	}
	t.Log(resp.Text())
}

func TestReqDelete(t *testing.T) {
	r := New("http://httpbin.org")
	r.Path = "delete"

	resp, err := r.Delete()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(resp.Text())
}

func TestReqPut(t *testing.T) {
	r := New("http://httpbin.org")
	r.Path = "put"

	resp, err := r.Put()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(resp.Text())
}

func TestReqPatch(t *testing.T) {
	r := New("http://httpbin.org")
	r.Path = "patch"

	resp, err := r.Patch()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(resp.Text())
}

func TestReqOptions_ExpectedErr(t *testing.T) {
	r := New("http://httpbin.org")
	r.Path = "options"

	// expected err because httpbin doesn't support Options
	_, err := r.Options()
	if err == nil {
		t.Fatal("Expected err, but got nil")
	}
	t.Log(err)
}

func TestReqSend_ExpectErr(t *testing.T) {
	r := New("ip-api.com")
	r.Path = "json"
	r.RetryTimes = 1

	_, err := r.Send()
	if err == nil {
		t.Fatal(err)
	}
	t.Log("Expected error:", err)
}

func TestReqSend_shouldRetryStatusCode(t *testing.T) {
	r := New("http://httpbin.org")
	r.Path = "get"
	r.RetryTimes = 1

	_, err := r.Post() // method not allowed
	if err == nil {
		t.Fatal(err)
	}
	if !strings.Contains(err.Error(), "got unwanted status code") {
		t.Fatal("Unexpected error msg", err)
	}
	t.Log("Expected error:", err)
}

func TestReqGetJSON_MiddlewareVals(t *testing.T) {
	r := New("http://httpbin.org")
	r.Path = "get"
	r.Body = Vals{
		{"name1", "val1"},
		{"name2", "val2"},
	}.JSON()
	mw := func() {
		r.Headers = Vals{
			HeaderAppJSON,
			{"Now", fmt.Sprint(time.Now().Unix())},
		}
	}
	r.Middleware = []func(){mw}
	resp, err := r.Get()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.Text(), "Now") {
		t.Fatal("Unexpected response:", resp.Text())
	}
	t.Log(resp.Text())
}

func TestReqPost_RespJSON(t *testing.T) {
	r := New("http://httpbin.org")
	r.Path = "post"
	r.Data = Vals{
		{"TestName", "TestVal"},
	}

	respData := struct {
		Data string `json:"data"`
	}{}

	resp, err := r.Post()
	if err != nil {
		t.Fatal(err)
	}

	err = resp.JSON(&respData)
	if err != nil {
		t.Fatal(err)
	}

	if respData.Data != "TestName=TestVal" {
		t.Fatal("Unexpected response:", resp.Text())
	}
	t.Log(resp.Text())
}

func TestReqPost_RespJSONExpectedErr(t *testing.T) {
	r := New("http://httpbin.org")
	r.Path = "post"
	r.Data = Vals{
		{"name1", "val1"},
		{"name2", "val2"},
	}

	respData := struct {
		Data string `json:"data"`
	}{}

	resp, err := r.Post()
	if err != nil {
		t.Fatal(err)
	}

	// incorrect call, should fail
	err = resp.JSON(respData)
	if err == nil {
		t.Fatal(err)
	}

	t.Log(resp.Text())
}

func TestReqGet_WProxy(t *testing.T) {
	r := New("http://ip-api.com")
	r.Path = "json"
	r.ProxyURL = os.Getenv("PROXY")
	r.RetryTimes = 1

	resp, err := r.Send()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(resp.Text())
}

func TestReq_shouldRetryOnStatusCode(t *testing.T) {

	assertions := []struct {
		code     int
		pairs    [][2]int
		expected bool
	}{
		{200, [][2]int{{400, 600}}, false},
		{505, [][2]int{{400, 600}}, true},
		{505, [][2]int{{400, 500}, {506, 600}}, false},
	}

	for _, a := range assertions {
		result := shouldRetryOnStatusCode(a.code, a.pairs)
		if result != a.expected {
			t.Errorf("%v != %v for %v\n", result, a.expected, a)
		}
	}
}

func TestReq_shouldRetryOnTextMarkers(t *testing.T) {

	assertions := []struct {
		text     string
		markers  []string
		expected bool
	}{
		{"", []string{"error", "Error"}, false},
		{"error", []string{"error", "Error"}, true},
		{"Error", []string{"error", "Error"}, true},
	}

	for _, a := range assertions {
		result := shouldRetryOnTextMarker([]byte(a.text), a.markers)
		if result != a.expected {
			t.Errorf("%v != %v for %v\n", result, a.expected, a)
		}
	}
}

func TestVals_String(t *testing.T) {
	v := Vals{{"name", Vals{{"k", "v"}}.JSON()},
		{"name2", "val2"}}
	s := fmt.Sprint(v)
	if s != `[{"name":{"k":"v"}} {"name2":"val2"}]` {
		t.Fatal("Unexpected v.String:", s)
	}
}

func TestVals_JSON1(t *testing.T) {
	v := Vals{{"n1", "v1"}, {"n2", "v2"}}
	s := v.JSON()
	if s != `{"n1":"v1","n2":"v2"}` {
		t.Fatal("Unexpected v.JSON:", s)
	}
}

func TestVals_JSON2(t *testing.T) {
	v := Vals{{"name", `["val1","val2"]`}}
	s := v.JSON()
	if s != `{"name":["val1","val2"]}` {
		t.Fatal("Unexpected v.JSON:", s)
	}
}

func TestVals_JSON3(t *testing.T) {
	v := Vals{{"name", Vals{{"nsub1", "vsub1"}}.JSON()}}
	s := v.JSON()
	if s != `{"name":{"nsub1":"vsub1"}}` {
		t.Fatal("Unexpected v.JSON:", s)
	}
}

func TestVals_Extend(t *testing.T) {
	v := Vals{{"name", "val"}}.Extend(
		Vals{{"name2", "val2"}})
	s := fmt.Sprint(v)
	if !strings.Contains(s, "val2") {
		t.Fatal("Unexpected v.String:", s)
	}
}

func Test_buildFullURL(t *testing.T) {
	base := "http://httpbin.org"
	path := "get"
	params := Vals{{"c", "d"}}
	fullURL, err := buildFullURL(base, path, params)
	if err != nil {
		t.Fatal(err)
	}
	if fullURL != "http://httpbin.org/get?c=d" {
		t.Fatal(fullURL)
	}
	t.Log(fullURL)
}
