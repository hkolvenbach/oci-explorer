package badge

import (
	"bytes"
	"encoding/json"
	"text/template"

	"github.com/hkolvenbach/oci-explorer/score"
)

// ShieldsResponse is the Shields.io endpoint JSON schema.
type ShieldsResponse struct {
	SchemaVersion int    `json:"schemaVersion"`
	Label         string `json:"label"`
	Message       string `json:"message"`
	Color         string `json:"color"`
	LogoSVG       string `json:"logoSvg,omitempty"`
	IsError       bool   `json:"isError,omitempty"`
}

const (
	// badgeLabel is the label text shown on all badges.
	badgeLabel = "supply chain score"

	// faviconSVG is the OCI Explorer icon for the shields.io JSON logoSvg field.
	// Canonical source: web/public/favicon.svg
	faviconSVG = `<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 32 32'><rect width='32' height='32' rx='6' fill='#f97316'/><path d='M16 6l-9 4.5v11L16 26l9-4.5v-11L16 6z' fill='none' stroke='white' stroke-width='1.5' stroke-linejoin='round'/><path d='M7 10.5L16 15l9-4.5M16 15v11' fill='none' stroke='white' stroke-width='1.5' stroke-linejoin='round'/></svg>`

	// faviconPathData contains the SVG path elements from the favicon, inlined
	// into the badge SVG template at 14x14 (scale 0.4375 of the 32x32 viewBox).
	// Canonical source: web/public/favicon.svg
	faviconPathData = `<rect width="32" height="32" rx="6" fill="#f97316"/>` +
		`<path d="M16 6l-9 4.5v11L16 26l9-4.5v-11L16 6z" fill="none" stroke="white" stroke-width="1.5" stroke-linejoin="round"/>` +
		`<path d="M7 10.5L16 15l9-4.5M16 15v11" fill="none" stroke="white" stroke-width="1.5" stroke-linejoin="round"/>`
)

// badgeTmpl is a Shields.io flat-style SVG badge with an embedded favicon icon.
var badgeTmpl = template.Must(template.New("badge").Parse(`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="{{.TotalWidth}}" height="20" role="img" aria-label="{{.AriaLabel}}">
<title>{{.AriaLabel}}</title>
<linearGradient id="s" x2="0" y2="100%">
<stop offset="0" stop-color="#bbb" stop-opacity=".1"/>
<stop offset="1" stop-opacity=".1"/>
</linearGradient>
<clipPath id="r">
<rect width="{{.TotalWidth}}" height="20" rx="3" fill="#fff"/>
</clipPath>
<g clip-path="url(#r)">
<rect width="{{.LabelWidth}}" height="20" fill="#555"/>
<rect x="{{.LabelWidth}}" width="{{.ValueWidth}}" height="20" fill="#{{.Color}}"/>
<rect width="{{.TotalWidth}}" height="20" fill="url(#s)"/>
</g>
<g fill="#fff" text-anchor="middle" font-family="DejaVu Sans,Verdana,Geneva,sans-serif" font-size="110">
<text x="{{.LabelTextX}}0" y="150" fill="#010101" fill-opacity=".3" transform="scale(.1)" textLength="{{.LabelTextLen}}0" lengthAdjust="spacing">{{.Label}}</text>
<text x="{{.LabelTextX}}0" y="140" transform="scale(.1)" textLength="{{.LabelTextLen}}0" lengthAdjust="spacing">{{.Label}}</text>
<text x="{{.ValueTextX}}0" y="150" fill="#010101" fill-opacity=".3" transform="scale(.1)" textLength="{{.ValueTextLen}}0" lengthAdjust="spacing">{{.Message}}</text>
<text x="{{.ValueTextX}}0" y="140" transform="scale(.1)" textLength="{{.ValueTextLen}}0" lengthAdjust="spacing">{{.Message}}</text>
</g>
<g transform="translate(5,3) scale(0.4375)">` + faviconPathData + `</g>
</svg>`))

// errorBadgeTmpl renders a gray error badge.
var errorBadgeTmpl = template.Must(template.New("errorbadge").Parse(`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="{{.TotalWidth}}" height="20" role="img" aria-label="{{.Label}}: {{.Message}}">
<title>{{.Label}}: {{.Message}}</title>
<linearGradient id="s" x2="0" y2="100%">
<stop offset="0" stop-color="#bbb" stop-opacity=".1"/>
<stop offset="1" stop-opacity=".1"/>
</linearGradient>
<clipPath id="r">
<rect width="{{.TotalWidth}}" height="20" rx="3" fill="#fff"/>
</clipPath>
<g clip-path="url(#r)">
<rect width="{{.LabelWidth}}" height="20" fill="#555"/>
<rect x="{{.LabelWidth}}" width="{{.ValueWidth}}" height="20" fill="#9f9f9f"/>
<rect width="{{.TotalWidth}}" height="20" fill="url(#s)"/>
</g>
<g fill="#fff" text-anchor="middle" font-family="DejaVu Sans,Verdana,Geneva,sans-serif" font-size="110">
<text x="{{.LabelTextX}}0" y="150" fill="#010101" fill-opacity=".3" transform="scale(.1)" textLength="{{.LabelTextLen}}0" lengthAdjust="spacing">{{.Label}}</text>
<text x="{{.LabelTextX}}0" y="140" transform="scale(.1)" textLength="{{.LabelTextLen}}0" lengthAdjust="spacing">{{.Label}}</text>
<text x="{{.ValueTextX}}0" y="150" fill="#010101" fill-opacity=".3" transform="scale(.1)" textLength="{{.ValueTextLen}}0" lengthAdjust="spacing">{{.Message}}</text>
<text x="{{.ValueTextX}}0" y="140" transform="scale(.1)" textLength="{{.ValueTextLen}}0" lengthAdjust="spacing">{{.Message}}</text>
</g>
</svg>`))

