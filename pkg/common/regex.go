package common

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// Custom parsing library for `accuknoxcli` to parse, validate and process regex based patterns.
//
// Principle regex patterns to process different type of types that we are using
// -{1,2}          			: Matches one or two dashes at the start
// (?P<flag>)      			: Named capture group for flag name, alphanumeric and hyphen allowed
// (?:=|\s+)       			: Non-capturing group for either '=' or one or more whitespaces
// (?P<value>)     			: Named capture group for the value
// ["']?(true|false)["']? 	: Optional quotes around true or false values
// [\w-=:]+(,[\w-=:]+)* 	: Comma-separated values, each consisting of word characters, hyphens, equals, and colons
// [\d,]+  					: One or more digits, separated by commas
// .+? 						: Any character (non-greedy match)
//
// Additional Information:
// 1. Named Capture Groups (?P<name>) are used for easier access to the matched values.
// 2. Non-Capturing Groups (?:...) are used when we don't need to retrieve the match.
// 3. Character Classes [...] are used to specify a character set.
// 4. Quantifiers (*, +, ?, {n}, {n,}, {n,m}) specify the number of occurrences.
//
// References (suggested readings):
// 1. Basics of regular expressions: 	https://cs.lmu.edu/~ray/notes/regex/
// 2. Go Regex Package Documentation: 	https://pkg.go.dev/regexp
// 3. Google RE2 Syntax: 				https://github.com/google/re2/wiki/Syntax
// 4. CRAN RE2 Syntax Guide: 			https://cran.r-project.org/web/packages/re2/vignettes/re2_syntax.html
// 5. Theoretical basis of regex: 		https://home.cs.colorado.edu/~astr3586/courses/csci3434/lec04.pdf [Advanced reading]
//
// Limitations:
// These regex patterns are based on Go's regex engine, which is re2 and is not fully POSIX compliant.
// It does not support backtracking, so certain complex patterns like backreferences are not available.
// To gaurantee O(n) time regex processing Go's regex engine will now allow negative lookaheads.

// Parser is a struct that contains regex patterns to process different types of arguments.
// It also contains a map to store parsed flags and their values.
type Parser struct {
	stringPattern          *regexp.Regexp
	boolFlagPattern        *regexp.Regexp
	stringSliceFlagPattern *regexp.Regexp
	intFlagPattern         *regexp.Regexp
	floatFlagPattern       *regexp.Regexp
	regexFlagPattern       *regexp.Regexp
	parsedFlags            map[string]string
}

// NewParser returns a new instance of Parser. It takes args and the complete structure of args (aka options).
// If the args parameter is not empty, it will parse the string flags and store them in the parsedFlags map.
func NewParser() *Parser {
	return &Parser{
		stringPattern:          regexp.MustCompile(`-{1,2}(?P<flag>[a-zA-Z0-9-_:]+)(?:=|\s+)(?P<value>r:"[^"]+"|r:'[^']+'|".+?"|'.+?'|[^"'\s]+)`),
		boolFlagPattern:        regexp.MustCompile(`-{1,2}([a-zA-Z-]+)(?:[ \t]+(?P<value>true|false))?`),
		stringSliceFlagPattern: regexp.MustCompile(`-{1,2}(?P<flag>[a-zA-Z0-9_-]+)(?:=|\s+)["']?(?P<value>([\w-=:]+(,[\w-=:]+)*))["']?(?: |$)`),
		intFlagPattern:         regexp.MustCompile(`-{1,2}(?P<flag>[a-zA-Z-]+)(?:=|\s+)["']?(?P<value>[\d,]+)["']?$`),
		floatFlagPattern:       regexp.MustCompile(`-{1,2}(?P<flag>[a-zA-Z-]+)(?:=|\s+)["']?(?P<value>[\d.,]+)["']?$`),
		regexFlagPattern:       regexp.MustCompile(`-{1,2}(?P<flag>[a-zA-Z0-9-_:]+)(?:=|\s+)["']?(?P<value>.+?)["']?$`),
		parsedFlags:            make(map[string]string),
	}
}

