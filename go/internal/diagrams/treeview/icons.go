package treeview

import (
	"strings"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/js"
)

// builtinPack is Mermaid's name for its built-in pack. Only `file` and `folder` are built in;
// every other icon name is offered to the host's icon resolver.
const builtinPack = "mermaid-treeview"

// glyphs are outlines on a 24 by 24 grid.
var glyphs = map[string]string{
	"folder": "M3 6.5A1.5 1.5 0 0 1 4.5 5h4.6l2.4 2.5h8A1.5 1.5 0 0 1 21 9v9a1.5 1.5 0 0 1-1.5 1.5h-15A1.5 1.5 0 0 1 3 18z",
	"file":   "M6.5 3h7L19 8.5v11a1.5 1.5 0 0 1-1.5 1.5h-11A1.5 1.5 0 0 1 5 19.5v-15A1.5 1.5 0 0 1 6.5 3zM13.5 3v5.5H19",
}

type iconSettings struct {
	showIcons       bool
	defaultIconPack string
	filenameIcons   *core.Object
	extensionIcons  *core.Object
}

// own is a string the map has as its own property.
func own(m *core.Object, key string) (string, bool) {
	s, ok := m.Value(key).(string)
	return s, ok
}

// detectIcon finds a file's icon in the configured maps. A file name wins over an extension, and
// extensions match in lower case with or without the leading dot.
func detectIcon(name string, config iconSettings) (string, bool) {
	if byName, _ := own(config.filenameIcons, name); byName != "" {
		return byName, true
	}
	if dot := strings.LastIndexByte(name, '.'); dot > 0 {
		ext := js.ToLower(name[dot:])
		if icon, ok := own(config.extensionIcons, ext); ok {
			return icon, true
		}
		return own(config.extensionIcons, ext[1:])
	}
	return "", false
}

func qualifyIcon(icon, defaultIconPack string) string {
	if strings.Contains(icon, ":") {
		return icon
	}
	if _, ok := glyphs[icon]; ok || defaultIconPack == "" {
		return builtinPack + ":" + icon
	}
	return defaultIconPack + ":" + icon
}

// getNodeIcon is the qualified icon name for a node, or "" when it shows none. An `icon()`
// annotation always wins; otherwise icons appear only with `showIcons`.
func getNodeIcon(node *TreeNode, config iconSettings) string {
	if node.Icon == "none" {
		return ""
	}
	if node.Icon != "" {
		return qualifyIcon(node.Icon, config.defaultIconPack)
	}
	if !config.showIcons {
		return ""
	}
	if !node.Directory {
		detected, _ := detectIcon(node.Name, config)
		if detected == "none" {
			return ""
		}
		if detected != "" {
			return qualifyIcon(detected, config.defaultIconPack)
		}
		return builtinPack + ":file"
	}
	return builtinPack + ":folder"
}

func iconConfig(config *core.Object) iconSettings {
	c := config.Section("treeView")
	pack, _ := c.Value("defaultIconPack").(string)
	return iconSettings{
		showIcons:       core.Truthy(c.Value("showIcons")),
		defaultIconPack: pack,
		filenameIcons:   c.Section("filenameIcons"),
		extensionIcons:  c.Section("extensionIcons"),
	}
}
