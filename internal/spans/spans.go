package spans

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/srackham/go-rimu/v11/internal/options"

	"github.com/srackham/go-rimu/v11/internal/expansion"
	"github.com/srackham/go-rimu/v11/internal/quotes"
	"github.com/srackham/go-rimu/v11/internal/replacements"
	"github.com/srackham/go-rimu/v11/internal/utils/re"
	"github.com/srackham/go-rimu/v11/internal/utils/str"
)

// macros and spans package dependency injections.
var MacrosRender func(text string, silent bool) string

type fragment struct {
	text     string
	done     bool
	verbatim string // Replacements text rendered verbatim.
}

func Render(source string) string {
	result := preReplacements(source)
	frags := []fragment{{text: result, done: false}}
	frags = fragQuotes(frags)
	frags = fragSpecials(frags)
	result = defrag(frags)
	return postReplacements(result)
}

// Converts fragments to a string.
func defrag(frags []fragment) (result string) {
	for _, frag := range frags {
		result += frag.text
	}
	return
}

// Fragment quotes in all fragments and return resulting fragments array.
func fragQuotes(frags []fragment) (result []fragment) {
	for _, frag := range frags {
		result = append(result, fragQuote(frag)...)
	}
	// Strip backlash from escaped quotes in non-done fragments.
	for i, frag := range result {
		if !frag.done {
			result[i].text = quotes.Unescape(frag.text)
		}
	}
	return
}

// Fragment quotes in a single fragment and return resulting fragments array.
func fragQuote(frag fragment) (result []fragment) {
	if frag.done {
		return []fragment{frag}
	}
	var match []int
	nextIndex := 0
	for {
		s := frag.text[nextIndex:]
		match = quotes.Find(s)
		if match == nil {
			return []fragment{frag}
		}
		// Check if quote is escaped.
		if s[match[0]] == '\\' {
			// Restart search after escaped opening quote.
			nextIndex += match[3]
			continue
		}
		// Add frag.text offsets.
		for i := range match {
			match[i] += nextIndex
		}
		break
	}
	quote := frag.text[match[2]:match[3]]
	quoted := frag.text[match[4]:match[5]]
	startIndex := match[0]
	endIndex := match[1]
	// Check for same closing quote one character further to the right.
	for endIndex < len(frag.text) && frag.text[endIndex] == quote[0] {
		// Move to closing quote one character to right.
		quoted += string(quote[0])
		endIndex += 1
	}
	// Arrive here if we have a matched quote.
	// The quote splits the input fragment into 5 or more output fragments:
	// Text before the quote, left quote tag, quoted text, right quote tag and text after the quote.
	def := quotes.GetDefinition(quote)
	before := frag.text[:startIndex]
	after := frag.text[endIndex:]
	result = append(result, fragment{text: before, done: false})
	result = append(result, fragment{text: def.OpenTag, done: true})
	if !def.Spans {
		// Spans are disabled so render the quoted text verbatim.
		quoted = str.ReplaceSpecialChars(quoted)
		quoted = strings.Replace(quoted, "\u0000", "\u0001", -1) // Substitute verbatim replacement placeholder.
		result = append(result, fragment{text: quoted, done: true})
	} else {
		// Recursively process the quoted text.
		result = append(result, fragQuote(fragment{text: quoted, done: false})...)
	}
	result = append(result, fragment{text: def.CloseTag, done: true})
	// Recursively process the following text.
	result = append(result, fragQuote(fragment{text: after, done: false})...)
	return
}

// Stores placeholder replacement fragments saved by `preReplacements()` and restored by `postReplacements()`.
var savedReplacements []fragment

// Return text with replacements replaced with placeholders (see `postReplacements()`).
func preReplacements(text string) (result string) {
	savedReplacements = nil
	frags := fragReplacements([]fragment{{text: text, done: false}})
	// Reassemble text with replacement placeholders.
	for _, frag := range frags {
		if frag.done {
			savedReplacements = append(savedReplacements, frag) // Save replaced text.
			result += string('\u0000')                          // Placeholder for replaced text.
		} else {
			result += frag.text
		}
	}
	return
}

