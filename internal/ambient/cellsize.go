package ambient

import (
	"fmt"
	"math"
	"os"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/term"
)

// CellPixels holds the terminal cell size in pixels. Zero values mean unknown.
type CellPixels struct {
	W float64
	H float64
}

// DefaultCellPixels is the fallback 9:17 cell (width:height).
func DefaultCellPixels() CellPixels { return CellPixels{W: 9, H: 17} }

// DetectCellPixels reads the real cell pixel size. It tries, in order:
//  1. TIOCGWINSZ (ws_xpixel/ws_ypixel over ws_col/ws_row)
//  2. The XTWINOPS size query (CSI 16 t — cell size in pixels), answered on
//     the controlling terminal; the reply has no trailing newline, so the
//     query is made in raw mode with a short read deadline. Only attempted
//     when f is a terminal, so headless callers (tests, dumps piped to a
//     file) never touch the user's terminal.
//  3. Fallback 9:17 cell.
//
// The tty argument, when non-nil, is used for the ioctl; the XTWINOPS
// query always runs on /dev/tty.
func DetectCellPixels(tty *os.File) CellPixels {
	if tty == nil {
		tty = os.Stdout
	}
	if cp, ok := cellPixelsFromWinsize(tty); ok {
		return cp
	}
	if term.IsTerminal(int(tty.Fd())) {
		if cp, ok := cellPixelsFromXTWINOPS(); ok {
			return cp
		}
	}
	return DefaultCellPixels()
}

type winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

func cellPixelsFromWinsize(f *os.File) (CellPixels, bool) {
	if f == nil {
		return CellPixels{}, false
	}
	var ws winsize
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&ws)))
	if errno != 0 || ws.Col == 0 || ws.Row == 0 || ws.Xpixel == 0 || ws.Ypixel == 0 {
		return CellPixels{}, false
	}
	return CellPixels{W: float64(ws.Xpixel) / float64(ws.Col), H: float64(ws.Ypixel) / float64(ws.Row)}, true
}

// cellPixelsFromXTWINOPS asks the terminal for its cell size in pixels
// (CSI 16 t) and parses the CSI 6 ; height ; width t reply. The query
// runs on a separately opened /dev/tty in raw mode; it gives up quietly
// after 300 ms.
func cellPixelsFromXTWINOPS() (CellPixels, bool) {
	fd, err := syscall.Open("/dev/tty", syscall.O_RDWR, 0)
	if err != nil {
		return CellPixels{}, false
	}
	defer syscall.Close(fd)
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return CellPixels{}, false
	}
	defer func() { _ = term.Restore(fd, oldState) }()
	f := os.NewFile(uintptr(fd), "/dev/tty")
	if _, err := f.Write([]byte("\x1b[16t")); err != nil {
		return CellPixels{}, false
	}
	// Raw mode reads block forever, so without a working deadline we bail.
	if err := f.SetReadDeadline(time.Now().Add(300 * time.Millisecond)); err != nil {
		return CellPixels{}, false
	}
	buf := make([]byte, 64)
	n, err := f.Read(buf)
	if err != nil {
		return CellPixels{}, false
	}
	return parseXTWINOPSCellSize(string(buf[:n]))
}

// parseXTWINOPSCellSize parses a CSI 6 ; height ; width t reply. Tolerant
// of leading garbage and of the CSI 4 ; height ; width t (text area)
// fallback shape; rejects anything else.
func parseXTWINOPSCellSize(reply string) (CellPixels, bool) {
	start := strings.Index(reply, "\x1b[")
	if start < 0 {
		return CellPixels{}, false
	}
	end := strings.IndexByte(reply[start:], 't')
	if end < 0 {
		return CellPixels{}, false
	}
	body := reply[start+2 : start+end]
	parts := strings.Split(body, ";")
	if len(parts) != 3 {
		return CellPixels{}, false
	}
	var ps [3]float64
	for i, p := range parts {
		v, err := parsePositiveFloat(strings.TrimSpace(p))
		if err != nil {
			return CellPixels{}, false
		}
		ps[i] = v
	}
	switch ps[0] {
	case 6: // cell size in pixels: CSI 6 ; height ; width t
		if ps[1] <= 0 || ps[2] <= 0 {
			return CellPixels{}, false
		}
		return CellPixels{W: ps[2], H: ps[1]}, true
	default:
		return CellPixels{}, false
	}
}

func parsePositiveFloat(s string) (float64, error) {
	v := 0.0
	_, err := fmt.Sscanf(s, "%f", &v)
	if err != nil {
		return 0, err
	}
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, fmt.Errorf("not finite")
	}
	return v, nil
}

// WinsizeCellPixels reads the cell size in pixels from TIOCGWINSZ only —
// no escape-sequence round trip. ok is false when the terminal does not
// report pixel geometry.
func WinsizeCellPixels(f *os.File) (CellPixels, bool) { return cellPixelsFromWinsize(f) }

// SubpixelAspect returns (cellH/rowsPerCell)/(cellW/colsPerCell) for mode.
// A result of 1 means square subpixels. Unknown/invalid inputs fall back to
// the 9:17 default cell.
func SubpixelAspect(mode string, cell CellPixels) float64 {
	if cell.W <= 0 || cell.H <= 0 {
		cell = DefaultCellPixels()
	}
	cols, rows := subcellsPerCell(ResolveMode(mode))
	if cols <= 0 || rows <= 0 {
		return 1
	}
	// subpixel width  = cell.W / cols
	// subpixel height = cell.H / rows
	// aspect = subpixel height / subpixel width
	return (cell.H / float64(rows)) / (cell.W / float64(cols))
}

func subcellsPerCell(mode string) (cols, rows int) {
	switch ResolveMode(mode) {
	case ModeHalfblock:
		return 1, 2
	case ModeBraille:
		return 2, 4
	case ModeQuadrant:
		return 2, 2
	case ModeSextant:
		return 2, 3
	case ModeASCII:
		return 1, 1
	default:
		return 1, 2
	}
}