// parse extracts the value of a specific flag from an input string using a regular expression pattern.
// It returns the matched value and an error, if any.
func (p *Parser) parse(input, flagName string, pattern *regexp.Regexp) (string, error) {
	allMatches := pattern.FindAllStringSubmatch(input, -1)
	if allMatches == nil {
		return "", fmt.Errorf("no match found for input: %s", input)
	}

	for _, matches := range allMatches {
		result := make(map[string]string)
		for i, name := range pattern.SubexpNames() {
			if i > 0 && i <= len(matches) {
				result[name] = matches[i]
			}
		}

		if flag, ok := result["flag"]; ok && (flag == flagName || strings.Trim(flag, "-") == flagName) {
			return result["value"], nil
		}
	}

	return "", fmt.Errorf("flag %s not found", flagName)
}

// ParseString extracts and returns the value of a string flag from the input string.
// It returns an error if the flag is not found or if there is a problem with the format.
func (p *Parser) ParseString(input, flagName string) (string, error) {
	value, err := p.parse(input, flagName, p.stringPattern)
	if err != nil {
		return "", err
	}

	return strings.Trim(value, "\"'"), nil
}

// ParseStringSlice extracts and returns the values of a string slice flag from the input string.
// The values are separated by commas.
// Returns an error if the flag is not found, if no valid elements are found, or if an empty element is found.
func (p *Parser) ParseStringSlice(input, flagName string) ([]string, error) {
	value, err := p.parse(input, flagName, p.stringSliceFlagPattern)
	if err != nil {
		return nil, err
	}

	words := strings.Split(value, ",")
	if len(words) == 0 {
		return nil, fmt.Errorf("no valid elements found for flag %s", flagName)
	}

	for _, word := range words {
		if word == "" {
			return nil, fmt.Errorf("invalid empty element found for flag %s", flagName)
		}
	}

	return words, nil
}

// ParseInt extracts and returns the integer values of a flag from the input string.
// The integer values are separated by commas.
// Returns an error if the flag is not found or if any of the values are not integers.
func (p *Parser) ParseInt(input, flagName string) ([]int, error) {
	value, err := p.parse(input, flagName, p.intFlagPattern)
	if err != nil {
		return nil, err
	}

	values := strings.Split(value, ",")
	intValues := make([]int, 0, len(values))
	for _, value := range values {
		intValue, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf("invalid integer value for flag %s: %v", flagName, err)
		}
		intValues = append(intValues, intValue)
	}

	return intValues, nil
}

// ParseFloat extracts and returns the floating-point values of a flag from the input string.
// The float values are separated by commas.
// Returns an error if the flag is not found or if any of the values are not floats.
func (p *Parser) ParseFloat(input, flagName string) ([]float64, error) {
	value, err := p.parse(input, flagName, p.floatFlagPattern)
	if err != nil {
		return nil, err
	}

	values := strings.Split(value, ",")
	floatValues := make([]float64, 0, len(values))
	for _, value := range values {
		floatValue, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid float value for flag %s: %v", flagName, err)
		}
		floatValues = append(floatValues, floatValue)
	}

	return floatValues, nil
}

// It supports the special prefix "r:" to differentiate it from a normal string.
// Returns an error if the flag is not found, if the regex is invalid, or if the pattern is empty.
func (p *Parser) ParseRegex(input, flagName string) (*regexp.Regexp, error) {
	value, err := p.parse(input, flagName, p.regexFlagPattern)
	if err != nil {
		return nil, err
	}

	unquotedValue := strings.Trim(value, "\"'")
	if !strings.HasPrefix(unquotedValue, "r:") {
		return nil, nil
	}

	regexPattern := strings.TrimPrefix(unquotedValue, "r:")
	if regexPattern == "" {
		return nil, fmt.Errorf("empty regex pattern for flag %s is not valid", flagName)
	}

	if regexPattern == "(?i)" || regexPattern == "(?m)" || regexPattern == "(?s)" || regexPattern == "(?U)" {
		return nil, fmt.Errorf("regex flag %s provided but no actual pattern", regexPattern)
	}

	compiledRegex, err := regexp.Compile(regexPattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern for flag %s: %v", flagName, err)
	}

	return compiledRegex, nil
}

// TODO: Modular

