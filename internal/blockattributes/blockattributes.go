package blockattributes

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/srackham/go-rimu/internal/expansion"
	"github.com/srackham/go-rimu/internal/options"
	"github.com/srackham/go-rimu/internal/spans"
	"github.com/srackham/go-rimu/internal/utils/re"
	"github.com/srackham/go-rimu/internal/utils/stringlist"
)

func init() {
	Init()
}

var Classes string    // Space separated HTML class names.
var Id string         // HTML element id.
var Css string        // HTML CSS styles.
var Attributes string // Other HTML element attributes.
var Options expansion.Options

var ids stringlist.StringList // List of allocated HTML ids.

// Init resets options to default values.
func Init() {
	// TODO
	Classes = ""
	Id = ""
	Css = ""
	Attributes = ""
	Options = expansion.Options{}
	ids = nil
}

func Parse(text string) bool {
	// Parse Block Attributes.
	// class names = $1, id = $2, css-properties = $3, html-attributes = $4, block-options = $5
	text = spans.ReplaceInline(text, expansion.Options{Macros: true})
	m := regexp.MustCompile(`^\\?\.((?:\s*[a-zA-Z][\w\-]*)+)*(?:\s*)?(#[a-zA-Z][\w\-]*\s*)?(?:\s*)?(?:"(.+?)")?(?:\s*)?(\[.+])?(?:\s*)?([+-][ \w+-]+)?$`).FindStringSubmatch(text)
	if m == nil {
		return false
	}
	for i, v := range m {
		m[i] = strings.Trim(v, " \n")
	}
	if !options.SkipBlockAttributes() {
		if m[1] != "" { // HTML element class names.
			if Classes != "" {
				Classes += " "
			}
			Classes += m[1]
		}
		if m[2] != "" { // HTML element id.
			Id = m[2][1:]
		}
		if m[3] != "" { // CSS properties.
			if Css != "" && !strings.HasSuffix(Css, ";") {
				Css += ";"
			}
			if Css != "" {
				Css += " "
			}
			Css += m[3]
		}
		if m[4] != "" && !options.IsSafeModeNz() { // HTML attributes.
			if Attributes != "" {
				Attributes += " "
			}
			Attributes += strings.Trim(m[4][1:len(m[4])-1], " \n")
		}
		Options = expansion.Parse(m[5])
	}
	return true
}

// Inject HTML attributes into the HTML `tag` and return result.
// Consume HTML attributes unless the `tag` argument is blank.
func Inject(tag string) string {
	if tag == "" {
		return tag
	}
	attrs := ""
	if Classes != "" {
		if regexp.MustCompile(`(?i)class=".*?"`).MatchString(tag) {
			// Inject class names into existing class attribute.
			tag = regexp.MustCompile(`(?i)class="(.*?)"`).ReplaceAllString(tag, "class=\""+Classes+" $1\"")
		} else {
			attrs = "class=\"" + Classes + "\""
		}
	}
	if Id != "" {
		Id = strings.ToLower(Id)
		has_id := regexp.MustCompile(`(?i)id=".*?"`).MatchString(tag)
		if has_id || ids.IndexOf(Id) >= 0 {
			options.ErrorCallback("duplicate \"id\" attribute: " + Id)
		} else {
			ids.Push(Id)
		}
		if !has_id {
			attrs += " id=\"" + Id + "\""
		}
	}
	if Css != "" {
		if regexp.MustCompile(`(?i)style=".*?"`).MatchString(tag) {
			// Inject CSS styles into existing style attribute.
			tag = re.ReplaceAllStringSubmatchFunc(regexp.MustCompile(`(?i)style="(.*?)"`), tag, func(match []string) string {
				css := strings.Trim(match[1], " \n")
				if !strings.HasSuffix(css, ";") {
					css += ";"
				}
				return "style=\"" + css + " " + Css + "\""
			})
		} else {
			attrs += " style=\"" + Css + "\""
		}
	}
	if Attributes != "" {
		attrs += " " + Attributes
	}
	attrs = strings.TrimLeft(attrs, " \n")
	if attrs != "" {
		m := regexp.MustCompile(`(?i)^(<[a-z]+|<h[1-6])(?:[ >])`).FindStringSubmatch(tag) // Match start tag.
		if m != nil {
			before := m[1]
			after := tag[len(m[1]):]
			tag = before + " " + attrs + after
		}
	}
	// Consume the attributes.
	Classes = ""
	Id = ""
	Css = ""
	Attributes = ""
	return tag
}

func Slugify(text string) string {
	slug := text
	slug = regexp.MustCompile(`\W+`).ReplaceAllString(slug, "-") // Replace non-alphanumeric characters with dashes.
	slug = regexp.MustCompile(`-+`).ReplaceAllString(slug, "-")  // Replace multiple dashes with single dash.
	slug = strings.Trim(slug, "-")                               // Trim leading and trailing dashes.
	slug = strings.ToLower(slug)
	if slug == "" {
		slug = "x"
	}
	if ids.IndexOf(slug) > -1 { // Another element already has that id.
		i := 2
		for ids.IndexOf(slug+"-"+fmt.Sprint(i)) > -1 {
			i++
		}
		slug += "-" + fmt.Sprint(i)
	}
	return slug
}