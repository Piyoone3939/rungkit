// Command rungkit is the composition root: it wires the infrastructure
// adapters into the application workspace and hands it to the CLI.
package main

import (
	"os"

	"rungkit/internal/application/workspace"
	"rungkit/internal/infrastructure/export/melsec"
	"rungkit/internal/infrastructure/export/st"
	"rungkit/internal/infrastructure/filestore"
	"rungkit/internal/infrastructure/jsoncodec"
	"rungkit/internal/infrastructure/ldt"
	"rungkit/internal/infrastructure/ldtest"
	"rungkit/internal/presentation/cli"
)

func newWorkspace(opt cli.Options) *workspace.Workspace {
	return workspace.New(workspace.Deps{
		Files:     filestore.Store{},
		Notation:  ldt.Notation{},
		Scenarios: ldtest.Parser{},
		Exporters: []workspace.Exporter{
			melsec.Exporter{},
			st.Exporter{ProgramName: opt.STProgramName},
			jsoncodec.Exporter{},
			ldt.Exporter{},
		},
	})
}

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, newWorkspace))
}
