package cfg

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// Deprecation is a TOML key that was renamed. The old key keeps working
// (its value is merged into the replacement) but warns on every load.
type Deprecation struct {
	Path        []string // old TOML key path, e.g. {"ports", "forward"}
	Name        string   // old key as users write it, e.g. "[ports] forward"
	Replacement string   // new key, e.g. "[cell] ports"
	Message     string   // what the user should do, shown verbatim, e.g. `use [cell] ports = ["3000"] instead`
}

// Deprecations lists every deprecated TOML key. Remove an entry together
// with its struct field when the old key is dropped.
var Deprecations = []Deprecation{
	{
		Path:        []string{"cell", "thin"},
		Name:        "[cell] thin",
		Replacement: "(none)",
		Message:     "remove it; thin is the only build mode",
	},
	{
		Path:        []string{"cell", "libvirt_uri"},
		Name:        "[cell] libvirt_uri",
		Replacement: "(none)",
		Message:     "remove it; the libvirt engine was retired, run `cell --engine winkit` on the macOS host instead",
	},
	{
		Path:        []string{"cell", "libvirt_path_map"},
		Name:        "[cell] libvirt_path_map",
		Replacement: "(none)",
		Message:     "remove it; the libvirt engine was retired, run `cell --engine winkit` on the macOS host instead",
	},
	{
		Path:        []string{"cell", "vagrant_provider"},
		Name:        "[cell] vagrant_provider",
		Replacement: "(none)",
		Message:     `remove it; the vagrant engine was removed, use [cell] os = "macos" for a Tart VM`,
	},
	{
		Path:        []string{"cell", "vagrant_box"},
		Name:        "[cell] vagrant_box",
		Replacement: "(none)",
		Message:     `remove it; the vagrant engine was removed, use [cell] os = "macos" for a Tart VM`,
	},
	{
		Path:        []string{"cell", "qemu_project_sync"},
		Name:        "[cell] qemu_project_sync",
		Replacement: "(none)",
		Message:     "remove it; the libvirt engine was retired and it was the only engine that synced the project",
	},
	{
		Path:        []string{"cell", "qemu_disk_size_gb"},
		Name:        "[cell] qemu_disk_size_gb",
		Replacement: "(none)",
		Message:     "remove it; it has no effect",
	},
	{
		Path:        []string{"cell", "qemu_display"},
		Name:        "[cell] qemu_display",
		Replacement: "(none)",
		Message:     "remove it; it has no effect",
	},
	{
		Path:        []string{"cell", "qemu_ssh_host"},
		Name:        "[cell] qemu_ssh_host",
		Replacement: "(none)",
		Message:     "remove it; it has no effect",
	},
	{
		Path:        []string{"cell", "qemu_ssh_port"},
		Name:        "[cell] qemu_ssh_port",
		Replacement: "[cell] winkit_ssh_port",
		Message:     "use [cell] winkit_ssh_port = 2222 instead",
	},
	{
		Path:        []string{"cell", "qemu_windows_iso"},
		Name:        "[cell] qemu_windows_iso",
		Replacement: "[cell] winkit_windows_iso",
		Message:     `use [cell] winkit_windows_iso = "~/Downloads/Win11_ARM64.iso" instead`,
	},
	{
		Path:        []string{"cell", "qemu_cpus"},
		Name:        "[cell] qemu_cpus",
		Replacement: "[cell] winkit_cpus",
		Message:     "use [cell] winkit_cpus = 4 instead",
	},
	{
		Path:        []string{"cell", "qemu_memory_gb"},
		Name:        "[cell] qemu_memory_gb",
		Replacement: "[cell] winkit_memory_gb",
		Message:     "use [cell] winkit_memory_gb = 8 instead",
	},
	{
		Path:        []string{"cell", "gui"},
		Name:        "[cell] gui",
		Replacement: "[gui] enabled",
		Message:     "use [gui] enabled = false instead",
	},
	{
		Path:        []string{"cell", "nixhome"},
		Name:        "[cell] nixhome",
		Replacement: "[nix] nixhome",
		Message:     `use [nix] nixhome = "~/dev/home" instead`,
	},
	{
		Path:        []string{"packages", "npm"},
		Name:        "[packages.npm]",
		Replacement: "[packages.node]",
		Message:     `use [packages.node] "prettier" = "^3" instead`,
	},
	{
		Path:        []string{"packages", "nix", "stable"},
		Name:        "[packages.nix] stable",
		Replacement: "[cell] packages",
		Message:     `use [cell] packages = ["htop"] instead`,
	},
	{
		Path:        []string{"volumes"},
		Name:        "[[volumes]]",
		Replacement: "[cell] volumes",
		Message:     `use [cell] volumes = ["/host:/container:ro"] instead`,
	},
	{
		Path:        []string{"ports", "forward"},
		Name:        "[ports] forward",
		Replacement: "[cell] ports",
		Message:     `use [cell] ports = ["3000", "8080:3000"] instead`,
	},
	{
		Path:        []string{"mcp", "enabled"},
		Name:        "[mcp] enabled",
		Replacement: "[cell] mcps",
		Message:     `use [cell] mcps = ["playwright"] instead`,
	},
	{
		Path:        []string{"op"},
		Name:        "[op]",
		Replacement: "[secrets.onepassword]",
		Message:     `use [secrets.onepassword] documents = ["prod-api-keys"] instead`,
	},
	{
		Path:        []string{"llm", "use_ollama"},
		Name:        "[llm] use_ollama",
		Replacement: "[llm] provider",
		Message:     `use [llm] provider = "ollama" instead`,
	},
	{
		Path:        []string{"llm", "use_openrouter"},
		Name:        "[llm] use_openrouter",
		Replacement: "[llm] provider",
		Message:     `use [llm] provider = "openrouter" instead`,
	},
	{
		Path:        []string{"llm", "models", "default"},
		Name:        "[llm.models] default",
		Replacement: "[llm] model",
		Message:     `use [llm] provider = "openrouter" and model = "deepseek/deepseek-v4-pro" instead`,
	},
	{
		Path:        []string{"llm", "models", "providers"},
		Name:        "[llm.models.providers]",
		Replacement: "[llm.providers]",
		Message:     `use [llm.providers.ollama] models = ["qwen3:8b"] instead`,
	},
}

