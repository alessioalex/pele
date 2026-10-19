// Package layout ports src/layout: the layered (Sugiyama-style) layout of one flat graph, the
// compound layout of nested groups built on it, and the swimlane layout.
//
// Layered ranks run down the y axis; callers rotate the result for other directions. See
// src/layout/layered.ts for what the layout of edges aims for; the port keeps its structure,
// iteration order and floating-point operation order so that it gives identical numbers.
package layout

import (
	"cmp"
	"math"
	"slices"
)

type Kind uint8

const (
	KindNode Kind = iota
	KindDummy
	KindLabel
	KindStartPort
	KindEndPort
)

type LNode struct {
	W, H float64
	Kind Kind
	Pin  float64
	// How much of the side that edges meet may be shared out between them; 0 keeps them all at its middle.
	Span float64
	X, Y float64
	Rank int
}

type LEdge struct {
	Tail, Head     int
	MinLen         float64
	LabelW, LabelH float64
	TailDx, HeadDx float64
	// An end that is held where its offset puts it, and takes no part in sharing out a node's side.
	TailFixed, HeadFixed bool
	Reversed             bool
	Points               []float64
	LabelX, LabelY       float64
}

type LayeredOptions struct {
	NodeSep, EdgeSep, RankSep, PortSep float64
	// Route edges at right angles, each on a track of its own across the gap between two ranks.
	Tracks bool
	// Draw every edge straight from point to point: none is given a level to turn onto.
	Straight bool
	// The caller sets the ends of the edges on one side of a node apart itself.
	Apart bool
	// The room that the marker at the head of an edge takes; 0 when unset.
	HeadRoom float64
}

type LayeredResult struct {
	Width, Height float64
}

// NewLNode is lnode(w, h, kind).
func NewLNode(w, h float64, kind Kind) *LNode {
	return &LNode{W: w, H: h, Kind: kind}
}

// NewLEdge is ledge(tail, head, minlen, labelW, labelH); TS defaults are 1, 0, 0.
func NewLEdge(tail, head int, minlen, labelW, labelH float64) *LEdge {
	return &LEdge{Tail: tail, Head: head, MinLen: minlen, LabelW: labelW, LabelH: labelH}
}

const (
	maxSweeps    = 24
	maxDummies   = 20000
	portPitch    = 16.0
	minPitch     = 8.0
	minSlope     = 0.125
	maxGap       = 3.0
	trackMargin  = 18.0
	trackPitch   = 10.0
	trackSpread  = 14.0
	levelWide    = 2.0
	levelMargin  = 28.0
	levelPitch   = 12.0
	levelLead    = 12.0
	levelClear   = 8.0
	levelTouch   = 4.0
	crossedSlope = 0.5
	inLine       = 0.5
	snapReach    = 12.0
	leanClear    = 40.0
	maxWeighed   = 400
)

const (
	endHeld uint8 = iota
	endLeaves
	endArrives
)

func isDummy(kind Kind) bool {
	return kind == KindDummy || kind == KindLabel
}

