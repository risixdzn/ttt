package ui

import (
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/term"
)

var gitMarkStyle = map[diff.LineChangeKind]term.Style{
	diff.LineAdded:    term.StyleScrollMarkAdded,
	diff.LineModified: term.StyleScrollMarkModified,
	diff.LineDeleted:  term.StyleScrollMarkDeleted,
}

// gitMarkRank decides which change shows where several share a spot.
var gitMarkRank = map[diff.LineChangeKind]int{
	diff.LineAdded:    1,
	diff.LineModified: 2,
	diff.LineDeleted:  3,
}

// gitScrollMarks must count items exactly as the scrollbar's TotalItems does
// (visual rows under wrap, visible lines under folds), or marks drift from the
// thumb.
func (e *EditorPaneWidget) gitScrollMarks(foldsActive bool, editorW, tabW int) []ScrollMark {
	changes := e.LineChanges
	if len(changes) == 0 {
		return nil
	}

	var marks []ScrollMark
	add := func(item, count int, kind diff.LineChangeKind) {
		style := gitMarkStyle[kind]
		if len(marks) > 0 {
			last := &marks[len(marks)-1]
			if last.Style == style && last.Item+last.Count == item {
				last.Count += count
				return
			}
		}
		marks = append(marks, ScrollMark{Item: item, Count: count, Style: style, Rank: gitMarkRank[kind]})
	}

	switch {
	case e.WordWrap:
		row := 0
		for lineIdx, line := range e.Buf.Lines {
			rows := wrapLineVisualRows(line, editorW, tabW)
			if lineIdx < len(changes) && changes[lineIdx] != diff.LineUnchanged {
				add(row, rows, changes[lineIdx])
			}
			row += rows
		}
	case foldsActive:
		// Lines hidden in a collapsed fold count toward its header line, so
		// changes inside a fold still show.
		visible := e.cachedVisibleLines
		for visibleIdx, start := range visible {
			if start >= len(changes) {
				break // changes lag behind the buffer until the next git diff
			}
			end := len(changes)
			if visibleIdx+1 < len(visible) {
				end = min(visible[visibleIdx+1], end)
			}
			if kind := strongestChange(changes[start:end]); kind != diff.LineUnchanged {
				add(visibleIdx, 1, kind)
			}
		}
	default:
		for lineIdx, kind := range changes {
			if kind != diff.LineUnchanged {
				add(lineIdx, 1, kind)
			}
		}
	}
	return marks
}

func strongestChange(changes []diff.LineChangeKind) diff.LineChangeKind {
	strongest := diff.LineUnchanged
	for _, kind := range changes {
		if gitMarkRank[kind] > gitMarkRank[strongest] {
			strongest = kind
		}
	}
	return strongest
}