// ParseRegexSlice handles flags that have a "r:" prefix that is an argument that can be of type
// regex or a string slice, it can handle both the cases of a string slice or a regex slice
// in consolidated way.
func (p *Parser) ParseRegexSlice(value, rawArgs, flag string) ([]string, []*regexp.Regexp, error) {
	var parsedSlice []string
	var parsedRegexList []*regexp.Regexp
	var err error

	if strings.HasPrefix(value, "r:") {
		argsList := strings.Split(rawArgs, " ")
		for i, arg := range argsList {
			if strings.HasPrefix(arg, "--"+flag+"=r:") || strings.HasPrefix(arg, "-"+flag+"=r:") {
				regexPattern := strings.TrimPrefix(arg, "--"+flag+"=r:")
				regexPattern = strings.Trim(regexPattern, `"`)
				parsedRegex, err := regexp.Compile(regexPattern)
				if err != nil {
					return nil, nil, err
				}
				parsedRegexList = append(parsedRegexList, parsedRegex)
			} else if arg == "--"+flag || arg == "-"+flag {
				if i+1 < len(argsList) && strings.HasPrefix(argsList[i+1], "r:") {
					regexPattern := strings.TrimPrefix(argsList[i+1], "r:")
					regexPattern = strings.Trim(regexPattern, `"`)
					parsedRegex, err := regexp.Compile(regexPattern)
					if err != nil {
						return nil, nil, err
					}
					parsedRegexList = append(parsedRegexList, parsedRegex)
				}
			}
		}
	} else {
		parsedSlice, err = p.ParseStringSlice(rawArgs, flag)
	}

	return parsedSlice, parsedRegexList, err
}

// TODO: Optimize the function. Current time complexity is roughly O(mk + kn),
// where m is the length of the input string, k is the number of flags, and
// n is the number of fields in targetStruct if provided.
// In future we can try using reflection to populate the target in runtime completely.

// FlagsToMap returns a map of flag names to their parsed string values.
// It also handles both longhand (--flag=value) and shorthand (-f value) notations.
// It fills the provided targetStruct with the parsed values if not nil.
func (p *Parser) FlagsToMap(input string, targetStruct interface{}) (map[string]string, error) {
	if strings.HasSuffix(strings.TrimSpace(input), "--") {
		return nil, errors.New("trailing '--' is not allowed")
	}

	result := make(map[string]string)
	seenFlags := make(map[string]bool) // track flags that have already been processed

	// Boolean flags
	boolMatches := p.boolFlagPattern.FindAllStringSubmatch(input, -1)
	for _, matches := range boolMatches {
		flagName := strings.Trim(matches[1], "-")

		isBooleanFlag := true
		var flagValue string = "true"

		if len(matches) > 2 && matches[2] != "" {
			flagValue = strings.ToLower(matches[2])
			isBooleanFlag = flagValue == "true" || flagValue == "false"
		}

		if targetStruct != nil && isBooleanFlag {
			v := reflect.ValueOf(targetStruct).Elem()
			fieldName := flagToFieldName(v, flagName)
			field := v.FieldByName(fieldName)
			if field.IsValid() && field.Kind() == reflect.Bool {
				field.SetBool(flagValue == "true")
			} else {
				continue
			}
		}

		result[flagName] = flagValue

		index := strings.Index(input, matches[0])
		if index != -1 {
			input = input[:index] + input[index+len(matches[0]):]
		}
	}

	if strings.TrimSpace(input) == "" {
		return result, nil
	}

	allMatches := p.stringPattern.FindAllStringSubmatch(input, -1)
	if allMatches == nil {
		return nil, fmt.Errorf("no match found for input: %s", input)
	}

	for _, matches := range allMatches {
		flagName := ""
		flagValue := ""
		for i, name := range p.stringPattern.SubexpNames() {
			if i > 0 && i <= len(matches) {
				if name == "flag" {
					flagName = strings.Trim(matches[i], "-")
				} else if name == "value" {
					if strings.HasPrefix(matches[i], "r:") {
						flagValue = matches[i]
					} else {
						flagValue = strings.Trim(matches[i], "\"'")
					}
				}
			}
		}

		if seenFlags[flagName] {
			return nil, fmt.Errorf("flag appears multiple times: %s", flagName)
		}
		seenFlags[flagName] = true

		// Check for conflicting flags (shorthand and longhand notation should not be used together)
		// shorthand is just the first letter of longhand
		var conflictingFlag string
		if len(flagName) == 1 {
			for longhand := range result {
				if strings.HasPrefix(longhand, flagName) {
					conflictingFlag = longhand
					break
				}
			}
		} else {
			conflictingFlag = string(flagName[0])
		}

		if existingValue, exists := result[conflictingFlag]; exists && existingValue != flagValue {
			return nil, fmt.Errorf("conflicting values for flag: %s", flagName)
		}

		result[flagName] = flagValue
	}

	return result, nil
}

func flagToFieldName(v reflect.Value, flag string) string {
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("flag")
		if tag == flag {
			return field.Name
		}
	}
	return ""
}
