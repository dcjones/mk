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
func dorecipe(target string, u *node, e *edge, dryrun bool) bool {
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
	vars["prereq"] = prereqs

	input := expandRecipeSigils(e.r.recipe, vars)
	sh := "sh"
	args := []string{}

	if len(e.r.shell) > 0 {
		sh = e.r.shell[0]
		args = e.r.shell[1:]
	}

	mkPrintRecipe(target, input, e.r.attributes.quiet)

	if dryrun {
		return true
	}

	_, success := subprocess(
		sh,
		args,
		input,
		false)

	return success
}

// Execute a subprocess (typically a recipe).
//
// Args:
//   program: Program path or name located in PATH
//   input: String piped into the program's stdin
//   capture_out: If true, capture and return the program's stdout rather than echoing it.
//
// Returns
//   (output, success)
//   output is an empty string of catputer_out is false, or the collected output from the profram is true.
//
//   success is true if the exit code was 0 and false otherwise
//
func subprocess(program string,
	args []string,
	input string,
	capture_out bool) (string, bool) {
	program_path, err := exec.LookPath(program)
	if err != nil {
		log.Fatal(err)
	}

	proc_args := []string{program}
	proc_args = append(proc_args, args...)

	stdin_pipe_read, stdin_pipe_write, err := os.Pipe()
	if err != nil {
		log.Fatal(err)
	}

	attr := os.ProcAttr{Files: []*os.File{stdin_pipe_read, os.Stdout, os.Stderr}}

	output := make([]byte, 0)
	capture_done := make(chan bool)
	if capture_out {
		stdout_pipe_read, stdout_pipe_write, err := os.Pipe()
		if err != nil {
			log.Fatal(err)
		}

		attr.Files[1] = stdout_pipe_write

		go func() {
			buf := make([]byte, 1024)
			for {
				n, err := stdout_pipe_read.Read(buf)

				if err == io.EOF && n == 0 {
					break
				} else if err != nil {
					log.Fatal(err)
				}

				output = append(output, buf[:n]...)
			}

			capture_done <- true
		}()
	}

	proc, err := os.StartProcess(program_path, proc_args, &attr)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		_, err := stdin_pipe_write.WriteString(input)
		if err != nil {
			log.Fatal(err)
		}

		err = stdin_pipe_write.Close()
		if err != nil {
			log.Fatal(err)
		}
	}()

	state, err := proc.Wait()

	if attr.Files[1] != os.Stdout {
		attr.Files[1].Close()
	}

	if err != nil {
		log.Fatal(err)
	}

	// wait until stdout copying in finished
	if capture_out {
		<-capture_done
	}

	return string(output), state.Success()
}
