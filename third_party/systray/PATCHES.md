This is github.com/gogpu/systray v0.3.0 (MIT licensed). The only source
change is in internal/platform_windows.go: invoke OnRightClick for
WM_CONTEXTMENU as well as WM_RBUTTONUP, before showing the menu. This lets
GoRig refresh its organization list on Windows version-4 tray icons and
keyboard context-menu invocation. Keep the patch in mind when upgrading.
