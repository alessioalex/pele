package kanban

import "github.com/alessioalex/pele/go/internal/diagrams/common"

func tokenize(src string) common.ScanTokens {
	return common.TokenizeOutline(src, true)
}

func parseKanban(src string, db common.OutlineDb) {
	common.ParseOutline(src, db, true)
}
