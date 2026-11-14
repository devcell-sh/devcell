package docker_test

import (
	"os"
	"strings"
	"testing"

	"github.com/devcell-sh/devcell/internal/cfg"
	"github.com/devcell-sh/devcell/internal/config"
	"github.com/devcell-sh/devcell/internal/engine/docker"
	"github.com/devcell-sh/devcell/internal/storage"
)

func storageArgv(plan *storage.Plan) []string {
	e := makeEnv()
	c := config.Load("/tmp/storagetest", e)
	spec := docker.RunSpec{
		Config:      c,
		CellCfg:     cfg.CellConfig{},
		Binary:      "claude",
		StoragePlan: plan,
	}
	return docker.BuildArgv(spec,
		docker.FSFunc(func(string) error { return os.ErrNotExist }),
		func(string) (string, error) { return "", os.ErrNotExist },
	)
}

func TestBuildArgv_StorageMountsPresent(t *testing.T) {
	plan := storage.Resolve(storage.Input{
		CacheDir:    "/test/cache",
		SharedCache: true,
		StackTag:    "go-abcd1234",
	})
	argv := storageArgv(&plan)
	wantVolume := "/test/cache/v1/shared/go/mod:/var/cache/devcell/go/mod"
	if !hasArg(argv, wantVolume) {
		t.Errorf("expected volume %q in argv", wantVolume)
	}
	wantBuild := "/test/cache/v1/shared/env/go-abcd1234/go-build:/var/cache/devcell/go/build"
	if !hasArg(argv, wantBuild) {
		t.Errorf("expected volume %q in argv", wantBuild)
	}
}

func TestBuildArgv_StorageEnvVarsPresent(t *testing.T) {
	plan := storage.Resolve(storage.Input{
		CacheDir:    "/test/cache",
		SharedCache: true,
		StackTag:    "go",
	})
	argv := storageArgv(&plan)
	wantEnvs := map[string]string{
		"GOMODCACHE":              "/var/cache/devcell/go/mod",
		"GOCACHE":                 "/var/cache/devcell/go/build",
		"npm_config_cache":        "/var/cache/devcell/npm",
		"UV_CACHE_DIR":            "/var/cache/devcell/uv",
		"MISE_DATA_DIR":           "/var/cache/devcell/mise",
		"DEVCELL_WINKIT_CACHE_DIR": "/var/cache/devcell/winkit",
	}
	for k, v := range wantEnvs {
		want := k + "=" + v
		if !hasArg(argv, want) {
			t.Errorf("expected env %q in argv", want)
		}
	}
}

func TestBuildArgv_StorageMiseMountPresent(t *testing.T) {
	plan := storage.Resolve(storage.Input{
		CacheDir:    "/test/cache",
		SharedCache: true,
		StackTag:    "go",
	})
	argv := storageArgv(&plan)
	wantVolume := "/test/cache/v1/shared/mise:/var/cache/devcell/mise"
	if !hasArg(argv, wantVolume) {
		t.Errorf("expected mise volume %q in argv", wantVolume)
	}
	wantEnv := "MISE_DATA_DIR=/var/cache/devcell/mise"
	if !hasArg(argv, wantEnv) {
		t.Errorf("expected mise env %q in argv", wantEnv)
	}
}

func TestBuildArgv_StorageNilPlan(t *testing.T) {
	argv := storageArgv(nil)
	for _, a := range argv {
		if strings.Contains(a, "/var/cache/devcell") {
			t.Errorf("nil storage plan should not produce cache paths, found %q", a)
		}
	}
}

func TestBuildArgv_StoragePrivatePaths(t *testing.T) {
	plan := storage.Resolve(storage.Input{
		CacheDir:      "/test/cache",
		SharedCache:   false,
		WorkspaceName: "main",
		CellName:      "proj-0",
		BaseDir:       "/home/user/proj",
		StackTag:      "go",
	})
	argv := storageArgv(&plan)

	// GoMod, GoBuild, Npm, Uv should be under /v1/private/
	for _, m := range []storage.Mount{plan.GoMod, plan.GoBuild, plan.Npm, plan.Uv} {
		if !strings.Contains(m.HostPath, "/v1/private/") {
			t.Errorf("%s host path should be under /v1/private/, got %q", m.EnvVar, m.HostPath)
		}
	}

	// Mise should still be shared
	if !strings.Contains(plan.Mise.HostPath, "/v1/shared/mise") {
		t.Errorf("mise should be shared even in private mode, got %q", plan.Mise.HostPath)
	}

	// Winkit should still be shared
	if !strings.Contains(plan.Winkit.HostPath, "/v1/shared/winkit") {
		t.Errorf("winkit should be shared even in private mode, got %q", plan.Winkit.HostPath)
	}

	// Check that the private mount appears in argv
	found := false
	for _, a := range argv {
		if strings.Contains(a, "/v1/private/") && strings.Contains(a, ":/var/cache/devcell/") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected private mount path in argv")
	}
}
