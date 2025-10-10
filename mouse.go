package main

import (
	"fmt"
	"syscall"
)

// Define the POINT structure for GetCursorPos
type POINT struct {
	X, Y int32
}

// Windows API constants for mouse_event
const (
	MOUSEEVENTF_LEFTDOWN = 0x0002 // Left button down
	MOUSEEVENTF_LEFTUP   = 0x0004 // Left button up
	MOUSEEVENTF_MOVE     = 0x0001 // Mouse movement
)

func moveMouse(newX int32, newY int32) {
	user32 := syscall.NewLazyDLL("user32.dll")

	// SetCursorPos
	setCursorPos := user32.NewProc("SetCursorPos")
	_, _, _ = syscall.SyscallN(setCursorPos.Addr(), uintptr(newX), uintptr(newY))
	fmt.Printf("Cursor moved to: X=%d, Y=%d\n", newX, newY)

}
