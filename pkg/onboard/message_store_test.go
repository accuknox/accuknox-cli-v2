package onboard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"github.com/Masterminds/sprig"
)

func TestMessageStoreTemplates(t *testing.T) {
	for _, setting := range []bool{false, true} {
		name := fmt.Sprint(setting)
		t.Run(name, func(t *testing.T) {
			cc := ClusterConfig{AgentsVersion: "v0.12.8", CPNodeAddr: "127.0.0.1", FeederMessageStore: setting, VMAdapterMessageStore: setting, SumengineMessageStore: setting}
			// Explicit false must survive persisted cluster configuration.
			data, err := json.Marshal(cc)
			if err != nil {
				t.Fatal(err)
			}
			cc = ClusterConfig{}
			if err := json.Unmarshal(data, &cc); err != nil {
				t.Fatal(err)
			}
			ic := InitCPNodeConfig(cc, "", "spire.example:443", "", "", "", "", false)
			if err := ic.CreateBaseTemplateConfig(); err != nil {
				t.Fatal(err)
			}
			jc := JoinClusterConfig(cc, "", "", "", "", "", "", "", "", "")
			if err := jc.CreateBaseNodeConfig(); err != nil {
				t.Fatal(err)
			}
			for _, args := range []TemplateConfigArgs{ic.TCArgs, jc.TCArgs} {
				// Worker rendering sets release metadata after base configuration.
				args.ReleaseVersion = cc.AgentsVersion
				args.KubeArmorVMAdapterImage = "accuknox/kubearmor-vm-adapter:v0.2.0"
				for _, tc := range []struct{ file, key string }{
					{"templates/docker-compose_cp-node.yaml", "--message-store-enabled="},
					{"templates/docker-compose_cp-node.yaml", "MESSAGE_STORE_ENABLED="},
					{"templates/systemdTemplates/feeder-service-env", "MESSAGE_STORE_ENABLED="},
					{"templates/docker-compose_node.yaml", "--message-store-enabled="},
					{"templates/systemdTemplates/vm-adapter-config.yaml", "message-store-enabled: "},
					{"templates/sumengine-config.yaml", "message-store:\n    enable: "},
				} {
					raw, err := os.ReadFile(tc.file)
					if err != nil {
						t.Fatal(err)
					}
					tmpl, err := template.New(tc.file).Funcs(sprig.GenericFuncMap()).Parse(string(raw))
					if err != nil {
						t.Fatal(err)
					}
					var out bytes.Buffer
					if err := tmpl.Execute(&out, args); err != nil {
						t.Fatal(err)
					}
					if strings.Contains(tc.file, "sumengine-config") && !strings.Contains(out.String(), `path: "/opt/accuknox-sumengine/message-store"`) {
						t.Fatal("sumengine must use a durable path outside /tmp")
					}
					if !strings.Contains(out.String(), tc.key+fmt.Sprint(setting)) {
						t.Fatalf("%s missing setting %v", tc.file, setting)
					}
				}
			}
		})
	}
}

func TestVMAdapterMessageStoreVersion(t *testing.T) {
	for _, file := range []string{"templates/docker-compose_cp-node.yaml", "templates/docker-compose_node.yaml"} {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		tmpl, err := template.New(file).Funcs(sprig.GenericFuncMap()).Parse(string(raw))
		if err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct {
			image string
			want  bool
		}{
			{"accuknox/kubearmor-vm-adapter:v0.1.99", false},
			{"accuknox/kubearmor-vm-adapter:v0.2.0-rc.1", false},
			{"accuknox/kubearmor-vm-adapter:v0.2.0", true},
			{"accuknox/kubearmor-vm-adapter:0.2.1", true},
			{"registry.example:5000/kubearmor-vm-adapter:v0.3.0", true},
			{"accuknox/kubearmor-vm-adapter:latest", false},
			{"accuknox/kubearmor-vm-adapter", false},
		} {
			for _, setting := range []bool{false, true} {
				t.Run(file+"/"+tc.image+"/"+fmt.Sprint(setting), func(t *testing.T) {
					args := TemplateConfigArgs{
						ReleaseVersion:          "v0.12.8",
						KubeArmorVMAdapterImage: tc.image,
						VMAdapterMessageStore:   setting,
					}
					var out bytes.Buffer
					if err := tmpl.Execute(&out, args); err != nil {
						t.Fatal(err)
					}
					if got := strings.Contains(out.String(), "--message-store-enabled="); got != (tc.want || setting) {
						t.Fatalf("flag present = %v, want %v", got, tc.want || setting)
					}
					if (tc.want || setting) && !strings.Contains(out.String(), "--message-store-enabled="+fmt.Sprint(setting)) {
						t.Fatal("incorrect message store setting")
					}
				})
			}
		}
	}
}

func TestPrepareSumengineMessageStore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "accuknox-sumengine", "message-store")
	if err := prepareSumengineMessageStore(dir, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("disabled store created a directory")
	}
	if os.Geteuid() != 0 && (os.Geteuid() != 1000 || os.Getegid() != 1000) {
		t.Skip("preparing UID 1000 storage requires root or UID/GID 1000")
	}
	if err := prepareSumengineMessageStore(dir, true); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0700 {
		t.Fatalf("unexpected permissions: %v", info.Mode())
	}
	// Re-onboarding must not remove queued data.
	marker := filepath.Join(path, "pending-message")
	if err := os.WriteFile(marker, []byte("pending"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := prepareSumengineMessageStore(dir, true); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(marker)
	if err != nil || string(data) != "pending" {
		t.Fatalf("queued data changed: %q, %v", data, err)
	}
}
