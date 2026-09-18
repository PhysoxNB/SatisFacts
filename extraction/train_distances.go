package extraction

import (
	"container/heap"
	"math"
	"sort"
)

// trackGraph is the rail network as a weighted graph. We use it to find
// shortest paths between train stations for distance calculations.
type trackGraph struct {
	nodeIdx  map[string]int           // connection pathName -> node index
	adj      [][]graphEdge            // adjacency list
	nodePos  [][3]float64             // position in meters (handy for snapping stations to nearby nodes)
	segments map[string]*TrackSegment // segment pathName -> data (only set for per-train directed graphs)
}

type graphEdge struct {
	to     int
	weight float64 // meters
	segKey string  // empty for junction links, segment pathName for track edges
}

// pqItem is one entry in the Dijkstra priority queue.
type pqItem struct {
	node int
	dist float64
}

type priorityQueue []pqItem

func (pq priorityQueue) Len() int           { return len(pq) }
func (pq priorityQueue) Less(i, j int) bool { return pq[i].dist < pq[j].dist }
func (pq priorityQueue) Swap(i, j int)      { pq[i], pq[j] = pq[j], pq[i] }
func (pq *priorityQueue) Push(x interface{}) {
	*pq = append(*pq, x.(pqItem))
}
func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[:n-1]
	return item
}

// sortedKeys returns a map's keys in sorted order. The raw track data lives in
// maps, and ranging over those directly means node indices come out in a
// different order on every run. That matters more than it sounds: both ends of
// a track segment are stored at the same position, so findNearestNode hits an
// exact tie for every station and settles it on node index. Random indices then
// mean a station snaps to a different end of the same track piece run to run,
// and route distances wobble. Sorting the keys pins all of that down.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// buildTrackGraph turns the raw track segments and connection components into
// a directed weighted graph. Each track segment becomes edges weighted by its
// spline length, and mConnectedComponents links become zero-weight edges.
// Signal data is used to remove blocked directions - trains can only travel
// from observed -> guarded, not the other way around.
func (e *ExtractedData) buildTrackGraph() *trackGraph {
	g := &trackGraph{
		nodeIdx:  make(map[string]int),
		segments: make(map[string]*TrackSegment),
	}

	getOrCreateNode := func(pathName string, pos [3]float64) int {
		if idx, ok := g.nodeIdx[pathName]; ok {
			return idx
		}
		idx := len(g.nodeIdx)
		g.nodeIdx[pathName] = idx
		g.adj = append(g.adj, nil)
		g.nodePos = append(g.nodePos, pos)
		return idx
	}

	// Track segments: add both directions initially.
	// Each end goes in at its own spline endpoint. Using the actor origin for
	// both (which is what we used to do) put the two ends in the same spot and
	// left findNearestNode with a coin flip between them.
	for _, segKey := range sortedKeys(e.rawTrackSegments) {
		seg := e.rawTrackSegments[segKey]
		pos0, pos1 := seg.Pos, seg.Pos
		if seg.HasEndpoints {
			pos0, pos1 = seg.Conn0Pos, seg.Conn1Pos
		}
		idx0 := getOrCreateNode(seg.Conn0, pos0)
		idx1 := getOrCreateNode(seg.Conn1, pos1)
		w := seg.Length
		g.adj[idx0] = append(g.adj[idx0], graphEdge{to: idx1, weight: w, segKey: segKey})
		g.adj[idx1] = append(g.adj[idx1], graphEdge{to: idx0, weight: w, segKey: segKey})
		g.segments[segKey] = seg
	}

	// Connection components: add both directions initially (zero cost)
	for _, connPath := range sortedKeys(e.rawTrackConns) {
		conn := e.rawTrackConns[connPath]
		idx := getOrCreateNode(connPath, [3]float64{})
		for _, otherPath := range conn.ConnectedTo {
			otherIdx := getOrCreateNode(otherPath, [3]float64{})
			g.adj[idx] = append(g.adj[idx], graphEdge{to: otherIdx, weight: 0})
			g.adj[otherIdx] = append(g.adj[otherIdx], graphEdge{to: idx, weight: 0})
		}
	}

	// Signals enforce one-way travel. A signal allows observed -> guarded
	// but blocks guarded -> observed. We remove the blocked edges.
	for _, sigKey := range sortedKeys(e.rawSignals) {
		sig := e.rawSignals[sigKey]
		for _, guardedPath := range sig.Guarded {
			guardedIdx, ok := g.nodeIdx[guardedPath]
			if !ok {
				continue
			}
			for _, observedPath := range sig.Observed {
				observedIdx, ok := g.nodeIdx[observedPath]
				if !ok {
					continue
				}
				// Remove the edge from guarded -> observed (the blocked direction)
				g.removeEdge(guardedIdx, observedIdx)
			}
		}
	}

	return g
}

