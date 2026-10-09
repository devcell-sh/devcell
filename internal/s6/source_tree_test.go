package s6

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sourceTreeRoot returns the path to the s6 service source definitions.
// The community-home repo is bind-mounted at this path inside the dev cell.
func sourceTreeRoot(t *testing.T) string {
	t.Helper()
	root := os.Getenv("S6_SOURCE_TREE")
	if root == "" {
		root = "/Users/dmitry/dev/devcell-sh/home/modules/s6"
	}
	if _, err := os.Stat(root); err != nil {
		t.Skipf("s6 source tree not available at %s (set S6_SOURCE_TREE to override)", root)
	}
	return root
}

// loadServices reads all service dirs from the source tree.
type svcInfo struct {
	name    string
	svcType string // "oneshot", "longrun", "bundle"
	deps    []string
	content []string // bundle contents
	hasRun  bool
	hasUp   bool
	hasDown bool
	hasFD   bool
	fdVal   string
	hasData    bool
	hasCheck   bool
	hasLog     bool
	hasLogRun  bool
	bootGroup  string
	bootStep   string
}

func loadServices(t *testing.T, root string) map[string]svcInfo {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("reading source tree: %v", err)
	}

	svcs := make(map[string]svcInfo)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		dir := filepath.Join(root, name)
		info := svcInfo{name: name}

		if b, err := os.ReadFile(filepath.Join(dir, "type")); err == nil {
			info.svcType = strings.TrimSpace(string(b))
		}

		if deps, err := os.ReadDir(filepath.Join(dir, "dependencies.d")); err == nil {
			for _, d := range deps {
				info.deps = append(info.deps, d.Name())
			}
		}

		if contents, err := os.ReadDir(filepath.Join(dir, "contents.d")); err == nil {
			for _, c := range contents {
				info.content = append(info.content, c.Name())
			}
		}

		// Check for scripts in platform dirs and shared
		for _, sub := range []string{"", "linux", "darwin"} {
			base := dir
			if sub != "" {
				base = filepath.Join(dir, sub)
			}
			if _, err := os.Stat(filepath.Join(base, "run")); err == nil {
				info.hasRun = true
			}
			if _, err := os.Stat(filepath.Join(base, "up")); err == nil {
				info.hasUp = true
			}
			if _, err := os.Stat(filepath.Join(base, "down")); err == nil {
				info.hasDown = true
			}
		}

		if b, err := os.ReadFile(filepath.Join(dir, "notification-fd")); err == nil {
			info.hasFD = true
			info.fdVal = strings.TrimSpace(string(b))
		}

		if st, err := os.Stat(filepath.Join(dir, "data")); err == nil && st.IsDir() {
			info.hasData = true
			if _, err := os.Stat(filepath.Join(dir, "data", "check")); err == nil {
				info.hasCheck = true
			}
			if b, err := os.ReadFile(filepath.Join(dir, "data", "boot-group")); err == nil {
				info.bootGroup = strings.TrimSpace(string(b))
			}
			if b, err := os.ReadFile(filepath.Join(dir, "data", "boot-step")); err == nil {
				info.bootStep = strings.TrimSpace(string(b))
			}
		}

		if st, err := os.Stat(filepath.Join(dir, "log")); err == nil && st.IsDir() {
			info.hasLog = true
			if _, err := os.Stat(filepath.Join(dir, "log", "run")); err == nil {
				info.hasLogRun = true
			}
		}

		svcs[name] = info
	}
	return svcs
}

func TestSourceTree_EveryServiceHasType(t *testing.T) {
	root := sourceTreeRoot(t)
	svcs := loadServices(t, root)

	for name, info := range svcs {
		if info.svcType == "" {
			t.Errorf("%s: missing type file", name)
			continue
		}
		switch info.svcType {
		case "oneshot", "longrun", "bundle":
		default:
			t.Errorf("%s: invalid type %q (must be oneshot, longrun, or bundle)", name, info.svcType)
		}
	}
}

func TestSourceTree_OneshotsHaveUp(t *testing.T) {
	root := sourceTreeRoot(t)
	svcs := loadServices(t, root)

	for name, info := range svcs {
		if info.svcType != "oneshot" {
			continue
		}
		if !info.hasUp {
			t.Errorf("%s: oneshot service missing up script (checked shared, linux/, darwin/)", name)
		}
	}
}

