// Package cell holds the cell-semantics layer that every engine (docker,
// tart, winkit) shares: the environment a guest receives and the shell
// quoting used to hand it over. Engines decide how to transport these values
// (docker -e flags, `env K=V` over tart exec, WSL over ssh); they must not
// decide what the values are.
package cell

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/DimmKirr/devcell/internal/cfg"
)

// Defaults for any git identity field no precedence tier supplies.
const (
	DefaultGitName  = "DevCell"
	DefaultGitEmail = "devcell@devcell.io"
)

// DefaultLocale is used for LANG and LC_ALL when neither [cell].locale nor a
// usable host $LANG is set.
const DefaultLocale = "en_US.UTF-8"

// GitSource names the precedence tier a GitIdentity came from.
type GitSource string

const (
	GitSourceHostEnv   GitSource = "host env"
	GitSourceConfig    GitSource = "[git]"
	GitSourceGitConfig GitSource = "git config"
	GitSourceDefault   GitSource = "default"
)

// gitEnvKeys are the host variables that select the host env tier.
var gitEnvKeys = [4]string{"GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL"}

// GitIdentity is the author and committer identity handed to the guest.
// Every field is non-empty once resolved.
type GitIdentity struct {
	AuthorName     string
	AuthorEmail    string
	CommitterName  string
	CommitterEmail string
	Source         GitSource
}

// ResolveGitIdentity picks the guest git identity. Tiers are taken whole: the
// first tier that sets anything wins, and any field it leaves empty falls back
// to DefaultGitName / DefaultGitEmail rather than to a lower tier.
//
//  1. Host env: any of GIT_AUTHOR_NAME, GIT_AUTHOR_EMAIL, GIT_COMMITTER_NAME,
//     GIT_COMMITTER_EMAIL is set.
//  2. [git] in .devcell.toml: any field set. Committer fields default to the
//     author fields (cfg.GitSection.ResolvedCommitter*).
//  3. Host `git config user.name` / `user.email`, via gitConfig. Used for both
//     author and committer. A nil gitConfig skips this tier.
//  4. DefaultGitName / DefaultGitEmail.
//
// A nil getenv reads no host env.
func ResolveGitIdentity(getenv func(string) string, git cfg.GitSection, gitConfig func(string) string) GitIdentity {
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	var id GitIdentity
	switch {
	case hasGitEnv(getenv):
		id = GitIdentity{
			AuthorName:     getenv("GIT_AUTHOR_NAME"),
			AuthorEmail:    getenv("GIT_AUTHOR_EMAIL"),
			CommitterName:  getenv("GIT_COMMITTER_NAME"),
			CommitterEmail: getenv("GIT_COMMITTER_EMAIL"),
			Source:         GitSourceHostEnv,
		}
	case git.HasIdentity():
		id = GitIdentity{
			AuthorName:     git.AuthorName,
			AuthorEmail:    git.AuthorEmail,
			CommitterName:  git.ResolvedCommitterName(),
			CommitterEmail: git.ResolvedCommitterEmail(),
			Source:         GitSourceConfig,
		}
	default:
		id.Source = GitSourceDefault
		if gitConfig != nil {
			name, email := gitConfig("user.name"), gitConfig("user.email")
			if name != "" || email != "" {
				id = GitIdentity{
					AuthorName: name, AuthorEmail: email,
					CommitterName: name, CommitterEmail: email,
					Source: GitSourceGitConfig,
				}
			}
		}
	}
	id.AuthorName = orDefault(id.AuthorName, DefaultGitName)
	id.AuthorEmail = orDefault(id.AuthorEmail, DefaultGitEmail)
	id.CommitterName = orDefault(id.CommitterName, DefaultGitName)
	id.CommitterEmail = orDefault(id.CommitterEmail, DefaultGitEmail)
	return id
}

func hasGitEnv(getenv func(string) string) bool {
	for _, k := range gitEnvKeys {
		if getenv(k) != "" {
			return true
		}
	}
	return false
}

