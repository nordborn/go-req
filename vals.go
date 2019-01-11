package req

import (
	"fmt"
	"net/url"
)

// val represents singe name:value pair
type val struct {
	Name  string
	Value string
}

func (v *val) String() string {
	return fmt.Sprintf(`{"%v": "%v"}`, v.Name, v.Value)
}

// Vals is a slice of *val.
// It's the way to se all necessary request parameters in the package:
// r.Params = req.Vals{{"n1", "v1"}, {"n2", "v2"}}
// r.Data = req.Vals{{"n1", "v1"}, {"n2", "v2"}}
// r.Body = req.Vals{{"n1", "v1"}, {"n2", "v2"}}.JSON()
// r.Headers = req.Vals{{"User-Agent", "Req"}}
// We use it of url.Values to keep ordering of parameters
type Vals []*val

// URLEncode similar to net/url.Encode for url.Values, but for Vals
func (vals Vals) URLEncode() (urlEncoded string) {
	for i, param := range vals {
		enc := url.QueryEscape(param.Name) + "=" + url.QueryEscape(param.Value)
		if i == 0 {
			urlEncoded = enc
		} else {
			urlEncoded += "&" + enc
		}
	}
	return urlEncoded
}

// JSON method returns _ORDERED_ map of Vals as JSON string
// like {"v.Name": "v.Val", ...}.
// Use it for simple cases when v.Name and v.Val can be correctly
// converted just with fmt.Sprintf(`"%s":"%s"`, v.Name, v.Value),
// or use json.Unmarshal for more complex cases
func (vals Vals) JSON() string {
	s := "{"
	for i, v := range vals {
		if i > 0 {
			s += ","
		}
		s += fmt.Sprintf(`"%s":"%s"`, v.Name, v.Value)
	}
	s += "}"
	return s
}

// Extend adds more Vals to existing Vals
// and returns new Vals
func (vals Vals) Extend(more Vals) Vals {
	return append(vals, more...)
}