func filled(n, v int) []int {
	s := make([]int, n)
	for i := range s {
		s[i] = v
	}
	return s
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func sortRange(list []int, from, to int, key []int) {
	for i := from + 1; i < to; i++ {
		item := list[i]
		k := key[item]
		j := i - 1
		for j >= from && (key[list[j]] > k || (key[list[j]] == k && list[j] > item)) {
			list[j+1] = list[j]
			j--
		}
		list[j+1] = item
	}
}

// pieces are the pieces of edges that each cross one gap (see Pieces in layered.ts).
type pieces struct {
	from, to []float64
	rank     []int
	lower    []int
	track    []int
	levels   []int
	wide     []uint8
}

type graph struct {
	n, real, maxRank int
	W                []float64
	KIND             []Kind
	RANK             []int
	// Segment endpoints, and where each segment attaches on them.
	EF, ET           []int
	EFD, ETD         []float64
	succStart, succE []int
	predStart, predE []int
	layerStart       []int
	order, pos       []int
	offsets          bool
}

func Layered(nodes []*LNode, edges []*LEdge, opt LayeredOptions) LayeredResult {
	real := len(nodes)
	if real == 0 {
		return LayeredResult{}
	}
	m := len(edges)

	// Every edge gets a middle rank of its own when labels or parallel edges need the room.
	step := 1
	if m > 0 {
		seen := make(map[int]struct{}, m)
		for _, e := range edges {
			a := min(e.Tail, e.Head)
			b := max(e.Tail, e.Head)
			key := a*real + b
			_, dup := seen[key]
			if e.LabelW > 0 || dup {
				step = 2
				break
			}
			seen[key] = struct{}{}
		}
	}

	Orient(nodes, edges)
	Rank(nodes, edges, step)

	span := make([]int, m)
	dummies := 0
	for ei := 0; ei < m; ei++ {
		e := edges[ei]
		span[ei] = absInt(nodes[e.Head].Rank - nodes[e.Tail].Rank)
		if span[ei] > 1 {
			dummies += span[ei] - 1
		}
	}
	direct := make([]bool, m)
	if dummies > maxDummies {
		var longest []int
		for ei := 0; ei < m; ei++ {
			if span[ei] > 1 {
				longest = append(longest, ei)
			}
		}
		slices.SortStableFunc(longest, func(a, b int) int {
			if d := cmp.Compare(span[b], span[a]); d != 0 {
				return d
			}
			return cmp.Compare(a, b)
		})
		for _, ei := range longest {
			if dummies <= maxDummies {
				break
			}
			direct[ei] = true
			dummies -= span[ei] - 1
		}
	}

	n := real + dummies
	W := make([]float64, n)
	H := make([]float64, n)
	KIND := make([]Kind, n)
	RANK := make([]int, n)
	maxRank := 0
	for i := 0; i < real; i++ {
		node := nodes[i]
		W[i] = node.W
		H[i] = node.H
		KIND[i] = node.Kind
		RANK[i] = node.Rank
		if node.Rank > maxRank {
			maxRank = node.Rank
		}
	}

	// The graph that gets ordered and positioned: every edge cut into rank-to-rank segments.
	segments := 0
	for ei := 0; ei < m; ei++ {
		if !direct[ei] {
			segments += span[ei]
		}
	}
	EF := make([]int, segments)
	ET := make([]int, segments)
	EFD := make([]float64, segments)
	ETD := make([]float64, segments)
	firstDummy := filled(m, -1)
	// Which way the edges with a label in each rank head: 1 down, 2 up, 3 both.
	heads := make([]uint8, maxRank+1)
	labelNode := filled(m, -1)
	spans := false
	for i := 0; i < real; i++ {
		if nodes[i].Span > 0 && nodes[i].Kind == KindNode {
			spans = true
		}
	}
	// What each segment's end is to the node it meets, for the ends that may move along the node's side.
	var EFK, ETK []uint8
	if spans {
		EFK = make([]uint8, segments)
		ETK = make([]uint8, segments)
	}
	firstSeg := filled(m, -1)
	lastSeg := filled(m, -1)
	offsets := false
	{
		next := real
		ce := 0
		for ei := 0; ei < m; ei++ {
			if direct[ei] {
				continue
			}
			e := edges[ei]
			a, b := e.Tail, e.Head
			aDx, bDx := e.TailDx, e.HeadDx
			aFixed, bFixed := e.TailFixed, e.HeadFixed
			if e.Reversed {
				a, b = e.Head, e.Tail
				aDx, bDx = e.HeadDx, e.TailDx
				aFixed, bFixed = e.HeadFixed, e.TailFixed
			}
			if aDx != 0 || bDx != 0 {
				offsets = true
			}
			ra := RANK[a]
			rb := RANK[b]
			labelRank := -1
			if e.LabelW > 0 {
				labelRank = ra + ((rb - ra) >> 1)
				if step == 2 && rb-ra >= 4 {
					if labelRank&1 == 1 {
						if e.Reversed {
							labelRank++
						} else {
							labelRank--
						}
					}
				} else if step == 2 && labelRank&1 == 0 {
					if e.Reversed {
						labelRank++
					} else {
						labelRank--
					}
				}
			}
			prev := a
			if rb-ra > 1 {
				firstDummy[ei] = next
			}
			firstSeg[ei] = ce
			if spans && !aFixed {
				if e.Reversed {
					EFK[ce] = endArrives
				} else {
					EFK[ce] = endLeaves
				}
			}
			for r := ra + 1; r < rb; r++ {
				d := next
				next++
				RANK[d] = r
				if r == labelRank {
					W[d] = e.LabelW
					H[d] = e.LabelH
					KIND[d] = KindLabel
					labelNode[ei] = d
					if e.Reversed {
						heads[r] |= 2
					} else {
						heads[r] |= 1
					}
				} else {
					KIND[d] = KindDummy
				}
				EF[ce] = prev
				ET[ce] = d
				if prev == a {
					EFD[ce] = aDx
				} else {
					EFD[ce] = 0
				}
				ce++
				prev = d
			}
			EF[ce] = prev
			ET[ce] = b
			if prev == a {
				EFD[ce] = aDx
			} else {
				EFD[ce] = 0
			}
			ETD[ce] = bDx
			if spans && !bFixed {
				if e.Reversed {
					ETK[ce] = endLeaves
				} else {
					ETK[ce] = endArrives
				}
			}
			lastSeg[ei] = ce
			ce++
		}
	}

	succStart := make([]int, n+1)
	predStart := make([]int, n+1)
	for e := 0; e < segments; e++ {
		succStart[EF[e]+1]++
		predStart[ET[e]+1]++
	}
	for i := 0; i < n; i++ {
		succStart[i+1] += succStart[i]
		predStart[i+1] += predStart[i]
	}
	succE := make([]int, segments)
	predE := make([]int, segments)
	{
		sf := slices.Clone(succStart[:n])
		pf := slices.Clone(predStart[:n])
		for e := 0; e < segments; e++ {
			succE[sf[EF[e]]] = e
			sf[EF[e]]++
			predE[pf[ET[e]]] = e
			pf[ET[e]]++
		}
	}

	layerStart := make([]int, maxRank+2)
	for i := 0; i < n; i++ {
		layerStart[RANK[i]+1]++
	}
	for r := 0; r <= maxRank; r++ {
		layerStart[r+1] += layerStart[r]
	}
	order := make([]int, n)
	pos := make([]int, n)

	// Initial order: depth-first from the top so that subtrees start out contiguous.
	{
		byRank := make([]int, n)
		fill := slices.Clone(layerStart[:maxRank+1])
		for i := 0; i < n; i++ {
			byRank[fill[RANK[i]]] = i
			fill[RANK[i]]++
		}
		copy(fill, layerStart[:maxRank+1])
		visited := make([]bool, n)
		var stack []int
		for k := 0; k < n; k++ {
			if visited[byRank[k]] {
				continue
			}
			stack = append(stack, byRank[k])
			for len(stack) > 0 {
				v := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				if visited[v] {
					continue
				}
				visited[v] = true
				order[fill[RANK[v]]] = v
				fill[RANK[v]]++
				for j := succStart[v+1] - 1; j >= succStart[v]; j-- {
					w := ET[succE[j]]
					if !visited[w] {
						stack = append(stack, w)
					}
				}
			}
		}
	}

	pinned := false
	for i := 0; i < real; i++ {
		if nodes[i].Pin != 0 {
			pinned = true
		}
	}
	if pinned {
		pinOf := func(v int) float64 {
			if v < real {
				return nodes[v].Pin
			}
			return 0
		}
		index := make([]int, n)
		for r := 0; r <= maxRank; r++ {
			layer := slices.Clone(order[layerStart[r]:layerStart[r+1]])
			for i, v := range layer {
				index[v] = i
			}
			slices.SortStableFunc(layer, func(a, b int) int {
				if d := jsCmp(pinOf(a) - pinOf(b)); d != 0 {
					return d
				}
				return cmp.Compare(index[a], index[b])
			})
			copy(order[layerStart[r]:], layer)
		}
	}
	setPositions := func() {
		for r := 0; r <= maxRank; r++ {
			s := layerStart[r]
			for i, e := s, layerStart[r+1]; i < e; i++ {
				pos[order[i]] = i - s
			}
		}
	}
	setPositions()

	g := &graph{n: n, real: real, maxRank: maxRank, W: W, KIND: KIND, RANK: RANK, EF: EF, ET: ET, EFD: EFD, ETD: ETD,
		succStart: succStart, succE: succE, predStart: predStart, predE: predE, layerStart: layerStart, order: order, pos: pos, offsets: offsets}

	if segments > 0 && maxRank > 0 {
		reduceCrossings(g, nodes)
		setPositions()
	}

	if spans {
		spread(g, nodes, EFK, ETK)
	}

	xs := position(g, opt)

	empty := emptyRanks(g)
	if spans {
		free := freeEnds(g, EFK, ETK)
		alignLabels(g, nodes, xs, free, opt, empty)
		settle(g, nodes, xs, EFK, ETK, free, empty)
		snap(g, xs, empty, opt)
		for ei := 0; ei < m; ei++ {
			if direct[ei] {
				continue
			}
			e := edges[ei]
			aDx := EFD[firstSeg[ei]]
			bDx := ETD[lastSeg[ei]]
			if e.Reversed {
				e.TailDx, e.HeadDx = bDx, aDx
			} else {
				e.TailDx, e.HeadDx = aDx, bDx
			}
		}
	}

	// A rank that holds nothing but the bends of edges is only there because some other edge has a
	// label or a twin; its bends move onto the line between the ends of their edges.
	onLine := make([]uint8, n)
	if empty != nil && !opt.Tracks {
		sep := opt.EdgeSep
		if opt.Apart {
			sep = 0
		}
		straighten(g, xs, empty, firstDummy, span, sep, onLine)
	}
	if !opt.Tracks {
		pastLabels(g, xs, firstDummy, span, opt.EdgeSep, onLine)
	}
	// An edge is cut where it passes a rank that holds something, and each piece crosses one gap.
	pieceStart := make([]int, m+1)
	var pc *pieces
	if opt.Tracks || (spans && !opt.Straight) {
		pc = &pieces{}
	}
	trackCount := make([]int, maxRank+1)
	if pc != nil {
		for ei := 0; ei < m; ei++ {
			pieceStart[ei] = len(pc.from)
			if direct[ei] {
				continue
			}
			d0 := firstDummy[ei]
			// An edge that keeps a bend to stay clear of its twin is drawn through the bend, as it is.
			twin := false
			end := d0
			if d0 >= 0 {
				end = d0 + span[ei] - 1
			}
			for d := d0; d < end; d++ {
				// So is one that has a bend left out in a rank of labels: its curve crosses that rank.
				bare := empty != nil && empty[RANK[d]] == 1
				if !opt.Tracks && bare != (onLine[d] == 1) {
					twin = true
				}
			}
			if twin {
				continue
			}
			x := xs[EF[firstSeg[ei]]] + EFD[firstSeg[ei]]
			rank := RANK[EF[firstSeg[ei]]]
			for d := d0; d < end; d++ {
				if empty != nil && empty[RANK[d]] != 0 {
					continue
				}
				pc.from = append(pc.from, x)
				pc.to = append(pc.to, xs[d])
				pc.rank = append(pc.rank, rank)
				pc.lower = append(pc.lower, d)
				x = xs[d]
				rank = RANK[d]
			}
			pc.from = append(pc.from, x)
			pc.to = append(pc.to, xs[ET[lastSeg[ei]]]+ETD[lastSeg[ei]])
			pc.rank = append(pc.rank, rank)
			pc.lower = append(pc.lower, -1)
		}
		pieceStart[m] = len(pc.from)
	}

	// Ranks that hold only border ports or their pass-through dummies need little room of their own.
	portOnly := func(r int, kind Kind) bool {
		found := false
		for i := layerStart[r]; i < layerStart[r+1]; i++ {
			k := KIND[order[i]]
			if k == kind {
				found = true
			} else if k != KindDummy {
				return false
			}
		}
		return found
	}
	startRanks := 0
	if portOnly(0, KindStartPort) {
		startRanks = step
	}
	endRanks := 0
	if portOnly(maxRank, KindEndPort) {
		endRanks = step
	}
	fstep := float64(step)
	// The longest way across that any edge travels in the gap below each rank.
	run := make([]float64, maxRank+1)
	for e := 0; e < segments; e++ {
		across := math.Abs(xs[EF[e]] + EFD[e] - xs[ET[e]] - ETD[e])
		if across > run[RANK[EF[e]]] {
			run[RANK[EF[e]]] = across
		}
	}
	bandTop := make([]float64, maxRank+1)
	bandBottom := make([]float64, maxRank+1)
	usual := func(r int) float64 {
		if r < startRanks || r >= maxRank-endRanks {
			return opt.PortSep / fstep
		}
		return opt.RankSep / fstep
	}
	// The room under each rank. Edges that are not cut into pieces need more where they run far across.
	gapUnder := make([]float64, maxRank+1)
	for r := 0; r < maxRank; r++ {
		if pc != nil {
			gapUnder[r] = usual(r)
		} else {
			gapUnder[r] = max(usual(r), min((maxGap*opt.RankSep)/fstep, run[r]*minSlope))
		}
	}
	// A rank between two ranks of nodes, which holds labels and bends only.
	between := make([]uint8, maxRank+1)
	for r := range between {
		between[r] = 1
	}
	for i := 0; i < real; i++ {
		between[RANK[i]] = 0
	}
	for r := 1; r < maxRank; r++ {
		if opt.Tracks || between[r] == 0 || (heads[r] != 1 && heads[r] != 2) {
			continue
		}
		at := r
		if heads[r] == 1 {
			at = r - 1
		}
		shift := min(opt.HeadRoom/2, gapUnder[at]/2)
		if heads[r] == 1 {
			gapUnder[r-1] += -shift
			gapUnder[r] += shift
		} else {
			gapUnder[r-1] += shift
			gapUnder[r] += -shift
		}
	}
	// The part of its rank that an edge runs straight through at a bend or label.
	inset := func(d int) float64 {
		if KIND[d] == KindDummy && between[RANK[d]] != 0 && !opt.Tracks {
			return (bandBottom[RANK[d]] - bandTop[RANK[d]]) / 4
		}
		return 0
	}
	margin, leastPitch, mostPitch := levelMargin, levelPitch, levelPitch
	if opt.Tracks {
		margin, leastPitch, mostPitch = trackMargin, trackPitch, trackSpread
	}
	below := make([]int, maxRank+1)
	if pc != nil {
		room := make([]float64, maxRank+1)
		for r := 0; r < maxRank; r++ {
			if empty != nil && empty[r] != 0 {
				continue
			}
			next := r + 1
			room[r] = gapUnder[r]
			for next < maxRank && empty != nil && empty[next] != 0 {
				room[r] += gapUnder[next]
				next++
			}
			below[r] = next
		}
		pc.wide = make([]uint8, 0, len(pc.from))
		for k := range pc.from {
			if math.Abs(pc.from[k]-pc.to[k]) > levelWide*room[pc.rank[k]] {
				pc.wide = append(pc.wide, 1)
			} else {
				pc.wide = append(pc.wide, 0)
			}
		}
		tall := make([]float64, maxRank+1)
		assignTracks(pc, trackCount, !opt.Tracks, room, tall)
		for r := 0; r < maxRank; r++ {
			if trackCount[r] > 0 {
				gapUnder[r] += max(0, 2*margin+float64(float64(trackCount[r]-1)*leastPitch)-room[r])
			} else if tall[r] > 0 {
				gapUnder[r] += max(0, min(maxGap*opt.RankSep, tall[r])-room[r])
			}
		}
	}
	y := 0.0
	for r := 0; r <= maxRank; r++ {
		h := 0.0
		for i := layerStart[r]; i < layerStart[r+1]; i++ {
			if H[order[i]] > h {
				h = H[order[i]]
			}
		}
		bandTop[r] = y
		bandBottom[r] = y + h
		y += h + gapUnder[r]
	}
	height := y
	// Where a piece runs across its gap, or would on another track of its group.
	trackY := func(piece, onTrack int) float64 {
		r := pc.rank[piece]
		top := bandBottom[r]
		bottom := bandTop[below[r]]
		count := pc.levels[piece]
		if opt.Tracks {
			count = trackCount[r]
		}
		pitch := 0.0
		if count > 1 {
			pitch = min(mostPitch, (bottom-top-2*margin)/float64(count-1))
		}
		return (top+bottom)/2 + float64((float64(onTrack)-float64(count-1)/2)*pitch)
	}

	minX := math.Inf(1)
	maxX := math.Inf(-1)
	for i := 0; i < n; i++ {
		half := W[i] / 2
		if xs[i]-half < minX {
			minX = xs[i] - half
		}
		if xs[i]+half > maxX {
			maxX = xs[i] + half
		}
	}
	// Where edges spread along their nodes, a rank whose nodes only send edges on stands on one
	// line, and a rank whose nodes only receive them hangs from one.
	arrives := make([]bool, maxRank+1)
	leaves := make([]bool, maxRank+1)
	for i := 0; i < real; i++ {
		if KIND[i] != KindNode || !spans {
			continue
		}
		if predStart[i+1] > predStart[i] {
			arrives[RANK[i]] = true
		}
		if succStart[i+1] > succStart[i] {
			leaves[RANK[i]] = true
		}
	}
	for i := 0; i < real; i++ {
		node := nodes[i]
		r := node.Rank
		node.X = xs[i] - minX
		if KIND[i] == KindNode && leaves[r] && !arrives[r] {
			node.Y = bandBottom[r] - node.H/2
		} else if KIND[i] == KindNode && arrives[r] && !leaves[r] {
			node.Y = bandTop[r] + node.H/2
		} else {
			node.Y = (bandTop[r] + bandBottom[r]) / 2
		}
	}

	for ei := 0; ei < m; ei++ {
		e := edges[ei]
		first, last := nodes[e.Tail], nodes[e.Head]
		firstDx, lastDx := e.TailDx, e.HeadDx
		if e.Reversed {
			first, last = nodes[e.Head], nodes[e.Tail]
			firstDx, lastDx = e.HeadDx, e.TailDx
		}
		pts := []float64{first.X + firstDx, first.Y + first.H/2}
		d0 := firstDummy[ei]
		if pc != nil && pieceStart[ei+1] > pieceStart[ei] {
			for k := pieceStart[ei]; k < pieceStart[ei+1]; k++ {
				if pc.track[k] >= 0 && (opt.Tracks || pc.wide[k] == 1) {
					across := trackY(k, pc.track[k])
					pts = append(pts, pc.from[k]-minX, across, pc.to[k]-minX, across)
				} else if pc.track[k] > 0 {
					// A short piece among levels runs straight until it is past the level before its
					// own, and turns in one S from there.
					pts = append(pts, pc.from[k]-minX, trackY(k, pc.track[k]-1))
				}
				d := pc.lower[k]
				if d < 0 {
					continue
				}
				pts = append(pts, xs[d]-minX, bandTop[RANK[d]]+inset(d))
				if bandBottom[RANK[d]] > bandTop[RANK[d]] {
					pts = append(pts, xs[d]-minX, bandBottom[RANK[d]]-inset(d))
				}
			}
		} else if d0 >= 0 {
			for d, end := d0, d0+span[ei]-1; d < end; d++ {
				if onLine[d] != 0 {
					continue
				}
				x := xs[d] - minX
				top := bandTop[RANK[d]]
				bottom := bandBottom[RANK[d]]
				pts = append(pts, x, top+inset(d))
				if bottom > top {
					pts = append(pts, x, bottom-inset(d))
				}
			}
		}
		pts = append(pts, last.X+lastDx, last.Y-last.H/2)
		if e.Reversed {
			for i, j := 0, len(pts)-2; i < j; i, j = i+2, j-2 {
				pts[i], pts[j] = pts[j], pts[i]
				pts[i+1], pts[j+1] = pts[j+1], pts[i+1]
			}
		}
		e.Points = pts
		ln := labelNode[ei]
		if ln >= 0 {
			e.LabelX = xs[ln] - minX
			e.LabelY = (bandTop[RANK[ln]] + bandBottom[RANK[ln]]) / 2
		} else if e.LabelW > 0 {
			e.LabelX = (pts[0] + pts[len(pts)-2]) / 2
			e.LabelY = (pts[1] + pts[len(pts)-1]) / 2
		}
	}

	return LayeredResult{Width: maxX - minX, Height: height}
}

// jsCmp is a numeric sort comparator's result as Array.prototype.sort reads it: NaN is equal.
func jsCmp(d float64) int {
	switch {
	case d < 0:
		return -1
	case d > 0:
		return 1
	}
	return 0
}

// Orient reverses the edges that close a cycle, found by depth-first search in declaration order.
func Orient(nodes []*LNode, edges []*LEdge) {
	n := len(nodes)
	m := len(edges)
	start := make([]int, n+1)
	for i := 0; i < m; i++ {
		e := edges[i]
		e.Reversed = nodes[e.Head].Kind == KindStartPort || nodes[e.Tail].Kind == KindEndPort
		if !e.Reversed {
			start[e.Tail+1]++
		}
	}
	for i := 0; i < n; i++ {
		start[i+1] += start[i]
	}
	out := make([]int, start[n])
	{
		fill := slices.Clone(start[:n])
		for i := 0; i < m; i++ {
			if !edges[i].Reversed {
				out[fill[edges[i].Tail]] = i
				fill[edges[i].Tail]++
			}
		}
	}
	state := make([]uint8, n)
	cursor := slices.Clone(start[:n])
	var stack []int
	for root := 0; root < n; root++ {
		if state[root] != 0 {
			continue
		}
		stack = append(stack, root)
		state[root] = 1
		for len(stack) > 0 {
			v := stack[len(stack)-1]
			if cursor[v] < start[v+1] {
				e := edges[out[cursor[v]]]
				cursor[v]++
				w := e.Head
				if state[w] == 1 {
					e.Reversed = true
				} else if state[w] == 0 {
					state[w] = 1
					stack = append(stack, w)
				}
			} else {
				state[v] = 2
				stack = stack[:len(stack)-1]
			}
		}
	}
}

// Rank gives every node its rank by longest path, then moves loosely attached nodes toward the
// side that has more edges.
func Rank(nodes []*LNode, edges []*LEdge, step int) {
	n := len(nodes)
	m := len(edges)
	from := make([]int, m)
	to := make([]int, m)
	length := make([]int, m)
	outStart := make([]int, n+1)
	inStart := make([]int, n+1)
	for i := 0; i < m; i++ {
		e := edges[i]
		a, b := e.Tail, e.Head
		if e.Reversed {
			a, b = e.Head, e.Tail
		}
		from[i] = a
		to[i] = b
		// Stored in an Int32Array.
		length[i] = int(toInt32(max(1, e.MinLen) * float64(step)))
		outStart[a+1]++
		inStart[b+1]++
	}
	for i := 0; i < n; i++ {
		outStart[i+1] += outStart[i]
		inStart[i+1] += inStart[i]
	}
	outE := make([]int, m)
	inE := make([]int, m)
	{
		of := slices.Clone(outStart[:n])
		inf := slices.Clone(inStart[:n])
		for i := 0; i < m; i++ {
			outE[of[from[i]]] = i
			of[from[i]]++
			inE[inf[to[i]]] = i
			inf[to[i]]++
		}
	}

	r := make([]int, n)
	indeg := make([]int, n)
	for i := 0; i < n; i++ {
		indeg[i] = inStart[i+1] - inStart[i]
	}
	topo := make([]int, n)
	count := 0
	for i := 0; i < n; i++ {
		if indeg[i] == 0 {
			topo[count] = i
			count++
		}
	}
	for q := 0; q < count; q++ {
		v := topo[q]
		for k := outStart[v]; k < outStart[v+1]; k++ {
			ei := outE[k]
			w := to[ei]
			if r[v]+length[ei] > r[w] {
				r[w] = r[v] + length[ei]
			}
			indeg[w]--
			if indeg[w] == 0 {
				topo[count] = w
				count++
			}
		}
	}

	for pass := 0; pass < 8; pass++ {
		changed := false
		for q := count - 1; q >= 0; q-- {
			v := topo[q]
			ins := inStart[v+1] - inStart[v]
			outs := outStart[v+1] - outStart[v]
			if ins == 0 && outs == 0 {
				continue
			}
			lo := math.Inf(-1)
			hi := math.Inf(1)
			for k := inStart[v]; k < inStart[v+1]; k++ {
				ei := inE[k]
				lo = max(lo, float64(r[from[ei]]+length[ei]))
			}
			for k := outStart[v]; k < outStart[v+1]; k++ {
				ei := outE[k]
				hi = min(hi, float64(r[to[ei]]-length[ei]))
			}
			next := r[v]
			if outs > ins && !math.IsInf(hi, 1) {
				next = int(hi)
			} else if ins > outs && !math.IsInf(lo, -1) {
				next = int(lo)
			}
			if next != r[v] {
				r[v] = next
				changed = true
			}
		}
		if !changed {
			break
		}
	}

	lo := math.MaxInt
	hi := math.MinInt
	startPorts := false
	for i := 0; i < n; i++ {
		kind := nodes[i].Kind
		if kind == KindStartPort {
			startPorts = true
		}
		if kind == KindStartPort || kind == KindEndPort {
			continue
		}
		if r[i] < lo {
			lo = r[i]
		}
		if r[i] > hi {
			hi = r[i]
		}
	}
	if lo == math.MaxInt {
		lo = 0
		hi = 0
	}
	for i := 0; i < n; i++ {
		if nodes[i].Kind == KindStartPort {
			r[i] = lo - step
		} else if nodes[i].Kind == KindEndPort {
			r[i] = hi + step
		}
	}
	if startPorts {
		lo -= step
	}
	for i := 0; i < n; i++ {
		nodes[i].Rank = r[i] - lo
	}
}

// toInt32 is the conversion of a store into an Int32Array.
func toInt32(x float64) int32 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return 0
	}
	x = math.Trunc(x)
	if x >= -2147483648 && x <= 2147483647 {
		return int32(x)
	}
	return int32(uint32(int64(math.Mod(x, 4294967296))))
}

