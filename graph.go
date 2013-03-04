package main

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// A dependency graph
type graph struct {
	root  *node            // the intial target's node
	nodes map[string]*node // map targets to their nodes
}

// An edge in the graph.
type edge struct {
	v       *node    // node this edge directs to
	stem    string   // stem matched for meta-rule applications
	matches []string // regular expression matches
	r       *rule
}

// Current status of a node in the build.
type nodeStatus int

const (
	nodeStatusReady nodeStatus = iota
	nodeStatusStarted
	nodeStatusDone
	nodeStatusFailed
)

// A node in the dependency graph
type node struct {
	r         *rule             // rule to be applied
	name      string            // target name
	prog      string            // custom program to compare times
	t         time.Time         // file modification time
	exists    bool              // does a non-virtual target exist
	prereqs   []*edge           // prerequisite rules
	status    nodeStatus        // current state of the node in the build
	mutex     sync.Mutex        // exclusivity for the status variable
	listeners []chan nodeStatus // channels to notify of completion
}

// Create a new node
func (g *graph) newnode(name string) *node {
	u := &node{name: name}
	info, err := os.Stat(name)
	if err == nil {
		u.t = info.ModTime()
		u.exists = true
	} else {
		_, ok := err.(*os.PathError)
		if ok {
			u.exists = false
		} else {
			mkError(err.Error())
		}
	}
	g.nodes[name] = u
	return u
}

// Print a graph in graphviz format.
func (g *graph) visualize(w io.Writer) {
	fmt.Fprintln(w, "digraph mk {")
	for t, u := range g.nodes {
		for i := range u.prereqs {
			if u.prereqs[i].v != nil {
				fmt.Fprintf(w, "    \"%s\" -> \"%s\";\n", t, u.prereqs[i].v.name)
			}
		}
	}
	fmt.Fprintln(w, "}")
}

// Create a new arc.
func (u *node) newedge(v *node, r *rule) *edge {
	e := &edge{v: v, r: r}
	u.prereqs = append(u.prereqs, e)
	return e
}

// Create a dependency graph for the given target.
func buildgraph(rs *ruleSet, target string) *graph {
	g := &graph{nil, make(map[string]*node)}

	// keep track of how many times each rule is visited, to avoid cycles.
	rulecnt := make([]int, len(rs.rules))
	g.root = applyrules(rs, g, target, rulecnt)

	return g
}

// Recursively match the given target to a rule in the rule set to construct the
// full graph.
func applyrules(rs *ruleSet, g *graph, target string, rulecnt []int) *node {
	u, ok := g.nodes[target]
	if ok {
		return u
	}
	u = g.newnode(target)

	// does the target match a concrete rule?

	ks, ok := rs.targetrules[target]
	if ok {
		for ki := range ks {
			k := ks[ki]
			if rulecnt[k] > maxRuleCnt {
				continue
			}

			r := &rs.rules[k]

			// skip meta-rules
			if r.ismeta {
				continue
			}

			// skip rules that have no effect
			if r.recipe == "" && len(r.prereqs) == 0 {
				continue
			}

			rulecnt[k] += 1
			if len(r.prereqs) == 0 {
				u.newedge(nil, r)
			} else {
				for i := range r.prereqs {
					u.newedge(applyrules(rs, g, r.prereqs[i], rulecnt), r)
				}
			}
			rulecnt[k] -= 1
		}
	}

	// find applicable metarules
	for k := range rs.rules {
		if rulecnt[k] > maxRuleCnt {
			continue
		}

		r := &rs.rules[k]

		if !r.ismeta {
			continue
		}

		// skip rules that have no effect
		if r.recipe == "" && len(r.prereqs) == 0 {
			continue
		}

		for j := range r.targets {
			mat := r.targets[j].match(target)
			if mat == nil {
				continue
			}

			var stem string
			var matches []string

			if r.attributes.regex {
				matches = mat
			} else {
				stem = mat[1]
			}

			rulecnt[k] += 1
			if len(r.prereqs) == 0 {
				e := u.newedge(nil, r)
				e.stem = stem
				e.matches = matches
			} else {
				for i := range r.prereqs {
					var prereq string
					if r.attributes.regex {
						// TODO: write substituteRegexpRefs and use that here
						prereq = r.prereqs[i]
					} else {
						prereq = expandSuffixes(r.prereqs[i], stem)
					}

					e := u.newedge(applyrules(rs, g, prereq, rulecnt), r)
					e.stem = stem
					e.matches = matches
				}
			}
			rulecnt[k] -= 1
		}
	}

	return u
}
