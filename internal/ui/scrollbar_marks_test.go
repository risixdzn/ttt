package ui

import (
	"fmt"
	"slices"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/core/cursor"
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/core/fold"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/view"
)

func TestScrollbarMarkSingleItemInHugeRangeStaysVisible(t *testing.T) {
	s := Scrollbar{Height: 10, TotalItems: 100000, Marks: []ScrollMark{{Item: 50000, Count: 1, Style: term.StyleScrollMarkAdded, Rank: 1}}}
	var marked []int
	for i, st := range s.markSlots() {
		if st != term.StyleDefault {
			marked = append(marked, i)
		}
	}
	if fmt.Sprint(marked) != "[40]" {
		t.Fatalf("marked slots = %v, want [40]", marked)
	}
}

func TestScrollbarMarkHigherRankWinsSharedSlot(t *testing.T) {
	s := Scrollbar{Height: 2, TotalItems: 400, Marks: []ScrollMark{
		{Item: 0, Count: 1, Style: term.StyleScrollMarkDeleted, Rank: 3},
		{Item: 1, Count: 1, Style: term.StyleScrollMarkAdded, Rank: 1},
		{Item: 2, Count: 1, Style: term.StyleScrollMarkModified, Rank: 2},
	}}
	if got := s.markSlots()[0]; got != term.StyleScrollMarkDeleted {
		t.Fatalf("shared slot = %v, want deleted", got)
	}
}

func drawnSlots(t *testing.T, c term.Cell) [cellSlots]term.Style {
	t.Helper()
	fill := map[term.Style]term.Style{
		term.StyleScrollbarFill:      term.StyleScrollbar,
		term.StyleScrollbarThumbFill: term.StyleScrollbarThumb,
	}
	var out [cellSlots]term.Style
	if c.Ch == '█' {
		for i := range out {
			out[i] = c.Style
		}
		return out
	}
	for _, g := range legacyGlyphs {
		if g.ch != c.Ch {
			continue
		}
		bg := c.BgStyle
		if f, ok := fill[bg]; ok {
			bg = f
		}
		for i := range out {
			out[i] = bg
			if g.mask&(1<<i) != 0 {
				out[i] = c.Style
			}
		}
		return out
	}
	t.Fatalf("unknown track glyph %q", c.Ch)
	return out
}

func slotPattern(marks map[int]term.Style) []term.Style {
	slots := make([]term.Style, cellSlots)
	for i, st := range marks {
		slots[i] = st
	}
	return slots
}

func TestScrollbarMarkCellReproducesPattern(t *testing.T) {
	base := term.StyleScrollbar
	add, mod, del := term.StyleScrollMarkAdded, term.StyleScrollMarkModified, term.StyleScrollMarkDeleted
	full := func(st term.Style) map[int]term.Style {
		m := map[int]term.Style{}
		for i := range cellSlots {
			m[i] = st
		}
		return m
	}
	tests := []struct {
		name   string
		glyphs []scrollGlyph
		marks  map[int]term.Style
	}{
		{"no marks", blockGlyphs, nil},
		{"full cell", blockGlyphs, full(add)},
		{"top half", blockGlyphs, map[int]term.Style{0: mod, 1: mod, 2: mod, 3: mod}},
		{"bottom half", blockGlyphs, map[int]term.Style{4: del, 5: del, 6: del, 7: del}},
		{"top eighth", blockGlyphs, map[int]term.Style{0: add}},
		{"bottom eighth", blockGlyphs, map[int]term.Style{7: add}},
		{"bottom quarter", blockGlyphs, map[int]term.Style{6: mod, 7: mod}},
		{"two colors split", blockGlyphs, map[int]term.Style{0: add, 1: add, 2: add, 3: add, 4: mod, 5: mod, 6: mod, 7: mod}},
		{"thin bar row 3", legacyGlyphs, map[int]term.Style{3: add}},
		{"thin bar row 6", legacyGlyphs, map[int]term.Style{6: del}},
		{"upper quarter", legacyGlyphs, map[int]term.Style{0: mod, 1: mod}},
	}
	for _, tt := range tests {
		want := slotPattern(tt.marks)
		for i := range want {
			if want[i] == term.StyleDefault {
				want[i] = base
			}
		}
		got := drawnSlots(t, markCell(base, slotPattern(tt.marks), tt.glyphs))
		if fmt.Sprint(got[:]) != fmt.Sprint(want) {
			t.Errorf("%s: drew %v, want %v", tt.name, got, want)
		}
	}
}

