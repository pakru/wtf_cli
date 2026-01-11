package ui

import (
	"os"
	"syscall"
	"unsafe"

	"github.com/creack/pty"
)

// ResizePTY updates the PTY dimensions
func ResizePTY(ptyFile *os.File, width, height int) error {
	if ptyFile == nil {
		return nil
	}
	
	size := &pty.Winsize{
		Rows: uint16(height),
		Cols: uint16(width),
	}
	
	return pty.Setsize(ptyFile, size)
}

// GetPTYSize returns the current PTY size
func GetPTYSize(ptyFile *os.File) (width, height int, err error) {
	if ptyFile == nil {
		return 80, 24, nil
	}
	
	size, err := pty.GetsizeFull(ptyFile)
	if err != nil {
		return 80, 24, err
	}
	
	return int(size.Cols), int(size.Rows), nil
}

// ioctl constants for terminal operations
const (
	TIOCGWINSZ = 0x5413
	TIOCSWINSZ = 0x5414
)

// winsize represents terminal window size
type winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

// getTerminalSize gets the size of a terminal
func getTerminalSize(fd uintptr) (width, height int, err error) {
	var ws winsize
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		fd,
		uintptr(TIOCGWINSZ),
		uintptr(unsafe.Pointer(&ws)),
	)
	
	if errno != 0 {
		return 0, 0, errno
	}
	
	return int(ws.Col), int(ws.Row), nil
}
