package client

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestAttributeDefaultUnmarshal(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
		set   bool
	}{
		{"null is no default", `null`, nil, false},
		{"empty string is no default", `""`, nil, false},
		{"whitespace is no default", `"   "`, nil, false},
		{"a string", `"us-east"`, []string{"us-east"}, true},
		{"a number", `42`, []string{"42"}, true},
		{"a decimal", `1.5`, []string{"1.5"}, true},
		{"a list", `["acme","globex"]`, []string{"acme", "globex"}, true},
		{"a list of numbers", `[1,2]`, []string{"1", "2"}, true},
		{"an empty list", `[]`, []string{}, true},

		// The case the whole json.Number choice exists for: this value is
		// beyond 2^53 - 1, and a float64 would round it to ...992.
		{"a number past the float64 safe range", `9007199254740993`, []string{"9007199254740993"}, true},
		{"the same value as a string", `"9007199254740993"`, []string{"9007199254740993"}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got AttributeDefault
			if err := json.Unmarshal([]byte(tc.input), &got); err != nil {
				t.Fatalf("unmarshal %s: %v", tc.input, err)
			}
			if got.Set != tc.set {
				t.Errorf("Set = %v, want %v", got.Set, tc.set)
			}
			if !reflect.DeepEqual(got.Values, tc.want) {
				t.Errorf("Values = %#v, want %#v", got.Values, tc.want)
			}
		})
	}
}

func TestAttributeDefaultUnmarshalRejectsOtherShapes(t *testing.T) {
	for _, input := range []string{`true`, `{"a":1}`, `[true]`} {
		var got AttributeDefault
		if err := json.Unmarshal([]byte(input), &got); err == nil {
			t.Errorf("%s was accepted, want an error", input)
		}
	}
}

func TestUserAttributeDefaultValueInContext(t *testing.T) {
	// A multi-valued attribute's default arrives as a list, which is the shape
	// the old *string field could not hold at all.
	const payload = `{"id":"a1","name":"region","label":"Region","type":"String",
		"multiple_values":true,"default_value":["us-east","eu-west"],"system":false}`

	var attr UserAttribute
	if err := json.Unmarshal([]byte(payload), &attr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !attr.MultipleValues {
		t.Fatal("MultipleValues = false, want true")
	}
	if want := []string{"us-east", "eu-west"}; !reflect.DeepEqual(attr.DefaultValue.Values, want) {
		t.Errorf("DefaultValue.Values = %#v, want %#v", attr.DefaultValue.Values, want)
	}
}

func TestUserAttributeInputBody(t *testing.T) {
	desc := "why it exists"

	t.Run("single valued sends a scalar", func(t *testing.T) {
		in := UserAttributeInput{
			Name: "region", Label: "Region", Type: "String",
			Description: &desc,
			Default:     AttributeDefault{Values: []string{"us-east"}, Set: true},
		}
		body := in.body(true)
		if got := body["default_value"]; got != "us-east" {
			t.Errorf("default_value = %#v, want the bare string", got)
		}
		if body["type"] != "String" {
			t.Errorf("type = %#v, want String on create", body["type"])
		}
		if body["description"] != desc {
			t.Errorf("description = %#v", body["description"])
		}
	})

	t.Run("multi valued sends a list", func(t *testing.T) {
		in := UserAttributeInput{
			Name: "region", Label: "Region", Type: "String", MultipleValues: true,
			Default: AttributeDefault{Values: []string{"a", "b"}, Set: true},
		}
		body := in.body(true)
		want := []string{"a", "b"}
		if got, ok := body["default_value"].([]string); !ok || !reflect.DeepEqual(got, want) {
			t.Errorf("default_value = %#v, want %#v", body["default_value"], want)
		}
	})

	t.Run("an unset default clears it", func(t *testing.T) {
		in := UserAttributeInput{Name: "region", Label: "Region", Type: "String"}
		body := in.body(true)
		got, present := body["default_value"]
		if !present {
			t.Fatal("default_value is absent; it has to be sent as null to clear a default")
		}
		if got != nil {
			t.Errorf("default_value = %#v, want nil", got)
		}
	})

	t.Run("update omits the immutable type", func(t *testing.T) {
		in := UserAttributeInput{Name: "region", Label: "Region", Type: "String"}
		if _, present := in.body(false)["type"]; present {
			t.Error("type was sent on update; it is immutable and the API ignores it")
		}
	})

	t.Run("an absent description is omitted rather than nulled", func(t *testing.T) {
		in := UserAttributeInput{Name: "region", Label: "Region", Type: "String"}
		if _, present := in.body(true)["description"]; present {
			t.Error("description was sent; PUT keeps omitted fields, so sending null would clear it")
		}
	})
}

func TestSelectableValuesSubjectQuery(t *testing.T) {
	if got := (SelectableValuesSubject{UserID: "m1"}).query().Encode(); got != "userId=m1" {
		t.Errorf("user query = %q", got)
	}
	if got := (SelectableValuesSubject{GroupID: "g1"}).query().Encode(); got != "groupId=g1" {
		t.Errorf("group query = %q", got)
	}
	if got := (SelectableValuesSubject{}).query().Encode(); got != "" {
		t.Errorf("empty subject query = %q, want empty", got)
	}
}
