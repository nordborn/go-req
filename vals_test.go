package req

import (
	"fmt"
	"strings"
	"testing"
)

func TestVals_String(t *testing.T) {
	v := Vals{{"name", Vals{{"k", "v"}}.JSON()},
		{"name2", "val2"}}
	s := fmt.Sprint(v)
	if s != `["name":{"k":"v"} "name2":"val2"]` {
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

func TestVals_JSON11(t *testing.T) {
	v := Vals{{"n1", "v1"}, {"n2", 2}}
	s := v.JSON()
	if s != `{"n1":"v1","n2":2}` {
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

func TestVals_JSON4(t *testing.T) {
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
