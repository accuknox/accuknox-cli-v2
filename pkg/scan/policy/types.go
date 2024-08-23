package policy

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Global Alias: stores the label of an endpoint
type LabelMap = map[string]string

/* KubeArmor system policy */

// KubeArmorPolicy Structure
type KubeArmorPolicy struct {
	APIVersion string            `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	Kind       string            `json:"kind,omitempty" yaml:"kind,omitempty"`
	Metadata   metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	// Spec KnoxSystemSpec   `json:"spec,omitempty" yaml:"spec,omitempty"`
	Spec HostSecuritySpec `json:"spec" yaml:"spec"`
}

type HostSecuritySpec struct {
	NodeSelector NodeSelectorType `json:"nodeSelector" yaml:"nodeSelector"`

	Process      ProcessType      `json:"process,omitempty" yaml:"process,omitempty"`
	File         FileType         `json:"file,omitempty" yaml:"file,omitempty"`
	Network      NetworkType      `json:"network,omitempty" yaml:"network,omitempty"`
	Capabilities CapabilitiesType `json:"capabilities,omitempty" yaml:"capabilities,omitempty"`
	Syscalls     SyscallsType     `json:"syscalls,omitempty" yaml:"syscalls,omitempty"`

	AppArmor string `json:"apparmor,omitempty" yaml:"apparmor,omitempty"`

	Severity int      `json:"severity" yaml:"severity"`
	Tags     []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Message  string   `json:"message,omitempty" yaml:"message,omitempty"`
	Action   string   `json:"action" yaml:"action"`
}

type NodeSelectorType struct {
	MatchLabels map[string]string `json:"matchLabels,omitempty" yaml:"matchLabels,omitempty"`
	Identities  []string          `json:"identities,omitempty" yaml:"identities,omitempty"`
}

type ProcessType struct {
	MatchPaths       []ProcessPathType      `json:"matchPaths,omitempty" yaml:"matchPaths,omitempty"`
	MatchDirectories []ProcessDirectoryType `json:"matchDirectories,omitempty" yaml:"matchDirectories,omitempty"`
	MatchPatterns    []ProcessPatternType   `json:"matchPatterns,omitempty" yaml:"matchPatterns,omitempty"`

	Severity int      `json:"severity,omitempty" yaml:"severity,omitempty"`
	Tags     []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Message  string   `json:"message,omitempty" yaml:"message,omitempty"`
	Action   string   `json:"action,omitempty" yaml:"action,omitempty"`
}

type ProcessPathType struct {
	Path       string            `json:"path,omitempty" yaml:"path,omitempty"`
	ExecName   string            `json:"execname,omitempty" yaml:"execname,omitempty"`
	OwnerOnly  bool              `json:"ownerOnly,omitempty" yaml:"ownerOnly,omitempty"`
	FromSource []MatchSourceType `json:"fromSource,omitempty" yaml:"fromSource,omitempty"`

	Severity int      `json:"severity,omitempty" yaml:"severity,omitempty"`
	Tags     []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Message  string   `json:"message,omitempty" yaml:"message,omitempty"`
	Action   string   `json:"action,omitempty" yaml:"action,omitempty"`
}

type MatchSourceType struct {
	Path string `json:"path,omitempty" yaml:"path,omitempty"`
}

type ProcessDirectoryType struct {
	Directory  string            `json:"dir" yaml:"dir"`
	Recursive  bool              `json:"recursive,omitempty" yaml:"recursive,omitempty"`
	OwnerOnly  bool              `json:"ownerOnly,omitempty" yaml:"ownerOnly,omitempty"`
	FromSource []MatchSourceType `json:"fromSource,omitempty" yaml:"fromSource,omitempty"`

	Severity int      `json:"severity,omitempty" yaml:"severity,omitempty"`
	Tags     []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Message  string   `json:"message,omitempty" yaml:"message,omitempty"`
	Action   string   `json:"action,omitempty" yaml:"action,omitempty"`
}

type FileType struct {
	MatchPaths       []FilePathType      `json:"matchPaths,omitempty" yaml:"matchPaths,omitempty"`
	MatchDirectories []FileDirectoryType `json:"matchDirectories,omitempty" yaml:"matchDirectories,omitempty"`
	MatchPatterns    []FilePatternType   `json:"matchPatterns,omitempty" yaml:"matchPatterns,omitempty"`

	Severity int      `json:"severity,omitempty" yaml:"severity,omitempty"`
	Tags     []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Message  string   `json:"message,omitempty" yaml:"message,omitempty"`
	Action   string   `json:"action,omitempty" yaml:"action,omitempty"`
}

type FilePathType struct {
	Path       string            `json:"path" yaml:"path"`
	ReadOnly   bool              `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
	OwnerOnly  bool              `json:"ownerOnly,omitempty" yaml:"ownerOnly,omitempty"`
	FromSource []MatchSourceType `json:"fromSource,omitempty" yaml:"fromSource,omitempty"`

	Severity int      `json:"severity,omitempty" yaml:"severity,omitempty"`
	Tags     []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Message  string   `json:"message,omitempty" yaml:"message,omitempty"`
	Action   string   `json:"action,omitempty" yaml:"action,omitempty"`
}

