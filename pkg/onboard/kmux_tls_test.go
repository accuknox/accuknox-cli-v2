package onboard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"text/template"
)

func TestKmuxTLSVerification(t *testing.T) {
	for _, insecure := range []bool{false, true} {
		t.Run(fmt.Sprint(insecure), func(t *testing.T) {
			raw, err := json.Marshal(ClusterConfig{InsecureTLS: insecure})
			if err != nil {
				t.Fatal(err)
			}
			var cc ClusterConfig
			if err := json.Unmarshal(raw, &cc); err != nil {
				t.Fatal(err)
			}
			for name, config := range map[string]string{"consumer": kmuxConsumerConfig, "publisher": kmuxPublisherConfig} {
				tmpl, err := template.New(name).Funcs(template.FuncMap{"quote": func(s string) string { return fmt.Sprintf("%q", s) }}).Parse(config)
				if err != nil {
					t.Fatal(err)
				}
				var output bytes.Buffer
				args := KmuxConfigTemplateArgs{InsecureTLS: cc.InsecureTLS, TlsEnabled: true}
				if err := tmpl.Execute(&output, args); err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(output.String(), "skip-verify: "+fmt.Sprint(insecure)) {
					t.Fatalf("%s: incorrect TLS verification setting", name)
				}
				if !strings.Contains(output.String(), "enabled: true") {
					t.Fatalf("%s: TLS was disabled", name)
				}
			}
		})
	}
}