// sideEnds lists in ids the ends that may move along one side of a node (0 the side its edges
// arrive at, 1 the other), in the order of the nodes at their other ends. room receives the least
// distance between each end and the one before it, and the result is their sum.
func sideEnds(g *graph, v, side int, span float64, kinds []uint8, ids *[]int, room *[]float64) float64 {
	start, list, other := g.succStart, g.succE, g.ET
	if side == 0 {
		start, list, other = g.predStart, g.predE, g.EF
	}
	pos := g.pos
	*ids = (*ids)[:0]
	for k := start[v]; k < start[v+1]; k++ {
		if kinds[list[k]] != endHeld {
			*ids = append(*ids, list[k])
		}
	}
	id := *ids
	if len(id) < 2 {
		return 0
	}
	slices.SortStableFunc(id, func(a, b int) int {
		if d := cmp.Compare(pos[other[a]], pos[other[b]]); d != 0 {
			return d
		}
		return cmp.Compare(a, b)
	})
	fill := func(share func(leaves, same bool) float64) float64 {
		total := 0.0
		rm := (*room)[:0]
		rm = append(rm, 0)
		for k := 1; k < len(id); k++ {
			same := kinds[id[k]] == kinds[id[k-1]]
			rm = append(rm, share(same && kinds[id[k]] == endLeaves, same))
			total += rm[k]
		}
		*room = rm
		return total
	}
	total := fill(func(bool, bool) float64 { return 1 })
	if span/total < minPitch {
		total = fill(func(leaves, _ bool) float64 {
			if leaves {
				return 0.5
			}
			return 1
		})
	}
	if span/total < minPitch {
		total = fill(func(_, same bool) float64 {
			if same {
				return 0
			}
			return 1
		})
	}
	if total == 0 {
		return 0
	}
	pitch := min(portPitch, span/total)
	rm := *room
	for k := 1; k < len(id); k++ {
		rm[k] *= pitch
	}
	return total * pitch
}

