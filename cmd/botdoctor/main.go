package main

import (
	"fmt"
	"os"

	"gitflic.ru/skif4er/doctortools/internal/products/botdoctor"
)

func main() {
	if err := botdoctor.Run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		os.Exit(1)
	}
}
