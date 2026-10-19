package git

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/js"
)

const (
	typeNormal = iota
	typeReverse
	typeHighlight
	typeMerge
	typeCherryPick
)

// Commit is a commit of the graph.
type Commit struct {
	ID      string
	Message string
	Seq     int
	Type    int
	Tags    []string
	Parents []string
	Branch  string
	// -1 when the merge gave no type.
	CustomType int
	CustomID   bool
}

// Model is a parsed gitGraph.
type Model struct {
	Title     string
	AccTitle  string
	AccDescr  string
	Direction string
	// Keyed by commit id; ids holds them in the order they first appeared.
	Commits map[string]*Commit
	ids     []string
	// The id of each branch's latest commit, or "" while the branch has none.
	Branches map[string]string
	// Branch names in drawing order: by `order`, then by creation.
	Lanes         []string
	CurrentBranch string
	Head          *Commit
	Warnings      []string
}

func fail(message string) {
	panic(&core.Error{Message: message, Code: core.CodeSemantic, Type: "gitGraph"})
}

func typeCode(name string) int {
	switch name {
	case "REVERSE":
		return typeReverse
	case "HIGHLIGHT":
		return typeHighlight
	}
	return typeNormal
}

// trimLineIndents is .replace(/\n\s+/g, '\n').
func trimLineIndents(s string) string {
	if !strings.Contains(s, "\n") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\n' {
			j := i + 1
			for j < len(s) {
				r, size := js.RuneAt(s, j)
				if !js.IsSpace(r) {
					break
				}
				j += size
			}
			b.WriteByte('\n')
			i = j
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

type branchState struct {
	head    string
	hasHead bool
}

func buildGit(ast *gitGraphAst, config *core.Object, title string) *Model {
	git := config.Section("gitGraph")
	mainName := "main"
	if s, ok := git.Value("mainBranchName").(string); ok {
		mainName = s
	}
	mainOrder := 0.0
	if n, ok := git.Value("mainBranchOrder").(float64); ok {
		mainOrder = n
	}

	commits := map[string]*Commit{}
	var ids []string
	branches := map[string]branchState{mainName: {}}
	orderNames := []string{mainName}
	orders := map[string]float64{mainName: mainOrder}
	var warnings []string
	var head *Commit
	current := mainName
	direction := "LR"
	seq := 0

	// Mermaid appends seven random hex digits to the sequence number. These digits are derived
	// from the sequence number instead, so the same text always gives the same ids.
	nextID := func() string {
		for salt := float64(seq + 1); ; salt += 0x9e3779b1 {
			h := js.ToUint32(salt) * 0x9e3779b1
			id := strconv.Itoa(seq) + "-" + fmt.Sprintf("%08x", h)[:7]
			if _, ok := commits[id]; !ok {
				return id
			}
		}
	}

	add := func(c *Commit) {
		head = c
		if _, ok := commits[c.ID]; !ok {
			ids = append(ids, c.ID)
		}
		commits[c.ID] = c
		branches[current] = branchState{c.ID, true}
	}

	checkout := func(branch string) {
		b, ok := branches[branch]
		if !ok {
			fail(`Trying to checkout branch which is not yet created. (Help try using "branch ` + branch + `")`)
		}
		current = branch
		head = nil
		if b.hasHead && b.head != "" {
			head = commits[b.head]
		}
	}

	headOf := func(branch string) *Commit {
		b := branches[branch]
		if b.hasHead && b.head != "" {
			return commits[b.head]
		}
		return nil
	}

	if ast.dir != "" {
		direction = ast.dir
	}
	for _, st := range ast.statements {
		switch st.kind {
		case stCommit:
			typ := typeNormal
			if st.typ != "" {
				typ = typeCode(st.typ)
			}
			msg := ""
			if st.message != nil {
				msg = *st.message
			}
			id := ""
			if st.id != nil {
				id = *st.id
			}
			if id == "" {
				id = nextID()
			}
			c := &Commit{ID: id, Message: msg, Seq: seq, Type: typ, Tags: slices.Clone(st.tags), Parents: []string{}, Branch: current, CustomType: -1}
			seq++
			if c.Tags == nil {
				c.Tags = []string{}
			}
			if head != nil {
				c.Parents = []string{head.ID}
			}
			if _, ok := commits[c.ID]; ok {
				warnings = append(warnings, "Commit ID "+c.ID+" already exists")
			}
			add(c)

		case stBranch:
			name := st.name
			if _, ok := branches[name]; ok {
				fail(`Trying to create an existing branch. (Help: Either use a new name if you want create a new branch or try using "checkout ` + name + `")`)
			}
			if head != nil {
				branches[name] = branchState{head.ID, true}
			} else {
				branches[name] = branchState{}
			}
			order := 0.0
			if st.order != nil {
				order = *st.order
			}
			if _, ok := orders[name]; !ok {
				orderNames = append(orderNames, name)
			}
			orders[name] = order
			checkout(name)

		case stMerge:
			other := st.branch
			customID := ""
			if st.id != nil {
				customID = *st.id
			}
			customType := -1
			if st.typ != "" {
				customType = typeCode(st.typ)
			}
			currentCommit := headOf(current)
			otherState, otherExists := branches[other]
			otherCommit := headOf(other)
			if currentCommit != nil && otherCommit != nil && currentCommit.Branch == other {
				fail("Cannot merge branch '" + other + "' into itself.")
			}
			if current == other {
				fail(`Incorrect usage of "merge". Cannot merge a branch to itself`)
			}
			if currentCommit == nil {
				fail(`Incorrect usage of "merge". Current branch (` + current + `)has no commits`)
			}
			if !otherExists {
				fail(`Incorrect usage of "merge". Branch to be merged (` + other + `) does not exist`)
			}
			if otherCommit == nil {
				fail(`Incorrect usage of "merge". Branch to be merged (` + other + `) has no commits`)
			}
			if currentCommit == otherCommit {
				fail(`Incorrect usage of "merge". Both branches have same head`)
			}
			if customID != "" {
				if _, ok := commits[customID]; ok {
					fail(`Incorrect usage of "merge". Commit with id:` + customID + ` already exists, use different custom id`)
				}
			}
			id := customID
			if id == "" {
				id = nextID()
			}
			parents := []string{}
			if head != nil {
				parents = []string{head.ID, otherState.head}
			}
			c := &Commit{
				ID:         id,
				Message:    "merged branch " + other + " into " + current,
				Seq:        seq,
				Parents:    parents,
				Branch:     current,
				Type:       typeMerge,
				CustomType: customType,
				CustomID:   customID != "",
				Tags:       slices.Clone(st.tags),
			}
			seq++
			if c.Tags == nil {
				c.Tags = []string{}
			}
			add(c)

		case stCheckout:
			checkout(st.branch)

		case stCherryPicking:
			var tags []string
			hasTags := len(st.tags) != 0
			if hasTags {
				tags = st.tags
			}
			parent := ""
			if st.parent != nil {
				parent = *st.parent
			}
			var source *Commit
			if st.id != nil && *st.id != "" {
				source = commits[*st.id]
			}
			if source == nil {
				fail(`Incorrect usage of "cherryPick". Source commit id should exist and provided`)
			}
			if parent != "" && !slices.Contains(source.Parents, parent) {
				fail("Invalid operation: The specified parent commit is not an immediate parent of the cherry-picked commit.")
			}
			if source.Type == typeMerge && parent == "" {
				fail("Incorrect usage of cherry-pick: If the source commit is a merge commit, an immediate parent commit must be specified.")
			}
			if source.Branch == current {
				fail(`Incorrect usage of "cherryPick". Source commit is already on current branch`)
			}
			cur := branches[current]
			if _, ok := commits[cur.head]; !cur.hasHead || cur.head == "" || !ok {
				fail(`Incorrect usage of "cherry-pick". Current branch (` + current + `)has no commits`)
			}
			var picked []string
			if hasTags {
				picked = []string{}
				for _, t := range tags {
					if t != "" {
						picked = append(picked, t)
					}
				}
			} else {
				tag := "cherry-pick:" + source.ID
				if source.Type == typeMerge {
					tag += "|parent:" + parent
				}
				picked = []string{tag}
			}
			parents := []string{}
			if head != nil {
				parents = []string{head.ID, source.ID}
			}
			c := &Commit{
				ID:         nextID(),
				Message:    "cherry-picked " + source.Message + " into " + current,
				Seq:        seq,
				Parents:    parents,
				Branch:     current,
				Type:       typeCherryPick,
				CustomType: -1,
				Tags:       picked,
			}
			seq++
			add(c)
		}
	}

	// A branch made without an order sorts by creation among those of order zero, as in Mermaid.
	type lane struct {
		name  string
		order float64
	}
	lanes := make([]lane, len(orderNames))
	for i, n := range orderNames {
		lanes[i] = lane{n, orders[n]}
	}
	slices.SortStableFunc(lanes, func(a, b lane) int { return js.Cmp(a.order - b.order) })
	names := make([]string, len(lanes))
	for i, l := range lanes {
		names[i] = l.name
	}

	m := &Model{
		Title:         ast.title,
		Direction:     direction,
		Commits:       commits,
		ids:           ids,
		Lanes:         names,
		CurrentBranch: current,
		Head:          head,
		Warnings:      warnings,
	}
	m.Branches = map[string]string{}
	for k, v := range branches {
		m.Branches[k] = v.head
	}
	if m.Title == "" {
		m.Title = title
	}
	if ast.accTitle != "" {
		m.AccTitle = js.TrimStart(ast.accTitle)
	}
	if ast.accDescr != "" {
		m.AccDescr = trimLineIndents(ast.accDescr)
	}
	return m
}