// spread gives the edges that meet one side of a node places of their own along it, around its middle.
func spread(g *graph, nodes []*LNode, EFK, ETK []uint8) {
	var ids []int
	var room []float64
	for v := 0; v < g.real; v++ {
		span := min(nodes[v].Span, g.W[v])
		if span <= 0 || nodes[v].Kind != KindNode {
			continue
		}
		for side := 0; side < 2; side++ {
			kinds := EFK
			if side == 0 {
				kinds = ETK
			}
			total := sideEnds(g, v, side, span, kinds, &ids, &room)
			if total == 0 {
				continue
			}
			dx := g.EFD
			if side == 0 {
				dx = g.ETD
			}
			at := -total / 2
			for k := 0; k < len(ids); k++ {
				at += room[k]
				dx[ids[k]] = at
			}
		}
	}
}

// follow gives the segment of an edge that reaches the next point held in place, on the way up
// from segment e (side 0) or down: past the bends in ranks that hold nothing else.
func follow(g *graph, empty []uint8, e, side int) int {
	if empty == nil {
		return e
	}
	for hops := 0; hops < 100000; hops++ {
		u := g.ET[e]
		if side == 0 {
			u = g.EF[e]
		}
		if g.KIND[u] != KindDummy || empty[g.RANK[u]] == 0 {
			break
		}
		if side == 0 {
			e = g.predE[g.predStart[u]]
		} else {
			e = g.succE[g.succStart[u]]
		}
	}
	return e
}

// freeEnds counts the ends that may move along it that each node has on the side its edges
// arrive at, and on the other.
func freeEnds(g *graph, EFK, ETK []uint8) [2][]int {
	free := [2][]int{make([]int, g.n), make([]int, g.n)}
	for e := range g.EF {
		if ETK[e] != endHeld {
			free[0][g.ET[e]]++
		}
		if EFK[e] != endHeld {
			free[1][g.EF[e]]++
		}
	}
	return free
}

// Ease is how far along the flow the handles of a plain S reach, as a share of the way it travels
// along it.
func Ease(across, along float64) float64 {
	d := math.Abs(across)
	if d == 0 || math.IsNaN(d) {
		d = 1
	}
	return max(0.25, min(0.5, (4*math.Abs(along))/d))
}

// alignLabels brings the labels and bends of each edge into line with an end of the edge.
func alignLabels(g *graph, nodes []*LNode, xs []float64, free [2][]int, opt LayeredOptions, empty []uint8) {
	sep := opt.EdgeSep
	n, real, maxRank := g.n, g.real, g.maxRank
	KIND, RANK, W, EF, ET, EFD, ETD := g.KIND, g.RANK, g.W, g.EF, g.ET, g.EFD, g.ETD
	succStart, succE, predStart, predE, layerStart, order, pos := g.succStart, g.succE, g.predStart, g.predE, g.layerStart, g.order, g.pos
	placed := func(v int) bool {
		return KIND[v] == KindLabel || (KIND[v] == KindDummy && !(empty != nil && empty[RANK[v]] == 1))
	}
	// Where segment e ends on the way up (side 0) or down, and how many ends share that side of
	// the node there.
	end := func(e, side int) (float64, int) {
		u := ET[e]
		x := xs[u] + ETD[e]
		if side == 0 {
			u = EF[e]
			x = xs[u] + EFD[e]
		}
		if u < real && nodes[u].Kind == KindNode {
			return x, free[1-side][u]
		}
		return x, 0
	}
	// The room the neighbours in its rank leave a label or bend.
	room := func(v int) (float64, float64) {
		s := layerStart[RANK[v]]
		i := s + pos[v]
		clear := func(other int) float64 {
			var c float64
			if KIND[other] == KindNode {
				c = opt.NodeSep/2 + sep/2
			} else if KIND[other] == KIND[v] {
				c = sep
			} else {
				c = sep / 2
			}
			return W[other]/2 + W[v]/2 + c
		}
		lo, hi := math.Inf(-1), math.Inf(1)
		if i > s {
			lo = xs[order[i-1]] + clear(order[i-1])
		}
		if i+1 < layerStart[RANK[v]+1] {
			hi = xs[order[i+1]] - clear(order[i+1])
		}
		return lo, hi
	}
	choose := func(x, upX float64, upN int, downX float64, downN int, lo, hi float64) float64 {
		var upFirst bool
		if upN != downN {
			upFirst = upN < downN
		} else {
			upFirst = math.Abs(upX-x) <= math.Abs(downX-x)
		}
		c0, c1 := downX, upX
		if upFirst {
			c0, c1 = upX, downX
		}
		if c0 >= lo && c0 <= hi {
			return c0
		}
		if c1 >= lo && c1 <= hi {
			return c1
		}
		return max(lo, min(hi, c0))
	}
	done := make([]bool, n)
	var chain []int
	for r := 1; r < maxRank; r++ {
		for i := layerStart[r]; i < layerStart[r+1]; i++ {
			if !placed(order[i]) || done[order[i]] {
				continue
			}
			// The edge's labels and bends from its first node to its last, and the room they share.
			top := order[i]
			for isDummy(KIND[EF[predE[predStart[top]]]]) {
				top = EF[predE[predStart[top]]]
			}
			bottom := top
			lo := math.Inf(-1)
			hi := math.Inf(1)
			chain = chain[:0]
			for d := top; isDummy(KIND[d]); d = ET[succE[succStart[d]]] {
				bottom = d
				if !placed(d) {
					continue
				}
				done[d] = true
				chain = append(chain, d)
				least, most := room(d)
				if least > lo {
					lo = least
				}
				if most < hi {
					hi = most
				}
			}
			if lo <= hi {
				ux, un := end(predE[predStart[top]], 0)
				dx, dn := end(succE[succStart[bottom]], 1)
				x := choose(xs[chain[0]], ux, un, dx, dn, lo, hi)
				for _, d := range chain {
					xs[d] = x
				}
				continue
			}
			for _, d := range chain {
				least, most := room(d)
				if least > most {
					continue
				}
				ux, un := end(follow(g, empty, predE[predStart[d]], 0), 0)
				dx, dn := end(follow(g, empty, succE[succStart[d]], 1), 1)
				xs[d] = choose(xs[d], ux, un, dx, dn, least, most)
			}
		}
	}
}