// HostGitConfig returns `git config --get <key>` as seen from the current
// directory on the host, or "" when git is missing or the key is unset.
func HostGitConfig(key string) string {
	out, err := exec.Command("git", "config", "--get", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// FillOpenRouterKey fills OPENROUTER_API_KEY from the environment. Called
// after 1Password resolution so the key is available. Env builders that need
// the key set OPENROUTER_API_KEY to "" as a placeholder; the engine fills it.
func FillOpenRouterKey(env map[string]string) error {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("--openrouter requires OPENROUTER_API_KEY env var (set it or add to [secrets.onepassword] documents)")
	}
	env["OPENROUTER_API_KEY"] = apiKey
	return nil
}

// EnvInput is everything GuestEnv needs. Runtimes fill it from the loaded
// cfg.CellConfig plus what they know about the guest.
type EnvInput struct {
	CellName  string              // DEVCELL_CELL_NAME; omitted when empty
	AppName   string              // APP_NAME; omitted when empty
	Workspace string              // WORKSPACE, the project path inside the guest; omitted when empty
	Git       cfg.GitSection      // [git]
	Timezone  string              // [cell].timezone
	Locale    string              // [cell].locale
	Env       map[string]string   // [env], already expanded against the host
	Mise      map[string]string   // [mise]
	Getenv    func(string) string // host env lookup; nil reads nothing
	GitConfig func(string) string // host `git config` lookup (HostGitConfig); nil skips that tier
}

// GuestEnv returns the KEY=VALUE pairs every runtime hands to its guest, in
// this fixed order (later pairs win for duplicate keys, matching both
// `docker run -e` and `env`):
//
//	APP_NAME, DEVCELL_CELL_NAME, DEVCELL_CONTAINER=1, IS_SANDBOX=1, DEVCELL_PROJECT_DIR, WORKSPACE
//	GIT_AUTHOR_NAME, GIT_AUTHOR_EMAIL, GIT_COMMITTER_NAME, GIT_COMMITTER_EMAIL (ResolveGitIdentity)
//	TZ:          [cell].timezone > host $TZ > omitted
//	LANG/LC_ALL: [cell].locale > host $LANG (unless C or POSIX) > DefaultLocale
//	[env]        sorted by key, so a user key overrides any built-in above
//	[mise]       sorted by key, as MISE_<UPPER_KEY>
//
// TERM is not included: it describes the host terminal the session is
// attached to, and each runtime forwards it with its own transport.
func GuestEnv(in EnvInput) []string {
	getenv := in.Getenv
	if getenv == nil {
		getenv = func(string) string { return "" }
	}

	var out []string
	add := func(k, v string) { out = append(out, k+"="+v) }
	addIfSet := func(k, v string) {
		if v != "" {
			add(k, v)
		}
	}

	addIfSet("APP_NAME", in.AppName)
	addIfSet("DEVCELL_CELL_NAME", in.CellName)
	add("DEVCELL_CONTAINER", "1")
	add("IS_SANDBOX", "1") // deprecated: use DEVCELL_CONTAINER instead
	addIfSet("DEVCELL_PROJECT_DIR", in.Workspace)
	addIfSet("WORKSPACE", in.Workspace) // deprecated: use DEVCELL_PROJECT_DIR instead

	id := ResolveGitIdentity(getenv, in.Git, in.GitConfig)
	add("GIT_AUTHOR_NAME", id.AuthorName)
	add("GIT_AUTHOR_EMAIL", id.AuthorEmail)
	add("GIT_COMMITTER_NAME", id.CommitterName)
	add("GIT_COMMITTER_EMAIL", id.CommitterEmail)

	addIfSet("TZ", orDefault(in.Timezone, getenv("TZ")))

	locale := in.Locale
	if locale == "" {
		if l := getenv("LANG"); l != "C" && l != "POSIX" {
			locale = l
		}
	}
	locale = orDefault(locale, DefaultLocale)
	add("LANG", locale)
	add("LC_ALL", locale)

	for _, k := range sortedKeys(in.Env) {
		add(k, in.Env[k])
	}
	for _, k := range sortedKeys(in.Mise) {
		add("MISE_"+strings.ToUpper(k), in.Mise[k])
	}
	return out
}

func orDefault(v, def string) string {
	if v != "" {
		return v
	}
	return def
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