func TestSourceTree_LongrunsHaveRun(t *testing.T) {
	root := sourceTreeRoot(t)
	svcs := loadServices(t, root)

	for name, info := range svcs {
		if info.svcType != "longrun" {
			continue
		}
		if !info.hasRun {
			t.Errorf("%s: longrun service missing run script (checked shared, linux/, darwin/)", name)
		}
	}
}

func TestSourceTree_BundlesHaveContents(t *testing.T) {
	root := sourceTreeRoot(t)
	svcs := loadServices(t, root)

	for name, info := range svcs {
		if info.svcType != "bundle" {
			continue
		}
		if len(info.content) == 0 {
			t.Errorf("%s: bundle has no contents.d entries", name)
		}
	}
}

func TestSourceTree_BundlesHaveNoScripts(t *testing.T) {
	root := sourceTreeRoot(t)
	svcs := loadServices(t, root)

	for name, info := range svcs {
		if info.svcType != "bundle" {
			continue
		}
		if info.hasRun {
			t.Errorf("%s: bundle must not have a run script", name)
		}
		if info.hasUp {
			t.Errorf("%s: bundle must not have an up script", name)
		}
	}
}

func TestSourceTree_DependenciesExist(t *testing.T) {
	root := sourceTreeRoot(t)
	svcs := loadServices(t, root)

	for name, info := range svcs {
		for _, dep := range info.deps {
			if _, ok := svcs[dep]; !ok {
				t.Errorf("%s: dependency %q does not exist as a service", name, dep)
			}
		}
	}
}

func TestSourceTree_BundleContentsExist(t *testing.T) {
	root := sourceTreeRoot(t)
	svcs := loadServices(t, root)

	for name, info := range svcs {
		if info.svcType != "bundle" {
			continue
		}
		for _, member := range info.content {
			if _, ok := svcs[member]; !ok {
				t.Errorf("%s: bundle member %q does not exist as a service", name, member)
			}
		}
	}
}

func TestSourceTree_UserBundleExists(t *testing.T) {
	root := sourceTreeRoot(t)
	svcs := loadServices(t, root)

	user, ok := svcs["user"]
	if !ok {
		t.Fatal("user bundle does not exist")
	}
	if user.svcType != "bundle" {
		t.Fatalf("user service type = %q, want bundle", user.svcType)
	}
	if len(user.content) == 0 {
		t.Fatal("user bundle has no contents")
	}
}

func TestSourceTree_UserBundleContainsAllServices(t *testing.T) {
	root := sourceTreeRoot(t)
	svcs := loadServices(t, root)

	user, ok := svcs["user"]
	if !ok {
		t.Fatal("user bundle does not exist")
	}

	bundled := make(map[string]bool)
	for _, m := range user.content {
		bundled[m] = true
	}

	for name, info := range svcs {
		if name == "user" {
			continue
		}
		if info.svcType == "bundle" {
			continue
		}
		if !bundled[name] {
			t.Errorf("service %q is not in the user bundle contents.d", name)
		}
	}
}

func TestSourceTree_NoCyclicDependencies(t *testing.T) {
	root := sourceTreeRoot(t)
	svcs := loadServices(t, root)

	// DFS cycle detection
	const (
		unvisited = iota
		visiting
		visited
	)
	state := make(map[string]int)
	var path []string

	var visit func(name string) bool
	visit = func(name string) bool {
		if state[name] == visited {
			return false
		}
		if state[name] == visiting {
			cycle := append(path, name)
			t.Errorf("dependency cycle detected: %s", strings.Join(cycle, " -> "))
			return true
		}
		state[name] = visiting
		path = append(path, name)

		info := svcs[name]
		for _, dep := range info.deps {
			if visit(dep) {
				return true
			}
		}
		for _, member := range info.content {
			if visit(member) {
				return true
			}
		}

		path = path[:len(path)-1]
		state[name] = visited
		return false
	}

	for name := range svcs {
		if state[name] == unvisited {
			visit(name)
		}
	}
}

func TestSourceTree_NotificationFdOnlyOnLongruns(t *testing.T) {
	root := sourceTreeRoot(t)
	svcs := loadServices(t, root)

	for name, info := range svcs {
		if info.hasFD && info.svcType != "longrun" {
			t.Errorf("%s: notification-fd only makes sense on longruns, but type = %q", name, info.svcType)
		}
	}
}

