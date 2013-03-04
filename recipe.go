// Various function for dealing with recipes.

package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"unicode/utf8"
)

// Try to unindent a recipe, so that it begins an column 0. (This is mainly for
// recipes in python, or other indentation-significant languages.)
func stripIndentation(s string, mincol int) string {
	// trim leading whitespace
	reader := bufio.NewReader(strings.NewReader(s))
	output := ""
	for {
		line, err := reader.ReadString('\n')
		col := 0
		i := 0
		for i < len(line) && col < mincol {
			c, w := utf8.DecodeRuneInString(line[i:])
			if strings.IndexRune(" \t\n", c) >= 0 {
				col += 1
				i += w
			} else {
				break
			}
		}

		output += line[i:]

		if err != nil {
			break
		}
	}

	return output
}

// Indent each line of a recipe.
func printIndented(out io.Writer, s string, ind int) {
	indentation := strings.Repeat(" ", ind)
	reader := bufio.NewReader(strings.NewReader(s))
	firstline := true
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			if !firstline {
				io.WriteString(out, indentation)
			}
			io.WriteString(out, line)
		}
		if err != nil {
			break
		}
		firstline = false
	}
}

// Execute a recipe.
func dorecipe(target string, u *node, e *edge) bool {
	vars := make(map[string][]string)
	vars["target"] = []string{target}
	if e.r.ismeta {
		if e.r.attributes.regex {
			for i := range e.matches {
				vars[fmt.Sprintf("stem%d", i)] = e.matches[i : i+1]
			}
		} else {
			vars["stem"] = []string{e.stem}
		}
	}

	// TODO: other variables to set
	// alltargets
	// newprereq

	prereqs := make([]string, 0)
	for i := range u.prereqs {
		if u.prereqs[i].r == e.r && u.prereqs[i].v != nil {
			prereqs = append(prereqs, u.prereqs[i].v.name)
		}
	}
	vars["prereqs"] = prereqs

	input := expandRecipeSigils(e.r.recipe, vars)
	sh := "sh"
	args := []string{}

	if len(e.r.shell) > 0 {
		sh = e.r.shell[0]
		args = e.r.shell[1:]
	}

	if !e.r.attributes.quiet {
		mkPrintRecipe(target, input)
	}

	if dryrun {
		return true
	}

	_, success := subprocess(
		sh,
		args,
		input,
		true,
		true,
		false)

	// TODO: update the timestamps of each target

	return success
}

// A monolithic function for executing subprocesses
func subprocess(program string,
	args []string,
	input string,
	echo_out bool,
	echo_err bool,
	capture_out bool) (string, bool) {
	cmd := exec.Command(program, args...)

	if echo_out {
		cmdout, err := cmd.StdoutPipe()
		if err == nil {
			go io.Copy(os.Stdout, cmdout)
		}
	}

	if echo_err {
		cmderr, err := cmd.StderrPipe()
		if err == nil {
			go io.Copy(os.Stderr, cmderr)
		}
	}

	if len(input) > 0 {
		cmdin, err := cmd.StdinPipe()
		if err == nil {
			go func() { cmdin.Write([]byte(input)); cmdin.Close() }()
		}
	}

	output := ""
	var err error
	if capture_out {
		var outbytes []byte
		outbytes, err = cmd.Output()
		output = string(outbytes)
		if output[len(output)-1] == '\n' {
			output = output[:len(output)-1]
		}
	} else {
		err = cmd.Run()
	}
	success := true

	if err != nil {
		exiterr, ok := err.(*exec.ExitError)
		if ok {
			success = exiterr.ProcessState.Success()
		} else {
			log.Fatal(err)
		}
	}

	return output, success
}
