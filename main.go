package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "docker_mini: chưa có lệnh nào. Xem ROADMAP.md")
	os.Exit(1)
}
