package ui

import (
	"slices"

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
	// Symbols for Legacy Computing glyphs are missing from many fonts.
	LegacyGlyphs bool
	dragging     bool
	dragOffset   int
}

// ScrollMark highlights Count items starting at Item on the track. Its style
// must set the same color as foreground and background: a marked cell is a
// block glyph whose two colors can each come from either side.
type ScrollMark struct {
	Item  int
	Count int
	Style term.Style
	Rank  int // wins over lower ranks sharing a slot
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
	slots := s.markSlots()
	glyphs := blockGlyphs
	if s.LegacyGlyphs {
		glyphs = legacyGlyphs
	}
	for y := 0; y < s.Height; y++ {
		base := term.StyleScrollbar
		if y >= thumbTop && y < thumbTop+thumbH {
			base = term.StyleScrollbarThumb
		}
		cell := term.Cell{Ch: '█', Style: base}
		if slots != nil {
			cell = markCell(base, slots[y*cellSlots:(y+1)*cellSlots], glyphs)
		}
		surface.SetCell(rx, ry+y, cell)
	}
}

const cellSlots = 8

// Every mark keeps at least one slot however many items share it, so a single
// changed line in a huge file stays visible.
func (s *Scrollbar) markSlots() []term.Style {
	if len(s.Marks) == 0 || s.TotalItems <= 0 {
		return nil
	}
	n := cellSlots * s.Height
	slots := make([]term.Style, n)
	ranks := make([]int, n)
	slot := func(item int) int {
		return max(0, min(item*n/s.TotalItems, n-1))
	}
	for _, m := range s.Marks {
		last := slot(m.Item + max(m.Count, 1) - 1)
		for i := slot(m.Item); i <= last; i++ {
			if slots[i] == term.StyleDefault || m.Rank > ranks[i] {
				slots[i], ranks[i] = m.Style, m.Rank
			}
		}
	}
	return slots
}

// mask bit 0 is the top eighth; set bits show the foreground.
type scrollGlyph struct {
	ch   rune
	mask uint8
}

var blockGlyphs = []scrollGlyph{
	{'▀', 0x0f}, {'▄', 0xf0},
	{'▔', 0x01}, {'▁', 0x80}, {'▂', 0xc0}, {'▃', 0xe0},
	{'▅', 0xf8}, {'▆', 0xfc}, {'▇', 0xfe},
}

var legacyGlyphs = append(slices.Clone(blockGlyphs),
	scrollGlyph{'\U0001FB76', 0x02}, scrollGlyph{'\U0001FB77', 0x04},
	scrollGlyph{'\U0001FB78', 0x08}, scrollGlyph{'\U0001FB79', 0x10},
	scrollGlyph{'\U0001FB7A', 0x20}, scrollGlyph{'\U0001FB7B', 0x40},
	scrollGlyph{'\U0001FB82', 0x03}, scrollGlyph{'\U0001FB83', 0x07},
	scrollGlyph{'\U0001FB84', 0x1f}, scrollGlyph{'\U0001FB85', 0x3f},
	scrollGlyph{'\U0001FB86', 0x7f},
)

// Costs for drawing a slot in the wrong color. Hiding a mark costs most, so
// marks keep their place even when it means painting over some track; a mark
// color missing from the cell entirely is never worth it.
const (
	costTrackAsMark = 1
	costMarkAsOther = 1
	costMarkAsTrack = 3
	costMarkHidden  = 100
)

// A zero slot means the track. A cell holds only two colors, so with two mark
// colors present the track color gives way.
func markCell(base term.Style, slots []term.Style, glyphs []scrollGlyph) term.Cell {
	colors := []term.Style{base}
	for _, st := range slots {
		if st != term.StyleDefault && !slices.Contains(colors, st) {
			colors = append(colors, st)
		}
	}
	if len(colors) == 1 {
		return term.Cell{Ch: '█', Style: base}
	}
	want := func(i int) term.Style {
		if slots[i] == term.StyleDefault {
			return base
		}
		return slots[i]
	}
	cost := func(mask uint8, fg, bg term.Style) int {
		c := 0
		for i := range cellSlots {
			got := bg
			if mask&(1<<i) != 0 {
				got = fg
			}
			switch w := want(i); {
			case got == w:
			case w == base:
				c += costTrackAsMark
			case got == base:
				c += costMarkAsTrack
			default:
				c += costMarkAsOther
			}
		}
		for _, st := range colors[1:] {
			if (mask == 0 || st != fg) && (mask == 0xff || st != bg) {
				c += costMarkHidden
			}
		}
		return c
	}

	best := term.Cell{Ch: '█', Style: base}
	bestCost := cost(0xff, base, base)
	for _, st := range colors[1:] {
		if c := cost(0xff, st, st); c < bestCost {
			best, bestCost = term.Cell{Ch: '█', Style: st}, c
		}
	}
	for _, g := range glyphs {
		for _, fg := range colors {
			for _, bg := range colors {
				if fg == bg {
					continue
				}
				if c := cost(g.mask, fg, bg); c < bestCost {
					best, bestCost = term.Cell{Ch: g.ch, Style: fg, BgStyle: trackFill(bg)}, c
				}
			}
		}
	}
	return best
}

// Track styles only set a foreground, so they cannot serve as BgStyle.
func trackFill(st term.Style) term.Style {
	switch st {
	case term.StyleScrollbar:
		return term.StyleScrollbarFill
	case term.StyleScrollbarThumb:
		return term.StyleScrollbarThumbFill
	}
	return st
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
