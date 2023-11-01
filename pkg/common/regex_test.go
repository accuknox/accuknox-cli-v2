package common

import (
	"reflect"
	"regexp"
	"testing"
)

func TestParseString(t *testing.T) {
	tests := []struct {
		input    string
		flagName string
		expected string
		err      bool
	}{
		{"--gRPC=\"localhost:8080\"", "gRPC", "localhost:8080", false},
		{"-gRPC=\"localhost:8080\"", "gRPC", "localhost:8080", false},
		{"--format=json", "format", "json", false},
		{"-format=json", "format", "json", false},
		{"--gRPC=\"localhost=8080\"", "gRPC", "localhost=8080", false},
		{"--flag123=value123", "flag123", "value123", false},
		{"--flag_with_underscore=value_with_underscore", "flag_with_underscore", "value_with_underscore", false},
		{"--flag-with-dash=value-with-dash", "flag-with-dash", "value-with-dash", false},
		{"--flag=\"value with spaces\"", "flag", "value with spaces", false},
		{"--multi=part=value", "multi", "part=value", false},
		{"--multi:part=\"value:with:colons\"", "multi:part", "value:with:colons", false},
		{"-multi:part=\"value:with:colons\"", "multi:part", "value:with:colons", false},
		{"--strange=\"=:=:=value=:=:=\"", "strange", "=:=:=value=:=:=", false},
		{"-f value1", "f", "value1", false},
		{"--invalidFlag=value", "gRPC", "", true},
		{"", "flag", "", true},
		{"--=value", "flag", "", true},
		{"-=", "flag", "", true},
		{"-flag=", "flag", "", true},
		{"--flag=", "flag", "", true},
		{"=value", "flag", "", true},
	}

	parser := NewParser()
	for _, test := range tests {
		got, err := parser.ParseString(test.input, test.flagName)
		t.Logf("Input: %s, Flag: %s, Result: %s, Error: %v", test.input, test.flagName, got, err)
		if (err != nil && !test.err) || (err == nil && test.err) {
			t.Errorf("Expected error: %v, got: %v for input: %s", test.err, err, test.input)
		}
		if got != test.expected {
			t.Errorf("Expected: %s, got: %s for input: %s", test.expected, got, test.input)
		}
	}
}

func TestParseStringSlice(t *testing.T) {
	tests := []struct {
		input    string
		flagName string
		expected []string
		err      bool
	}{
		{"-flag=value1,value2,value3", "flag", []string{"value1", "value2", "value3"}, false},
		{"--flag=value1,value2,value3", "flag", []string{"value1", "value2", "value3"}, false},
		{"-flag=\"value1,value2,value3\"", "flag", []string{"value1", "value2", "value3"}, false},
		{"--flag=\"value1,value2,value3\"", "flag", []string{"value1", "value2", "value3"}, false},
		{"-my_flag-name=value1,value2,value3", "my_flag-name", []string{"value1", "value2", "value3"}, false},
		{"-flag123=value-1,value-2,value3", "flag123", []string{"value-1", "value-2", "value3"}, false},
		{"-flag1=value1,value2 -flag2=valueA,valueB", "flag1", []string{"value1", "value2"}, false},
		{"-f value1,value2,value3", "f", []string{"value1", "value2", "value3"}, false},
		{"-f value_1,value_2,value_3", "f", []string{"value_1", "value_2", "value_3"}, false},
		{"-f value-1,value-2,value-3", "f", []string{"value-1", "value-2", "value-3"}, false},
		{"--flag=value-1,value-2,value-3", "flag", []string{"value-1", "value-2", "value-3"}, false},
		{"-f k1=v1,k2=v2,k3=v3", "f", []string{"k1=v1", "k2=v2", "k3=v3"}, false},
		{"--flag=\"k1=v1,k2=v2,k3=v3\"", "flag", []string{"k1=v1", "k2=v2", "k3=v3"}, false},
		{"-f valueX", "f", []string{"valueX"}, false},
		{"-f VALUEX", "f", []string{"VALUEX"}, false},
		{"-f value:8989", "f", []string{"value:8989"}, false},
		{"--flag=value-1:8080,value-2:8080,value-3:8080", "flag", []string{"value-1:8080", "value-2:8080", "value-3:8080"}, false},
		{"--flag=value_1:8080,value_2:8080,value_3:8080", "flag", []string{"value_1:8080", "value_2:8080", "value_3:8080"}, false},
		{"-flag=value1,,value3", "flag", nil, true},
		{"-otherFlag=value1,value2,value3", "flag", nil, true},
		{"", "flag", nil, true},
		{"-flag=", "flag", nil, true},
		{"flag=value1,value2,value3", "flag", nil, true},
		{"-flag=value1,value2,", "flag", nil, true},
		{"-flag=,", "flag", nil, true},
		{"-flag=,value2,value3", "flag", nil, true},
	}

	parser := NewParser()
	for _, test := range tests {
		got, err := parser.ParseStringSlice(test.input, test.flagName)
		t.Logf("Input: %s, Flag: %s, Result: %s, Error: %v", test.input, test.flagName, got, err)
		if (err != nil && !test.err) || (err == nil && test.err) {
			t.Errorf("Expected error: %v, got: %v for input: %s", test.err, err, test.input)
		}
		if !reflect.DeepEqual(got, test.expected) {
			t.Errorf("Expected: %v, got: %v for input: %s", test.expected, got, test.input)
		}
	}
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		input    string
		flagName string
		expected []int
		err      bool
	}{
		{"-flag=42", "flag", []int{42}, false},
		{"--flag=42", "flag", []int{42}, false},
		{"--flag=1,2,3,4", "flag", []int{1, 2, 3, 4}, false},
		{"-otherFlag=42", "flag", nil, true},
		{"", "flag", nil, true},
		{"-flag=abcd", "flag", nil, true},
		{"-flag= 42", "flag", nil, true},
		{"-flag=", "flag", nil, true},
		{"-flag=42,", "flag", nil, true},
		{"-flag=,42", "flag", nil, true},
		{"-flag=,", "flag", nil, true},
	}

	parser := NewParser()
	for _, test := range tests {
		got, err := parser.ParseInt(test.input, test.flagName)
		t.Logf("Input: %s, Flag: %s, Result: %v, Error: %v", test.input, test.flagName, got, err)
		if (err != nil && !test.err) || (err == nil && test.err) {
			t.Errorf("Expected error: %v, got: %v for input: %s", test.err, err, test.input)
		}
		if !reflect.DeepEqual(got, test.expected) {
			t.Errorf("Expected: %v, got: %v for input: %s", test.expected, got, test.input)
		}
	}
}

