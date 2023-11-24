package recommend

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/accuknox/accuknox-cli-v2/pkg/common"
)

type Options struct {
	Namespace []string `flag:"namespace"`
	Labels    []string `flag:"labels"`
	Tags      []string `flag:"tags"`
	Policy    []string `flag:"policy"`
	Outdir    string   `flag:"out"`
	Grpc      string   `flag:"gRPC"`
	Dump      bool     `flag:"dump"`

	NamespaceRegex []*regexp.Regexp
	LabelsRegex    []*regexp.Regexp
	TagsRegex      []*regexp.Regexp
	SeveritySlice  []int
}

func (o *Options) noFilter() bool {
	return len(o.Namespace) == 0 && len(o.Labels) == 0 && len(o.Tags) == 0 && len(o.NamespaceRegex) == 0 && len(o.LabelsRegex) == 0 && len(o.TagsRegex) == 0
}

func ProcessArgs(rawArgs string) (*Options, error) {
	parsedOption := &Options{}
	parser := common.NewParser()

	flags, err := parser.FlagsToMap(rawArgs, parsedOption)
	if err != nil {
		return nil, wrapErr(err)
	}

	for flag, values := range flags {
		if strings.HasPrefix(values, "r:") && !isRegexAllowed(flag) {
			allowedFlags := getRegexAllowedFlags()
			return nil, fmt.Errorf("regex is not allowed for the flag: %s, currently allowed flags are: %s", flag, strings.Join(allowedFlags, ", "))
		}

		var regexList []*regexp.Regexp

		switch {
		case flag == "out" || flag == "o":
			parsedOption.Outdir, err = parser.ParseString(rawArgs, flag)

		case flag == "policy" || flag == "p":
			parsedOption.Policy, err = parser.ParseStringSlice(rawArgs, flag)

		case flag == "gRPC" || flag == "g":
			parsedOption.Grpc, err = parser.ParseString(rawArgs, flag)

		case flag == "severity" || flag == "s":
			parsedOption.SeveritySlice, err = parser.ParseInt(rawArgs, flag)

		case flag == "namespace" || flag == "n":
			parsedOption.Namespace, regexList, err = parser.ParseRegexSlice(values, rawArgs, flag)
			parsedOption.NamespaceRegex = regexList

		case flag == "tags" || flag == "t":
			parsedOption.Tags, regexList, err = parser.ParseRegexSlice(values, rawArgs, flag)
			parsedOption.TagsRegex = regexList

		case flag == "labels" || flag == "l":
			parsedOption.Labels, regexList, err = parser.ParseRegexSlice(values, rawArgs, flag)
			parsedOption.LabelsRegex = regexList

		case flag == "dump":
			parsedOption.Dump = true

		default:
			return nil, wrapErr(fmt.Errorf("unknown flag: %v", flag))
		}

		if err != nil {
			return nil, wrapErr(err)
		}
	}

	return parsedOption, nil
}

func wrapErr(err error) error {
	if err != nil {
		return fmt.Errorf("error parsing flag: %v", err)
	}
	return nil
}

func isRegexAllowed(flag string) bool {
	allowed := getRegexAllowedFlags()
	for _, allowedFlags := range allowed {
		if allowedFlags == flag {
			return true
		}
	}

	return false
}

func getRegexAllowedFlags() []string {
	return []string{"namespace", "labels", "tags", "n", "l", "t"}
}
