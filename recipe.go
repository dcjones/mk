
package main


import (
    "os/exec"
    "os"
    "io"
)


// A monolithic function for executing recipes.
func executeRecipe(program string,
                   args []string,
                   input string,
                   echo_out bool,
                   echo_err bool,
                   capture_out bool) string {
    cmd := exec.Command(program, args...)

    if echo_out {
        cmdout, err := cmd.StdoutPipe()
        if err != nil {
            go io.Copy(os.Stdout, cmdout)
        }
    }

    if echo_err {
        cmderr, err := cmd.StdoutPipe()
        if err != nil {
            go io.Copy(os.Stderr, cmderr)
        }
    }

    if len(input) > 0 {
        cmdin, err := cmd.StdinPipe()
        go func () { cmdin.Write([]byte(input)) }()
    }


    output := ""
    var err error
    if capture_out {
        output, err = cmd.Output()
    } else {
        err = cmd.Run()
    }

    if err != nil {
        // TODO: better error output
        log.Fatal("Recipe failed")
    }

    return output
}


