package cfg

import (
	"fmt"
	"sort"
	"strings"
)

// ConfigDrift describes what changed between two configs. Returns a
// human-readable summary, or "" if nothing drift-relevant changed.
func ConfigDrift(old, new CellConfig) string {
	var diffs []string

	if old.Cell.ResolvedStack() != new.Cell.ResolvedStack() {
		diffs = append(diffs, fmt.Sprintf("stack: %s → %s", old.Cell.ResolvedStack(), new.Cell.ResolvedStack()))
	}

	oldMods := sortedCopy(old.Cell.Modules)
	newMods := sortedCopy(new.Cell.Modules)
	if strings.Join(oldMods, ",") != strings.Join(newMods, ",") {
		diffs = append(diffs, fmt.Sprintf("modules: [%s] → [%s]", strings.Join(oldMods, ","), strings.Join(newMods, ",")))
	}

	oldVols := resolvedVolumes(old.Volumes)
	newVols := resolvedVolumes(new.Volumes)
	added, removed := diffLists(oldVols, newVols)
	if len(added) > 0 {
		diffs = append(diffs, fmt.Sprintf("+%d volumes", len(added)))
	}
	if len(removed) > 0 {
		diffs = append(diffs, fmt.Sprintf("-%d volumes", len(removed)))
	}

	addedEnv, removedEnv := diffMapKeys(old.Env, new.Env)
	changedEnv := diffMapValues(old.Env, new.Env)
	envChanges := len(addedEnv) + len(removedEnv) + len(changedEnv)
	if envChanges > 0 {
		diffs = append(diffs, fmt.Sprintf("%d env var(s)", envChanges))
	}

	addedPorts, removedPorts := diffLists(old.Ports.Forward, new.Ports.Forward)
	if len(addedPorts)+len(removedPorts) > 0 {
		diffs = append(diffs, fmt.Sprintf("ports changed"))
	}

	if old.Docker.Privileged != new.Docker.Privileged ||
		old.Docker.MemLimit != new.Docker.MemLimit ||
		old.Docker.CPULimit != new.Docker.CPULimit ||
		old.Docker.ShmSize != new.Docker.ShmSize ||
		strings.Join(old.Docker.CapAdd, ",") != strings.Join(new.Docker.CapAdd, ",") {
		diffs = append(diffs, "docker settings")
	}

	if old.GUI.ResolvedEnabled() != new.GUI.ResolvedEnabled() ||
		old.GUI.WM != new.GUI.WM ||
		old.GUI.Resolution != new.GUI.Resolution ||
		old.GUI.Scale != new.GUI.Scale {
		diffs = append(diffs, "GUI settings")
	}

	if len(diffs) == 0 {
		return ""
	}
	return strings.Join(diffs, ", ")
}

func sortedCopy(s []string) []string {
	c := make([]string, len(s))
	copy(c, s)
	sort.Strings(c)
	return c
}

func resolvedVolumes(vols []VolumeMount) []string {
	r := make([]string, len(vols))
	for i, v := range vols {
		r[i] = v.Resolved()
	}
	return r
}

func diffLists(old, new []string) (added, removed []string) {
	oldSet := make(map[string]bool, len(old))
	for _, v := range old {
		oldSet[v] = true
	}
	newSet := make(map[string]bool, len(new))
	for _, v := range new {
		newSet[v] = true
	}
	for _, v := range new {
		if !oldSet[v] {
			added = append(added, v)
		}
	}
	for _, v := range old {
		if !newSet[v] {
			removed = append(removed, v)
		}
	}
	return
}

func diffMapKeys(old, new map[string]string) (added, removed []string) {
	for k := range new {
		if _, ok := old[k]; !ok {
			added = append(added, k)
		}
	}
	for k := range old {
		if _, ok := new[k]; !ok {
			removed = append(removed, k)
		}
	}
	return
}

func diffMapValues(old, new map[string]string) []string {
	var changed []string
	for k, nv := range new {
		if ov, ok := old[k]; ok && ov != nv {
			changed = append(changed, k)
		}
	}
	return changed
}