func TestParseFloat(t *testing.T) {
	tests := []struct {
		input    string
		flagName string
		expected []float64
		err      bool
	}{
		{"-flag=42.2", "flag", []float64{42.2}, false},
		{"--flag=42.2", "flag", []float64{42.2}, false},
		{"--flag=1.1,2.2,3.3,4.4", "flag", []float64{1.1, 2.2, 3.3, 4.4}, false},
		{"-flag=", "flag", nil, true},
		{"-flag=42.2,", "flag", nil, true},
		{"-flag=,42.2", "flag", nil, true},
		{"-flag=,", "flag", nil, true},
		{"-otherFlag=42.2", "flag", nil, true},
		{"", "flag", nil, true},
		{"-flag=abcd", "flag", nil, true},
		{"-flag= 42.2", "flag", nil, true},
	}

	parser := NewParser()
	for _, test := range tests {
		got, err := parser.ParseFloat(test.input, test.flagName)
		t.Logf("Input: %s, Flag: %s, Result: %v, Error: %v", test.input, test.flagName, got, err)
		if (err != nil && !test.err) || (err == nil && test.err) {
			t.Errorf("Expected error: %v, got: %v for input: %s", test.err, err, test.input)
		}
		if !reflect.DeepEqual(got, test.expected) {
			t.Errorf("Expected: %v, got: %v for input: %s", test.expected, got, test.input)
		}
	}
}

func TestParseRegex(t *testing.T) {
	tests := []struct {
		input     string
		flagName  string
		expected  string
		shouldErr bool
	}{
		{"--regex=\"r:^a[a-z]*$\"", "regex", "^a[a-z]*$", false},
		{"-regex=\"r:[0-9]{2,4}\"", "regex", "[0-9]{2,4}", false},
		{"--pattern=\"r:.*abc.*\"", "pattern", ".*abc.*", false},
		{"-pattern=\"r:.*\"", "pattern", ".*", false},
		{"--pattern=\"r:.*[\"", "pattern", "", true},
		{"--pattern=\"r:", "pattern", "", true},
		{"--notSame=\"r:.*abc.*\"", "pattern", "", true},
		{"--pattern=\"r:", "pattern", "", true},
		{"", "pattern", "", true},
		{"-pattern=r:.*abc.*", "pattern", ".*abc.*", false},
		{"--pattern=\"r:^\\d{3}-\\w{2,4}$\"", "pattern", "^\\d{3}-\\w{2,4}$", false},
		{"--pattern=\"r:^.*(?:[aeiou]).*$\"", "pattern", "^.*(?:[aeiou]).*$", false},
		{"--regex=\"r:^a{2,}?\"", "regex", "^a{2,}?", false},
		{"--regex=\"r:^a{1,3}?b\"", "regex", "^a{1,3}?b", false},
		{"-pattern=\"r:(?:^|,)\\s*(.+?)(?:,|$)\"", "pattern", "(?:^|,)\\s*(.+?)(?:,|$)", false},
		{"-pattern=\"r:^[^-\\s]\"", "pattern", "^[^-\\s]", false},
		{"--pattern=\"r:(?i)case\"", "pattern", "(?i)case", false},
		{"--pattern=\"r:(?i)\"", "pattern", "", true},
	}

	parser := NewParser()
	for _, test := range tests {
		got, err := parser.ParseRegex(test.input, test.flagName)
		t.Logf("Input: %s, Flag: %s, Result: %v, Error: %v", test.input, test.flagName, got, err)
		if (err != nil) != test.shouldErr {
			t.Fatalf("Unexpected error state for input %s: %v", test.input, err)
		}
		if err == nil {
			if got.String() != test.expected {
				t.Errorf("Expected: %v, got: %v for input: %s", test.expected, got.String(), test.input)
			}
		}
	}
}

