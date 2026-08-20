package main

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/other/greatclips/internal/icssign"
	"os"
)

func main() {
	t := os.Args[1]
	body := os.Args[2]
	s := icssign.Sign(t + body)
	fmt.Print(s)
}
