package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// True if messages should be printed without fancy colors.
var nocolor bool = false

// True if we are ignoring timestamps and rebuilding everything.
var rebuildall bool = false

// Set of targets for which we are forcing rebuild
var rebuildtargets map[string]bool = make(map[string]bool)

// Lock on standard out, messages don't get interleaved too much.
var mkMsgMutex sync.Mutex

// The maximum number of times an rule may be applied.
const maxRuleCnt = 1

// Limit the number of recipes executed simultaneously.
var subprocsAllowed int

// Current subprocesses being executed
var subprocsRunning int

// Wakeup on a free subprocess slot.
var subprocsRunningCond *sync.Cond = sync.NewCond(&sync.Mutex{})

// Prevent more than one recipe at a time from trying to take over
var exclusiveSubproc = sync.Mutex{}

// Wait until there is an available subprocess slot.
func reserveSubproc() {
	subprocsRunningCond.L.Lock()
	for subprocsRunning >= subprocsAllowed {
		subprocsRunningCond.Wait()
	}
	subprocsRunning++
	subprocsRunningCond.L.Unlock()
}

// Free up another subprocess to run.
func finishSubproc() {
	subprocsRunningCond.L.Lock()
	subprocsRunning--
	subprocsRunningCond.Signal()
	subprocsRunningCond.L.Unlock()
}

// Make everyone wait while we
func reserveExclusiveSubproc() {
	exclusiveSubproc.Lock()
	// Wait until everything is done running
	stolen_subprocs := 0
	subprocsRunningCond.L.Lock()
	stolen_subprocs = subprocsAllowed - subprocsRunning
	subprocsRunning = subprocsAllowed
	for stolen_subprocs < subprocsAllowed {
		subprocsRunningCond.Wait()
		stolen_subprocs += subprocsAllowed - subprocsRunning
		subprocsRunning = subprocsAllowed
	}
}

func finishExclusiveSubproc() {
	subprocsRunning = 0
	subprocsRunningCond.Broadcast()
	subprocsRunningCond.L.Unlock()
	exclusiveSubproc.Unlock()
}

// Ansi color codes.
const (
	ansiTermDefault   = "\033[0m"
	ansiTermBlack     = "\033[30m"
	ansiTermRed       = "\033[31m"
	ansiTermGreen     = "\033[32m"
	ansiTermYellow    = "\033[33m"
	ansiTermBlue      = "\033[34m"
	ansiTermMagenta   = "\033[35m"
	ansiTermBright    = "\033[1m"
	ansiTermUnderline = "\033[4m"
)

// Build a node's prereqs. Block until completed.
//
func mkNodePrereqs(g *graph, u *node, e *edge, prereqs []*node, dryrun bool,
	required bool) nodeStatus {
	prereqstat := make(chan nodeStatus)
	pending := 0

	// build prereqs that need building
	for i := range prereqs {
		prereqs[i].mutex.Lock()
		switch prereqs[i].status {
		case nodeStatusReady, nodeStatusNop:
			go mkNode(g, prereqs[i], dryrun, required)
			fallthrough
		case nodeStatusStarted:
			prereqs[i].listeners = append(prereqs[i].listeners, prereqstat)
			pending++
		}
		prereqs[i].mutex.Unlock()
	}

	// wait until all the prereqs are built
	status := nodeStatusDone
	for pending > 0 {
		s := <-prereqstat
		pending--
		if s == nodeStatusFailed {
			status = nodeStatusFailed
		}
	}
	return status
}

// Build a target in the graph.
//
// This selects an appropriate rule (edge) and builds all prerequisites
// concurrently.
//
// Args:
//  g: Graph in which the node lives.
//  u: Node to (possibly) build.
//  dryrun: Don't actually build anything, just pretend.
//  required: Avoid building this node, unless its prereqs are out of date.
//
func mkNode(g *graph, u *node, dryrun bool, required bool) {
	// try to claim on this node
	u.mutex.Lock()
	if u.status != nodeStatusReady && u.status != nodeStatusNop {
		u.mutex.Unlock()
		return
	} else {
		u.status = nodeStatusStarted
	}
	u.mutex.Unlock()

	// when finished, notify the listeners
	finalstatus := nodeStatusDone
	defer func() {
		u.mutex.Lock()
		u.status = finalstatus
		for i := range u.listeners {
			u.listeners[i] <- u.status
		}
		u.listeners = u.listeners[0:0]
		u.mutex.Unlock()
	}()

	// there's no fucking rules, dude
	if len(u.prereqs) == 0 {
		if !(u.r != nil && u.r.attributes.virtual) && !u.exists {
			wd, _ := os.Getwd()
			mkError(fmt.Sprintf("don't know how to make %s in %s\n", u.name, wd))
		}
		finalstatus = nodeStatusNop
		return
	}

	// there should otherwise be exactly one edge with an associated rule
	prereqs := make([]*node, 0)
	var e *edge = nil
	for i := range u.prereqs {
		if u.prereqs[i].r != nil {
			e = u.prereqs[i]
		}
		if u.prereqs[i].v != nil {
			prereqs = append(prereqs, u.prereqs[i].v)
		}
	}

	// this should have been caught during graph building
	if e == nil {
		wd, _ := os.Getwd()
		mkError(fmt.Sprintf("don't know how to make %s in %s", u.name, wd))
	}

	prereqs_required := required && (e.r.attributes.virtual || !u.exists)
	mkNodePrereqs(g, u, e, prereqs, dryrun, prereqs_required)

	uptodate := true
	if !e.r.attributes.virtual {
		u.updateTimestamp()
		if !u.exists && required {
			uptodate = false
		} else if u.exists || required {
			for i := range prereqs {
				if u.t.Before(prereqs[i].t) || prereqs[i].status == nodeStatusDone {
					uptodate = false
				}
			}
		} else if required {
			uptodate = false
		}
	} else {
		uptodate = false
	}

	_, isrebuildtarget := rebuildtargets[u.name]
	if isrebuildtarget || rebuildall {
		uptodate = false
	}

	// make another pass on the prereqs, since we know we need them now
	if !uptodate {
		mkNodePrereqs(g, u, e, prereqs, dryrun, true)
	}

	// execute the recipe, unless the prereqs failed
	if !uptodate && finalstatus != nodeStatusFailed && len(e.r.recipe) > 0 {
		if e.r.attributes.exclusive {
			reserveExclusiveSubproc()
		} else {
			reserveSubproc()
		}

		if !dorecipe(u.name, u, e, dryrun) {
			finalstatus = nodeStatusFailed
		}
		u.updateTimestamp()

		if e.r.attributes.exclusive {
			finishExclusiveSubproc()
		} else {
			finishSubproc()
		}
	} else if finalstatus != nodeStatusFailed {
		finalstatus = nodeStatusNop
	}
}

