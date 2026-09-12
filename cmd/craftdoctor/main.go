package main

import (
	"fmt"
	"os"

	"gitflic.ru/skif4er/doctortools/internal/app"
)

func main() {
	if err := app.Run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		os.Exit(app.ExitCode(err))
	}
}
