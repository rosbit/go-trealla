package main

import (
	"github.com/rosbit/go-trealla"
	"os"
	"fmt"
	"unicode"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Printf("Usage: %s <trealla-exe> <prolog-file> <predict>[ <args>...]\n", os.Args[0])
		return
	}

	// 1. init prolog
	ctx, err := trealla.NewTrealla(os.Args[1])
	if err != nil {
		fmt.Printf("%v\n", err)
		return
	}
	// fmt.Printf("init ok\n")

	plFile, predict := os.Args[2], os.Args[3]
	// 2. consult prolog script file
	if err := ctx.LoadFile(plFile); err != nil {
		fmt.Printf("%v\n", err)
		return
	}
	// fmt.Printf("load %s ok\n", plFile)

	// 3. prepare arguments and variables
	args := make([]interface{}, len(os.Args) - 4)
	for i,j:=4,0; i<len(os.Args); i,j = i+1,j+1 {
		arg := []rune(os.Args[i])
		if len(arg) > 0 && (arg[0] == '_' || unicode.IsUpper(arg[0])) {
			args[j] = trealla.PlVar(os.Args[i])
		} else {
			args[j] = trealla.PlStrTerm(os.Args[i])
		}
	}

	// 4. query the predict with arguments and variables
	solutions, ok, err := ctx.Query(predict, args...)

	// 5. check the result
	//  5.1 error checking
	if err != nil {
		fmt.Printf("failed to query %s: %v\n", predict, err)
		return
	}
	//  5.2 proving checking with result `false`
	if !ok {
		fmt.Printf("false\n")
		return
	}
	//  5.3 proving checking with result `true`
	if solutions == nil {
		fmt.Printf("true\n")
		return
	}

	//  5.4 solutions processing
	i := 0
	for sol := range solutions {
		i += 1
		fmt.Printf("solution #%d: %#v\n", i, sol)
	}
}

