package helpers

import (
	"hedge/common/utils"
	"testing"
)

func TestContains(t *testing.T) {
	cases := []struct {
		s    []string
		e    string
		want bool
	}{
		{[]string{"apple", "banana", "orange"}, "banana", true},
		{[]string{"apple", "banana", "orange"}, "grape", false},
		{[]string{}, "anything", false},
		{[]string{"hello", "world"}, "hello", true},
		{[]string{"single"}, "single", true},
		{[]string{"single"}, "double", false},
	}

	for _, c := range cases {
		got := utils.Contains(c.s, c.e)
		if got != c.want {
			t.Errorf("Contains(%q, %q) == %t, want %t", c.s, c.e, got, c.want)
		}
	}
}

func TestConvCSVArrayToJSON(t *testing.T) {
	cases := []struct {
		cSVArray []string
		wantJSON string
		wantErr  string
	}{
		{
			[]string{"name,age", "Alice,30", "Bob,25"},
			`[{"name":"Alice","age":30},{"name":"Bob","age":25}]`,
			"",
		},
		{
			[]string{"id,active", "1,true", "2,false"},
			`[{"id":1,"active":true},{"id":2,"active":false}]`,
			"",
		},
		{
			[]string{},
			"",
			"data not available",
		},
		{
			[]string{"header"},
			"",
			"data not available",
		},
	}

	for _, c := range cases {
		gotJSON, gotErr := ConvCSVArrayToJSON(c.cSVArray)
		if gotErr != c.wantErr {
			t.Errorf("ConvCSVArrayToJSON() error = %v, wantErr %v", gotErr, c.wantErr)
			continue
		}
		if gotErr == "" && !jsonDeepEqual(gotJSON, []byte(c.wantJSON)) {
			t.Errorf("ConvCSVArrayToJSON() = %v, want %v", string(gotJSON), c.wantJSON)
		}
	}
}
