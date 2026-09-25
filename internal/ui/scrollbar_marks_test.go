package ui

import (
	"fmt"
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
	halves := s.markHalves()
	marked := 0
	for i, st := range halves {
		if st != term.StyleDefault {
			marked++
			if i != 10 {
				t.Fatalf("mark landed on half %d, want 10", i)
			}
		}
	}
	if marked != 1 {
		t.Fatalf("marked halves = %d, want 1", marked)
	}
}

func TestScrollbarMarkHigherRankWinsSharedHalf(t *testing.T) {
	s := Scrollbar{Height: 2, TotalItems: 400, Marks: []ScrollMark{
		{Item: 0, Count: 1, Style: term.StyleScrollMarkDeleted, Rank: 3},
		{Item: 1, Count: 1, Style: term.StyleScrollMarkAdded, Rank: 1},
		{Item: 2, Count: 1, Style: term.StyleScrollMarkModified, Rank: 2},
	}}
	if got := s.markHalves()[0]; got != term.StyleScrollMarkDeleted {
		t.Fatalf("shared half = %v, want deleted", got)
	}
}

func TestScrollbarMarkCellGlyphs(t *testing.T) {
	base := term.StyleScrollbar
	add, mod := term.StyleScrollMarkAdded, term.StyleScrollMarkModified
	tests := []struct {
		name        string
		top, bottom term.Style
		want        term.Cell
	}{
		{"none", 0, 0, term.Cell{Ch: '█', Style: base}},
		{"both same", add, add, term.Cell{Ch: '█', Style: add}},
		{"top only", add, 0, term.Cell{Ch: '▄', Style: base, BgStyle: add}},
		{"bottom only", 0, add, term.Cell{Ch: '▀', Style: base, BgStyle: add}},
		{"both differ", add, mod, term.Cell{Ch: '▀', Style: add, BgStyle: mod}},
	}
	for _, tt := range tests {
		if got := markCell(base, tt.top, tt.bottom); got != tt.want {
			t.Errorf("%s: got %+v, want %+v", tt.name, got, tt.want)
		}
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

	// TotalItems = 200+10-1 = 209, 20 halves: line 150 -> half 14 (top of row 7).
	got := grid[7][39]
	if got.Ch != '▄' || got.BgStyle != term.StyleScrollMarkModified {
		t.Fatalf("scrollbar row 7 = %+v, want modified mark on top half", got)
	}
	for y := range 10 {
		if y != 7 && grid[y][39].BgStyle != term.StyleDefault {
			t.Fatalf("unexpected mark on row %d: %+v", y, grid[y][39])
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