func mkError(msg string) {
	mkPrintError(msg)
	os.Exit(1)
}

func mkPrintError(msg string) {
	if !nocolor {
		os.Stderr.WriteString(ansiTermRed)
	}
	fmt.Fprintf(os.Stderr, "%s\n", msg)
	if !nocolor {
		os.Stderr.WriteString(ansiTermDefault)
	}
}

func mkPrintSuccess(msg string) {
	if nocolor {
		fmt.Println(msg)
	} else {
		fmt.Printf("%s%s%s\n", ansiTermGreen, msg, ansiTermDefault)
	}
}

func mkPrintMessage(msg string) {
	mkMsgMutex.Lock()
	if nocolor {
		fmt.Println(msg)
	} else {
		fmt.Printf("%s%s%s\n", ansiTermBlue, msg, ansiTermDefault)
	}
	mkMsgMutex.Unlock()
}

func mkPrintRecipe(target string, recipe string, quiet bool) {
	mkMsgMutex.Lock()
	if nocolor {
		fmt.Printf("%s: ", target)
	} else {
		fmt.Printf("%s%s%s → %s",
			ansiTermBlue+ansiTermBright+ansiTermUnderline, target,
			ansiTermDefault, ansiTermBlue)
	}
	if quiet {
		if nocolor {
			fmt.Println("...")
		} else {
			fmt.Println("…")
		}
	} else {
		printIndented(os.Stdout, recipe, len(target)+3)
		if len(recipe) == 0 {
			os.Stdout.WriteString("\n")
		}
	}
	if !nocolor {
		os.Stdout.WriteString(ansiTermDefault)
	}
	mkMsgMutex.Unlock()
}

func main() {
	var mkfilepath string
	var interactive bool
	var dryrun bool
	var shallowrebuild bool
	var quiet bool

	flag.StringVar(&mkfilepath, "f", "mkfile", "use the given file as mkfile")
	flag.BoolVar(&dryrun, "n", false, "print commands without actually executing")
	flag.BoolVar(&shallowrebuild, "r", false, "force building of just targets")
	flag.BoolVar(&rebuildall, "a", false, "force building of all dependencies")
	flag.IntVar(&subprocsAllowed, "p", 4, "maximum number of jobs to execute in parallel")
	flag.BoolVar(&interactive, "i", false, "prompt before executing rules")
	flag.BoolVar(&quiet, "q", false, "don't print recipes before executing them")
	flag.Parse()

	mkfile, err := os.Open(mkfilepath)
	if err != nil {
		mkError("no mkfile found")
	}
	input, _ := ioutil.ReadAll(mkfile)
	mkfile.Close()

	abspath, err := filepath.Abs(mkfilepath)
	if err != nil {
		mkError("unable to find mkfile's absolute path")
	}

	env := make(map[string][]string)
	for _, elem := range os.Environ() {
		vals := strings.SplitN(elem, "=", 2)
		env[vals[0]] = append(env[vals[0]], vals[1])
	}

	rs := parse(string(input), mkfilepath, abspath, env)
	if quiet {
		for i := range rs.rules {
			rs.rules[i].attributes.quiet = true
		}
	}

	targets := flag.Args()

	// build the first non-meta rule in the makefile, if none are given explicitly
	if len(targets) == 0 {
		for i := range rs.rules {
			if !rs.rules[i].ismeta {
				for j := range rs.rules[i].targets {
					targets = append(targets, rs.rules[i].targets[j].spat)
				}
				break
			}
		}
	}

	if len(targets) == 0 {
		fmt.Println("mk: nothing to mk")
		return
	}

	if shallowrebuild {
		for i := range targets {
			rebuildtargets[targets[i]] = true
		}
	}

	// Create a dummy virtual rule that depends on every target
	root := rule{}
	root.targets = []pattern{pattern{false, "", nil}}
	root.attributes = attribSet{false, false, false, false, false, false, false, true, false}
	root.prereqs = targets
	rs.add(root)

	if interactive {
		g := buildgraph(rs, "")
		mkNode(g, g.root, true, true)
		fmt.Print("Proceed? ")
		in := bufio.NewReader(os.Stdin)
		for {
			c, _, err := in.ReadRune()
			if err != nil {
				return
			} else if strings.IndexRune(" \n\t\r", c) >= 0 {
				continue
			} else if c == 'y' {
				break
			} else {
				return
			}
		}
	}

	g := buildgraph(rs, "")
	mkNode(g, g.root, dryrun, true)
}