// removeEdge deletes all edges from node 'from' to node 'to' from the
// adjacency list. This is how we enforce signal directionality.
func (g *trackGraph) removeEdge(from, to int) {
	edges := g.adj[from]
	// In-place filter: reuse the same backing array by writing to edges[:0].
	filtered := edges[:0]
	for _, e := range edges {
		if e.to != to {
			filtered = append(filtered, e)
		}
	}
	g.adj[from] = filtered
}

// findNearestSegment finds the track segment whose line is closest to the
// given world position, using point-to-segment distance. Only segments with
// real endpoints are considered.
func (g *trackGraph) findNearestSegment(pos [3]float64) (segKey string, seg *TrackSegment, ok bool) {
	bestDist := math.MaxFloat64
	for key, s := range g.segments {
		if !s.HasEndpoints {
			continue
		}
		a, b := s.Conn0Pos, s.Conn1Pos
		ab := [3]float64{b[0] - a[0], b[1] - a[1], b[2] - a[2]}
		abLen2 := ab[0]*ab[0] + ab[1]*ab[1] + ab[2]*ab[2]
		if abLen2 == 0 {
			continue
		}
		ap := [3]float64{pos[0] - a[0], pos[1] - a[1], pos[2] - a[2]}
		t := (ap[0]*ab[0] + ap[1]*ab[1] + ap[2]*ab[2]) / abLen2
		if t < 0 {
			t = 0
		} else if t > 1 {
			t = 1
		}
		closest := [3]float64{a[0] + t*ab[0], a[1] + t*ab[1], a[2] + t*ab[2]}
		dx, dy, dz := pos[0]-closest[0], pos[1]-closest[1], pos[2]-closest[2]
		dist := dx*dx + dy*dy + dz*dz
		if dist < bestDist {
			bestDist = dist
			segKey = key
			seg = s
			ok = true
		}
	}
	return
}