// settle moves an end along its side when that lets its edge run straight, once the nodes have
// their places; any other end leans towards the point its edge runs to.
func settle(g *graph, nodes []*LNode, xs []float64, EFK, ETK []uint8, free [2][]int, empty []uint8) {
	real, W, EF, ET, EFD, ETD := g.real, g.W, g.EF, g.ET, g.EFD, g.ETD
	succStart, succE, predStart, predE, pos := g.succStart, g.succE, g.predStart, g.predE, g.pos
	var ids []int
	var room []float64
	var all []int
	var sum []float64
	var count []float64
	// Where the end of segment e on node v should be, from the middle of its side.
	wanted := func(e, side, v int, span float64, again bool) float64 {
		if free[side][v] < 2 {
			return 0
		}
		e = follow(g, empty, e, side)
		u := ET[e]
		point := xs[u] + ETD[e]
		kinds := ETK
		if side == 0 {
			u = EF[e]
			point = xs[u] + EFD[e]
			kinds = EFK
		}
		// Once the other end has its place, this one comes into line with it if it can.
		if again && math.Abs(point-xs[v]) <= span/2+inLine {
			return max(-span/2, min(span/2, point-xs[v]))
		}
		// The part of the other side that its end may move along, if it may move.
		middle := point
		width := 0.0
		if u < real && nodes[u].Kind == KindNode && kinds[e] != endHeld && free[1-side][u] >= 2 {
			width = max(0, min(nodes[u].Span, W[u]))
			if width > 0 {
				middle = xs[u]
			}
		}
		lo := max(xs[v]-span/2, middle-width/2)
		hi := min(xs[v]+span/2, middle+width/2)
		// Two sides with no part in line: the end leans towards the other, if its side is long enough.
		if lo > hi+inLine {
			lean := max(0, span/2-leanClear)
			return max(-lean, min(lean, point-xs[v]))
		}
		// Both ends settle on one place in the part the two sides share: nearer the middle of the narrower.
		place := (float64(xs[v]*width) + float64(middle*span)) / (width + span)
		return max(lo, min(hi, place)) - xs[v]
	}
	moved := true
	for pass := 0; moved && pass < 6; pass++ {
		moved = false
		for v := 0; v < real; v++ {
			span := min(nodes[v].Span, W[v])
			if span <= 0 || nodes[v].Kind != KindNode {
				continue
			}
			for side := 0; side < 2; side++ {
				kinds, dx, other, start, list := EFK, EFD, ET, succStart, succE
				if side == 0 {
					kinds, dx, other, start, list = ETK, ETD, EF, predStart, predE
				}
				sideEnds(g, v, side, span, kinds, &ids, &room)
				if len(ids) == 0 {
					continue
				}
				if len(ids) == 1 {
					if len(room) == 0 {
						room = append(room, 0)
					}
					room[0] = 0
				}
				// Every end on the side in order, the held ones among them, which divide the rest into runs.
				all = all[:0]
				for k := start[v]; k < start[v+1]; k++ {
					all = append(all, list[k])
				}
				slices.SortStableFunc(all, func(a, b int) int {
					if d := cmp.Compare(pos[other[a]], pos[other[b]]); d != 0 {
						return d
					}
					return cmp.Compare(a, b)
				})
				k := 0
				lo := -span / 2
				for i := 0; i <= len(all); i++ {
					held := i < len(all) && kinds[all[i]] == endHeld
					if i < len(all) && !held {
						continue
					}
					size := 0
					for j := i - 1; j >= 0 && kinds[all[j]] != endHeld; j-- {
						size++
					}
					hi := span / 2
					if held {
						hi = dx[all[i]] - portPitch
					}
					if size > 0 {
						// The places nearest the wanted ones that keep the order and the room between
						// neighbours: an isotonic fit, by pooling neighbours that would otherwise be too close.
						sum = sum[:0]
						count = count[:0]
						before := 0.0
						for j := 0; j < size; j++ {
							if j > 0 {
								before += room[k+j]
							}
							sum = append(sum, wanted(ids[k+j], side, v, span, pass > 0)-before)
							count = append(count, 1)
							// Ends that share a place for want of room stay together.
							together := j > 0 && room[k+j] == 0
							for len(sum) > 1 && (together || sum[len(sum)-2]/count[len(count)-2] > sum[len(sum)-1]/count[len(count)-1]) {
								together = false
								top := sum[len(sum)-1]
								sum = sum[:len(sum)-1]
								pooled := count[len(count)-1]
								count = count[:len(count)-1]
								sum[len(sum)-1] += top
								count[len(count)-1] += pooled
							}
						}
						total := before
						j := 0
						before = 0
						for b := 0; b < len(sum); b++ {
							// With too little room between two held ends, the run sits in the middle of what there is.
							var level float64
							if hi-total < lo {
								level = (lo + hi - total) / 2
							} else {
								level = max(lo, min(hi-total, sum[b]/count[b]))
							}
							for c := 0; float64(c) < count[b]; c, j = c+1, j+1 {
								if j > 0 {
									before += room[k+j]
								}
								if math.Abs(dx[ids[k+j]]-level-before) > 0.01 {
									moved = true
								}
								dx[ids[k+j]] = level + before
							}
						}
						k += size
					}
					if held {
						lo = dx[all[i]] + portPitch
					}
				}
			}
		}
	}
}

// halfRoom is half the room a node, bend or port takes across the flow, with its share of the gap beside it.
func halfRoom(g *graph, opt LayeredOptions, v int) float64 {
	if g.KIND[v] == KindNode {
		return g.W[v]/2 + opt.NodeSep/2
	}
	return g.W[v]/2 + opt.EdgeSep/2
}

// snap brings edges that are nearly straight into line, by moving a node a little across the flow
// along with everything it is in line with already, where the nodes beside them leave room.
func snap(g *graph, xs []float64, empty []uint8, opt LayeredOptions) {
	n, KIND, RANK, EF, ET, EFD, ETD := g.n, g.KIND, g.RANK, g.EF, g.ET, g.EFD, g.ETD
	layerStart, order, pos := g.layerStart, g.order, g.pos
	passed := func(u int) bool {
		return empty != nil && KIND[u] == KindDummy && empty[RANK[u]] == 1
	}
	var a, b []int
	var da, db []float64
	for e := range EF {
		if passed(EF[e]) {
			continue
		}
		f := follow(g, empty, e, 1)
		a = append(a, EF[e])
		b = append(b, ET[f])
		da = append(da, EFD[e])
		db = append(db, ETD[f])
	}
	off := func(k int) float64 { return xs[b[k]] + db[k] - xs[a[k]] - da[k] }
	// The nodes in line with each other move as one.
	block := make([]int, n)
	members := make([][]int, n)
	for i := 0; i < n; i++ {
		block[i] = i
		members[i] = []int{i}
	}
	join := func(x, y int) {
		keep := block[x]
		drop := block[y]
		if keep == drop {
			return
		}
		if len(members[keep]) < len(members[drop]) {
			keep, drop = drop, keep
		}
		for _, v := range members[drop] {
			block[v] = keep
			members[keep] = append(members[keep], v)
		}
		members[drop] = nil
	}
	var near []int
	for k := range a {
		d := math.Abs(off(k))
		if d < 0.01 {
			join(a[k], b[k])
		} else if d <= snapReach {
			near = append(near, k)
		}
	}
	if len(near) == 0 {
		return
	}
	slices.SortStableFunc(near, func(x, y int) int {
		if d := jsCmp(math.Abs(off(x)) - math.Abs(off(y))); d != 0 {
			return d
		}
		return cmp.Compare(x, y)
	})
	gap := func(v int) float64 { return halfRoom(g, opt, v) }
	// Whether every node of a block can move by shift and keep its room from those beside it.
	free := func(id int, shift float64) bool {
		for _, v := range members[id] {
			s := layerStart[RANK[v]]
			end := layerStart[RANK[v]+1]
			i := s + pos[v]
			if i > s && block[order[i-1]] != id && xs[v]+shift-xs[order[i-1]] < gap(v)+gap(order[i-1])-0.01 {
				return false
			}
			if i+1 < end && block[order[i+1]] != id && xs[order[i+1]]-xs[v]-shift < gap(v)+gap(order[i+1])-0.01 {
				return false
			}
		}
		return true
	}
	for _, k := range near {
		d := off(k)
		if math.Abs(d) < 0.01 {
			join(a[k], b[k])
			continue
		}
		if math.Abs(d) > snapReach || block[a[k]] == block[b[k]] {
			continue
		}
		// The smaller of the two blocks moves, if it can; else the other.
		upper := block[a[k]]
		lower := block[b[k]]
		first := lower
		if len(members[upper]) <= len(members[lower]) {
			first = upper
		}
		second := upper
		if first == upper {
			second = lower
		}
		for _, id := range [2]int{first, second} {
			shift := -d
			if id == upper {
				shift = d
			}
			if !free(id, shift) {
				continue
			}
			for _, v := range members[id] {
				xs[v] += shift
			}
			join(a[k], b[k])
			break
		}
	}
}

// emptyRanks marks the ranks that hold nothing but the bends of edges, if there are any.
func emptyRanks(g *graph) []uint8 {
	maxRank, KIND, layerStart, order := g.maxRank, g.KIND, g.layerStart, g.order
	empty := make([]uint8, maxRank+1)
	any := false
	for r := 1; r < maxRank; r++ {
		only := layerStart[r+1] > layerStart[r]
		for i := layerStart[r]; only && i < layerStart[r+1]; i++ {
			if KIND[order[i]] != KindDummy {
				only = false
			}
		}
		if only {
			empty[r] = 1
			any = true
		}
	}
	if any {
		return empty
	}
	return nil
}

// hypot is Math.hypot of two values as V8 computes it (see js.Hypot), without allocating.
func hypot(x, y float64) float64 {
	nan := false
	top := 0.0
	var ax, ay float64
	if math.IsNaN(x) {
		nan = true
	} else {
		ax = math.Abs(x)
		top = ax
	}
	if math.IsNaN(y) {
		nan = true
	} else {
		ay = math.Abs(y)
		if ay > top {
			top = ay
		}
	}
	if math.IsInf(top, 1) {
		return math.Inf(1)
	}
	if nan {
		return math.NaN()
	}
	if top == 0 {
		return 0
	}
	sum, comp := 0.0, 0.0
	v := ax / top
	summand := float64(v*v) - comp
	pre := sum + summand
	comp = (pre - sum) - summand
	sum = pre
	v = ay / top
	summand = float64(v*v) - comp
	sum += summand
	return math.Sqrt(sum) * top
}

const samples = 16

