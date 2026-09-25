package ui

import (
	"github.com/eugenioenko/ttt/internal/term"

	"github.com/gdamore/tcell/v3"
)

type Scrollbar struct {
	X          int // absolute screen X for hit testing
	Y          int // absolute screen Y for hit testing
	Height     int
	TotalItems int
	TopItem    int
	Marks      []ScrollMark
	dragging   bool
	dragOffset int
}

// ScrollMark highlights Count items starting at Item on the track. Its style
// must set the same color as foreground and background: a track cell shows two
// marks at once by drawing a half block, which takes one color from each.
type ScrollMark struct {
	Item  int
	Count int
	Style term.Style
	Rank  int // wins over lower ranks sharing a half cell
}

func (s *Scrollbar) Visible() bool {
	return s.TotalItems > s.Height && s.Height > 0
}

func (s *Scrollbar) ThumbPos() (top, height int) {
	if s.TotalItems <= s.Height {
		return 0, s.Height
	}
	height = s.Height * s.Height / s.TotalItems
	if height < 1 {
		height = 1
	}
	scrollable := s.TotalItems - s.Height
	top = s.TopItem * (s.Height - height) / scrollable
	if top+height > s.Height {
		top = s.Height - height
	}
	return
}

func (s *Scrollbar) Render(surface Surface, rx, ry int) {
	if !s.Visible() {
		return
	}
	thumbTop, thumbH := s.ThumbPos()
	halves := s.markHalves()
	for y := 0; y < s.Height; y++ {
		base := term.StyleScrollbar
		if y >= thumbTop && y < thumbTop+thumbH {
			base = term.StyleScrollbarThumb
		}
		cell := term.Cell{Ch: '█', Style: base}
		if halves != nil {
			cell = markCell(base, halves[2*y], halves[2*y+1])
		}
		surface.SetCell(rx, ry+y, cell)
	}
}

// markHalves resolves Marks to one style per half cell of the track, so the
// track has twice the resolution of its rows. Every mark lands on at least one
// half cell however many items share it, so a single changed line in a huge
// file stays visible.
func (s *Scrollbar) markHalves() []term.Style {
	if len(s.Marks) == 0 || s.TotalItems <= 0 {
		return nil
	}
	n := 2 * s.Height
	halves := make([]term.Style, n)
	ranks := make([]int, n)
	half := func(item int) int {
		h := item * n / s.TotalItems
		return max(0, min(h, n-1))
	}
	for _, m := range s.Marks {
		last := half(m.Item + max(m.Count, 1) - 1)
		for h := half(m.Item); h <= last; h++ {
			if halves[h] == term.StyleDefault || m.Rank > ranks[h] {
				halves[h], ranks[h] = m.Style, m.Rank
			}
		}
	}
	return halves
}

func markCell(base, top, bottom term.Style) term.Cell {
	switch {
	case top == term.StyleDefault && bottom == term.StyleDefault:
		return term.Cell{Ch: '█', Style: base}
	case top == bottom:
		return term.Cell{Ch: '█', Style: top}
	case bottom == term.StyleDefault:
		return term.Cell{Ch: '▄', Style: base, BgStyle: top}
	case top == term.StyleDefault:
		return term.Cell{Ch: '▀', Style: base, BgStyle: bottom}
	default:
		return term.Cell{Ch: '▀', Style: top, BgStyle: bottom}
	}
}

func (s *Scrollbar) HandleEvent(ev tcell.Event) (newTopItem int, consumed bool) {
	mev, ok := ev.(*tcell.EventMouse)
	if !ok {
		return s.TopItem, false
	}

	mx, my := mev.Position()
	btn := mev.Buttons()

	if s.dragging {
		if btn == tcell.ButtonNone {
			s.dragging = false
			return s.TopItem, false
		}
		if btn&tcell.Button1 != 0 {
			relY := my - s.Y
			return s.posToTopItem(relY - s.dragOffset), true
		}
	}

	if btn&tcell.Button1 != 0 && mx == s.X && my >= s.Y && my < s.Y+s.Height {
		relY := my - s.Y
		thumbTop, thumbH := s.ThumbPos()

		s.dragging = true
		if relY >= thumbTop && relY < thumbTop+thumbH {
			s.dragOffset = relY - thumbTop
		} else {
			s.dragOffset = thumbH / 2
			return s.posToTopItem(relY - s.dragOffset), true
		}
		return s.TopItem, true
	}

	return s.TopItem, false
}

