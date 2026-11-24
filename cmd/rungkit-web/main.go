//go:build js && wasm

// Command rungkit-web is the composition root of the browser version: it
// wires the in-memory file store and the adapters into a workspace and hands
// it to the web presentation. Build: web/build.sh
package main

import (
	"github.com/Piyoone3939/rungkit/internal/application/workspace"
	"github.com/Piyoone3939/rungkit/internal/infrastructure/export/melsec"
	"github.com/Piyoone3939/rungkit/internal/infrastructure/export/st"
	"github.com/Piyoone3939/rungkit/internal/infrastructure/filestore"
	"github.com/Piyoone3939/rungkit/internal/infrastructure/jsoncodec"
	"github.com/Piyoone3939/rungkit/internal/infrastructure/ldt"
	"github.com/Piyoone3939/rungkit/internal/infrastructure/ldtest"
	"github.com/Piyoone3939/rungkit/internal/infrastructure/plcopen"
	"github.com/Piyoone3939/rungkit/internal/infrastructure/render"
	"github.com/Piyoone3939/rungkit/internal/presentation/web"
)

var version = "dev"

func main() {
	mem := filestore.NewMemory()
	ws := workspace.New(workspace.Deps{
		Files:     mem,
		Notation:  ldt.Notation{},
		Scenarios: ldtest.Parser{},
		Exporters: []workspace.Exporter{
			melsec.Exporter{},
			melsec.CSVExporter{ProjectName: "Main"},
			st.Exporter{ProgramName: "Main"},
			plcopen.Exporter{ProgramName: "Main"},
			jsoncodec.Exporter{},
			ldt.Exporter{},
			render.ASCII{},
			render.SVG{},
		},
	})
	web.Serve(ws, mem, version)
}
