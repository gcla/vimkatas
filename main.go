package main

import (
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/gcla/gowid"
	"github.com/gcla/gowid/examples"
	"github.com/gcla/gowid/widgets/columns"
	"github.com/gcla/gowid/widgets/fill"
	"github.com/gcla/gowid/widgets/framed"
	"github.com/gcla/gowid/widgets/holder"
	"github.com/gcla/gowid/widgets/pile"
	"github.com/gcla/gowid/widgets/styled"
	"github.com/gcla/gowid/widgets/terminal"
	"github.com/gcla/gowid/widgets/text"
	"github.com/gdamore/tcell"
	log "github.com/sirupsen/logrus"
)

//======================================================================

type ResizeableColumnsWidget struct {
	*columns.Widget
	offset int
}

func NewResizeableColumns(widgets []gowid.IContainerWidget) *ResizeableColumnsWidget {
	res := &ResizeableColumnsWidget{}
	res.Widget = columns.New(widgets)
	return res
}

func (w *ResizeableColumnsWidget) WidgetWidths(size gowid.IRenderSize, focus gowid.Selector, focusIdx int, app gowid.IApp) []int {
	widths := w.Widget.WidgetWidths(size, focus, focusIdx, app)
	addme := w.offset
	if widths[0]+addme < 0 {
		addme = -widths[0]
	} else if widths[2]-addme < 0 {
		addme = widths[2]
	}
	widths[0] += addme
	widths[2] -= addme
	return widths
}

func (w *ResizeableColumnsWidget) Render(size gowid.IRenderSize, focus gowid.Selector, app gowid.IApp) gowid.ICanvas {
	return columns.Render(w, size, focus, app)
}

func (w *ResizeableColumnsWidget) RenderSubWidgets(size gowid.IRenderSize, focus gowid.Selector, focusIdx int, app gowid.IApp) []gowid.ICanvas {
	return columns.RenderSubWidgets(w, size, focus, focusIdx, app)
}

func (w *ResizeableColumnsWidget) RenderedSubWidgetsSizes(size gowid.IRenderSize, focus gowid.Selector, focusIdx int, app gowid.IApp) []gowid.IRenderBox {
	return columns.RenderedSubWidgetsSizes(w, size, focus, focusIdx, app)
}

func (w *ResizeableColumnsWidget) SubWidgetSize(size gowid.IRenderSize, newX int, sub gowid.IWidget, dim gowid.IWidgetDimension) gowid.IRenderSize {
	return w.Widget.SubWidgetSize(size, newX, sub, dim)
}

//======================================================================

type ResizeablePileWidget struct {
	*pile.Widget
	offset int
}

func NewResizeablePile(widgets []gowid.IContainerWidget) *ResizeablePileWidget {
	res := &ResizeablePileWidget{}
	res.Widget = pile.New(widgets)
	return res
}

type PileAdjuster struct {
	widget    *ResizeablePileWidget
	origSizer pile.IPileBoxMaker
}

func (f PileAdjuster) MakeBox(w gowid.IWidget, size gowid.IRenderSize, focus gowid.Selector, app gowid.IApp) gowid.IRenderBox {
	adjustedSize := size
	var box gowid.RenderBox
	isbox := false
	switch s2 := size.(type) {
	case gowid.IRenderBox:
		box.C = s2.BoxColumns()
		box.R = s2.BoxRows()
		isbox = true
	}
	i := 0
	for ; i < len(f.widget.SubWidgets()); i++ {
		if w == f.widget.SubWidgets()[i] {
			break
		}
	}
	if i == len(f.widget.SubWidgets()) {
		panic("Unexpected pile state!")
	}
	if isbox {
		switch i {
		case 0:
			if box.R+f.widget.offset < 0 {
				f.widget.offset = -box.R
			}
			box.R += f.widget.offset
		case 2:
			if box.R-f.widget.offset < 0 {
				f.widget.offset = box.R
			}
			box.R -= f.widget.offset
		}
		adjustedSize = box
	}
	return f.origSizer.MakeBox(w, adjustedSize, focus, app)
}

func (w *ResizeablePileWidget) FindNextSelectable(dir gowid.Direction, wrap bool) (int, bool) {
	return gowid.FindNextSelectableFrom(w, w.Focus(), dir, wrap)
}

func (w *ResizeablePileWidget) UserInput(ev interface{}, size gowid.IRenderSize, focus gowid.Selector, app gowid.IApp) bool {
	return pile.UserInput(w, ev, size, focus, app)
}

func (w *ResizeablePileWidget) Render(size gowid.IRenderSize, focus gowid.Selector, app gowid.IApp) gowid.ICanvas {
	return pile.Render(w, size, focus, app)
}

func (w *ResizeablePileWidget) RenderedSubWidgetsSizes(size gowid.IRenderSize, focus gowid.Selector, focusIdx int, app gowid.IApp) []gowid.IRenderBox {
	res, _ := pile.RenderedChildrenSizes(w, size, focus, focusIdx, app)
	return res
}

func (w *ResizeablePileWidget) RenderSubWidgets(size gowid.IRenderSize, focus gowid.Selector, focusIdx int, app gowid.IApp) []gowid.ICanvas {
	return pile.RenderSubwidgets(w, size, focus, focusIdx, app)
}

func (w *ResizeablePileWidget) RenderBoxMaker(size gowid.IRenderSize, focus gowid.Selector, focusIdx int, app gowid.IApp, sizer pile.IPileBoxMaker) ([]gowid.IRenderBox, []gowid.IRenderSize) {
	x := &PileAdjuster{
		widget:    w,
		origSizer: sizer,
	}
	return pile.RenderBoxMaker(w, size, focus, focusIdx, app, x)
}

