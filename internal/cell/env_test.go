package cell_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/DimmKirr/devcell/internal/cell"
	"github.com/DimmKirr/devcell/internal/cfg"
)

// envMap returns a getenv func backed by m.
func envMap(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// gitConfigMap returns a git-config lookup backed by m that counts calls.
func gitConfigMap(m map[string]string, calls *int) func(string) string {
	return func(k string) string {
		if calls != nil {
			*calls++
		}
		return m[k]
	}
}

// lookup returns the value of the LAST KEY=VALUE pair for key, mirroring
// how both `docker run -e` and `env` resolve duplicates.
func lookup(t *testing.T, pairs []string, key string) (string, bool) {
	t.Helper()
	val, found := "", false
	for _, p := range pairs {
		for i := 0; i < len(p); i++ {
			if p[i] == '=' {
				if p[:i] == key {
					val, found = p[i+1:], true
				}
				break
			}
		}
	}
	return val, found
}

func wantEnv(t *testing.T, pairs []string, key, want string) {
	t.Helper()
	got, ok := lookup(t, pairs, key)
	if !ok {
		t.Errorf("%s missing from %v", key, pairs)
		return
	}
	if got != want {
		t.Errorf("%s = %q, want %q", key, got, want)
	}
}

func wantNoEnv(t *testing.T, pairs []string, key string) {
	t.Helper()
	if got, ok := lookup(t, pairs, key); ok {
		t.Errorf("%s should be absent, got %q in %v", key, got, pairs)
	}
}

// --- git identity precedence ---

func TestResolveGitIdentity_HostEnvWinsOverTOMLAndGitConfig(t *testing.T) {
	calls := 0
	id := cell.ResolveGitIdentity(
		envMap(map[string]string{"GIT_AUTHOR_NAME": "EnvName", "GIT_AUTHOR_EMAIL": "env@example.com"}),
		cfg.GitSection{AuthorName: "TomlName", AuthorEmail: "toml@example.com"},
		gitConfigMap(map[string]string{"user.name": "GitName", "user.email": "git@example.com"}, &calls),
	)
	if id.Source != cell.GitSourceHostEnv {
		t.Errorf("source = %q, want %q", id.Source, cell.GitSourceHostEnv)
	}
	if id.AuthorName != "EnvName" || id.AuthorEmail != "env@example.com" {
		t.Errorf("author = %q <%s>, want EnvName <env@example.com>", id.AuthorName, id.AuthorEmail)
	}
	// Committer fields the host env does not set fall back to the defaults,
	// not to [git] or git config: the host env tier is taken as a whole.
	if id.CommitterName != cell.DefaultGitName || id.CommitterEmail != cell.DefaultGitEmail {
		t.Errorf("committer = %q <%s>, want defaults", id.CommitterName, id.CommitterEmail)
	}
	if calls != 0 {
		t.Errorf("git config must not be consulted when host env sets an identity, got %d calls", calls)
	}
}

func TestResolveGitIdentity_AnySingleHostEnvVarSelectsHostEnvTier(t *testing.T) {
	id := cell.ResolveGitIdentity(
		envMap(map[string]string{"GIT_COMMITTER_EMAIL": "c@example.com"}),
		cfg.GitSection{AuthorName: "TomlName"},
		nil,
	)
	if id.Source != cell.GitSourceHostEnv {
		t.Fatalf("source = %q, want %q", id.Source, cell.GitSourceHostEnv)
	}
	if id.CommitterEmail != "c@example.com" {
		t.Errorf("committer email = %q, want c@example.com", id.CommitterEmail)
	}
	if id.AuthorName != cell.DefaultGitName {
		t.Errorf("author name = %q, want default %q", id.AuthorName, cell.DefaultGitName)
	}
}

func TestResolveGitIdentity_TOMLWinsOverGitConfig(t *testing.T) {
	calls := 0
	id := cell.ResolveGitIdentity(
		envMap(nil),
		cfg.GitSection{AuthorName: "TomlName", AuthorEmail: "toml@example.com"},
		gitConfigMap(map[string]string{"user.name": "GitName", "user.email": "git@example.com"}, &calls),
	)
	if id.Source != cell.GitSourceConfig {
		t.Errorf("source = %q, want %q", id.Source, cell.GitSourceConfig)
	}
	want := cell.GitIdentity{
		AuthorName: "TomlName", AuthorEmail: "toml@example.com",
		CommitterName: "TomlName", CommitterEmail: "toml@example.com",
		Source: cell.GitSourceConfig,
	}
	if id != want {
		t.Errorf("got %+v, want %+v", id, want)
	}
	if calls != 0 {
		t.Errorf("git config must not be consulted when [git] sets an identity, got %d calls", calls)
	}
}

func TestResolveGitIdentity_TOMLExplicitCommitter(t *testing.T) {
	id := cell.ResolveGitIdentity(envMap(nil), cfg.GitSection{
		AuthorName: "A", AuthorEmail: "a@example.com",
		CommitterName: "C", CommitterEmail: "c@example.com",
	}, nil)
	if id.CommitterName != "C" || id.CommitterEmail != "c@example.com" {
		t.Errorf("committer = %q <%s>, want C <c@example.com>", id.CommitterName, id.CommitterEmail)
	}
}

func TestResolveGitIdentity_TOMLPartialFillsDefaults(t *testing.T) {
	// An empty GIT_AUTHOR_NAME makes `git commit` fail, so a partial [git]
	// must never hand the guest an empty field.
	id := cell.ResolveGitIdentity(envMap(nil), cfg.GitSection{AuthorEmail: "only@example.com"}, nil)
	if id.Source != cell.GitSourceConfig {
		t.Fatalf("source = %q, want %q", id.Source, cell.GitSourceConfig)
	}
	if id.AuthorName != cell.DefaultGitName || id.CommitterName != cell.DefaultGitName {
		t.Errorf("names = %q/%q, want default %q", id.AuthorName, id.CommitterName, cell.DefaultGitName)
	}
	if id.AuthorEmail != "only@example.com" || id.CommitterEmail != "only@example.com" {
		t.Errorf("emails = %q/%q, want only@example.com", id.AuthorEmail, id.CommitterEmail)
	}
}

func TestResolveGitIdentity_GitConfigWhenNoEnvNoTOML(t *testing.T) {
	id := cell.ResolveGitIdentity(envMap(nil), cfg.GitSection{},
		gitConfigMap(map[string]string{"user.name": "GitName", "user.email": "git@example.com"}, nil))
	want := cell.GitIdentity{
		AuthorName: "GitName", AuthorEmail: "git@example.com",
		CommitterName: "GitName", CommitterEmail: "git@example.com",
		Source: cell.GitSourceGitConfig,
	}
	if id != want {
		t.Errorf("got %+v, want %+v", id, want)
	}
}

func TestResolveGitIdentity_GitConfigPartialFillsDefaults(t *testing.T) {
	id := cell.ResolveGitIdentity(envMap(nil), cfg.GitSection{},
		gitConfigMap(map[string]string{"user.name": "GitName"}, nil))
	if id.Source != cell.GitSourceGitConfig {
		t.Fatalf("source = %q, want %q", id.Source, cell.GitSourceGitConfig)
	}
	if id.AuthorName != "GitName" || id.CommitterName != "GitName" {
		t.Errorf("names = %q/%q, want GitName", id.AuthorName, id.CommitterName)
	}
	if id.AuthorEmail != cell.DefaultGitEmail || id.CommitterEmail != cell.DefaultGitEmail {
		t.Errorf("emails = %q/%q, want default", id.AuthorEmail, id.CommitterEmail)
	}
}

func TestResolveGitIdentity_DefaultsWhenNothingSet(t *testing.T) {
	for name, gitConfig := range map[string]func(string) string{
		"nil lookup":   nil,
		"empty lookup": gitConfigMap(nil, nil),
	} {
		t.Run(name, func(t *testing.T) {
			id := cell.ResolveGitIdentity(envMap(nil), cfg.GitSection{}, gitConfig)
			want := cell.GitIdentity{
				AuthorName: "DevCell", AuthorEmail: "devcell@devcell.io",
				CommitterName: "DevCell", CommitterEmail: "devcell@devcell.io",
				Source: cell.GitSourceDefault,
			}
			if id != want {
				t.Errorf("got %+v, want %+v", id, want)
			}
		})
	}
}

func TestResolveGitIdentity_NilGetenv(t *testing.T) {
	id := cell.ResolveGitIdentity(nil, cfg.GitSection{}, nil)
	if id.Source != cell.GitSourceDefault {
		t.Errorf("source = %q, want %q", id.Source, cell.GitSourceDefault)
	}
}

// --- GuestEnv ---

func TestGuestEnv_GitIdentityFromGitConfigTier(t *testing.T) {
	env := cell.GuestEnv(cell.EnvInput{
		GitConfig: gitConfigMap(map[string]string{"user.name": "GitName", "user.email": "git@example.com"}, nil),
	})
	wantEnv(t, env, "GIT_AUTHOR_NAME", "GitName")
	wantEnv(t, env, "GIT_AUTHOR_EMAIL", "git@example.com")
	wantEnv(t, env, "GIT_COMMITTER_NAME", "GitName")
	wantEnv(t, env, "GIT_COMMITTER_EMAIL", "git@example.com")
}

func TestGuestEnv_TimezoneTOMLWinsOverHost(t *testing.T) {
	env := cell.GuestEnv(cell.EnvInput{
		Timezone: "Europe/Prague",
		Getenv:   envMap(map[string]string{"TZ": "America/New_York"}),
	})
	wantEnv(t, env, "TZ", "Europe/Prague")
}

func TestGuestEnv_TimezoneFromHost(t *testing.T) {
	env := cell.GuestEnv(cell.EnvInput{Getenv: envMap(map[string]string{"TZ": "America/New_York"})})
	wantEnv(t, env, "TZ", "America/New_York")
}

func TestGuestEnv_TimezoneOmittedWhenUnset(t *testing.T) {
	env := cell.GuestEnv(cell.EnvInput{Getenv: envMap(nil)})
	wantNoEnv(t, env, "TZ")
}

func TestGuestEnv_Locale(t *testing.T) {
	cases := []struct {
		name     string
		locale   string
		hostLang string
		want     string
	}{
		{"toml wins over host", "de_DE.UTF-8", "fr_FR.UTF-8", "de_DE.UTF-8"},
		{"host LANG", "", "fr_FR.UTF-8", "fr_FR.UTF-8"},
		{"host C falls back", "", "C", "en_US.UTF-8"},
		{"host POSIX falls back", "", "POSIX", "en_US.UTF-8"},
		{"unset falls back", "", "", "en_US.UTF-8"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := cell.GuestEnv(cell.EnvInput{
				Locale: tc.locale,
				Getenv: envMap(map[string]string{"LANG": tc.hostLang}),
			})
			wantEnv(t, env, "LANG", tc.want)
			wantEnv(t, env, "LC_ALL", tc.want)
		})
	}
}