// assignTracks gives every piece that does not run straight down a track across its gap, and
// counts the tracks of each gap. With curved, the tracks are levels for curves to turn onto, and
// only the pieces that would run along or too near another get one. room is the height of the gap
// under each rank, and tall receives the height that a gap whose curves mostly cross each other
// should have instead of levels.
func assignTracks(pc *pieces, trackCount []int, curved bool, room, tall []float64) {
	from, to, rank, wide := pc.from, pc.to, pc.rank, pc.wide
	total := len(from)
	pc.track = make([]int, total)
	pc.levels = make([]int, total)
	track, levels := pc.track, pc.levels
	// byRank keeps the insertion order of a Map.
	var rankOrder []int
	byRank := map[int][]int{}
	for k := 0; k < total; k++ {
		track[k] = -1
		levels[k] = 0
		if math.Abs(from[k]-to[k]) < inLine {
			continue
		}
		list, ok := byRank[rank[k]]
		if !ok {
			rankOrder = append(rankOrder, rank[k])
		}
		byRank[rank[k]] = append(list, k)
	}
	LO := make([]float64, total)
	HI := make([]float64, total)
	for k := 0; k < total; k++ {
		LO[k] = min(from[k], to[k])
		HI[k] = max(from[k], to[k])
	}
	within := func(x float64, k int) int {
		if x >= LO[k]-inLine && x <= HI[k]+inLine {
			return 1
		}
		return 0
	}
	mates := func(a, b int) bool {
		return math.Abs(from[a]-from[b]) < inLine || math.Abs(to[a]-to[b]) < inLine
	}
	overlap := func(a, b int) bool { return LO[a] < HI[b]-inLine && LO[b] < HI[a]-inLine }
	// Whether a and b need tracks of their own.
	clash := func(a, b int) bool { return overlap(a, b) && !mates(a, b) }
	// Whether one of a and b could cross the other, depending on which is above.
	meet := func(a, b int) bool {
		return overlap(a, b) || within(to[a], b)+within(from[b], a)+within(to[b], a)+within(from[a], b) > 0
	}
	// How often a and b cross with a on the higher track.
	crossings := func(a, b int) int { return within(to[a], b) + within(from[b], a) }

	place := make([]int, total)
	// lay gives the pieces of one gap in list their tracks, and each the number of tracks that the
	// pieces it has to do with take between them. Returns the number of tracks.
	lay := func(list []int) int {
		slices.SortStableFunc(list, func(a, b int) int {
			if d := jsCmp(HI[b] - LO[b] - (HI[a] - LO[a])); d != 0 {
				return d
			}
			if d := jsCmp(LO[a] - LO[b]); d != 0 {
				return d
			}
			return cmp.Compare(a, b)
		})
		count := len(list)
		var above [][]int
		group := make([]int, count)
		for i := range group {
			group[i] = i
		}
		find := func(i int) int {
			for group[i] != i {
				group[i] = group[group[i]]
				i = group[i]
			}
			return i
		}
		sequence := list
		if count <= maxWeighed {
			// above[j] lists the pieces that should be above list[j]. A piece is placed once all that
			// should be above it are, and when none is free, the one with the fewest still waiting.
			waiting := make([]int, count)
			above = make([][]int, count)
			after := make([][]int, count)
			for i := 0; i < count; i++ {
				for j := i + 1; j < count; j++ {
					a := list[i]
					b := list[j]
					if !meet(a, b) {
						continue
					}
					ri := find(i)
					group[ri] = find(j)
					ab := crossings(a, b)
					ba := crossings(b, a)
					if ab == ba {
						continue
					}
					first, second := j, i
					if ab < ba {
						first, second = i, j
					}
					above[second] = append(above[second], first)
					after[first] = append(after[first], second)
					waiting[second]++
				}
			}
			placed := make([]bool, count)
			result := make([]int, 0, count)
			for done := 0; done < count; done++ {
				pick := -1
				for i := 0; i < count; i++ {
					if placed[i] {
						continue
					}
					if pick < 0 || waiting[i] < waiting[pick] {
						pick = i
					}
					if waiting[i] == 0 {
						break
					}
				}
				placed[pick] = true
				result = append(result, pick)
				for _, j := range after[pick] {
					waiting[j]--
				}
			}
			sequence = make([]int, count)
			for i, r := range result {
				sequence[i] = list[r]
			}
		}
		// Each piece takes the first track below those it should be under that has room for it.
		for i, k := range list {
			place[k] = i
		}
		var rows [][]int
		for _, k := range sequence {
			t := 0
			if above != nil {
				for _, i := range above[place[k]] {
					if track[list[i]] >= t {
						t = track[list[i]] + 1
					}
				}
			}
			for ; ; t++ {
				for len(rows) <= t {
					rows = append(rows, nil)
				}
				free := true
				for _, other := range rows[t] {
					if clash(k, other) {
						free = false
					}
				}
				if free {
					rows[t] = append(rows[t], k)
					break
				}
			}
			track[k] = t
		}
		first := make([]int, count)
		lastOf := make([]int, count)
		for i := range first {
			first[i] = math.MaxInt
			lastOf[i] = math.MinInt
		}
		for i, k := range list {
			root := find(i)
			first[root] = min(first[root], track[k])
			lastOf[root] = max(lastOf[root], track[k])
		}
		for i, k := range list {
			root := find(i)
			levels[k] = lastOf[root] - first[root] + 1
			if curved {
				track[k] -= first[root]
			}
		}
		return len(rows)
	}

	// The least distance between a and b as plain curves across a gap that leaves them height to
	// turn in: between points along each and the other drawn as a run of short lines.
	sample := func(k int, h float64, out []float64) {
		c := Ease(to[k]-from[k], h)
		for i := 0; i <= samples; i++ {
			t := float64(i) / samples
			u := 1 - t
			out[2*i] = from[k] + float64((to[k]-from[k])*t*t*(3-2*t))
			out[2*i+1] = h * (float64(3*c*t*u*u) + float64(3*(1-c)*t*t*u) + float64(t*t*t))
		}
	}
	lineA := make([]float64, 2*samples+2)
	lineB := make([]float64, 2*samples+2)
	reach := func(points, line []float64) float64 {
		least := math.Inf(1)
		for i := 0; i <= samples; i++ {
			px := points[2*i]
			py := points[2*i+1]
			for j := 0; j < samples; j++ {
				dx := line[2*j+2] - line[2*j]
				dy := line[2*j+3] - line[2*j+1]
				den := float64(dx*dx) + float64(dy*dy)
				if den == 0 || math.IsNaN(den) {
					den = 1
				}
				t := max(0, min(1, (float64((px-line[2*j])*dx)+float64((py-line[2*j+1])*dy))/den))
				d := hypot(px-line[2*j]-float64(t*dx), py-line[2*j+1]-float64(t*dy))
				if d < least {
					least = d
				}
			}
		}
		return least
	}
	nearest := func(a, b int, height float64) float64 {
		h := max(height/2, height-2*levelLead)
		sample(a, h, lineA)
		sample(b, h, lineB)
		return min(reach(lineA, lineB), reach(lineB, lineA))
	}

	inLevel := make([]bool, total)
	for _, r := range rankOrder {
		all := byRank[r]
		if !curved {
			trackCount[r] = lay(all)
			continue
		}
		if len(all) > maxWeighed {
			continue
		}
		// The pieces that turn onto levels: first those that run far across and have another such
		// piece to be told apart from, then any two that would pass too near each other as plain curves.
		var far []int
		for _, k := range all {
			if wide[k] == 1 {
				far = append(far, k)
			}
		}
		lay(far)
		var level []int
		for _, k := range far {
			if levels[k] > 1 && !inLevel[k] {
				inLevel[k] = true
				level = append(level, k)
			}
		}
		for i := 0; i < len(all); i++ {
			for j := i + 1; j < len(all); j++ {
				a := all[i]
				b := all[j]
				if (inLevel[a] && inLevel[b]) || !clash(a, b) || (from[a] < to[a]) != (from[b] < to[b]) {
					continue
				}
				clearance := levelTouch
				if wide[a] == 1 || wide[b] == 1 {
					clearance = levelClear
				}
				if nearest(a, b, room[r]) < clearance {
					if !inLevel[a] {
						inLevel[a] = true
						level = append(level, a)
					}
					if !inLevel[b] {
						inLevel[b] = true
						level = append(level, b)
					}
				}
			}
		}
		for _, k := range far {
			track[k] = -1
			levels[k] = 0
		}
		if len(level) == 0 {
			continue
		}
		crossing := 0
		alongside := 0
		for i := 0; i < len(level); i++ {
			for j := i + 1; j < len(level); j++ {
				a := level[i]
				b := level[j]
				if !overlap(a, b) {
					continue
				}
				if (from[a] < to[a]) != (from[b] < to[b]) {
					crossing++
				} else if !mates(a, b) {
					alongside++
				}
			}
		}
		if crossing > alongside {
			for _, k := range all {
				if (HI[k]-LO[k])*crossedSlope > tall[r] {
					tall[r] = (HI[k] - LO[k]) * crossedSlope
				}
			}
			for _, k := range level {
				inLevel[k] = false
			}
			continue
		}
		// With them go the short pieces that could cross one of them, so that each has its place
		// among the levels and crosses none.
		beside := func(k int) bool {
			for _, j := range level {
				if meet(k, j) {
					return true
				}
			}
			return false
		}
		var members []int
		for _, k := range all {
			if inLevel[k] || (wide[k] != 1 && beside(k)) {
				members = append(members, k)
			}
		}
		lay(members)
		most := 0
		for _, k := range members {
			if levels[k] > most {
				most = levels[k]
			}
		}
		trackCount[r] = most
		for _, k := range level {
			inLevel[k] = false
		}
	}
}

