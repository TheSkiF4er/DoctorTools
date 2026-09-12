package main

import (
	"fmt"
	"os"

	"gitflic.ru/skif4er/doctortools/internal/products/depdoctor"
)

func main() {
	if err := depdoctor.Run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		os.Exit(1)
	}
}
