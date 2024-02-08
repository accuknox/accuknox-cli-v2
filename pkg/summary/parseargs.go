package summary

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/accuknox/accuknox-cli-v2/pkg/common"
)

// Options Structure: consider pod name
type Options struct {
	GRPC                string   `flag:"gRPC"`
	Operation           string   `flag:"operation"`
	BaselineSummaryPath string   `flag:"baseline"`
	View                string   `flag:"view"`
	OutputTo            string   `flag:"out"`
	Workloads           []string `flag:"workloads"`
	Namespace           []string `flag:"namespaces"`
	IgnorePath          []string `flag:"ignore-paths"`
	Source              []string `flag:"source"`
	Destination         []string `flag:"destination"`
	Command             []string `flag:"command"`
	Labels              []string `flag:"labels"`
	IgnoreCommand       []string `flag:"ignore-command"`
	Dump                bool     `flag:"dump"`
	Glance              bool     `flag:"glance"`
	Debug               bool     `flag:"debug"`
	NoTUI               bool     `flag:"no-tui"`

	NamespaceRegex    []*regexp.Regexp
	ResourceTypeRegex []*regexp.Regexp
	ResourceNameRegex []*regexp.Regexp
	IgnorePathsRegex  []*regexp.Regexp
	SourceRegex       []*regexp.Regexp
	DestinationRegex  []*regexp.Regexp
	LabelsRegex       []*regexp.Regexp
	CommandRegex      []*regexp.Regexp
	WorkloadsRegex    []*regexp.Regexp
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
	parsedOption := &Options{}
	parser := common.NewParser()

	flags, err := parser.FlagsToMap(rawArgs, parsedOption)
	if err != nil {
		return nil, wrapErr(err)
	}

	for flag, values := range flags {
		if flag != "gRPC" && !isRegexAllowed(flag) && strings.ContainsAny(values, common.SpecialRegexChars) {
			allowedFlags := getRegexAllowedFlags()
			return nil, fmt.Errorf("found special regex characters: `%s`, regex is not allowed for the flag: %s, currently allowed flags are: %s", common.SpecialRegexChars, flag, strings.Join(allowedFlags, ", "))
		}

		var regexList []*regexp.Regexp

		switch {
		case flag == "gRPC":
			parsedOption.GRPC, err = parser.ParseString(rawArgs, flag)

		case flag == "operation":
			parsedOption.Operation, err = parser.ParseString(rawArgs, flag)

		case flag == "baseline":
			parsedOption.BaselineSummaryPath, err = parser.ParseString(rawArgs, flag)

		case flag == "view" || flag == "v":
			parsedOption.View, err = parser.ParseString(rawArgs, flag)

		case flag == "namespaces":
			parsedOption.Namespace, regexList, err = parser.ParseRegexSlice(values, flag)
			parsedOption.NamespaceRegex = regexList

		case flag == "ignore-path":
			parsedOption.IgnorePath, regexList, err = parser.ParseRegexSlice(values, flag)
			parsedOption.IgnorePathsRegex = regexList

		case flag == "source":
			parsedOption.Source, regexList, err = parser.ParseRegexSlice(values, flag)
			parsedOption.SourceRegex = regexList

		case flag == "destination":
			parsedOption.Destination, regexList, err = parser.ParseRegexSlice(values, flag)
			parsedOption.DestinationRegex = regexList

		case flag == "ignore-command":
			parsedOption.IgnoreCommand, regexList, err = parser.ParseRegexSlice(values, flag)
			parsedOption.CommandRegex = regexList

		case flag == "labels":
			parsedOption.Labels, regexList, err = parser.ParseRegexSlice(values, flag)
			parsedOption.LabelsRegex = regexList

		case flag == "workload":
			parsedOption.Workloads, regexList, err = parser.ParseRegexSlice(values, flag)
			parsedOption.WorkloadsRegex = regexList

		case flag == "out":
			parsedOption.OutputTo, err = parser.ParseString(rawArgs, flag)

		case flag == "dump":
			parsedOption.Dump = true

		case flag == "no-tui":
			parsedOption.NoTUI = true

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
	return []string{"workload", "ignore-command", "baseline", "ignore-path", "namespace", "labels", "source", "n", "l", "s"}
}
