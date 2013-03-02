package main

import (
    "flag"
    "fmt"
    "io/ioutil"
    "os"
)


// The maximum number of times an rule may be applied.
const max_rule_cnt = 3


func mk(rs *ruleSet, target string, dryrun bool) {

    // Build a graph



    // 1. Introduce special variables into the ruleSet
}


func mkError(msg string) {
    fmt.Fprintf(os.Stderr, "mk: %s\n", msg)
    os.Exit(1)
}


func main() {
    var mkfilepath string
    var dryrun bool
    flag.StringVar(&mkfilepath, "f", "mkfile", "use the given file as mkfile")
    flag.BoolVar(&dryrun, "n", false, "print commands without actually executing")
    flag.Parse()

    mkfile, err := os.Open(mkfilepath)
    if err != nil {
        mkError("no mkfile found")
    }
    input, _ := ioutil.ReadAll(mkfile)
    mkfile.Close()

    rs := parse(string(input), mkfilepath)
    targets := flag.Args()

    // build the first non-meta rule in the makefile, if none are given explicitly
    for i := range rs.rules {
        if !rs.rules[i].ismeta {
            for j := range rs.rules[i].targets {
                targets = append(targets, rs.rules[i].targets[j].spat)
            }
        }
    }

    if len(targets) == 0 {
        fmt.Println("mk: nothing to mk")
        return
    }

    for _, target := range targets {
        //fmt.Printf("building: %q\n", target)
        g := buildgraph(rs, target)
        g.visualize(os.Stdout)
        //mk(rs, target, dryrun)
    }

}
