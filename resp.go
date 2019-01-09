package req

import (
	"encoding/json"
	"github.com/nordborn/go-errow"
	"net/http"
)

// Resp represents the HTTP response
// Resp public fields:
// Content - response body as slice of bytes
// RespRaw - underlying *http.Response, it's public to provide ability for low-level access
type Resp struct {
	Content []byte
	RespRaw *http.Response
	text    string
}

// Text returns string of Content of the resp.
// It will be cached after first call
func (resp *Resp) Text() string {
	if resp.text == "" && resp.Content != nil {
		resp.text = string(resp.Content)
	}
	return resp.text
}

// JSON allows to unmarshal response content to a structure
// Example:
// data := struct {
//   Name string
// }{}
// resp, _ := req.Get(...)
// err := resp.JSON(&data)
//
func (resp *Resp) JSON(unmarshalToPtr interface{}) error {
	err := json.Unmarshal(resp.Content, unmarshalToPtr)
	if err != nil {
		return errow.Wrap(err)
	}
	return nil
}
