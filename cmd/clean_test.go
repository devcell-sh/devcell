package main

import "testing"

func TestCleanCmd_RegisteredOnRoot(t *testing.T) {
	for _, c := range rootCmd.Commands() {
		if c.Name() == "clean" {
			return
		}
	}
	t.Fatal("`cell clean` command not registered on root")
}

func TestCleanCmd_HasRunE(t *testing.T) {
	if cleanCmd.RunE == nil {
		t.Fatal("bare `cell clean` must have a RunE (runs all safe operations)")
	}
}

func TestCleanCacheCmd_RegisteredOnClean(t *testing.T) {
	for _, c := range cleanCmd.Commands() {
		if c.Name() == "cache" {
			return
		}
	}
	t.Fatal("`cell clean cache` command not registered on clean")
}

func TestCleanBuildCmd_RegisteredOnClean(t *testing.T) {
	for _, c := range cleanCmd.Commands() {
		if c.Name() == "build" {
			return
		}
	}
	t.Fatal("`cell clean build` command not registered on clean")
}

func TestCleanNixCmd_RegisteredOnClean(t *testing.T) {
	for _, c := range cleanCmd.Commands() {
		if c.Name() == "nix" {
			return
		}
	}
	t.Fatal("`cell clean nix` command not registered on clean")
}

func TestCleanCacheCmd_FlagsExist(t *testing.T) {
	for _, name := range []string{"dry-run", "yes", "all", "workspace"} {
		if cleanCacheCmd.Flags().Lookup(name) == nil {
			t.Errorf("clean cache must support --%s", name)
		}
	}
}

func TestCleanBuildCmd_FlagsExist(t *testing.T) {
	for _, name := range []string{"force", "yes"} {
		if cleanBuildCmd.Flags().Lookup(name) == nil {
			t.Errorf("clean build must support --%s", name)
		}
	}
}

func TestCleanNixCmd_FlagsExist(t *testing.T) {
	for _, name := range []string{"force", "yes"} {
		if cleanNixCmd.Flags().Lookup(name) == nil {
			t.Errorf("clean nix must support --%s", name)
		}
	}
}

func TestCleanSolitaryCmd_RegisteredOnClean(t *testing.T) {
	for _, c := range cleanCmd.Commands() {
		if c.Name() == "solitary" {
			return
		}
	}
	t.Fatal("`cell clean solitary` command not registered on clean")
}

func TestCleanSolitaryCmd_FlagsExist(t *testing.T) {
	for _, name := range []string{"dry-run", "yes"} {
		if cleanSolitaryCmd.Flags().Lookup(name) == nil {
			t.Errorf("clean solitary must support --%s", name)
		}
	}
}

func TestCleanCmd_QuietFlagExists(t *testing.T) {
	if cleanCmd.Flags().Lookup("quiet") == nil {
		t.Error("clean must support --quiet")
	}
}

func TestCleanCmd_DryRunFlagExists(t *testing.T) {
	if cleanCmd.PersistentFlags().Lookup("dry-run") == nil {
		t.Error("clean must support --dry-run")
	}
}

func TestBuildPruneCmd_IsDeprecated(t *testing.T) {
	if pruneCmd.Deprecated == "" {
		t.Error("`cell build prune` should be deprecated in favor of `cell clean build`")
	}
}
