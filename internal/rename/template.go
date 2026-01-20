package rename

// segment represents either a literal string or a placeholder.
type segment struct {
	isPlaceholder bool
	value         string // placeholder name (without braces) or literal text
}

// parseTemplate parses a template string into segments.
// Placeholders are {name}, escaped braces are {{ and }}.
func parseTemplate(template string) []segment {
	if template == "" {
		return nil
	}

	var segments []segment
	var current []rune
	inPlaceholder := false

	runes := []rune(template)
	for i := 0; i < len(runes); i++ {
		r := runes[i]

		// Check for escaped braces
		if r == '{' && i+1 < len(runes) && runes[i+1] == '{' {
			current = append(current, '{')
			i++ // skip next brace
			continue
		}
		if r == '}' && i+1 < len(runes) && runes[i+1] == '}' {
			current = append(current, '}')
			i++ // skip next brace
			continue
		}

		if r == '{' && !inPlaceholder {
			// Start of placeholder - save any accumulated literal
			if len(current) > 0 {
				segments = append(segments, segment{isPlaceholder: false, value: string(current)})
				current = nil
			}
			inPlaceholder = true
			continue
		}

		if r == '}' && inPlaceholder {
			// End of placeholder
			segments = append(segments, segment{isPlaceholder: true, value: string(current)})
			current = nil
			inPlaceholder = false
			continue
		}

		current = append(current, r)
	}

	// Handle remaining content
	if len(current) > 0 {
		segments = append(segments, segment{isPlaceholder: inPlaceholder, value: string(current)})
	}

	return segments
}
