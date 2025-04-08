package req

import "testing"

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
