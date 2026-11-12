package docker

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

// `cell build df` nix section — the standalone "what's on the nix volume"
// analysis surface: counts, root NAMES, stale markers, per-root metadata.
// Same safety contract as the CELL-390 startup probe (which only shows
// counts): read-only, runs in a volume-mounted container, never invokes
// nix, never evaluates auto/ roots.

// nixRootsListScript emits one line per devcell root and one per meta:
//
//	root name=<file> stale=0|1
//	meta hash=<h> cell=<c> stack=<s> nixpkgs=<rev> size=<bytes>
//
// size is the closure size (sum of nix-store --query --size over the
// transitive closure). Zero when the profile link is stale or absent.
// nix-store queries are read-only (no GC root mutation, unlike --print-dead).
const nixRootsListScript = `for l in /nix/var/nix/gcroots/devcell/*-profile /nix/var/nix/gcroots/devcell/*-generation; do
  [ -L "$l" ] || continue
  if [ -e "$l" ]; then s=0; else s=1; fi
  echo "root name=$(basename "$l") stale=$s"
done
for m in /nix/var/nix/gcroots/devcell/*-meta; do
  [ -f "$m" ] || continue
  h=$(basename "$m" | cut -d- -f1)
  c=$(grep '^cell=' "$m" 2>/dev/null | cut -d= -f2)
  [ -z "$c" ] && c=$(grep '^project=' "$m" 2>/dev/null | cut -d= -f2)
  sz=0
  p="/nix/var/nix/gcroots/devcell/${h}-profile"
  if [ -L "$p" ] && [ -e "$p" ]; then
    t=$(readlink -f "$p")
    for _s in $(nix-store -qR "$t" 2>/dev/null | xargs nix-store --query --size 2>/dev/null); do
      sz=$((sz + _s))
    done
  fi
  echo "meta hash=$h cell=$c stack=$(grep '^stack=' "$m" 2>/dev/null | cut -d= -f2) nixpkgs=$(grep '^nixpkgs=' "$m" 2>/dev/null | cut -d= -f2) size=$sz"
done
_vb=$(du -sb /nix 2>/dev/null | cut -f1)
echo "volume_bytes=${_vb:-0}"
du -sb /nix/store/* 2>/dev/null | sort -rn | head -10 | while read -r _sb _sp; do
  echo "top bytes=$_sb name=$(basename "$_sp")"
done`

// NixDFReportScript is the health probe plus the root/meta listing —
// still purely read-only.
const NixDFReportScript = NixHealthProbeScript + "\n" + nixRootsListScript

// NixRootEntry is one GC root symlink on the volume.
type NixRootEntry struct {
	Name  string
	Stale bool
}

// NixRootMeta is the parsed -meta file for one root hash.
type NixRootMeta struct {
	Hash         string
	Cell         string // comma-separated cell names sharing this hash
	Stack        string
	Nixpkgs      string
	ClosureBytes int64 // sum of nix-store --query --size over the closure; 0 if stale/unknown
}

// NixStorePath is one of the top store paths by size.
type NixStorePath struct {
	Name  string
	Bytes int64
}

// NixStoreReport is the full parsed df report for the nix volume.
type NixStoreReport struct {
	Volume      string
	VolumeBytes int64 // du -sb /nix; 0 if unavailable
	Health      NixStoreHealth
	Roots       []NixRootEntry
	Metas       []NixRootMeta
	TopPaths    []NixStorePath
}