func TestScrollbarMarkCellNeverHidesAMark(t *testing.T) {
	base := term.StyleScrollbarThumb
	add, del := term.StyleScrollMarkAdded, term.StyleScrollMarkDeleted
	for _, glyphs := range [][]scrollGlyph{blockGlyphs, legacyGlyphs} {
		for a := range cellSlots {
			for d := range cellSlots {
				if a == d {
					continue
				}
				slots := slotPattern(map[int]term.Style{a: add, d: del})
				got := drawnSlots(t, markCell(base, slots, glyphs))
				if !slices.Contains(got[:], add) || !slices.Contains(got[:], del) {
					t.Fatalf("added at %d, deleted at %d: drew %v, a mark is hidden", a, d, got)
				}
			}
		}
	}
}

func TestScrollbarMarkCellBlocksFallBackToNearestHalf(t *testing.T) {
	base := term.StyleScrollbar
	add := term.StyleScrollMarkAdded
	got := drawnSlots(t, markCell(base, slotPattern(map[int]term.Style{2: add}), blockGlyphs))
	if got[2] != add || got[6] != base {
		t.Fatalf("mid-top mark without legacy glyphs drew %v, want it in the top half", got)
	}
}

func newMarksTestEditor(n int) *EditorPaneWidget {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i)
	}
	buf := &buffer.Buffer{Lines: lines}
	return NewEditorPaneWidget(buf, &cursor.Cursor{}, &view.Viewport{Width: 40, Height: 10})
}

func TestEditorRendersGitChangeOnScrollbar(t *testing.T) {
	e := newMarksTestEditor(200)
	changes := make([]diff.LineChangeKind, 200)
	changes[150] = diff.LineModified
	e.LineChanges = changes
	e.SetRect(Rect{W: 40, H: 10})

	grid := makeGrid(40, 10)
	e.Render(NewRenderSurface(grid, Rect{W: 40, H: 10}))

	// TotalItems = 200+10-1 = 209, 80 slots: line 150 -> slot 57, row 7.
	for y := range 10 {
		slots := drawnSlots(t, grid[y][39])
		if marked := slices.Contains(slots[:], term.StyleScrollMarkModified); marked != (y == 7) {
			t.Fatalf("scrollbar row %d = %+v, want the modified mark only on row 7", y, grid[y][39])
		}
	}
}

func TestGitScrollMarksMergeRuns(t *testing.T) {
	e := newMarksTestEditor(10)
	e.LineChanges = []diff.LineChangeKind{0, 1, 1, 1, 2, 0, 1, 0, 0, 3}
	got := e.gitScrollMarks(false, 40, 4)
	want := []ScrollMark{
		{Item: 1, Count: 3, Style: term.StyleScrollMarkAdded, Rank: 1},
		{Item: 4, Count: 1, Style: term.StyleScrollMarkModified, Rank: 2},
		{Item: 6, Count: 1, Style: term.StyleScrollMarkAdded, Rank: 1},
		{Item: 9, Count: 1, Style: term.StyleScrollMarkDeleted, Rank: 3},
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("marks = %v, want %v", got, want)
	}
}

func TestGitScrollMarksAttributeFoldedChangesToHeader(t *testing.T) {
	e := newMarksTestEditor(10)
	e.Folds = fold.NewState()
	e.Folds.SetRanges([]fold.Range{{StartLine: 2, EndLine: 6}})
	e.Folds.Toggle(2)
	e.cachedVisibleLines = e.Folds.VisibleLines(10)
	changes := make([]diff.LineChangeKind, 10)
	changes[4] = diff.LineAdded
	changes[5] = diff.LineDeleted
	changes[8] = diff.LineModified
	e.LineChanges = changes

	got := e.gitScrollMarks(true, 40, 4)
	want := []ScrollMark{
		{Item: 2, Count: 1, Style: term.StyleScrollMarkDeleted, Rank: 3},
		{Item: 4, Count: 1, Style: term.StyleScrollMarkModified, Rank: 2},
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("marks = %v, want %v (visible %v)", got, want, e.cachedVisibleLines)
	}
}

func TestGitScrollMarksUseVisualRowsUnderWordWrap(t *testing.T) {
	e := newMarksTestEditor(3)
	e.WordWrap = true
	e.Buf.Lines = []string{"aaaa bbbb cccc dddd", "short", "x"}
	e.LineChanges = []diff.LineChangeKind{diff.LineModified, 0, diff.LineAdded}

	got := e.gitScrollMarks(false, 5, 4)
	first := wrapLineVisualRows(e.Buf.Lines[0], 5, 4)
	want := []ScrollMark{
		{Item: 0, Count: first, Style: term.StyleScrollMarkModified, Rank: 2},
		{Item: first + 1, Count: 1, Style: term.StyleScrollMarkAdded, Rank: 1},
	}
	if first < 2 || fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("marks = %v, want %v", got, want)
	}
}
