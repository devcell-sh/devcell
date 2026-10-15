package ux_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/devcell-sh/devcell/internal/ux"
	streak "github.com/dimmkirr/go-streak-chart"
)

// CELL-657: the boot panel can be driven remotely. EventPanel records every
// mutation as a BootEvent; BootPanel.Apply replays them. The two must end in
// the same grid so a popup renders exactly what the inline panel would.

var testGroups = []ux.PanelGroup{
	{Label: "Prepare", Steps: []string{"Docker", "Network", "Image pin"}},
	{Label: "Configure", Steps: []string{"Secrets", "Git"}},
	{Label: "Boot", Steps: []string{"Container"}},
	{Label: "Services", Steps: []string{"Shell", "GUI"}},
}

// scriptedBoot exercises every BootView method the docker engine uses.
func scriptedBoot(v ux.BootView) {
	_ = v.Phase("Docker", func() error { return nil })
	_ = v.Phase("Network", func() error { return errors.New("no route — eth0 down") })
	_ = v.PhaseDetailed("Image pin", func() (string, error) { return "sha256:abc", nil })
	_ = v.PhaseDetailedRunningWarn("Loading secrets (authorize)", "Secrets", func() (string, bool, error) { return "1 failed", true, nil })
	_ = v.PhaseDetailedRunning("Looking up git", "Git", func() (string, error) { return "dmitry", nil })
	v.UpdateText("waiting for container")
	v.SetBoot("Boot", "Container", streak.Running, "Starting container")
	v.SetBoot("Boot", "Container", streak.Done, "Container running")
	v.SetBoot("Services", "Shell", streak.Running, "shell-rc activating")
	v.WarnBoot("gui-config warned")
	v.Seal("GUI")
	v.PromoteRunning()
	v.PromotePending()
	v.Message("almost there", streak.Running)
	v.Finish("Cell ready")
	v.ClearBelowGroups()
}

// gridString flattens a grid snapshot: statuses per row plus row messages.
func gridString(g *streak.Grid) string {
	var b strings.Builder
	for r := 0; r < g.Rows(); r++ {
		for c := 0; c < g.RowCols(r); c++ {
			s, _ := g.Get(r, c)
			fmt.Fprintf(&b, "%s ", s)
		}
		m := g.RowMessage(r)
		fmt.Fprintf(&b, "| %s (%s)\n", m.Text, m.Status)
	}
	return b.String()
}

func TestEventPanel_ReplayMatchesDirectPanel(t *testing.T) {
	ux.LogPlainText = true
	t.Cleanup(func() { ux.LogPlainText = false })

	direct := ux.NewBootPanel(testGroups)
	scriptedBoot(direct)

	replayed := ux.NewBootPanel(testGroups)
	ep := ux.NewEventPanel(testGroups)
	if err := ep.Bind(replayed); err != nil {
		t.Fatal(err)
	}
	scriptedBoot(ep)

	if got, want := gridString(replayed.Snapshot()), gridString(direct.Snapshot()); got != want {
		t.Errorf("replayed grid differs from direct grid\n got:\n%s\nwant:\n%s", got, want)
	}
}

// Events emitted before a sink is bound are buffered and replayed on Bind,
// so a popup that connects late shows the full history.
func TestEventPanel_LateBindReplaysBuffer(t *testing.T) {
	ux.LogPlainText = true
	t.Cleanup(func() { ux.LogPlainText = false })

	direct := ux.NewBootPanel(testGroups)
	scriptedBoot(direct)

	ep := ux.NewEventPanel(testGroups)
	scriptedBoot(ep) // nothing bound yet
	late := ux.NewBootPanel(testGroups)
	if err := ep.Bind(late); err != nil {
		t.Fatal(err)
	}
	if got, want := gridString(late.Snapshot()), gridString(direct.Snapshot()); got != want {
		t.Errorf("late-bound grid differs\n got:\n%s\nwant:\n%s", got, want)
	}
	if ev := ep.Events(); len(ev) == 0 || ev[0].Kind != ux.EventGroups {
		t.Errorf("first buffered event must carry the groups, got %+v", ev[:min(1, len(ev))])
	}
}

type recordingSink struct{ kinds []string }

func (r *recordingSink) Apply(ev ux.BootEvent) error {
	r.kinds = append(r.kinds, ev.Kind)
	return nil
}

// Exactly one sink at a time: binding a new one detaches the old one.
func TestEventPanel_SingleSink(t *testing.T) {
	ep := ux.NewEventPanel(testGroups)
	first, second := &recordingSink{}, &recordingSink{}
	_ = ep.Bind(first)
	ep.Seal("Docker")
	_ = ep.Bind(second)
	ep.Seal("Network")
	if n := len(first.kinds); n != 2 { // groups + one done
		t.Errorf("first sink got %d events after being replaced, want 2: %v", n, first.kinds)
	}
	if n := len(second.kinds); n != 3 { // groups + both done (replayed + live)
		t.Errorf("second sink got %d events, want 3: %v", n, second.kinds)
	}
}

func TestBootEvent_JSONRoundTrip(t *testing.T) {
	in := ux.BootEvent{Kind: ux.EventSet, Group: "Boot", Step: "Container", Status: streak.Warning.String(), Text: "slow"}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out ux.BootEvent
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(out, in) {
		t.Errorf("round trip changed the event: %+v -> %+v", in, out)
	}
	if s, ok := ux.ParseStatus("warning"); !ok || s != streak.Warning {
		t.Errorf("ParseStatus(warning) = %v, %v", s, ok)
	}
	if _, ok := ux.ParseStatus("bogus"); ok {
		t.Error("ParseStatus must reject unknown statuses")
	}
}

// Replaying an unknown kind is an error, not a silent skip.
func TestBootPanel_ApplyRejectsUnknownKind(t *testing.T) {
	ux.LogPlainText = true
	t.Cleanup(func() { ux.LogPlainText = false })
	p := ux.NewBootPanel(testGroups)
	if err := p.Apply(ux.BootEvent{Kind: "teleport"}); err == nil {
		t.Error("Apply should reject an unknown event kind")
	}
}
