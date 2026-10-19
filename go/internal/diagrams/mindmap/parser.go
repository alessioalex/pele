package mindmap

import "github.com/alessioalex/pele/go/internal/diagrams/common"

func parseMindmap(src string, db common.OutlineDb) { common.ParseOutline(src, db, false) }
