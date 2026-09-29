package container_test

import (
	"context"
	"fmt"
	"log"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DimmKirr/devcell/internal/cfg"
	"github.com/DimmKirr/devcell/internal/testutil"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type exampleSpec struct {
	name     string
	binaries []string
	buildCmd string
}

var exampleSpecs = []exampleSpec{
	{
		name: "go-api",
		binaries: []string{
			"go", "gopls", "golangci-lint",
			"zsh", "git", "curl", "jq", "rg", "task",
		},
		buildCmd: "cd /workspace && go build -o /dev/null .",
	},
	{
		name: "python-api",
		binaries: []string{
			"python3", "uv",
			"zsh", "git", "curl", "jq", "rg", "task",
		},
		buildCmd: `python3 -c "import ast; ast.parse(open('/workspace/app.py').read())"`,
	},
	{
		name: "nodejs-api",
		binaries: []string{
			"node", "npm", "hugo",
			"zsh", "git", "curl", "jq", "rg", "task",
		},
		buildCmd: "node --check /workspace/server.js",
	},
	{
		name: "pcb-schematics",
		binaries: []string{
			"kicad-cli", "ngspice", "kicad-mcp",
			"zsh", "git", "curl", "jq", "rg", "task",
		},
		buildCmd: "kicad-cli version",
	},
	{
		name: "aws-infrastructure",
		binaries: []string{
			"terraform", "aws", "helm", "kubectl", "packer",
			"opentofu-mcp-server",
			"drawio", "inkscape",
			"zsh", "git", "curl", "jq", "rg", "task",
		},
		buildCmd: "terraform version && aws --version",
	},
}

// --- Tier 1: config validation (fast, always runs) ---

func TestExamples_ConfigValid(t *testing.T) {
	root := testutil.RepoRoot()
	for _, ex := range exampleSpecs {
		t.Run(ex.name, func(t *testing.T) {
			p := filepath.Join(root, "examples", ex.name, ".devcell.toml")
			if _, err := os.Stat(p); err != nil {
				t.Fatalf(".devcell.toml missing: %v", err)
			}
			c, err := cfg.LoadFile(p)
			if err != nil {
				t.Fatalf("LoadFile: %v", err)
			}
			if err := cfg.ValidateStack(c.Cell.Stack); err != nil {
				t.Errorf("ValidateStack(%q): %v", c.Cell.Stack, err)
			}
			if len(c.Cell.Modules) == 0 {
				t.Error("no modules declared")
			}
			if len(c.Cell.Packages) > 0 {
				np := cfg.NixPackages{Stable: c.Cell.Packages}
				if err := cfg.ValidateNixPackageNames(np); err != nil {
					t.Errorf("ValidateNixPackageNames: %v", err)
				}
			}
		})
	}
}

// --- Tier 2: container E2E (slow, builds images) ---

func TestExamples_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("long: builds images for each example; run without -short")
	}

	for _, ex := range exampleSpecs {
		ex := ex
		t.Run(ex.name, func(t *testing.T) {
			outDir := testutil.TestResultsDir(t, hostBaseDirFn)
			imageTag := buildExampleImage(t, ex.name)
			c := startExampleContainer(t, imageTag)
			copyExampleFiles(t, c, ex.name)

			t.Run("binaries", func(t *testing.T) {
				var results []string
				for _, bin := range ex.binaries {
					bin := bin
					t.Run(bin, func(t *testing.T) {
						out, code := asUser(t, c, "command -v "+bin)
						status := "OK"
						if code != 0 {
							status = fmt.Sprintf("MISSING (exit %d)", code)
							t.Errorf("%s: not found (exit %d, output: %s)", bin, code, out)
						}
						results = append(results, fmt.Sprintf("%s: %s  %s", bin, status, strings.TrimSpace(out)))
					})
				}
				binDir := testutil.TestResultsDir(t, hostBaseDirFn)
				os.WriteFile(filepath.Join(binDir, "binaries.txt"),
					[]byte(strings.Join(results, "\n")+"\n"), 0o644)
			})

			if ex.buildCmd != "" {
				t.Run("build", func(t *testing.T) {
					buildDir := testutil.TestResultsDir(t, hostBaseDirFn)
					out, code := asUser(t, c, ex.buildCmd)
					os.WriteFile(filepath.Join(buildDir, "build.log"),
						[]byte(out), 0o644)
					if code != 0 {
						t.Errorf("build failed (exit %d):\n%s", code, out)
					}
				})
			}

			if ex.name == "aws-infrastructure" {
				t.Run("drawio_export", func(t *testing.T) {
					if os.Getenv("DEVCELL_E2E") != "1" {
						t.Skip("drawio export needs Xvfb; set DEVCELL_E2E=1 on hosts with virtual display")
					}
					drawioDir := testutil.TestResultsDir(t, hostBaseDirFn)
					out, code := asUser(t, c,
						"drawio --export --format png --output /tmp/arch.png /workspace/architecture.drawio")
					os.WriteFile(filepath.Join(drawioDir, "drawio-export.log"),
						[]byte(out), 0o644)
					if code != 0 {
						t.Fatalf("drawio export failed (exit %d):\n%s", code, out)
					}
					pngData, code := exec(t, c, []string{"cat", "/tmp/arch.png"})
					if code != 0 || len(pngData) == 0 {
						t.Error("PNG file not created or empty")
					} else {
						os.WriteFile(filepath.Join(drawioDir, "architecture.png"),
							[]byte(pngData), 0o644)
					}
				})
			}

			_ = outDir
		})
	}
}