func TestGuestEnv_CellIdentity(t *testing.T) {
	env := cell.GuestEnv(cell.EnvInput{CellName: "bunkhouse", AppName: "proj-3", Workspace: "/proj-3"})
	wantEnv(t, env, "DEVCELL_CELL_NAME", "bunkhouse")
	wantEnv(t, env, "APP_NAME", "proj-3")
	wantEnv(t, env, "WORKSPACE", "/proj-3")
	wantEnv(t, env, "IS_SANDBOX", "1")
	wantEnv(t, env, "DEVCELL_PROJECT_DIR", "/proj-3")
	wantEnv(t, env, "DEVCELL_CONTAINER", "1")
}

func TestGuestEnv_EmptyIdentityFieldsOmitted(t *testing.T) {
	env := cell.GuestEnv(cell.EnvInput{})
	wantNoEnv(t, env, "DEVCELL_CELL_NAME")
	wantNoEnv(t, env, "APP_NAME")
	wantNoEnv(t, env, "WORKSPACE")
	wantEnv(t, env, "IS_SANDBOX", "1")
	wantNoEnv(t, env, "DEVCELL_PROJECT_DIR")
	wantEnv(t, env, "DEVCELL_CONTAINER", "1")
}

func TestGuestEnv_EnvPassthroughSortedAndOverridesBuiltins(t *testing.T) {
	env := cell.GuestEnv(cell.EnvInput{
		Timezone: "Europe/Prague",
		Env:      map[string]string{"ZED": "z", "ALPHA": "a b", "TZ": "UTC", "EMPTY": ""},
	})
	wantEnv(t, env, "ALPHA", "a b")
	wantEnv(t, env, "ZED", "z")
	wantEnv(t, env, "EMPTY", "")
	// [env] is emitted after the built-ins, so a user key wins.
	wantEnv(t, env, "TZ", "UTC")
}

