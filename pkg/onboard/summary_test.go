package onboard

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/accuknox/accuknox-cli-v2/pkg/common"
)

func TestSummarySchemaValidation(t *testing.T) {
	for _, tc := range []struct {
		schema, version   string
		discover, invalid bool
	}{
		{"", "v0.11.0", false, false}, {"v2", "v0.10.0", true, false},
		{"v3", "v0.11.0", false, false}, {"v3", "v0.11.0", true, true},
		{"v3", "v0.10.0", false, true}, {"v4", "v0.11.0", false, true},
	} {
		cc := ClusterConfig{SummarySchemaVersion: tc.schema, AgentsVersion: tc.version, DeployDiscover: tc.discover}
		if err := cc.ValidateSummarySchema(); (err != nil) != tc.invalid {
			t.Errorf("%+v: %v", tc, err)
		}
	}
}

func TestSummaryTemplates(t *testing.T) {
	for _, schema := range []string{"v2", "v3"} {
		t.Run(schema, func(t *testing.T) {
			cc := ClusterConfig{SummarySchemaVersion: schema, AgentsVersion: "v0.11.0", RMQTopicPrefix: "test"}
			ic := InitCPNodeConfig(cc, "", "spire.example:443", "", "", "", "", false)
			if err := ic.CreateBaseTemplateConfig(); err != nil {
				t.Fatal(err)
			}
			args := ic.TCArgs
			expectedTopic := "test-summary-v2"
			if schema == "v3" {
				expectedTopic = "test-summary-v3"
			}
			if args.SummaryTopic != expectedTopic {
				t.Fatalf("topic %q, want %q", args.SummaryTopic, expectedTopic)
			}
			cc.CPNodeAddr = "127.0.0.1"
			jc := JoinClusterConfig(cc, "", "", "", "", "", "", "", "", "")
			if err := jc.CreateBaseNodeConfig(); err != nil {
				t.Fatal(err)
			}
			if jc.TCArgs.SummaryTopic != expectedTopic || jc.TCArgs.SummarySchemaVersion != schema {
				t.Fatal("worker schema/topic mismatch")
			}

			if args.SummaryTopic != cc.summaryTopic() || args.DiscoveryEngineVersion != strings.TrimPrefix(schema, "v") {
				t.Fatalf("incorrect summary args: %s %s", args.SummaryTopic, args.DiscoveryEngineVersion)
			}
			for _, f := range []string{"templates/sumengine-config.yaml", "templates/docker-compose_cp-node.yaml", "templates/systemdTemplates/feeder-service-env"} {
				raw, err := os.ReadFile(f)
				if err != nil {
					t.Fatal(err)
				}
				tmpl, err := template.New(f).Funcs(sprig.GenericFuncMap()).Parse(string(raw))
				if err != nil {
					t.Fatal(err)
				}
				var out bytes.Buffer
				if err := tmpl.Execute(&out, args); err != nil {
					t.Fatal(err)
				}
				want := "SUMMARY_TOPIC=" + cc.summaryTopic()
				if strings.Contains(f, "sumengine-config") {
					want = "schema-version: " + schema
				}
				if !strings.Contains(f, "sumengine-config") && !strings.Contains(out.String(), "DISCOVERYENGINE_VERSION="+strings.TrimPrefix(schema, "v")) {
					t.Fatalf("%s has incorrect Feeder version", f)
				}
				if strings.Contains(f, "sumengine-config") && !strings.Contains(out.String(), "summary-event: "+expectedTopic) {
					t.Fatal("sumengine topic mismatch")
				}
				if !strings.Contains(out.String(), want) {
					t.Fatalf("%s missing %q", f, want)
				}
			}
			var kmux KmuxConfigTemplateArgs
			populateKmuxArgs(&kmux, "feeder-service", common.KmuxSummaryFileName, "test", "host", "conn", schema)
			if kmux.QueueName != "test-summary-"+schema+"-feeder-service" {
				t.Fatal(kmux.QueueName)
			}
		})
	}
}

func TestSummaryTopicPrefix(t *testing.T) {
	for _, tc := range []struct{ schema, prefix, want string }{
		{"", "cluster-a", "cluster-a-summary-v2"},
		{"v2", "cluster-a", "cluster-a-summary-v2"},
		{"v3", "cluster-a", "cluster-a-summary-v3"},
		{"v2", "", "summary-v2"},
		{"v3", "", "summary-v3"},
	} {
		cc := ClusterConfig{SummarySchemaVersion: tc.schema, RMQTopicPrefix: tc.prefix}
		if got := cc.summaryTopic(); got != tc.want {
			t.Errorf("schema=%q prefix=%q: got %q, want %q", tc.schema, tc.prefix, got, tc.want)
		}
	}
}
