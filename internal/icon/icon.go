package icon

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed transfor.ico
var IconBytes []byte

var AppIcon = &fyne.StaticResource{
	StaticName:    "transfor.ico",
	StaticContent: IconBytes,
}
