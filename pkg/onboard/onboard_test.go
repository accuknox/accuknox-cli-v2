package onboard

import "testing"

func TestGetImage(t *testing.T) {
	tests := []struct {
		name             string
		customRegistry   string
		defaultRegistry  string
		defaultRepo      string
		customImage      string
		defaultImage     string
		customTag        string
		defaultTag       string
		tagPrefixToTrim  string
		tagSuffix        string
		preserveUpstream bool
		result           string
	}{
		{
			name:             "basic",
			customRegistry:   "",
			defaultRegistry:  "docker.io",
			defaultRepo:      "kubearmor",
			customImage:      "",
			defaultImage:     "kubearmor/kubearmor",
			customTag:        "",
			defaultTag:       "v1.3.8",
			preserveUpstream: true,
			result:           "docker.io/kubearmor/kubearmor:v1.3.8",
		},
		{
			name:             "custom image",
			customRegistry:   "",
			defaultRegistry:  "public.ecr.aws/k9v9d5v2",
			defaultRepo:      "kubearmor",
			customImage:      "docker.io/kubearmor/kubearmor:v1.0.0",
			defaultImage:     "kubearmor/kubearmor",
			customTag:        "",
			defaultTag:       "v1.3.8",
			preserveUpstream: true,
			result:           "docker.io/kubearmor/kubearmor:v1.0.0",
		},
		{
			name:             "registry in default image != default registry",
			customRegistry:   "",
			defaultRegistry:  "docker.io",
			defaultRepo:      "kubearmor",
			customImage:      "",
			defaultImage:     "public.ecr.aws/k9v9d5v2/shared-informer-agent",
			customTag:        "",
			defaultTag:       "v1.3.8",
			preserveUpstream: true,
			result:           "public.ecr.aws/k9v9d5v2/shared-informer-agent:v1.3.8",
		},
		{
			name:             "registry in custom image != default registry",
			customRegistry:   "",
			defaultRegistry:  "docker.io",
			defaultRepo:      "kubearmor",
			customImage:      "public.ecr.aws/k9v9d5v2/kubearmor/kubearmor:v1.0.0",
			defaultImage:     "kubearmor/kubearmor",
			customTag:        "",
			defaultTag:       "v1.3.8",
			preserveUpstream: true,
			result:           "public.ecr.aws/k9v9d5v2/kubearmor/kubearmor:v1.0.0",
		},
		{
			name:             "no registry in custom image",
			customRegistry:   "",
			defaultRegistry:  "public.ecr.aws/k9v9d5v2",
			defaultRepo:      "kubearmor",
			customImage:      "kubearmor/kubearmor:v1.3.8",
			defaultImage:     "kubearmor/kubearmor",
			customTag:        "",
			defaultTag:       "v1.3.8",
			preserveUpstream: true,
			result:           "public.ecr.aws/k9v9d5v2/kubearmor/kubearmor:v1.3.8",
		},
		{
			name:             "custom image without tag",
			customRegistry:   "",
			defaultRegistry:  "public.ecr.aws/k9v9d5v2",
			defaultRepo:      "kubearmor",
			customImage:      "docker.io/kubearmor/kubearmor",
			defaultImage:     "kubearmor/kubearmor",
			customTag:        "",
			defaultTag:       "v1.3.8",
			preserveUpstream: true,
			result:           "docker.io/kubearmor/kubearmor:v1.3.8",
		},
		{
			name:             "custom registry",
			customRegistry:   "public.ecr.aws/k9v9d5v2",
			defaultRegistry:  "docker.io",
			defaultRepo:      "kubearmor",
			customImage:      "",
			defaultImage:     "kubearmor/kubearmor",
			customTag:        "",
			defaultTag:       "v1.3.8",
			preserveUpstream: true,
			result:           "public.ecr.aws/k9v9d5v2/kubearmor/kubearmor:v1.3.8",
		},
		{
			name:             "custom registry + custom image without repo",
			customRegistry:   "public.ecr.aws/k9v9d5v2",
			defaultRegistry:  "docker.io",
			defaultRepo:      "kubearmor",
			customImage:      "localhost:5000/kubearmor:v1.3.8",
			defaultImage:     "kubearmor/kubearmor",
			customTag:        "",
			defaultTag:       "v1.3.8",
			preserveUpstream: false,
			result:           "localhost:5000/kubearmor:v1.3.8",
		},
		{
			name:             "custom registry + custom image with repo",
			customRegistry:   "public.ecr.aws/k9v9d5v2",
			defaultRegistry:  "docker.io",
			defaultRepo:      "kubearmor",
			customImage:      "localhost:5000/kubearmor/kubearmor:v1.3.8",
			defaultImage:     "kubearmor/kubearmor",
			customTag:        "",
			defaultTag:       "v1.3.8",
			preserveUpstream: true,
			result:           "localhost:5000/kubearmor/kubearmor:v1.3.8",
		},
		{
			name:             "systemd custom registry",
			customRegistry:   "public.ecr.aws/k9v9d5v2",
			defaultRegistry:  "docker.io",
			defaultRepo:      "kubearmor",
			customImage:      "",
			defaultImage:     "kubearmor/kubearmor-systemd",
			customTag:        "",
			defaultTag:       "v1.3.8",
			tagPrefixToTrim:  "v",
			tagSuffix:        "_linux-amd64",
			preserveUpstream: true,
			result:           "public.ecr.aws/k9v9d5v2/kubearmor/kubearmor-systemd:1.3.8_linux-amd64",
		},
		{
			name:             "systemd custom registry + custom tag",
			customRegistry:   "public.ecr.aws/k9v9d5v2",
			defaultRegistry:  "docker.io",
			defaultRepo:      "kubearmor",
			customImage:      "",
			defaultImage:     "kubearmor/kubearmor-systemd",
			customTag:        "v1.0.0",
			defaultTag:       "v1.3.8",
			tagPrefixToTrim:  "v",
			tagSuffix:        "_linux-amd64",
			preserveUpstream: true,
			result:           "public.ecr.aws/k9v9d5v2/kubearmor/kubearmor-systemd:1.0.0_linux-amd64",
		},
		{
			name:             "systemd custom registry + custom tag + custom image without tag",
			customRegistry:   "public.ecr.aws/k9v9d5v2",
			defaultRegistry:  "docker.io",
			defaultRepo:      "kubearmor",
			customImage:      "localhost:5000/kubearmor/kubearmor-systemd",
			defaultImage:     "kubearmor/kubearmor-systemd",
			customTag:        "v1.0.0",
			defaultTag:       "v1.3.8",
			tagPrefixToTrim:  "v",
			tagSuffix:        "_linux-amd64",
			preserveUpstream: true,
			result:           "localhost:5000/kubearmor/kubearmor-systemd:1.0.0_linux-amd64",
		},
		{
			name:             "systemd custom registry + no preserve upstream",
			customRegistry:   "public.ecr.aws/k9v9d5v2",
			defaultRegistry:  "docker.io",
			defaultRepo:      "kubearmor",
			customImage:      "",
			defaultImage:     "kubearmor/kubearmor-systemd",
			customTag:        "v1.0.0",
			defaultTag:       "v1.3.8",
			tagPrefixToTrim:  "v",
			tagSuffix:        "_linux-amd64",
			preserveUpstream: false,
			result:           "public.ecr.aws/k9v9d5v2/kubearmor-systemd:1.0.0_linux-amd64",
		},
		{
			name:             "systemd custom registry + custom image with custom repo",
			customRegistry:   "public.ecr.aws/k9v9d5v2",
			defaultRegistry:  "docker.io",
			defaultRepo:      "kubearmor",
			customImage:      "localhost:5000/accuknox-systemd/kubearmor/kubearmor-systemd",
			defaultImage:     "kubearmor/kubearmor-systemd",
			customTag:        "v1.0.0",
			defaultTag:       "v1.3.8",
			tagPrefixToTrim:  "v",
			tagSuffix:        "_linux-amd64",
			preserveUpstream: false,
			result:           "localhost:5000/accuknox-systemd/kubearmor/kubearmor-systemd:1.0.0_linux-amd64",
		},
	}

	for _, config := range tests {
		t.Run(config.name, func(t *testing.T) {
			result, err := getImage(
				VMMode_Systemd,
				config.customRegistry, config.defaultRegistry,
				config.defaultRepo, config.customImage, config.defaultImage,
				config.customTag, config.defaultTag, config.tagPrefixToTrim,
				config.tagSuffix, config.preserveUpstream)

			if err != nil {
				t.Errorf("error in getImage(): %s", err.Error())
			}

			if result != config.result {
				t.Fail()
				t.Logf("getImage() %s, want: %s, got: %s", config.name, config.result, result)
			}
		})

	}

}
