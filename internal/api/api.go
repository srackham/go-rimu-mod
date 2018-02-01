package api

import (
	"github.com/srackham/go-rimu/internal/blockattributes"
	"github.com/srackham/go-rimu/internal/delimitedblocks"
	"github.com/srackham/go-rimu/internal/iotext"
	"github.com/srackham/go-rimu/internal/lineblocks"
	"github.com/srackham/go-rimu/internal/lists"
	"github.com/srackham/go-rimu/internal/macros"
	"github.com/srackham/go-rimu/internal/options"
	"github.com/srackham/go-rimu/internal/quotes"
	"github.com/srackham/go-rimu/internal/replacements"
)

func init() {
	// Dependency injectiion so we can use api functions in imported packages without incuring import cycle errors.
	options.ApiInit = Init
	delimitedblocks.ApiRender = Render
}

// Init TODO
func Init() {
	blockattributes.Init()
	options.Init()
	delimitedblocks.Init()
	macros.Init()
	quotes.Init()
	replacements.Init()
}

// Render TODO
func Render(source string) string {
	reader := iotext.NewReader(source)
	writer := iotext.NewWriter()
	for !reader.Eof() {
		reader.SkipBlankLines()
		if reader.Eof() {
			break
		}
		if lineblocks.Render(reader, writer, nil) {
			continue
		}
		if lists.Render(reader, writer) {
			continue
		}
		if delimitedblocks.Render(reader, writer, nil) {
			continue
		}
		// This code should never be executed (normal paragraphs should match anything).
		panic("no matching delimited block found")
	}
	return writer.String()
}