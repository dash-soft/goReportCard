package pdf

import _ "embed"

//go:embed embed/logo.png
var Logo []byte

//go:embed embed/MapleMono-BoldItalic.ttf
var FontBoldItalic []byte

//go:embed embed/MapleMono-Italic.ttf
var FontItalic []byte
