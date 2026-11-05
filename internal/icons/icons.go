// Package icons contains the shared GoRig Base icons used by the tray and UI.
package icons

import _ "embed"

// BasePNG is used by the Windows notification-area icon.
//
//go:embed base.png
var BasePNG []byte

// BaseICO is the same artwork, in a multi-resolution Windows icon format.
//
//go:embed base.ico
var BaseICO []byte