func TestSourceTree_NotificationFdValueIs3(t *testing.T) {
	root := sourceTreeRoot(t)
	svcs := loadServices(t, root)

	for name, info := range svcs {
		if !info.hasFD {
			continue
		}
		if info.fdVal != "3" {
			t.Errorf("%s: notification-fd = %q, want 3 (s6-rc convention)", name, info.fdVal)
		}
	}
}

func TestSourceTree_NotifyoncheckHasCheckScript(t *testing.T) {
	root := sourceTreeRoot(t)
	svcs := loadServices(t, root)

	for name, info := range svcs {
		if !info.hasFD || info.svcType != "longrun" {
			continue
		}
		if name == "nix-daemon" {
			// nix-daemon writes to fd 3 natively, no s6-notifyoncheck needed
			continue
		}
		if !info.hasCheck {
			t.Errorf("%s: has notification-fd but no data/check script (needed by s6-notifyoncheck)", name)
		}
	}
}

func TestSourceTree_CheckScriptsAreExecutable(t *testing.T) {
	root := sourceTreeRoot(t)
	svcs := loadServices(t, root)

	for name, info := range svcs {
		if !info.hasCheck {
			continue
		}
		checkPath := filepath.Join(root, name, "data", "check")
		fi, err := os.Stat(checkPath)
		if err != nil {
			t.Errorf("%s: cannot stat data/check: %v", name, err)
			continue
		}
		if fi.Mode()&0111 == 0 {
			t.Errorf("%s: data/check is not executable (mode %o)", name, fi.Mode())
		}
	}
}

func TestSourceTree_RunScriptsAreExecutable(t *testing.T) {
	root := sourceTreeRoot(t)
	svcs := loadServices(t, root)

	for name, info := range svcs {
		if info.svcType != "longrun" || !info.hasRun {
			continue
		}
		for _, sub := range []string{"", "linux", "darwin"} {
			dir := filepath.Join(root, name)
			if sub != "" {
				dir = filepath.Join(dir, sub)
			}
			runPath := filepath.Join(dir, "run")
			fi, err := os.Stat(runPath)
			if err != nil {
				continue
			}
			if fi.Mode()&0111 == 0 {
				t.Errorf("%s/%srun: not executable (mode %o)", name, sub+"/", fi.Mode())
			}
		}
	}
}

func TestSourceTree_UpScriptsAreExecutable(t *testing.T) {
	root := sourceTreeRoot(t)
	svcs := loadServices(t, root)

	for name, info := range svcs {
		if info.svcType != "oneshot" || !info.hasUp {
			continue
		}
		for _, sub := range []string{"", "linux", "darwin"} {
			dir := filepath.Join(root, name)
			if sub != "" {
				dir = filepath.Join(dir, sub)
			}
			upPath := filepath.Join(dir, "up")
			fi, err := os.Stat(upPath)
			if err != nil {
				continue
			}
			if fi.Mode()&0111 == 0 {
				t.Errorf("%s/%sup: not executable (mode %o)", name, sub+"/", fi.Mode())
			}
		}
	}
}

func TestSourceTree_LongrunsWithNotifyoncheckInRunScript(t *testing.T) {
	root := sourceTreeRoot(t)
	svcs := loadServices(t, root)

	for name, info := range svcs {
		if !info.hasFD || info.svcType != "longrun" || name == "nix-daemon" {
			continue
		}
		// Check that the run script mentions s6-notifyoncheck
		found := false
		for _, sub := range []string{"", "linux", "darwin"} {
			dir := filepath.Join(root, name)
			if sub != "" {
				dir = filepath.Join(dir, sub)
			}
			runPath := filepath.Join(dir, "run")
			b, err := os.ReadFile(runPath)
			if err != nil {
				continue
			}
			if strings.Contains(string(b), "s6-notifyoncheck") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s: has notification-fd but run script does not use s6-notifyoncheck", name)
		}
	}
}

func TestSourceTree_LongrunsHaveLogger(t *testing.T) {
	root := sourceTreeRoot(t)
	svcs := loadServices(t, root)

	for name, info := range svcs {
		if info.svcType != "longrun" {
			continue
		}
		if !info.hasLog {
			t.Errorf("%s: longrun missing log/ directory (s6-rc-compile creates the pipeline from it)", name)
			continue
		}
		if !info.hasLogRun {
			t.Errorf("%s: log/ directory exists but has no run script", name)
		}
	}
}