// DeprecatedUse records a deprecated key found in a specific config file.
type DeprecatedUse struct {
	Deprecation
	File string
}

// Warning is the user-facing line for this use. File is the config file the
// deprecated key was read from; env var deprecations have no file, so the
// prefix is omitted.
func (u DeprecatedUse) Warning() string {
	msg := fmt.Sprintf("%s is deprecated and will be removed in a future release: %s", u.Name, u.Message)
	if u.File == "" {
		return msg
	}
	return fmt.Sprintf("%s: %s", u.File, msg)
}

// renamedEnvVars maps each env var to the deprecated name it replaced. The
// old name is still read when the new one is unset (Getenv) and warns once
// per invocation (ApplyEnv records it, checkConfig prints it).
var renamedEnvVars = []struct{ name, deprecated string }{
	{"DEVCELL_WINKIT_SSH_PORT", "DEVCELL_QEMU_SSH_PORT"},
	{"DEVCELL_WINKIT_WINDOWS_ISO", "DEVCELL_QEMU_WINDOWS_ISO"},
	{"DEVCELL_WINKIT_CPUS", "DEVCELL_QEMU_CPUS"},
	{"DEVCELL_WINKIT_MEMORY_GB", "DEVCELL_QEMU_MEMORY_GB"},
	{"DEVCELL_WINKIT_CACHE_DIR", "DEVCELL_QEMU_CACHE_DIR"},
	{"DEVCELL_WINKIT_ACCEL", "DEVCELL_QEMU_ACCEL"},
}

// Getenv is os.Getenv for an env var that replaced a deprecated name: when
// name is unset it falls back to the deprecated one. It does not warn;
// ApplyEnv does, once per invocation.
func Getenv(name string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	for _, r := range renamedEnvVars {
		if r.name == name {
			return os.Getenv(r.deprecated)
		}
	}
	return ""
}

// renamedEnvUses returns a DeprecatedUse for every deprecated env var name
// that is set while its replacement is not.
func renamedEnvUses(getenv func(string) string) []DeprecatedUse {
	var out []DeprecatedUse
	for _, r := range renamedEnvVars {
		v := getenv(r.deprecated)
		if v == "" || getenv(r.name) != "" {
			continue
		}
		out = append(out, DeprecatedUse{Deprecation: Deprecation{
			Name:        r.deprecated,
			Replacement: r.name,
			Message:     fmt.Sprintf("use %s instead, e.g. export %s=%s", r.name, r.name, v),
		}})
	}
	return out
}

func detectDeprecations(md toml.MetaData, file string) []DeprecatedUse {
	var out []DeprecatedUse
	for _, d := range Deprecations {
		if md.IsDefined(d.Path...) {
			out = append(out, DeprecatedUse{Deprecation: d, File: file})
		}
	}
	return out
}