// straighten moves the bends in ranks that hold only bends onto the straight line between the
// nearest points of their edges that are held in place, and marks the ones that can be left out
// of the route. sep is the room kept between the bends of twin edges, or 0 to leave those out as well.
func straighten(g *graph, xs []float64, empty []uint8, firstDummy, span []int, sep float64, onLine []uint8) {
	maxRank, EF, ET, EFD, ETD := g.maxRank, g.EF, g.ET, g.EFD, g.ETD
	succStart, succE, predStart, predE, layerStart, order := g.succStart, g.succE, g.predStart, g.predE, g.layerStart, g.order

	want := make([]float64, len(xs))
	from := make([]float64, len(xs))
	to := make([]float64, len(xs))
	for ei := range firstDummy {
		d0 := firstDummy[ei]
		if d0 < 0 {
			continue
		}
		end := d0 + span[ei] - 1
		// A bend's rank is its place in the run, counted from the rank after the edge's first node.
		base := g.RANK[d0]
		for d := d0; d < end; d++ {
			if empty[base+d-d0] == 0 {
				continue
			}
			last := d
			for last+1 < end && empty[base+last+1-d0] != 0 {
				last++
			}
			above := predE[predStart[d]]
			below := succE[succStart[last]]
			x0 := xs[EF[above]] + EFD[above]
			x1 := xs[ET[below]] + ETD[below]
			steps := float64(last - d + 2)
			for k := d; k <= last; k++ {
				want[k] = x0 + ((x1-x0)*float64(k-d+1))/steps
				from[k] = x0
				to[k] = x1
			}
			d = last
		}
	}

	// Only twins stay off the line: bends of edges that run between the same two points. They take
	// places a little apart, around the line.
	var twins []int
	var row []int
	for r := 1; r < maxRank; r++ {
		if empty[r] == 0 {
			continue
		}
		s := layerStart[r]
		end := layerStart[r+1]
		row = append(row[:0], order[s:end]...)
		slices.SortStableFunc(row, func(a, b int) int {
			if d := jsCmp(from[a] - from[b]); d != 0 {
				return d
			}
			if d := jsCmp(to[a] - to[b]); d != 0 {
				return d
			}
			return cmp.Compare(a, b)
		})
		for i := 0; i < len(row); {
			twins = twins[:0]
			j := i
			for ; j < len(row) && math.Abs(from[row[j]]-from[row[i]]) < 1 && math.Abs(to[row[j]]-to[row[i]]) < 1; j++ {
				twins = append(twins, row[j])
			}
			// The twins keep the order the rank gave them, so that two edges do not swap sides on the way.
			slices.SortStableFunc(twins, func(a, b int) int { return cmp.Compare(g.pos[a], g.pos[b]) })
			for k := 0; k < len(twins); k++ {
				d := twins[k]
				xs[d] = want[d] + float64((float64(k)-float64(len(twins)-1)/2)*sep)
				if len(twins) == 1 || sep == 0 {
					onLine[d] = 1
				}
			}
			i = j
		}
	}
}

// pastLabels leaves out a bend in a rank that holds labels and bends but no nodes where the edge
// would pass clear of every label anyway.
func pastLabels(g *graph, xs []float64, firstDummy, span []int, clear float64, onLine []uint8) {
	maxRank, W, KIND, RANK, EF, ET, EFD, ETD := g.maxRank, g.W, g.KIND, g.RANK, g.EF, g.ET, g.EFD, g.ETD
	succStart, succE, predStart, predE, layerStart, order := g.succStart, g.succE, g.predStart, g.predE, g.layerStart, g.order
	labelled := make([]bool, maxRank+1)
	any := false
	for r := 1; r < maxRank; r++ {
		labels := false
		others := false
		for i := layerStart[r]; i < layerStart[r+1]; i++ {
			kind := KIND[order[i]]
			if kind == KindLabel {
				labels = true
			} else if kind != KindDummy {
				others = true
			}
		}
		if labels && !others {
			labelled[r] = true
			any = true
		}
	}
	if !any {
		return
	}
	for ei := range firstDummy {
		d0 := firstDummy[ei]
		if d0 < 0 {
			continue
		}
		for d, end := d0, d0+span[ei]-1; d < end; d++ {
			if KIND[d] != KindDummy || !labelled[RANK[d]] {
				continue
			}
			// The points on either side that the edge is held at, past any bends already left out.
			first := d
			last := d
			for first > d0 && onLine[first-1] != 0 {
				first--
			}
			for last+1 < end && onLine[last+1] != 0 {
				last++
			}
			above := predE[predStart[first]]
			below := succE[succStart[last]]
			from := xs[EF[above]] + EFD[above]
			to := xs[ET[below]] + ETD[below]
			lo := min(from, to) - clear
			hi := max(from, to) + clear
			free := true
			r := RANK[d]
			for i := layerStart[r]; free && i < layerStart[r+1]; i++ {
				label := order[i]
				if KIND[label] == KindLabel && xs[label]+W[label]/2 > lo && xs[label]-W[label]/2 < hi {
					free = false
				}
			}
			if !free {
				continue
			}
			onLine[d] = 1
			// The bends left out between the two points lie on the line between them.
			steps := float64(last - first + 2)
			for k := first; k <= last; k++ {
				xs[k] = from + ((to-from)*float64(k-first+1))/steps
			}
		}
	}
}

func reduceCrossings(g *graph, nodes []*LNode) {
	n, real, maxRank, W, EF, ET, EFD, ETD := g.n, g.real, g.maxRank, g.W, g.EF, g.ET, g.EFD, g.ETD
	succStart, succE, predStart, predE, layerStart, order, pos, offsets := g.succStart, g.succE, g.predStart, g.predE, g.layerStart, g.order, g.pos, g.offsets
	widest := 0
	for r := 0; r <= maxRank; r++ {
		widest = max(widest, layerStart[r+1]-layerStart[r])
	}
	// With offsets, edges that reach one node at different offsets take separate places along it.
	var place []int
	limit := widest
	if offsets {
		place = make([]int, len(EF))
		limit = widest + len(EF)
	}
	size := 1
	for size < limit {
		size <<= 1
	}
	tree := make([]int32, 2*size)
	targets := make([]int, len(EF))
	var ids []int

	count := func() int {
		total := 0
		for r := 0; r < maxRank; r++ {
			southLen := layerStart[r+2] - layerStart[r+1]
			if offsets {
				places := 0
				for i := layerStart[r+1]; i < layerStart[r+2]; i++ {
					from := predStart[order[i]]
					to := predStart[order[i]+1]
					mixed := false
					for k := from + 1; k < to; k++ {
						if ETD[predE[k]] != ETD[predE[from]] {
							mixed = true
						}
					}
					if mixed {
						ids = append(ids[:0], predE[from:to]...)
						slices.SortStableFunc(ids, func(a, b int) int { return jsCmp(ETD[a] - ETD[b]) })
						for k := 0; k < len(ids); k++ {
							if k > 0 && ETD[ids[k]] != ETD[ids[k-1]] {
								places++
							}
							place[ids[k]] = places
						}
					} else {
						for k := from; k < to; k++ {
							place[predE[k]] = places
						}
					}
					places++
				}
				southLen = places
			}
			if southLen < 2 {
				continue
			}
			t := 0
			for i := layerStart[r]; i < layerStart[r+1]; i++ {
				v := order[i]
				from := succStart[v]
				to := succStart[v+1]
				t0 := t
				for k := from; k < to; k++ {
					if offsets {
						targets[t] = place[succE[k]]
					} else {
						targets[t] = pos[ET[succE[k]]]
					}
					t++
				}
				d := t - t0
				if d < 2 {
					continue
				}
				mixed := false
				if offsets {
					for k := from + 1; k < to; k++ {
						if EFD[succE[k]] != EFD[succE[from]] {
							mixed = true
						}
					}
				}
				if offsets && mixed {
					ids = append(ids[:0], succE[from:to]...)
					slices.SortStableFunc(ids, func(a, b int) int {
						if c := jsCmp(EFD[a] - EFD[b]); c != 0 {
							return c
						}
						return cmp.Compare(place[a], place[b])
					})
					for k := 0; k < d; k++ {
						targets[t0+k] = place[ids[k]]
					}
				} else if d <= 12 {
					for a := t0 + 1; a < t; a++ {
						item := targets[a]
						b := a - 1
						for b >= t0 && targets[b] > item {
							targets[b+1] = targets[b]
							b--
						}
						targets[b+1] = item
					}
				} else {
					slices.Sort(targets[t0:t])
				}
			}
			if t < 2 {
				continue
			}
			leaves := 1
			for leaves < southLen {
				leaves <<= 1
			}
			clear(tree[:2*leaves])
			for k := 0; k < t; k++ {
				i := targets[k] + leaves
				tree[i]++
				for i > 1 {
					if i&1 == 0 {
						total += int(tree[i+1])
					}
					i >>= 1
					tree[i]++
				}
			}
		}
		return total
	}

	best := count()
	if best == 0 {
		return
	}
	bestOrder := slices.Clone(order)
	bary := make([]float64, n)
	moving := make([]int, widest)
	leftBias := func(a, b int) int {
		if d := jsCmp(bary[a] - bary[b]); d != 0 {
			return d
		}
		return cmp.Compare(pos[a], pos[b])
	}
	rightBias := func(a, b int) int {
		if d := jsCmp(bary[a] - bary[b]); d != 0 {
			return d
		}
		return cmp.Compare(pos[b], pos[a])
	}

	sweep := func(down, biasRight bool) {
		start, list, other, otherDx := succStart, succE, ET, ETD
		if down {
			start, list, other, otherDx = predStart, predE, EF, EFD
		}
		for step := 1; step <= maxRank; step++ {
			r := maxRank - step
			if down {
				r = step
			}
			s := layerStart[r]
			e := layerStart[r+1]
			movable := 0
			for i := s; i < e; i++ {
				v := order[i]
				pin := 0.0
				if v < real {
					pin = nodes[v].Pin
				}
				if pin != 0 {
					bary[v] = pin * 1e9
					moving[movable] = v
					movable++
					continue
				}
				from := start[v]
				to := start[v+1]
				if from == to {
					bary[v] = -1
					continue
				}
				sum := 0.0
				for k := from; k < to; k++ {
					u := other[list[k]]
					sum += float64(pos[u])
					if offsets && W[u] > 0 {
						sum += otherDx[list[k]] / W[u]
					}
				}
				bary[v] = sum / float64(to-from)
				moving[movable] = v
				movable++
			}
			if movable < 2 {
				continue
			}
			if biasRight {
				slices.SortStableFunc(moving[:movable], rightBias)
			} else {
				slices.SortStableFunc(moving[:movable], leftBias)
			}
			k := 0
			for i := s; i < e; i++ {
				if bary[order[i]] != -1 {
					order[i] = moving[k]
					k++
				}
			}
			for i := s; i < e; i++ {
				pos[order[i]] = i - s
			}
		}
	}

	// How many pairs of nodes in one rank are not in the order they were given.
	outOfOrder := func() int {
		total := 0
		for r := 0; r <= maxRank; r++ {
			for i := layerStart[r]; i < layerStart[r+1]; i++ {
				if order[i] >= real {
					continue
				}
				for j := i + 1; j < layerStart[r+1]; j++ {
					if order[j] < order[i] {
						total++
					}
				}
			}
		}
		return total
	}
	settlePositions := func() {
		for r := 0; r <= maxRank; r++ {
			for i := layerStart[r]; i < layerStart[r+1]; i++ {
				pos[order[i]] = i - layerStart[r]
			}
		}
	}

	// The sweeps are run twice from the same start, going down first and going up first, and of two
	// orders with as few crossings the one that keeps more nodes in the order they were given wins.
	initial := slices.Clone(order)
	start := best
	small := n <= 600
	chosen := bestOrder
	chosenOut := math.MaxInt
	attempts := 1
	if small {
		attempts = 2
	}
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			copy(order, initial)
			settlePositions()
		}
		found := start
		foundOrder := slices.Clone(initial)
		for i, stale := 0, 0; stale < 4 && i < maxSweeps; i, stale = i+1, stale+1 {
			sweep((i+attempt)%2 == 0, i%4 >= 2)
			c := count()
			if c < found {
				found = c
				copy(foundOrder, order)
				stale = 0
				if c == 0 {
					break
				}
			}
		}
		copy(order, foundOrder)
		out := 0
		if small {
			out = outOfOrder()
		}
		if attempt == 0 || found < best || (found == best && out < chosenOut) {
			best = found
			chosen = foundOrder
			chosenOut = out
		}
	}
	copy(order, chosen)
	settlePositions()
	copy(bestOrder, chosen)

	// A drawing and its mirror image cross as often too. Of those two, likewise.
	if !small {
		return
	}
	for i := 0; i < real; i++ {
		if nodes[i].Pin != 0 {
			return
		}
	}
	if chosenOut == 0 {
		return
	}
	for r := 0; r <= maxRank; r++ {
		slices.Reverse(order[layerStart[r]:layerStart[r+1]])
	}
	settlePositions()
	if count() > best || outOfOrder() >= chosenOut {
		copy(order, bestOrder)
	}
}

