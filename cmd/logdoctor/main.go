package main

import (
	"fmt"
	"os"

	"gitflic.ru/skif4er/doctortools/internal/products/logdoctor"
)

func main() {
	if err := logdoctor.Run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		os.Exit(1)
	}
}
