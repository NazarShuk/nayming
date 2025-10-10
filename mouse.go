package main

import (
	"fmt"
	"syscall"
)

// Windows API constants for mouse_event
const (
	MOUSEEVENTF_LEFTDOWN  = 0x0002 // Left button down
	MOUSEEVENTF_LEFTUP    = 0x0004 // Left button up
	MOUSEEVENTF_RIGHTDOWN = 0x0008 // Right button down
	MOUSEEVENTF_RIGHTUP   = 0x0010 // Right button up
	MOUSEEVENTF_MOVE      = 0x0001 // Mouse movement
)

func moveMouse(newX int32, newY int32) {
	user32 := syscall.NewLazyDLL("user32.dll")

	// SetCursorPos
	setCursorPos := user32.NewProc("SetCursorPos")
	_, _, _ = syscall.SyscallN(setCursorPos.Addr(), uintptr(newX), uintptr(newY))
	fmt.Printf("Cursor moved to: X=%d, Y=%d\n", newX, newY)

}

func LeftMouseDown() {
	user32 := syscall.NewLazyDLL("user32.dll")
	// Simulate a left click at the new position
	mouseEvent := user32.NewProc("mouse_event")
	_, _, _ = syscall.SyscallN(mouseEvent.Addr(), uintptr(MOUSEEVENTF_LEFTDOWN), 0, 0, 0, 0)
}

func LeftMouseUp() {
	user32 := syscall.NewLazyDLL("user32.dll")
	// Simulate a left click at the new position
	mouseEvent := user32.NewProc("mouse_event")
	_, _, _ = syscall.SyscallN(mouseEvent.Addr(), uintptr(MOUSEEVENTF_LEFTUP), 0, 0, 0, 0)
}

func RightMouseDown() {
	user32 := syscall.NewLazyDLL("user32.dll")
	// Simulate a left click at the new position
	mouseEvent := user32.NewProc("mouse_event")
	_, _, _ = syscall.SyscallN(mouseEvent.Addr(), uintptr(MOUSEEVENTF_RIGHTDOWN), 0, 0, 0, 0)
}

func RightMouseUp() {
	user32 := syscall.NewLazyDLL("user32.dll")
	// Simulate a left click at the new position
	mouseEvent := user32.NewProc("mouse_event")
	_, _, _ = syscall.SyscallN(mouseEvent.Addr(), uintptr(MOUSEEVENTF_RIGHTUP), 0, 0, 0, 0)
}