// badgeData holds computed layout values for the SVG template.
type badgeData struct {
	AriaLabel    string
	Label        string
	Message      string
	Color        string
	LabelWidth   int
	ValueWidth   int
	TotalWidth   int
	LabelTextX   int // in tenths (template appends "0")
	ValueTextX   int // in tenths (template appends "0")
	LabelTextLen int // in tenths
	ValueTextLen int
}

// textWidth estimates pixel width of a string in Verdana 11px.
func textWidth(s string) int {
	w := 0
	for _, c := range s {
		switch {
		case c >= 'A' && c <= 'Z':
			w += 7
		case c == ' ':
			w += 4
		case c == '+':
			w += 7
		default:
			w += 6
		}
	}
	return w
}

// computeBadgeData calculates layout for a normal badge.
// The label section reserves 14px for the favicon icon on the left plus padding.
func computeBadgeData(label, message, color string) badgeData {
	const (
		iconWidth   = 14 // 32px * 0.4375
		leftPad     = 5  // translate(5,...)
		iconRight   = 4  // gap after icon
		sidePad     = 6  // padding on each side of text
	)

	labelTextW := textWidth(label)
	valueTextW := textWidth(message)

	// Label section: icon(14) + leftPad(5) + gap(4) + text + rightPad(6)
	labelWidth := leftPad + iconWidth + iconRight + labelTextW + sidePad
	// Value section: padding + text + padding
	valueWidth := sidePad + valueTextW + sidePad

	totalWidth := labelWidth + valueWidth

	// Text center X positions (in pixels, template multiplies by 10 and appends "0")
	// Label text centered accounting for icon taking left space
	labelIconOffset := leftPad + iconWidth + iconRight
	labelTextX := labelIconOffset + labelTextW/2 + sidePad/2

	valueCenterX := labelWidth + valueWidth/2

	return badgeData{
		AriaLabel:    badgeLabel + ": " + message,
		Label:        label,
		Message:      message,
		Color:        color,
		LabelWidth:   labelWidth,
		ValueWidth:   valueWidth,
		TotalWidth:   totalWidth,
		LabelTextX:   labelTextX,
		ValueTextX:   valueCenterX,
		LabelTextLen: labelTextW,
		ValueTextLen: valueTextW,
	}
}

// computeErrorBadgeData calculates layout for an error badge (no icon).
func computeErrorBadgeData(label, message string) badgeData {
	const sidePad = 6

	labelTextW := textWidth(label)
	valueTextW := textWidth(message)

	labelWidth := sidePad + labelTextW + sidePad
	valueWidth := sidePad + valueTextW + sidePad
	totalWidth := labelWidth + valueWidth

	labelTextX := labelWidth / 2
	valueCenterX := labelWidth + valueWidth/2

	return badgeData{
		Label:        label,
		Message:      message,
		LabelWidth:   labelWidth,
		ValueWidth:   valueWidth,
		TotalWidth:   totalWidth,
		LabelTextX:   labelTextX,
		ValueTextX:   valueCenterX,
		LabelTextLen: labelTextW,
		ValueTextLen: valueTextW,
	}
}

// RenderSVG produces a Shields.io flat-style SVG badge for the given score result.
func RenderSVG(result score.Result) []byte {
	data := computeBadgeData(badgeLabel, result.Grade, result.Color)
	var buf bytes.Buffer
	if err := badgeTmpl.Execute(&buf, data); err != nil {
		return RenderErrorSVG("render error")
	}
	return buf.Bytes()
}

// RenderErrorSVG produces a gray error SVG badge with the given message.
func RenderErrorSVG(message string) []byte {
	data := computeErrorBadgeData(badgeLabel, message)
	var buf bytes.Buffer
	if err := errorBadgeTmpl.Execute(&buf, data); err != nil {
		return []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="100" height="20"><rect width="100" height="20" fill="#9f9f9f"/></svg>`)
	}
	return buf.Bytes()
}

// RenderJSON produces a Shields.io endpoint JSON payload for the given result.
func RenderJSON(result score.Result) []byte {
	resp := ShieldsResponse{
		SchemaVersion: 1,
		Label:         badgeLabel,
		Message:       result.Grade,
		Color:         result.Color,
		LogoSVG:       faviconSVG,
	}
	out, err := json.Marshal(resp)
	if err != nil {
		return renderErrorJSONBytes("render error")
	}
	return out
}

// RenderErrorJSON produces a Shields.io endpoint error JSON payload.
func RenderErrorJSON(message string) []byte {
	return renderErrorJSONBytes(message)
}

func renderErrorJSONBytes(message string) []byte {
	resp := ShieldsResponse{
		SchemaVersion: 1,
		Label:         badgeLabel,
		Message:       message,
		Color:         "gray",
		IsError:       true,
	}
	out, _ := json.Marshal(resp)
	return out
}