//======================================================================

var app *gowid.App
var cols *ResizeableColumnsWidget
var pilew *ResizeablePileWidget
var twidgets []*terminal.Widget

//======================================================================

type handler struct{}

func (h handler) UnhandledInput(app gowid.IApp, ev interface{}) bool {
	handled := false

	if evk, ok := ev.(*tcell.EventKey); ok {
		switch evk.Key() {
		case tcell.KeyCtrlC, tcell.KeyEsc:
			handled = true
			for _, t := range twidgets {
				t.Signal(syscall.SIGINT)
			}
		case tcell.KeyCtrlBackslash:
			handled = true
			for _, t := range twidgets {
				t.Signal(syscall.SIGQUIT)
			}
		case tcell.KeyRune:
			handled = true
			switch evk.Rune() {
			case '>':
				cols.offset += 1
			case '<':
				cols.offset -= 1
			case '+':
				pilew.offset += 1
			case '-':
				pilew.offset -= 1
			default:
				handled = false
			}
		}
	}
	return handled
}

//======================================================================

func main() {
	var err error

	f := examples.RedirectLogger("terminal.log")
	defer f.Close()

	palette := gowid.Palette{
		"invred": gowid.MakePaletteEntry(gowid.ColorBlack, gowid.ColorRed),
		"line":   gowid.MakeStyledPaletteEntry(gowid.NewUrwidColor("black"), gowid.NewUrwidColor("light gray"), gowid.StyleBold),
	}

	hkDuration := terminal.HotKeyDuration{D: time.Second * 3}

	twidgets = make([]*terminal.Widget, 0)
	//foo := os.Env()
	os.Open("foo")
	tcommands := []string{
		//os.Getenv("SHELL"),
		"vim",
		//os.Getenv("SHELL"),
		//os.Getenv("SHELL"),

		//"less cell.go",
		//"vttest",
		//"emacs -nw -q ./cell.go",
	}

	for _, cmd := range tcommands {
		app, err := terminal.NewExt(terminal.Options{
			Command:           strings.Split(cmd, " "),
			HotKeyPersistence: &hkDuration,
			Scrollback:        100,
		})
		if err != nil {
			panic(err)
		}
		twidgets = append(twidgets, app)
	}

	ExpectedOutput := styled.New(
		text.NewFromContentExt(
			text.NewContent([]text.ContentSegment{
				text.StyledContent("Expected Output", gowid.MakePaletteRef("banner")),
			}),
			text.Options{
				Align: gowid.HAlignMiddle{},
			},
		),
		gowid.MakePaletteRef("streak"),
	)

	tips := styled.New(
		text.NewFromContentExt(
			text.NewContent([]text.ContentSegment{
				text.StyledContent("Tips", gowid.MakePaletteRef("banner")),
			}),
			text.Options{
				Align: gowid.HAlignMiddle{},
			},
		),
		gowid.MakePaletteRef("streak"),
	)

	tw := text.New(" VimKatas ")
	twi := styled.New(tw, gowid.MakePaletteRef("invred"))
	twp := holder.New(tw)

	vline := styled.New(fill.New('│'), gowid.MakePaletteRef("line"))
	hline := styled.New(fill.New('-'), gowid.MakePaletteRef("line"))

	pilew = NewResizeablePile([]gowid.IContainerWidget{
		&gowid.ContainerWidget{IWidget: twidgets[0], D: gowid.RenderWithWeight{W: 3}},
		&gowid.ContainerWidget{IWidget: hline, D: gowid.RenderWithUnits{U: 1}},
		&gowid.ContainerWidget{IWidget: ExpectedOutput, D: gowid.RenderWithWeight{W: 3}},
	})

	cols = NewResizeableColumns([]gowid.IContainerWidget{
		&gowid.ContainerWidget{IWidget: pilew, D: gowid.RenderWithWeight{W: 3}},
		&gowid.ContainerWidget{IWidget: vline, D: gowid.RenderWithUnits{U: 1}},
		&gowid.ContainerWidget{IWidget: tips, D: gowid.RenderWithWeight{W: 1}},

	})

	view := framed.New(cols, framed.Options{
		Frame:       framed.UnicodeFrame,
		TitleWidget: twp,
	})

	for _, t := range twidgets {
		t.OnProcessExited(gowid.WidgetCallback{Name: "cb",
			WidgetChangedFunction: func(app gowid.IApp, w gowid.IWidget) {
				app.Quit()
			},
		})
		t.OnBell(gowid.WidgetCallback{Name: "cb",
			WidgetChangedFunction: func(app gowid.IApp, w gowid.IWidget) {
				twp.SetSubWidget(twi, app)
				timer := time.NewTimer(time.Millisecond * 800)
				go func() {
					<-timer.C
					app.Run(gowid.RunFunction(func(app gowid.IApp) {
						twp.SetSubWidget(tw, app)
					}))
				}()
			},
		})
		t.OnSetTitle(gowid.WidgetCallback{Name: "cb",
			WidgetChangedFunction: func(app gowid.IApp, w gowid.IWidget) {
				w2 := w.(*terminal.Widget)
				tw.SetText(" "+w2.GetTitle()+" ", app)
			},
		})
	}

	app, err = gowid.NewApp(gowid.AppArgs{
		View:    view,
		Palette: &palette,
		Log:     log.StandardLogger(),
	})
	examples.ExitOnErr(err)

	app.MainLoop(handler{})
}