package main

import (
//"fmt"
//"io/ioutil"
//"os"
)

func main() {
	//input, _ := ioutil.ReadAll(os.Stdin)

	// TEST LEXING
	//_, tokens := lex(string(input))
	//for t := range tokens {
	//fmt.Printf("%s %s\n", t.typ, t.val)
	//}

	// TEST PARSING
	//rs := parse(string(input), "<stdin>")
	//fmt.Println(rs)

	// TEST STRING EXPANSION
	rules := &ruleSet{make(map[string][]string), make([]rule, 0)}
	println(rules.expand("\"This is a quote: \\\"\""))
}
