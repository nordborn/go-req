package req

import (
	"fmt"
	"net/url"
	"strings"
)

// val represents single key:value pair
type val struct {
	// key
	K string
	// value
	V any
}

// hasSeqSig detects [] and {}
func hasSeqSig(s string) bool {
	return (strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")) ||
		(strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]"))
}

func (v val) String() string {
	if val, ok := v.V.(string); ok {
		if hasSeqSig(val) {
			return fmt.Sprintf(`"%s":%s`, v.K, val)
		} else {
			return fmt.Sprintf(`"%v":"%v"`, v.K, val)
		}
	}
	return fmt.Sprintf(`"%v":%v`, v.K, v.V)
}

// Vals is a slice of *val.
// It's the way to se all necessary request parameters in the package:
// r.Params = req.Vals{{"n1", "v1"}, {"n2", "v2"}}
// r.Data = req.Vals{{"n1", "v1"}, {"n2", "v2"}}
// r.Body = req.Vals{{"n1", "v1"}, {"n2", "v2"}}.JSON()
// r.Headers = req.Vals{{"User-Agent", "Req"}}
// We use it of url.Values to keep ordering of parameters
type Vals []val

// URLEncode similar to net/url.Encode for url.Values, but for Vals
func (vals Vals) URLEncode() (urlEncoded string) {
	for i, param := range vals {
		enc := url.QueryEscape(param.K) + "=" + url.QueryEscape(fmt.Sprint(param.V))
		if i == 0 {
			urlEncoded = enc
		} else {
			urlEncoded += "&" + enc
		}
	}
	return urlEncoded
}

// JSON method returns _ORDERED_ map (in order of vals appearance) of Vals as JSON string
// like {"v.K": "v.V", ...}.
// Use it for simple cases when v.K and v.V can be correctly
// converted just with fmt.Sprintf(`"%s":"%s"`, v.K, v.V),
// or with fmt.Sprintf(`"%s":%s`, v.K, v.V) if v.V like "{...}" or "[...]".
// In other cases use json.Marshal
func (vals Vals) JSON() string {
	s := "{"
	for i, v := range vals {
		if i > 0 {
			s += ","
		}
		s += v.String()
	}
	s += "}"
	return s
}

// Extend adds more Vals to existing Vals
// and returns new Vals
func (vals Vals) Extend(more Vals) Vals {
	return append(vals, more...)
}