// position is Brandes-Köpf horizontal coordinate assignment: four aligned layouts, balanced.
func position(g *graph, opt LayeredOptions) []float64 {
	n, maxRank, W, KIND, EF, ET, EFD, ETD := g.n, g.maxRank, g.W, g.KIND, g.EF, g.ET, g.EFD, g.ETD
	succStart, succE, predStart, predE, layerStart, order, pos := g.succStart, g.succE, g.predStart, g.predE, g.layerStart, g.order, g.pos
	segments := len(EF)
	gap := make([]float64, n)
	for i := 0; i < n; i++ {
		gap[i] = halfRoom(g, opt, i)
	}

	// Neighbours in left-to-right order, so the medians can be read off directly.
	key := make([]int, segments)
	for e := 0; e < segments; e++ {
		key[e] = pos[ET[e]]
	}
	for v := 0; v < n; v++ {
		if succStart[v+1]-succStart[v] > 1 {
			sortRange(succE, succStart[v], succStart[v+1], key)
		}
	}
	for e := 0; e < segments; e++ {
		key[e] = pos[EF[e]]
	}
	for v := 0; v < n; v++ {
		if predStart[v+1]-predStart[v] > 1 {
			sortRange(predE, predStart[v], predStart[v+1], key)
		}
	}

	// An edge between two real nodes may not cross a dummy-to-dummy segment; mark those that would.
	conflict := make([]bool, segments)
	for r := 1; r <= maxRank; r++ {
		prevLen := layerStart[r] - layerStart[r-1]
		s := layerStart[r]
		end := layerStart[r+1]
		k0 := 0
		scan := s
		for i := s; i < end; i++ {
			v := order[i]
			inner := -1
			if isDummy(KIND[v]) {
				u := EF[predE[predStart[v]]]
				if isDummy(KIND[u]) {
					inner = u
				}
			}
			k1 := prevLen
			if inner >= 0 {
				k1 = pos[inner]
			}
			if inner >= 0 || i == end-1 {
				for j := scan; j <= i; j++ {
					w := order[j]
					wDummy := isDummy(KIND[w])
					for k := predStart[w]; k < predStart[w+1]; k++ {
						e := predE[k]
						up := pos[EF[e]]
						if (up < k0 || k1 < up) && !(wDummy && isDummy(KIND[EF[e]])) {
							conflict[e] = true
						}
					}
				}
				scan = i + 1
				k0 = k1
			}
		}
	}

	var results [4][]float64
	root := make([]int, n)
	align := make([]int, n)
	// Offset of each node from its block's root, so that aligned edges meet at their attachment points.
	shift := make([]float64, n)
	eFrom := make([]int, n)
	eTo := make([]int, n)
	eW := make([]float64, n)
	indeg := make([]int, n)
	outStart := make([]int, n+1)
	outE := make([]int, n)
	fill := make([]int, n)
	topo := make([]int, n)

	for variant := 0; variant < 4; variant++ {
		up := variant < 2
		right := variant&1 == 1
		start, list, other, ownDx, otherDx := succStart, succE, ET, EFD, ETD
		if up {
			start, list, other, ownDx, otherDx = predStart, predE, EF, ETD, EFD
		}
		for i := 0; i < n; i++ {
			root[i] = i
			align[i] = i
			shift[i] = 0
		}

		for step := 0; step <= maxRank; step++ {
			r := maxRank - step
			if up {
				r = step
			}
			s := layerStart[r]
			end := layerStart[r+1]
			adjacent := r + 1
			if up {
				adjacent = r - 1
			}
			adjacentLast := 0
			if adjacent >= 0 && adjacent <= maxRank {
				adjacentLast = layerStart[adjacent+1] - layerStart[adjacent] - 1
			}
			prevIdx := -1
			i, last, inc := s, end, 1
			if right {
				i, last, inc = end-1, s-1, -1
			}
			for ; i != last; i += inc {
				v := order[i]
				from := start[v]
				d := start[v+1] - from
				if d == 0 {
					continue
				}
				lo := from + ((d - 1) >> 1)
				hi := from + (d >> 1)
				for pass := 0; pass < 2; pass++ {
					if pass == 1 && lo == hi {
						break
					}
					at := hi
					if (pass == 0) != right {
						at = lo
					}
					e := list[at]
					w := other[e]
					idx := pos[w]
					if right {
						idx = adjacentLast - pos[w]
					}
					if align[v] == v && prevIdx < idx && !conflict[e] {
						align[w] = v
						root[v] = root[w]
						align[v] = root[v]
						shift[v] = shift[w] + otherDx[e] - ownDx[e]
						prevIdx = idx
					}
				}
			}
		}

		blockEdges := 0
		clear(indeg)
		clear(outStart)
		for r := 0; r <= maxRank; r++ {
			s := layerStart[r]
			end := layerStart[r+1]
			for i := s + 1; i < end; i++ {
				a, b := order[i-1], order[i]
				if right {
					a, b = order[i], order[i-1]
				}
				sep := gap[a] + gap[b]
				eFrom[blockEdges] = root[a]
				eTo[blockEdges] = root[b]
				if right {
					eW[blockEdges] = sep - shift[a] + shift[b]
				} else {
					eW[blockEdges] = sep + shift[a] - shift[b]
				}
				indeg[root[b]]++
				outStart[root[a]+1]++
				blockEdges++
			}
		}
		for i := 0; i < n; i++ {
			outStart[i+1] += outStart[i]
		}
		copy(fill, outStart[:n])
		for e := 0; e < blockEdges; e++ {
			outE[fill[eFrom[e]]] = e
			fill[eFrom[e]]++
		}

		x := make([]float64, n)
		count := 0
		for i := 0; i < n; i++ {
			if root[i] == i && indeg[i] == 0 {
				topo[count] = i
				count++
			}
		}
		for q := 0; q < count; q++ {
			v := topo[q]
			for k := outStart[v]; k < outStart[v+1]; k++ {
				e := outE[k]
				w := eTo[e]
				if x[v]+eW[e] > x[w] {
					x[w] = x[v] + eW[e]
				}
				indeg[w]--
				if indeg[w] == 0 {
					topo[count] = w
					count++
				}
			}
		}
		for q := count - 1; q >= 0; q-- {
			v := topo[q]
			lo := math.Inf(1)
			for k := outStart[v]; k < outStart[v+1]; k++ {
				e := outE[k]
				cand := x[eTo[e]] - eW[e]
				if cand < lo {
					lo = cand
				}
			}
			if !math.IsInf(lo, 1) && lo > x[v] {
				x[v] = lo
			}
		}
		xs := make([]float64, n)
		for i := 0; i < n; i++ {
			if right {
				xs[i] = -x[root[i]] + shift[i]
			} else {
				xs[i] = x[root[i]] + shift[i]
			}
		}
		results[variant] = xs
	}

	narrowest := 0
	bestWidth := math.Inf(1)
	var mins, maxs [4]float64
	for k := 0; k < 4; k++ {
		lo := math.Inf(1)
		hi := math.Inf(-1)
		xs := results[k]
		for i := 0; i < n; i++ {
			half := W[i] / 2
			if xs[i]-half < lo {
				lo = xs[i] - half
			}
			if xs[i]+half > hi {
				hi = xs[i] + half
			}
		}
		mins[k] = lo
		maxs[k] = hi
		if hi-lo < bestWidth {
			bestWidth = hi - lo
			narrowest = k
		}
	}
	for k := 0; k < 4; k++ {
		delta := maxs[narrowest] - maxs[k]
		if k&1 == 0 {
			delta = mins[narrowest] - mins[k]
		}
		if delta != 0 {
			xs := results[k]
			for i := 0; i < n; i++ {
				xs[i] += delta
			}
		}
	}

	out := make([]float64, n)
	for i := 0; i < n; i++ {
		a := results[0][i]
		b := results[1][i]
		c := results[2][i]
		d := results[3][i]
		if a > b {
			a, b = b, a
		}
		if c > d {
			c, d = d, c
		}
		if a > c {
			a, c = c, a
		}
		if b > d {
			b, d = d, b
		}
		if b > c {
			b, c = c, b
		}
		out[i] = (b + c) / 2
	}
	return out
}