// ParseNixStoreReport parses combined probe + listing output.
func ParseNixStoreReport(volume, out string) (NixStoreReport, error) {
	health, err := ParseNixStoreHealth(out)
	if err != nil {
		return NixStoreReport{}, err
	}
	r := NixStoreReport{Volume: volume, Health: health}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "volume_bytes=") {
			r.VolumeBytes, _ = strconv.ParseInt(strings.TrimPrefix(line, "volume_bytes="), 10, 64)
			continue
		}
		kind, rest, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		kv := map[string]string{}
		for _, field := range strings.Fields(rest) {
			if k, v, ok := strings.Cut(field, "="); ok {
				kv[k] = v
			}
		}
		switch kind {
		case "root":
			if kv["name"] != "" {
				r.Roots = append(r.Roots, NixRootEntry{Name: kv["name"], Stale: kv["stale"] == "1"})
			}
		case "meta":
			if kv["hash"] != "" {
				cell := kv["cell"]
				if cell == "" {
					cell = kv["project"] // backwards compat with pre-cell metas
				}
				closureBytes, _ := strconv.ParseInt(kv["size"], 10, 64)
				r.Metas = append(r.Metas, NixRootMeta{
					Hash:         kv["hash"],
					Cell:         cell,
					Stack:        kv["stack"],
					Nixpkgs:      kv["nixpkgs"],
					ClosureBytes: closureBytes,
				})
			}
		case "top":
			if kv["name"] != "" {
				b, _ := strconv.ParseInt(kv["bytes"], 10, 64)
				r.TopPaths = append(r.TopPaths, NixStorePath{Name: kv["name"], Bytes: b})
			}
		}
	}
	return r, nil
}

// CollectNixStore runs the report against the thin store volume. Any
// failure returns ok=false — the df section is simply omitted, matching
// the CELL-390 degrade-to-silence contract.
func CollectNixStore(ctx context.Context) (NixStoreReport, bool) {
	volume := ThinStoreVolume()
	out, err := exec.CommandContext(ctx, "docker", "run", "--rm",
		"-v", volume+":/nix", NixCoreImage, "/bin/sh", "-c", NixDFReportScript,
	).CombinedOutput()
	if err != nil {
		return NixStoreReport{}, false
	}
	r, perr := ParseNixStoreReport(volume, string(out))
	if perr != nil {
		return NixStoreReport{}, false
	}
	return r, true
}

// FormatNixStoreSection renders the nix block of `cell build df`. An
// empty report (no volume data) renders nothing.
func FormatNixStoreSection(r NixStoreReport, w io.Writer) {
	if r.Volume == "" && r.Health.TotalRoots == 0 && len(r.Roots) == 0 {
		return
	}
	metaByHash := make(map[string]NixRootMeta, len(r.Metas))
	for _, m := range r.Metas {
		metaByHash[m.Hash] = m
	}

	fmt.Fprintf(w, "\nNix store (%s):\n", r.Volume)
	fmt.Fprintf(w, "  %s, %s, %s (%d orphaned — reclaimable)\n",
		plural(r.Health.TotalRoots, "root"),
		plural(r.Health.ProfileHashes, "profile hash"),
		plural(r.Health.Generations, "generation"),
		r.Health.OrphanedGenerations,
	)
	if r.Health.DistinctRevs > 1 {
		fmt.Fprintf(w, "  drift: %d nixpkgs revs live (newest %s on %s)\n",
			r.Health.DistinctRevs, shortRev(r.Health.NewestRev),
			plural(r.Health.NewestProjects, "project"))
	}
	for _, root := range r.Roots {
		line := "  " + root.Name
		hash, _, _ := strings.Cut(root.Name, "-")
		if m, ok := metaByHash[hash]; ok && strings.HasSuffix(root.Name, "-profile") {
			parts := []string{}
			if m.Cell != "" {
				parts = append(parts, "cell="+m.Cell)
			}
			if m.Stack != "" {
				parts = append(parts, "stack="+m.Stack)
			}
			if m.Nixpkgs != "" {
				parts = append(parts, "nixpkgs="+shortRev(m.Nixpkgs))
			}
			if len(parts) > 0 {
				line += "  " + strings.Join(parts, " ")
			}
		}
		if root.Stale {
			line += "  (stale)"
		}
		fmt.Fprintln(w, line)
	}
	fmt.Fprintln(w, "  To reclaim: cell build prune")
}
