package summary

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/accuknox/accuknox-cli-v2/pkg/common"
)

// Options Structure
type Options struct {
	GRPC         string
	Labels       []string
	Namespace    []string
	Source       []string
	Destination  []string
	Operation    string
	Format       string
	RevDNSLookup bool // I dont really know how we integrate this

	LabelsRegex      []*regexp.Regexp
	NamespaceRegex   []*regexp.Regexp
	SourceRegex      []*regexp.Regexp
	DestinationRegex []*regexp.Regexp
}

func (o *Options) noFilters() bool {
	lr := len(o.LabelsRegex)
	nr := len(o.NamespaceRegex)
	sr := len(o.SourceRegex)
	dr := len(o.DestinationRegex)

	l := len(o.Labels)
	n := len(o.Namespace)
	s := len(o.Source)
	d := len(o.Destination)

	return lr == 0 && nr == 0 && sr == 0 && dr == 0 && l == 0 && n == 0 && s == 0 && d == 0
}
func ProcessArgs(rawArgs string) (*Options, error) {
	parsed := &Options{}
	parser := common.NewParser()

	flags, err := parser.FlagsToMap(rawArgs, parsed)
	if err != nil {
		return nil, err
	}

	for flag, value := range flags {
		if strings.HasPrefix(value, "r:") && !isRegexAllowed(flag) {
			allowedFlags := getRegexAllowedFlags()
			return nil, fmt.Errorf("regex is not allowed for the flag: %s, currently supported regex flags are: %s", flag, strings.Join(allowedFlags, ", "))
		}

		var regexList []*regexp.Regexp
		switch {
		case flag == "gRPC" || flag == "g":
			parsed.GRPC, err = parser.ParseString(rawArgs, flag)

		case flag == "operation" || flag == "o":
			parsed.Operation, err = parser.ParseString(rawArgs, flag)

		case flag == "format" || flag == "f":
			parsed.Format, err = parser.ParseString(rawArgs, flag)

		case flag == "labels" || flag == "l":
			parsed.Labels, regexList, err = parser.ParseRegexSlice(value, rawArgs, value)
			parsed.LabelsRegex = regexList

		case flag == "namespace" || flag == "n":
			parsed.Namespace, regexList, err = parser.ParseRegexSlice(value, rawArgs, flag)
			parsed.NamespaceRegex = regexList

		case flag == "source" || flag == "s":
			parsed.Source, regexList, err = parser.ParseRegexSlice(value, rawArgs, flag)
			parsed.SourceRegex = regexList

		case flag == "destination" || flag == "d":
			parsed.Destination, regexList, err = parser.ParseRegexSlice(value, rawArgs, flag)
			parsed.DestinationRegex = regexList

		case flag == "revdnslookup":
			parsed.RevDNSLookup = true

		default:
			return nil, wrapErr(fmt.Errorf("unknown flag: %s", flag))
		}

		if err != nil {
			return nil, wrapErr(err)
		}
	}

	return parsed, nil
}

func wrapErr(err error) error {
	if err != nil {
		return fmt.Errorf("error parsing flags: %v", err)
	}
	return nil
}

func isRegexAllowed(flag string) bool {
	allowedFlags := getRegexAllowedFlags()
	for _, allowedFlag := range allowedFlags {
		if allowedFlag == flag {
			return true
		}
	}

	return false
}

// Add shorthand and longhand notation for flags supporting regex
func getRegexAllowedFlags() []string {
	return []string{"namespace", "labels", "source", "n", "l", "s"}
}
