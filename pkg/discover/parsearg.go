package discover

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/accuknox/accuknox-cli-v2/pkg/common"
)

// `flag` tag is used for internal parsing via reflection please don't remove it and
// keep the cases either lowercase (no kebab-case or camel_case)
type Options struct {
	GRPC           string   `flag:"grpc"`
	Format         string   `flag:"format"`
	Kind           []string `flag:"policy"`
	Namespace      []string `flag:"namespace"`
	Labels         []string `flag:"labels"`
	Source         []string `flag:"source"`
	IncludeNetwork bool     `flag:"includenet"`

	NamespaceRegex []*regexp.Regexp
	LabelsRegex    []*regexp.Regexp
	SourceRegex    []*regexp.Regexp
}

func (p *Options) noFilters() bool {
	return len(p.Namespace) == 0 && len(p.NamespaceRegex) == 0 && len(p.Labels) == 0 && len(p.LabelsRegex) == 0 && len(p.Source) == 0 && len(p.SourceRegex) == 0 && !p.IncludeNetwork
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

		case flag == "format" || flag == "f":
			if value != "json" && value != "yaml" {
				return nil, wrapErr(fmt.Errorf("invalid format"))
			}
			parsed.Format, err = parser.ParseString(rawArgs, flag)

		case flag == "policy" || flag == "p":
			parsed.Kind, err = parser.ParseStringSlice(rawArgs, flag)

		case flag == "namespace" || flag == "n":
			parsed.Namespace, regexList, err = parser.ParseRegexSlice(value, rawArgs, flag)
			parsed.NamespaceRegex = regexList

		case flag == "labels" || flag == "l":
			parsed.Labels, regexList, err = parser.ParseRegexSlice(value, rawArgs, flag)
			parsed.LabelsRegex = regexList

		case flag == "source" || flag == "s":
			parsed.Source, regexList, err = parser.ParseRegexSlice(value, rawArgs, flag)
			parsed.SourceRegex = regexList

		case flag == "includenet":
			parsed.IncludeNetwork = true

		default:
			// This condition will never be hit since cobra will sort this out, just for unit tests
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
