
package main

import (
    "fmt"
    "os"
    "io/ioutil"
)

func main() {
    input, _ := ioutil.ReadAll(os.Stdin)
    l, tokens := lex(string(input))

    for t := range tokens {
        if t.typ == tokenError {
            fmt.Printf("Error: %s", l.errmsg)
            break
        }

        fmt.Println(t.String())
    }
}