func TestParseStringError(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		input   string
		flag    string
		wantErr bool
		want    interface{}
	}{
		{"", "flag", true, nil},
		{"--flag=value", "", true, nil},
		{"", "", true, nil},
	}

	for _, tt := range tests {
		_, err := parser.ParseString(tt.input, tt.flag)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseString() error = %v, wantErr %v", err, tt.wantErr)
		}
	}
}

func TestParseStringSliceError(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		input   string
		flags   string
		wantErr bool
		want    interface{}
	}{
		{"", "flag", true, nil},
		{"--flag=value1,value2", "", true, nil},
		{"", "", true, nil},
		{"--flag=", "flag", true, nil},
		{"--flag=value1,,value2", "flag", true, nil},
	}

	for _, tt := range tests {
		_, err := parser.ParseStringSlice(tt.input, tt.flags)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseStringSlice() error = %v, wantErr %v", err, tt.wantErr)
		}
	}
}

// TODO: Add few more cases
func TestParseRegexSlice(t *testing.T) {
	// Define the test cases
	tests := []struct {
		input       string
		value       string
		flagName    string
		expectedStr []string
		expectedRgx []*regexp.Regexp
		err         bool
	}{
		{
			input:       "-g localhost:8080 -n r:\"log_*\"",
			value:       "r:\"log_*\"",
			flagName:    "n",
			expectedStr: nil,
			expectedRgx: []*regexp.Regexp{regexp.MustCompile(`log_*`)},
			err:         false,
		},
	}

	p := NewParser()
	for _, test := range tests {
		strResults, rgxResults, err := p.ParseRegexSlice(test.value, test.input, test.flagName)

		if (err != nil) != test.err {
			t.Errorf("ParseRegexSlice() error = %v, wantErr %v", err, test.err)
			continue
		}

		if !reflect.DeepEqual(strResults, test.expectedStr) {
			t.Errorf("ParseRegexSlice() gotStr = %v, want %v", strResults, test.expectedStr)
		}

		if len(rgxResults) != len(test.expectedRgx) {
			t.Errorf("ParseRegexSlice() gotRgx length = %v, want %v", len(rgxResults), len(test.expectedRgx))
		} else {
			for i := range rgxResults {
				if rgxResults[i].String() != test.expectedRgx[i].String() {
					t.Errorf("ParseRegexSlice() gotRgx = %v, want %v", rgxResults[i], test.expectedRgx[i])
				}
			}
		}
	}
}

type SampleConfig struct {
	StringValue string   `flag:"s"`
	IntValue    int      `flag:"i"`
	BoolValue   bool     `flag:"b"`
	StringSlice []string `flag:"sl"`
}

var tests = []struct {
	input    string
	target   interface{}
	expected map[string]string
	err      bool
}{
	{
		input:    "--s=hello --i=123 --b=true",
		target:   &SampleConfig{},
		expected: map[string]string{"s": "hello", "i": "123", "b": "true"},
		err:      false,
	},
	{
		input:    "-s 'world' -i 456 -b false",
		target:   &SampleConfig{},
		expected: map[string]string{"s": "world", "i": "456", "b": "false"},
		err:      false,
	},
	{
		input:    "-b -s newstr --i=123,456,789",
		target:   &SampleConfig{},
		expected: map[string]string{"s": "newstr", "i": "123,456,789", "b": "true"},
	},
	{
		input:    "-sl newstr,oldstr",
		target:   &SampleConfig{},
		expected: map[string]string{"sl": "newstr,oldstr"},
	},
}

func TestFlagsToMap(t *testing.T) {
	// Initialize the parser here using your actual parser initialization code
	parser := NewParser()

	for _, test := range tests {
		got, err := parser.FlagsToMap(test.input, test.target)
		t.Logf("RESULT: %v", got)
		if (err != nil) != test.err {
			t.Errorf("FlagsToMap(%q, %v) expected error: %v, got: %v", test.input, test.target, test.err, err)
			continue
		}
		if err == nil && !reflect.DeepEqual(got, test.expected) {
			t.Errorf("FlagsToMap(%q, %v) = %v, want %v", test.input, test.target, got, test.expected)
		}
	}
}