func (s *Scrollbar) IsDragging() bool { return s.dragging }

func (s *Scrollbar) posToTopItem(thumbTop int) int {
	_, thumbH := s.ThumbPos()
	maxThumbTop := s.Height - thumbH
	if maxThumbTop <= 0 {
		return 0
	}
	if thumbTop < 0 {
		thumbTop = 0
	}
	if thumbTop > maxThumbTop {
		thumbTop = maxThumbTop
	}
	scrollable := s.TotalItems - s.Height
	top := thumbTop * scrollable / maxThumbTop
	if top < 0 {
		top = 0
	}
	if top > scrollable {
		top = scrollable
	}
	return top
}

type HScrollbar struct {
	X          int
	Y          int
	Width      int
	TotalCols  int
	LeftCol    int
	dragging   bool
	dragOffset int
}

func (s *HScrollbar) Visible() bool {
	return s.TotalCols > s.Width && s.Width > 0
}

func (s *HScrollbar) ThumbPos() (left, width int) {
	if s.TotalCols <= s.Width {
		return 0, s.Width
	}
	width = s.Width * s.Width / s.TotalCols
	if width < 1 {
		width = 1
	}
	scrollable := s.TotalCols - s.Width
	left = s.LeftCol * (s.Width - width) / scrollable
	if left+width > s.Width {
		left = s.Width - width
	}
	return
}

func (s *HScrollbar) Render(surface Surface, rx, ry int) {
	if !s.Visible() {
		return
	}
	thumbLeft, thumbW := s.ThumbPos()
	for x := 0; x < s.Width; x++ {
		if x >= thumbLeft && x < thumbLeft+thumbW {
			surface.SetCell(rx+x, ry, term.Cell{Ch: '▄', Style: term.StyleScrollbarThumb})
		} else {
			surface.SetCell(rx+x, ry, term.Cell{Ch: '▄', Style: term.StyleScrollbar})
		}
	}
}

func (s *HScrollbar) HandleEvent(ev tcell.Event) (newLeftCol int, consumed bool) {
	mev, ok := ev.(*tcell.EventMouse)
	if !ok {
		return s.LeftCol, false
	}

	mx, my := mev.Position()
	btn := mev.Buttons()

	if s.dragging {
		if btn == tcell.ButtonNone {
			s.dragging = false
			return s.LeftCol, false
		}
		if btn&tcell.Button1 != 0 {
			relX := mx - s.X
			return s.posToLeftCol(relX - s.dragOffset), true
		}
	}

	if btn&tcell.Button1 != 0 && my == s.Y && mx >= s.X && mx < s.X+s.Width {
		relX := mx - s.X
		thumbLeft, thumbW := s.ThumbPos()

		s.dragging = true
		if relX >= thumbLeft && relX < thumbLeft+thumbW {
			s.dragOffset = relX - thumbLeft
		} else {
			s.dragOffset = thumbW / 2
			return s.posToLeftCol(relX - s.dragOffset), true
		}
		return s.LeftCol, true
	}

	return s.LeftCol, false
}

func (s *HScrollbar) IsDragging() bool { return s.dragging }

func (s *HScrollbar) posToLeftCol(thumbLeft int) int {
	_, thumbW := s.ThumbPos()
	maxThumbLeft := s.Width - thumbW
	if maxThumbLeft <= 0 {
		return 0
	}
	if thumbLeft < 0 {
		thumbLeft = 0
	}
	if thumbLeft > maxThumbLeft {
		thumbLeft = maxThumbLeft
	}
	scrollable := s.TotalCols - s.Width
	left := thumbLeft * scrollable / maxThumbLeft
	if left < 0 {
		left = 0
	}
	if left > scrollable {
		left = scrollable
	}
	return left
}