func TestSourceTree_LogRunScriptsAreExecutable(t *testing.T) {
	root := sourceTreeRoot(t)
	svcs := loadServices(t, root)

	for name, info := range svcs {
		if !info.hasLogRun {
			continue
		}
		logRunPath := filepath.Join(root, name, "log", "run")
		fi, err := os.Stat(logRunPath)
		if err != nil {
			t.Errorf("%s: cannot stat log/run: %v", name, err)
			continue
		}
		if fi.Mode()&0111 == 0 {
			t.Errorf("%s: log/run is not executable (mode %o)", name, fi.Mode())
		}
	}
}

// bootPanelServices maps s6 service names that carry boot-panel metadata
// (data/boot-group + data/boot-step) to their expected group and step.
// These services emit devcell-event status directly, no -notify bridge.
var bootPanelServices = map[string]struct {
	group string
	step  string
}{
	"nix-daemon":      {"Boot", "Nix daemon"},
	"homedir":         {"Environment", "Home"},
	"shell-rc":        {"Environment", "Shell"},
	"mise":            {"Environment", "Mise"},
	"secrets":         {"Boot", "Secrets sync"},
	"claude-config":   {"Environment", "Agent config"},
	"codex-config":    {"Environment", "Agent config"},
	"gemini-config":   {"Environment", "Agent config"},
	"opencode-config": {"Environment", "Agent config"},
	"mcp-toggle":      {"Environment", "MCP"},
	"gui-config":      {"Environment", "GUI"},
	"postgres":        {"Boot", "Postgres"},
	"xvfb":            {"Services", "Display"},
	"window-manager":  {"Services", "Desktop"},
	"xrdp":            {"Services", "Remote access"},
	"pulseaudio":      {"Services", "Audio"},
	"dbus-session":    {"Services", "D-Bus"},
}

func TestSourceTree_NoNotifyBridges(t *testing.T) {
	root := sourceTreeRoot(t)
	svcs := loadServices(t, root)

	for name := range svcs {
		if strings.HasSuffix(name, "-notify") {
			t.Errorf("-notify bridge %q still exists (should have been deleted)", name)
		}
	}
}

func TestSourceTree_BootPanelServicesHaveMetadata(t *testing.T) {
	root := sourceTreeRoot(t)

	for name, expect := range bootPanelServices {
		groupPath := filepath.Join(root, name, "data", "boot-group")
		stepPath := filepath.Join(root, name, "data", "boot-step")

		groupB, err := os.ReadFile(groupPath)
		if err != nil {
			t.Errorf("%s: missing data/boot-group: %v", name, err)
			continue
		}
		stepB, err := os.ReadFile(stepPath)
		if err != nil {
			t.Errorf("%s: missing data/boot-step: %v", name, err)
			continue
		}

		gotGroup := strings.TrimSpace(string(groupB))
		gotStep := strings.TrimSpace(string(stepB))
		if gotGroup != expect.group {
			t.Errorf("%s: data/boot-group = %q, want %q", name, gotGroup, expect.group)
		}
		if gotStep != expect.step {
			t.Errorf("%s: data/boot-step = %q, want %q", name, gotStep, expect.step)
		}
	}
}

func TestSourceTree_BootPanelServicesEmitDevcellEvent(t *testing.T) {
	root := sourceTreeRoot(t)

	for name := range bootPanelServices {
		found := false
		for _, sub := range []string{"", "linux", "darwin"} {
			dir := filepath.Join(root, name)
			if sub != "" {
				dir = filepath.Join(dir, sub)
			}
			for _, script := range []string{"up", "run"} {
				b, err := os.ReadFile(filepath.Join(dir, script))
				if err != nil {
					continue
				}
				if strings.Contains(string(b), "devcell-event") {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			t.Errorf("%s: service script must call devcell-event for status events", name)
		}
	}
}

func TestSourceTree_NoDependencyOnSelf(t *testing.T) {
	root := sourceTreeRoot(t)
	svcs := loadServices(t, root)

	for name, info := range svcs {
		for _, dep := range info.deps {
			if dep == name {
				t.Errorf("%s: depends on itself", name)
			}
		}
		for _, member := range info.content {
			if member == name {
				t.Errorf("%s: bundle contains itself", name)
			}
		}
	}
}
