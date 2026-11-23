// Command rungkit is the composition root: it wires the infrastructure
// adapters into the application workspace and hands it to the CLI.
package main

import (
	"os"

	"github.com/Piyoone3939/rungkit/internal/application/workspace"
	"github.com/Piyoone3939/rungkit/internal/domain/i18n"
	"github.com/Piyoone3939/rungkit/internal/infrastructure/export/melsec"
	"github.com/Piyoone3939/rungkit/internal/infrastructure/export/st"
	"github.com/Piyoone3939/rungkit/internal/infrastructure/filestore"
	"github.com/Piyoone3939/rungkit/internal/infrastructure/jsoncodec"
	"github.com/Piyoone3939/rungkit/internal/infrastructure/ldt"
	"github.com/Piyoone3939/rungkit/internal/infrastructure/ldtest"
	"github.com/Piyoone3939/rungkit/internal/infrastructure/locale"
	"github.com/Piyoone3939/rungkit/internal/infrastructure/plcopen"
	"github.com/Piyoone3939/rungkit/internal/presentation/cli"
)

func newWorkspace(opt cli.Options) *workspace.Workspace {
	return workspace.New(workspace.Deps{
		Files:     filestore.Store{Root: opt.Root},
		Notation:  ldt.Notation{},
		Scenarios: ldtest.Parser{},
		Exporters: []workspace.Exporter{
			melsec.Exporter{},
			st.Exporter{ProgramName: opt.ProgramName},
			jsoncodec.Exporter{},
			ldt.Exporter{},
			plcopen.Exporter{ProgramName: opt.ProgramName},
		},
	})
}

func main() {
	i18n.SetLanguage(locale.Detect())
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, newWorkspace))
}
