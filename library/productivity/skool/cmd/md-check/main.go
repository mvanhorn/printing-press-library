package main

import (
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/productivity/skool/internal/classroomwrite"
)

func main() {
	raw, _ := os.ReadFile(os.Args[1])
	body, _ := classroomwrite.StripFrontmatter(string(raw))
	desc, err := classroomwrite.MarkdownToLessonDesc(body)
	if err != nil {
		panic(err)
	}
	fmt.Print(desc)
}