type FileDirectoryType struct {
	Directory  string            `json:"dir" yaml:"dir"`
	ReadOnly   bool              `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
	Recursive  bool              `json:"recursive,omitempty" yaml:"recursive,omitempty"`
	OwnerOnly  bool              `json:"ownerOnly,omitempty" yaml:"ownerOnly,omitempty"`
	FromSource []MatchSourceType `json:"fromSource,omitempty" yaml:"fromSource,omitempty"`

	Severity int      `json:"severity,omitempty" yaml:"severity,omitempty"`
	Tags     []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Message  string   `json:"message,omitempty" yaml:"message,omitempty"`
	Action   string   `json:"action,omitempty" yaml:"action,omitempty"`
}

type FilePatternType struct {
	Pattern   string `json:"pattern" yaml:"pattern"`
	ReadOnly  bool   `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
	OwnerOnly bool   `json:"ownerOnly,omitempty" yaml:"ownerOnly,omitempty"`

	Severity int      `json:"severity,omitempty" yaml:"severity,omitempty"`
	Tags     []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Message  string   `json:"message,omitempty" yaml:"message,omitempty"`
	Action   string   `json:"action,omitempty" yaml:"action,omitempty"`
}

type NetworkType struct {
	MatchProtocols []NetworkProtocolType `json:"matchProtocols,omitempty" yaml:"matchProtocols,omitempty"`

	Severity int      `json:"severity,omitempty" yaml:"severity,omitempty"`
	Tags     []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Message  string   `json:"message,omitempty" yaml:"message,omitempty"`
	Action   string   `json:"action,omitempty" yaml:"action,omitempty"`
}

type NetworkProtocolType struct {
	Protocol   string            `json:"protocol" yaml:"protocol"`
	FromSource []MatchSourceType `json:"fromSource,omitempty" yaml:"fromSource,omitempty"`

	Severity int      `json:"severity,omitempty" yaml:"severity,omitempty"`
	Tags     []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Message  string   `json:"message,omitempty" yaml:"message,omitempty"`
	Action   string   `json:"action,omitempty" yaml:"action,omitempty"`
}

type CapabilitiesCapabilityType struct {
	Capability string            `json:"capability" yaml:"capability"`
	FromSource []MatchSourceType `json:"fromSource,omitempty" yaml:"fromSource,omitempty"`

	Severity int      `json:"severity,omitempty" yaml:"severity,omitempty"`
	Tags     []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Message  string   `json:"message,omitempty" yaml:"message,omitempty"`
	Action   string   `json:"action,omitempty" yaml:"action,omitempty"`
}

type CapabilitiesType struct {
	MatchCapabilities []CapabilitiesCapabilityType `json:"matchCapabilities,omitempty" yaml:"matchCapabilities,omitempty"`

	Severity int      `json:"severity,omitempty" yaml:"severity,omitempty"`
	Tags     []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Message  string   `json:"message,omitempty" yaml:"message,omitempty"`
	Action   string   `json:"action,omitempty" yaml:"action,omitempty"`
}

type SyscallsType struct {
	MatchSyscalls []SyscallMatchType     `json:"matchSyscalls,omitempty" yaml:"matchSyscalls,omitempty"`
	MatchPaths    []SyscallMatchPathType `json:"matchPaths,omitempty" yaml:"matchPaths,omitempty"`

	Severity int      `json:"severity,omitempty" yaml:"severity,omitempty"`
	Tags     []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Message  string   `json:"message,omitempty" yaml:"message,omitempty"`
}

type SyscallMatchType struct {
	Syscalls   []string                `json:"syscall,omitempty" yaml:"syscall,omitempty"`
	FromSource []SyscallFromSourceType `json:"fromSource,omitempty" yaml:"fromSource,omitempty"`

	Severity int      `json:"severity,omitempty" yaml:"severity,omitempty"`
	Tags     []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Message  string   `json:"message,omitempty" yaml:"message,omitempty"`
}

type SyscallMatchPathType struct {
	Path       string                  `json:"path,omitempty" yaml:"path,omitempty"`
	Recursive  bool                    `json:"recursive,omitempty" yaml:"recursive,omitempty"`
	Syscalls   []string                `json:"syscall,omitempty" yaml:"syscall,omitempty"`
	FromSource []SyscallFromSourceType `json:"fromSource,omitempty" yaml:"fromSource,omitempty"`

	Severity int      `json:"severity,omitempty" yaml:"severity,omitempty"`
	Tags     []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Message  string   `json:"message,omitempty" yaml:"message,omitempty"`
}

type SyscallFromSourceType struct {
	Path      string `json:"path,omitempty" yaml:"path,omitempty"`
	Dir       string `json:"dir,omitempty" yaml:"dir,omitempty"`
	Recursive bool   `json:"recursive,omitempty" yaml:"recursive,omitempty"`
}

type ProcessPatternType struct {
	Pattern   string `json:"pattern" yaml:"pattern"`
	OwnerOnly bool   `json:"ownerOnly,omitempty" yaml:"ownerOnly,omitempty"`

	Severity int      `json:"severity,omitempty" yaml:"severity,omitempty"`
	Tags     []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Message  string   `json:"message,omitempty" yaml:"message,omitempty"`
	Action   string   `json:"action,omitempty" yaml:"action,omitempty"`
}
