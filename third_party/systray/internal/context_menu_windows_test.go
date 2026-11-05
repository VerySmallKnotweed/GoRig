//go:build windows

package internal

import "testing"

func TestRightClickCallbackForBothWindowsMenuEvents(t *testing.T) {
	for _, event := range []uintptr{wmRButtonUp, wmContextMenu} {
		called := false
		tray := &win32Tray{callbacks: &Callbacks{OnRightClick: func() { called = true }}}
		// With no native HMENU the menu display returns immediately.
		tray.handleTrayMessage(event)
		if !called {
			t.Errorf("OnRightClick not called for event %x", event)
		}
	}
}
