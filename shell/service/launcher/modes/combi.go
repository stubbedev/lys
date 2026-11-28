package modes

import (
	"context"
	"strings"

	"github.com/stubbedev/lys/shell/service/launcher"
)

// Combi merges several modes into one list, !bang filterable.
type Combi struct {
	children      []launcher.Mode
	displayFormat string
	// states are each child's last state, which a query that rebuilds
	// one child's list merges with the rest.
	states []launcher.ModeState
	// owners and locals map a merged row to its child and child row.
	owners []int
	locals []uint32
}

// NewCombi builds over child modes (rofi -combi-modes) with a row
// template over {mode} and {text}.
func NewCombi(children []launcher.Mode, displayFormat string) *Combi {
	return &Combi{children: children, displayFormat: displayFormat}
}

// Name is "combi".
func (*Combi) Name() string { return "combi" }

func (c *Combi) merge() launcher.ModeState {
	var items []launcher.Item
	c.owners, c.locals = nil, nil
	for ci, state := range c.states {
		mode := c.children[ci].Name()
		for li, it := range state.Items {
			if c.displayFormat != "{text}" {
				it.Display = launcher.Render(c.displayFormat, launcher.Values(combiVars(mode, it.Display)))
			}
			if state.MarkupRows {
				it.Flags |= launcher.FlagMarkup
			}
			items = append(items, it)
			c.owners = append(c.owners, ci)
			c.locals = append(c.locals, uint32(li))
		}
	}
	return launcher.ModeState{Items: items, Prompt: "combi"}
}

func combiVars(mode, text string) map[string]string {
	return map[string]string{"mode": mode, "text": text}
}

func (c *Combi) locate(index uint32) (int, uint32, bool) {
	if int(index) >= len(c.owners) {
		return 0, 0, false
	}
	return c.owners[index], c.locals[index], true
}

// reloadAll re-merges every child after one changed.
func (c *Combi) reloadAll(ctx context.Context) launcher.ModeState {
	c.states = make([]launcher.ModeState, len(c.children))
	for i, child := range c.children {
		c.states[i] = child.Load(ctx)
	}
	return c.merge()
}

// Load loads and merges every child.
func (c *Combi) Load(ctx context.Context) launcher.ModeState { return c.reloadAll(ctx) }

// Query asks every child that answers queries; the list is merged again
// when one of them changed.
func (c *Combi) Query(ctx context.Context, query string) (launcher.ModeState, bool) {
	changed := false
	for i, child := range c.children {
		q, ok := child.(launcher.Querier)
		if !ok {
			continue
		}
		if state, ok := q.Query(ctx, query); ok {
			c.states[i] = state
			changed = true
		}
	}
	if !changed {
		return launcher.ModeState{}, false
	}
	return c.merge(), true
}

// Activate routes a row to its owner; custom input goes to the first
// child that accepts it.
func (c *Combi) Activate(ctx context.Context, target launcher.Target, kind launcher.ActivateKind, input string) launcher.Action {
	if row, ok := target.Row(); ok {
		if child, local, ok := c.locate(row); ok {
			return c.forward(ctx, c.children[child].Activate(ctx, launcher.RowTarget(local), kind, input))
		}
	}
	for _, child := range c.children {
		if launcher.AllowsCustom(child) {
			return c.forward(ctx, child.Activate(ctx, launcher.NoRow, kind, input))
		}
	}
	return launcher.ActionNothing{}
}

// Delete routes shift-delete to the row's owner.
func (c *Combi) Delete(ctx context.Context, index uint32) launcher.Action {
	child, local, ok := c.locate(index)
	if !ok {
		return launcher.ActionNothing{}
	}
	return c.forward(ctx, launcher.Delete(ctx, c.children[child], local))
}

// Restricts reports whether a child's name starts with the bang.
func (c *Combi) Restricts(bang string) bool {
	if bang == "" {
		return false
	}
	for _, child := range c.children {
		if strings.HasPrefix(child.Name(), bang) {
			return true
		}
	}
	return false
}

// Subset masks the rows of children whose name starts with the bang.
func (c *Combi) Subset(bang string) []bool {
	return c.rowsOf(func(child launcher.Mode) bool { return strings.HasPrefix(child.Name(), bang) })
}

// Demoted marks the rows of the children that trail the query.
func (c *Combi) Demoted(query string) []bool {
	return c.rowsOf(func(child launcher.Mode) bool {
		t, ok := child.(launcher.Trailer)
		return ok && t.Trails(query)
	})
}

// rowsOf marks the rows of the children pick picks; nil when it picks
// none.
func (c *Combi) rowsOf(pick func(launcher.Mode) bool) []bool {
	picked := make([]bool, len(c.children))
	any := false
	for i, child := range c.children {
		picked[i] = pick(child)
		any = any || picked[i]
	}
	if !any {
		return nil
	}
	mask := make([]bool, len(c.owners))
	for i, owner := range c.owners {
		mask[i] = picked[owner]
	}
	return mask
}

// forward turns a child's reload into a re-merge of the whole list,
// keeping the message the child reloaded with (a failure).
func (c *Combi) forward(ctx context.Context, a launcher.Action) launcher.Action {
	if r, reload := a.(launcher.ActionReload); reload {
		state := c.reloadAll(ctx)
		state.Message = r.State.Message
		return launcher.ActionReload{State: state}
	}
	return a
}
