package main

import (
	"fmt"
	"io/ioutil"
	"os"
)

func main() {
	input, _ := ioutil.ReadAll(os.Stdin)
	rs := parse(string(input), "<stdin>")
	fmt.Println(rs)
}