// Replace replacements placeholders with replacements text from savedReplacements[].
func postReplacements(text string) string {
	return regexp.MustCompile(`[\x{0000}\x{0001}]`).ReplaceAllStringFunc(text, func(match string) string {
		var frag fragment
		frag, savedReplacements = savedReplacements[0], savedReplacements[1:] // Remove frag from start of list.
		if match == string('\u0000') {
			return frag.text
		} else {
			return str.ReplaceSpecialChars(frag.verbatim)
		}

	})
}

// Fragment replacements in all fragments and return resulting fragments array.
func fragReplacements(frags []fragment) (result []fragment) {
	result = frags
	for _, def := range replacements.Defs {
		var tmp []fragment
		for _, frag := range result {
			tmp = append(tmp, fragReplacement(frag, def)...)
		}
		result = tmp
	}
	return
}

// Fragment replacements in a single fragment for a single replacement definition.
// Return resulting fragments list.
func fragReplacement(frag fragment, def replacements.Definition) (result []fragment) {
	if frag.done {
		return []fragment{frag}
	}
	match := def.Match.FindStringIndex(frag.text)
	if match == nil {
		return []fragment{frag}
	}
	// Arrive here if we have a matched replacement.
	// The kluge is because Go regexp does not support `(?=re)`.
	pattern := def.Match.String()
	kludge := pattern == `\S\\`+"`" || pattern == `[a-zA-Z0-9]_[a-zA-Z0-9]`
	if kludge {
		match[1]--
	}
	// The replacement splits the input fragment into 3 output fragments:
	// Text before the replacement, replaced text and text after the replacement.
	before := frag.text[:match[0]]
	matched := frag.text[match[0]:match[1]]
	after := frag.text[match[1]:]
	result = append(result, fragment{text: before, done: false})
	var replacement string
	if kludge {
		replacement = matched
	} else if strings.HasPrefix(matched, "\\") {
		// Remove leading backslash.
		replacement = str.ReplaceSpecialChars(matched[1:])
	} else {
		submatches := def.Match.FindStringSubmatch(matched)
		if def.Filter == nil {
			replacement = ReplaceMatch(submatches, def.Replacement, expansion.Options{})
		} else {
			replacement = def.Filter(submatches)
		}
	}
	result = append(result, fragment{text: replacement, done: true, verbatim: matched})
	// Recursively process the remaining text.
	result = append(result, fragReplacement(fragment{text: after, done: false}, def)...)
	return
}

func fragSpecials(frags []fragment) (result []fragment) {
	// Replace special characters in all non-done fragments.
	result = make([]fragment, len(frags))
	for i, frag := range frags {
		if !frag.done {
			frag.text = str.ReplaceSpecialChars(frag.text)
		}
		result[i] = frag
	}
	return
}

// Replace pattern "$1" or "$$1", "$2" or "$$2"... in `replacement` with corresponding match groups
// from `match`. If pattern starts with one "$" character add specials to `opts`,
// if it starts with two "$" characters add spans to `opts`.
func ReplaceMatch(match []string, replacement string, opts expansion.Options) string {
	return re.ReplaceAllStringSubmatchFunc(regexp.MustCompile(`(\${1,2})(\d)`), replacement, func(arguments []string) (result string) {
		// Replace $1, $2 ... with corresponding match groups.
		switch {
		case arguments[1] == "$$":
			opts.Spans = true
		default:
			opts.Specials = true
		}
		i, _ := strconv.ParseInt(arguments[2], 10, strconv.IntSize) // match group number.
		if int(i) >= len(match) {
			options.ErrorCallback("undefined replacement group: " + arguments[0])
			return ""
		}
		result = match[i] // match group text.
		return ReplaceInline(result, opts)
	}, -1)
}

// Replace the inline elements specified in options in text and return the result.
func ReplaceInline(text string, opts expansion.Options) string {
	if opts.Macros {
		text = MacrosRender(text, false)
	}
	// Spans also expand special characters.
	switch {
	case opts.Spans:
		text = Render(text)
	case opts.Specials:
		text = str.ReplaceSpecialChars(text)
	}
	return text
}