// buildExampleImage builds an image for an example project by running
// `cell build` from the example directory. Reuses existing images
// when the tag (keyed to the current HEAD sha) already exists.
func buildExampleImage(t *testing.T, exampleName string) string {
	t.Helper()
	tag := fmt.Sprintf("devcell-user:example-%s-%s", exampleName, shortSHA())
	if imageExists(tag) {
		log.Printf("Reusing example image: %s", tag)
		return tag
	}

	cellBin, err := ensureCellBinary()
	if err != nil {
		t.Fatalf("ensureCellBinary: %v", err)
	}

	log.Printf("Building image for example %s: tag=%s", exampleName, tag)
	cmd := osexec.Command(cellBin, "build", "--image", tag, "--debug")
	cmd.Dir = filepath.Join("..", "examples", exampleName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("cell build for %s: %v", exampleName, err)
	}
	return tag
}

// startExampleContainer starts a container from an example image with
// the nix store volume mounted and standard env vars.
func startExampleContainer(t *testing.T, imageTag string) testcontainers.Container {
	t.Helper()
	requireDockerSocket(t)
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image: imageTag,
		Env: map[string]string{
			"HOST_USER": hostUser,
			"APP_NAME":  "test",
		},
		User: "0",
		Cmd:  []string{"tail", "-f", "/dev/null"},
		WaitingFor: wait.ForExec([]string{"pgrep", "tail"}).
			WithStartupTimeout(120 * 1e9),
		Mounts: testcontainers.Mounts(
			testcontainers.VolumeMount(thinVolumeName(), "/nix"),
		),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start container from %s: %v", imageTag, err)
	}
	t.Cleanup(func() { _ = c.Terminate(ctx) })
	return c
}

// copyExampleFiles copies the example project files into /workspace inside the
// container so build commands can find them.
func copyExampleFiles(t *testing.T, c testcontainers.Container, exampleName string) {
	t.Helper()
	ctx := context.Background()

	if _, code := exec(t, c, []string{"mkdir", "-p", "/workspace"}); code != 0 {
		t.Fatal("failed to create /workspace")
	}

	exampleDir := filepath.Join(testutil.RepoRoot(), "examples", exampleName)
	entries, err := os.ReadDir(exampleDir)
	if err != nil {
		t.Fatalf("read example dir: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		src := filepath.Join(exampleDir, e.Name())
		if err := c.CopyFileToContainer(ctx, src, "/workspace/"+e.Name(), 0o644); err != nil {
			t.Fatalf("copy %s to container: %v", e.Name(), err)
		}
	}
}