func TestGuestEnv_MisePassthrough(t *testing.T) {
	env := cell.GuestEnv(cell.EnvInput{
		Mise: map[string]string{"trusted_config_paths": "/", "experimental": "true"},
	})
	wantEnv(t, env, "MISE_TRUSTED_CONFIG_PATHS", "/")
	wantEnv(t, env, "MISE_EXPERIMENTAL", "true")
}

func TestGuestEnv_DeterministicOrder(t *testing.T) {
	in := cell.EnvInput{
		CellName:  "main",
		AppName:   "proj-1",
		Workspace: "/proj-1",
		Git:       cfg.GitSection{AuthorName: "Ada", AuthorEmail: "ada@example.com"},
		Timezone:  "UTC",
		Locale:    "en_GB.UTF-8",
		Env:       map[string]string{"B": "2", "A": "1", "C": "3"},
		Mise:      map[string]string{"y": "2", "x": "1"},
		Getenv:    envMap(nil),
	}
	want := []string{
		"APP_NAME=proj-1",
		"DEVCELL_CELL_NAME=main",
		"DEVCELL_CONTAINER=1",
		"IS_SANDBOX=1",
		"DEVCELL_PROJECT_DIR=/proj-1",
		"WORKSPACE=/proj-1",
		"GIT_AUTHOR_NAME=Ada",
		"GIT_AUTHOR_EMAIL=ada@example.com",
		"GIT_COMMITTER_NAME=Ada",
		"GIT_COMMITTER_EMAIL=ada@example.com",
		"TZ=UTC",
		"LANG=en_GB.UTF-8",
		"LC_ALL=en_GB.UTF-8",
		"A=1",
		"B=2",
		"C=3",
		"MISE_X=1",
		"MISE_Y=2",
	}
	for i := 0; i < 20; i++ {
		if got := cell.GuestEnv(in); !reflect.DeepEqual(got, want) {
			t.Fatalf("run %d:\n got %v\nwant %v", i, got, want)
		}
	}
}

// --- HostGitConfig ---

func TestHostGitConfig_ReadsGlobalConfig(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	gitconfig := filepath.Join(dir, "gitconfig")
	if err := os.WriteFile(gitconfig, []byte("[user]\n\tname = Grace Hopper\n\temail = grace@example.com\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", gitconfig)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Chdir(dir) // outside any repo, so no local config can interfere

	if got := cell.HostGitConfig("user.name"); got != "Grace Hopper" {
		t.Errorf("user.name = %q, want %q", got, "Grace Hopper")
	}
	if got := cell.HostGitConfig("user.email"); got != "grace@example.com" {
		t.Errorf("user.email = %q, want grace@example.com", got)
	}
	if got := cell.HostGitConfig("user.signingkey"); got != "" {
		t.Errorf("unset key = %q, want empty", got)
	}
}
