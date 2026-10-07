package main

import (
	"bytes"
	"encoding/xml"
	"io"
	"regexp"
	"strings"
)

var svgLocalURL = regexp.MustCompile(`^url\(#[a-zA-Z_][a-zA-Z0-9_.:-]*\)$`)

// Uploaded icons are a static SVG subset. Reject active content rather than
// trying to remove an incomplete list of scripting attributes from raw XML.
func safeSVG(data []byte) bool {
	tags := strings.Fields("svg g path rect circle ellipse line polyline polygon title desc defs linearGradient radialGradient stop clipPath mask use symbol")
	attrs := strings.Fields("id class xmlns version viewBox width height x y x1 x2 y1 y2 cx cy r rx ry d points fill fill-opacity fill-rule stroke stroke-width stroke-linecap stroke-linejoin stroke-miterlimit stroke-dasharray stroke-dashoffset stroke-opacity opacity transform preserveAspectRatio gradientUnits gradientTransform spreadMethod offset stop-color stop-opacity clip-path clip-rule mask href")
	allowed := func(list []string, name string) bool {
		for _, item := range list {
			if item == name {
				return true
			}
		}
		return false
	}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	depth, roots := 0, 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return roots == 1 && depth == 0
		}
		if err != nil {
			return false
		}
		switch node := token.(type) {
		case xml.StartElement:
			if node.Name.Space != "" && node.Name.Space != "http://www.w3.org/2000/svg" {
				return false
			}
			if !allowed(tags, node.Name.Local) {
				return false
			}
			if depth == 0 {
				roots++
				if roots != 1 || node.Name.Local != "svg" {
					return false
				}
			}
			depth++
			for _, attr := range node.Attr {
				if attr.Name.Space == "xmlns" {
					if attr.Value != "http://www.w3.org/2000/svg" && attr.Value != "http://www.w3.org/1999/xlink" {
						return false
					}
					continue
				}
				if attr.Name.Space != "" && attr.Name.Space != "http://www.w3.org/1999/xlink" {
					return false
				}
				if !allowed(attrs, attr.Name.Local) {
					return false
				}
				value := strings.TrimSpace(attr.Value)
				if attr.Name.Local == "href" && (!strings.HasPrefix(value, "#") || strings.ContainsAny(value, " \t\r\n")) {
					return false
				}
				lower := strings.ToLower(value)
				if strings.ContainsAny(value, "\\\x00") || strings.Contains(lower, "javascript:") || strings.Contains(lower, "data:") || strings.Contains(lower, "expression") || (strings.Contains(lower, "url(") && !svgLocalURL.MatchString(value)) {
					return false
				}
			}
		case xml.EndElement:
			depth--
		case xml.Directive:
			return false
		case xml.ProcInst:
			if node.Target != "xml" || roots != 0 {
				return false
			}
		case xml.CharData:
			if depth == 0 && strings.TrimSpace(string(node)) != "" {
				return false
			}
		}
	}
}