// buildDirectedForTrain creates a per-train directed graph from the
// signal-directed base graph. The locomotive's heading determines the travel
// direction on the nearest track segment, then DFS propagates that direction
// through any segments that are still bidirectional (i.e. unsignaled).
//
// Segments already directed by signals keep their signal direction - the DFS
// only directs segments that still have both directions. This ensures correct
// one-way distances on signaled networks, and correct heading-based direction
// on unsignaled loops (where only one train can run and there are no turnoffs).
func (g *trackGraph) buildDirectedForTrain(locoPos [3]float64, locoRot [4]float32, isReversed bool) *trackGraph {
	startSegKey, startSeg, ok := g.findNearestSegment(locoPos)
	if !ok {
		return g
	}

	forward := rotateByQuat([3]float64{1, 0, 0}, locoRot)
	if isReversed {
		forward[0], forward[1], forward[2] = -forward[0], -forward[1], -forward[2]
	}

	idx0 := g.nodeIdx[startSeg.Conn0]
	idx1 := g.nodeIdx[startSeg.Conn1]
	segDir := [3]float64{
		startSeg.Conn1Pos[0] - startSeg.Conn0Pos[0],
		startSeg.Conn1Pos[1] - startSeg.Conn0Pos[1],
		startSeg.Conn1Pos[2] - startSeg.Conn0Pos[2],
	}
	dot := forward[0]*segDir[0] + forward[1]*segDir[1] + forward[2]*segDir[2]

	var entryNode, exitNode int
	if dot >= 0 {
		entryNode, exitNode = idx0, idx1
	} else {
		entryNode, exitNode = idx1, idx0
	}

	// Build the directed graph: copy all edges from the signal-directed graph.
	directed := &trackGraph{
		nodeIdx:  g.nodeIdx,
		adj:      make([][]graphEdge, len(g.adj)),
		nodePos:  g.nodePos,
		segments: g.segments,
	}
	for i, edges := range g.adj {
		directed.adj[i] = append([]graphEdge(nil), edges...)
	}

	// DFS through the track network, directing segments that are still
	// bidirectional. Segments already directed by signals have only one
	// edge - the DFS follows it without changing anything.
	visited := make(map[string]bool)
	type dfsItem struct {
		entryNode, exitNode int
		segKey              string
		weight              float64
	}
	visited[startSegKey] = true

	// For the start segment: if it's still bidirectional (no signal),
	// remove the reverse direction.
	directed.removeEdge(exitNode, entryNode)

	stack := []dfsItem{{entryNode, exitNode, startSegKey, startSeg.Length}}

	for len(stack) > 0 {
		item := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		// At the exit node, DFS through junction edges to find track segments.
		junctionVisited := make(map[int]bool)
		junctionStack := []int{item.exitNode}
		for len(junctionStack) > 0 {
			jNode := junctionStack[len(junctionStack)-1]
			junctionStack = junctionStack[:len(junctionStack)-1]
			if junctionVisited[jNode] {
				continue
			}
			junctionVisited[jNode] = true

			for _, edge := range directed.adj[jNode] {
				if edge.segKey != "" {
					if !visited[edge.segKey] {
						visited[edge.segKey] = true
						// This segment is directed from jNode -> edge.to.
						// Remove the reverse direction if it still exists.
						directed.removeEdge(edge.to, jNode)
						stack = append(stack, dfsItem{jNode, edge.to, edge.segKey, edge.weight})
					}
				} else {
					if !junctionVisited[edge.to] {
						junctionStack = append(junctionStack, edge.to)
					}
				}
			}
		}
	}

	return directed
}

// findNearestNode snaps a world position to the closest graph node.
// We use this to find which track point a train station sits on.
func (g *trackGraph) findNearestNode(pos [3]float64) int {
	bestIdx := -1
	bestDist := math.MaxFloat64
	for i, p := range g.nodePos {
		// Skip nodes that don't have a real position (pure junction nodes)
		if p[0] == 0 && p[1] == 0 && p[2] == 0 {
			continue
		}
		dx := p[0] - pos[0]
		dy := p[1] - pos[1]
		dz := p[2] - pos[2]
		d := dx*dx + dy*dy + dz*dz
		if d < bestDist {
			bestDist = d
			bestIdx = i
		}
	}
	return bestIdx
}

// dijkstra runs a standard shortest-path search and returns the distance in
// meters between two nodes. Returns 0 if there's no connection.
func (g *trackGraph) dijkstra(src, dst int) float64 {
	if src < 0 || dst < 0 || src >= len(g.adj) || dst >= len(g.adj) {
		return 0
	}
	if src == dst {
		return 0
	}

	dist := make([]float64, len(g.adj))
	for i := range dist {
		dist[i] = math.MaxFloat64
	}
	dist[src] = 0

	pq := &priorityQueue{}
	heap.Push(pq, pqItem{node: src, dist: 0})

	for pq.Len() > 0 {
		cur := heap.Pop(pq).(pqItem)
		if cur.node == dst {
			return cur.dist
		}
		// Stale queue entry - we already found a better path to this node.
		if cur.dist > dist[cur.node] {
			continue
		}
		for _, edge := range g.adj[cur.node] {
			newDist := dist[cur.node] + edge.weight
			if newDist < dist[edge.to] {
				dist[edge.to] = newDist
				heap.Push(pq, pqItem{node: edge.to, dist: newDist})
			}
		}
	}

	return 0 // no path found - stations might be on disconnected track segments
}
