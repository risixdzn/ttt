package ui

import (
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/term"
)

// gitScrollMarks must count items exactly as the scrollbar's TotalItems does
// (visual rows under wrap, visible lines under folds), or marks drift from the
// thumb.
func (e *EditorPaneWidget) gitScrollMarks(foldsActive bool, editorW, tabW int) []ScrollMark {
	changes := e.LineChanges
	if len(changes) == 0 {
		return nil
	}
	marks := e.scrollMarks[:0]
	add := func(item, count int, kind diff.LineChangeKind) {
		style, rank := scrollMarkStyle(kind)
		if n := len(marks); n > 0 && marks[n-1].Style == style && marks[n-1].Item+marks[n-1].Count == item {
			marks[n-1].Count += count
			return
		}
		marks = append(marks, ScrollMark{Item: item, Count: count, Style: style, Rank: rank})
	}

	switch {
	case e.WordWrap:
		row := 0
		for i, line := range e.Buf.Lines {
			rows := wrapLineVisualRows(line, editorW, tabW)
			if i < len(changes) && changes[i] != diff.LineUnchanged {
				add(row, rows, changes[i])
			}
			row += rows
		}
	case foldsActive:
		// A collapsed fold's hidden lines are attributed to its header line,
		// so changes inside a fold still show.
		visible := e.cachedVisibleLines
		for v, start := range visible {
			end := len(changes)
			if v+1 < len(visible) {
				end = min(visible[v+1], end)
			}
			strongest := diff.LineUnchanged
			for i := start; i < end; i++ {
				if changes[i] != diff.LineUnchanged && scrollMarkRank(changes[i]) > scrollMarkRank(strongest) {
					strongest = changes[i]
				}
			}
			if strongest != diff.LineUnchanged {
				add(v, 1, strongest)
			}
		}
	default:
		for i, kind := range changes {
			if kind != diff.LineUnchanged {
				add(i, 1, kind)
			}
		}
	}
	e.scrollMarks = marks
	return marks
}

func scrollMarkStyle(kind diff.LineChangeKind) (term.Style, int) {
	switch kind {
	case diff.LineAdded:
		return term.StyleScrollMarkAdded, scrollMarkRank(kind)
	case diff.LineModified:
		return term.StyleScrollMarkModified, scrollMarkRank(kind)
	default:
		return term.StyleScrollMarkDeleted, scrollMarkRank(kind)
	}
}

func scrollMarkRank(kind diff.LineChangeKind) int {
	switch kind {
	case diff.LineAdded:
		return 1
	case diff.LineModified:
		return 2
	case diff.LineDeleted:
		return 3
	}
	return 0
}
