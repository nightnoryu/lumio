package portfolio

import _ "embed"

//go:embed viewer.js
var ViewerJS []byte

//go:embed page.css
var portfolioCSS string
