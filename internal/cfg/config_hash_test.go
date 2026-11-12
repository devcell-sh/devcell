package cfg

import "testing"

func TestConfigHash_StableForSameConfig(t *testing.T) {
	c := CellConfig{
		Cell: CellSection{Stack: "ultimate"},
		Volumes: []VolumeMount{
			{Mount: "/data"},
			{Mount: "/host:/container:ro"},
		},
	}
	h1 := ConfigHash(c)
	h2 := ConfigHash(c)
	if h1 != h2 {
		t.Fatalf("same config produced different hashes: %s vs %s", h1, h2)
	}
	if h1 == "" {
		t.Fatal("hash must not be empty")
	}
}

func TestConfigHash_ChangesOnStackChange(t *testing.T) {
	base := CellConfig{Cell: CellSection{Stack: "base"}}
	ultimate := CellConfig{Cell: CellSection{Stack: "ultimate"}}
	if ConfigHash(base) == ConfigHash(ultimate) {
		t.Fatal("different stacks must produce different hashes")
	}
}

func TestConfigHash_ChangesOnVolumeChange(t *testing.T) {
	a := CellConfig{Volumes: []VolumeMount{{Mount: "/data"}}}
	b := CellConfig{Volumes: []VolumeMount{{Mount: "/data"}, {Mount: "/extra"}}}
	if ConfigHash(a) == ConfigHash(b) {
		t.Fatal("different volumes must produce different hashes")
	}
}

func TestConfigHash_ChangesOnEnvChange(t *testing.T) {
	a := CellConfig{Env: map[string]string{"FOO": "bar"}}
	b := CellConfig{Env: map[string]string{"FOO": "baz"}}
	if ConfigHash(a) == ConfigHash(b) {
		t.Fatal("different env must produce different hashes")
	}
}

func TestConfigHash_ChangesOnPortChange(t *testing.T) {
	a := CellConfig{Ports: PortsSection{Forward: []string{"3000"}}}
	b := CellConfig{Ports: PortsSection{Forward: []string{"3000", "8080"}}}
	if ConfigHash(a) == ConfigHash(b) {
		t.Fatal("different ports must produce different hashes")
	}
}

func TestConfigHash_ChangesOnModuleChange(t *testing.T) {
	a := CellConfig{Cell: CellSection{Modules: []string{"go"}}}
	b := CellConfig{Cell: CellSection{Modules: []string{"go", "node"}}}
	if ConfigHash(a) == ConfigHash(b) {
		t.Fatal("different modules must produce different hashes")
	}
}

func TestConfigHash_ChangesOnDockerSettings(t *testing.T) {
	a := CellConfig{Docker: DockerSection{Privileged: true}}
	b := CellConfig{}
	if ConfigHash(a) == ConfigHash(b) {
		t.Fatal("different docker settings must produce different hashes")
	}
}

func TestConfigHash_ChangesOnGUI(t *testing.T) {
	enabled := true
	disabled := false
	a := CellConfig{GUI: GUISection{Enabled: &enabled}}
	b := CellConfig{GUI: GUISection{Enabled: &disabled}}
	if ConfigHash(a) == ConfigHash(b) {
		t.Fatal("different GUI settings must produce different hashes")
	}
}

func TestConfigHash_ChangesOnStorageSharedCache(t *testing.T) {
	shared := true
	private := false
	a := CellConfig{Storage: StorageSection{SharedCache: &shared}}
	b := CellConfig{Storage: StorageSection{SharedCache: &private}}
	if ConfigHash(a) == ConfigHash(b) {
		t.Fatal("different storage.shared_cache must produce different hashes")
	}
}

func TestConfigHash_ChangesOnStorageCacheDir(t *testing.T) {
	a := CellConfig{Storage: StorageSection{CacheDir: "/a"}}
	b := CellConfig{Storage: StorageSection{CacheDir: "/b"}}
	if ConfigHash(a) == ConfigHash(b) {
		t.Fatal("different storage.cache_dir must produce different hashes")
	}
}

func TestConfigHash_ChangesOnSolitaryProject(t *testing.T) {
	clone := SolitaryClone
	empty := SolitaryEmpty
	a := CellConfig{Solitary: SolitarySection{Directories: SolitaryDirectories{Project: &clone}}}
	b := CellConfig{Solitary: SolitarySection{Directories: SolitaryDirectories{Project: &empty}}}
	if ConfigHash(a) == ConfigHash(b) {
		t.Fatal("different solitary.directories.project must produce different hashes")
	}
}

func TestConfigHash_ChangesOnSolitaryHome(t *testing.T) {
	clone := SolitaryClone
	empty := SolitaryEmpty
	a := CellConfig{Solitary: SolitarySection{Directories: SolitaryDirectories{Home: &clone}}}
	b := CellConfig{Solitary: SolitarySection{Directories: SolitaryDirectories{Home: &empty}}}
	if ConfigHash(a) == ConfigHash(b) {
		t.Fatal("different solitary.directories.home must produce different hashes")
	}
}

func TestConfigHash_ChangesOnSolitaryMounts(t *testing.T) {
	clone := SolitaryClone
	empty := SolitaryEmpty
	a := CellConfig{Solitary: SolitarySection{Directories: SolitaryDirectories{Mounts: &clone}}}
	b := CellConfig{Solitary: SolitarySection{Directories: SolitaryDirectories{Mounts: &empty}}}
	if ConfigHash(a) == ConfigHash(b) {
		t.Fatal("different solitary.directories.mounts must produce different hashes")
	}
}

func TestConfigHash_ChangesOnSolitarySecrets(t *testing.T) {
	on := true
	off := false
	a := CellConfig{Solitary: SolitarySection{Secrets: &on}}
	b := CellConfig{Solitary: SolitarySection{Secrets: &off}}
	if ConfigHash(a) == ConfigHash(b) {
		t.Fatal("different solitary.secrets must produce different hashes")
	}
}

func TestConfigHash_ChangesOnAgentsPeerMemory(t *testing.T) {
	on := true
	off := false
	a := CellConfig{Agents: AgentsSection{PeerMemory: &on}}
	b := CellConfig{Agents: AgentsSection{PeerMemory: &off}}
	if ConfigHash(a) == ConfigHash(b) {
		t.Fatal("different agents.peer_memory must produce different hashes")
	}
}
