package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	craftapp "gitflic.ru/skif4er/doctortools/internal/app"
	"gitflic.ru/skif4er/doctortools/internal/products/apidoctor"
	"gitflic.ru/skif4er/doctortools/internal/products/botdoctor"
	"gitflic.ru/skif4er/doctortools/internal/products/configdoctor"
	"gitflic.ru/skif4er/doctortools/internal/products/depdoctor"
	"gitflic.ru/skif4er/doctortools/internal/products/deploydoctor"
	"gitflic.ru/skif4er/doctortools/internal/products/logdoctor"
	"gitflic.ru/skif4er/doctortools/internal/products/stackdoctor"
	"gitflic.ru/skif4er/doctortools/internal/version"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] == "help" || os.Args[1] == "--help" || os.Args[1] == "-h" {
		printHelp()
		return
	}
	switch os.Args[1] {
	case "version", "--version", "-v":
		printVersion(os.Args[2:])
		return
	case "products":
		printProducts(os.Args[2:])
		return
	case "doctor":
		if len(os.Args) >= 3 && os.Args[2] == "list" {
			printProducts(os.Args[3:])
			return
		}
		if len(os.Args) >= 3 && os.Args[2] == "health" {
			printHealth(os.Args[3:])
			return
		}
		fmt.Fprintln(os.Stderr, "Ошибка: поддерживаются команды doctor list и doctor health")
		os.Exit(64)
	}
	productAlias := strings.ToLower(os.Args[1])
	var err error
	switch productAlias {
	case "craft", "craftdoctor", "minecraft":
		err = craftapp.Run(os.Args[2:], os.Stdout, os.Stderr)
	case "stack", "stackdoctor", "fullstack":
		err = stackdoctor.Run(os.Args[2:], os.Stdout, os.Stderr)
	case "dep", "depdoctor", "dependencies":
		err = depdoctor.Run(os.Args[2:], os.Stdout, os.Stderr)
	case "config", "configdoctor", "nginx", "docker", "systemd":
		err = configdoctor.Run(os.Args[2:], os.Stdout, os.Stderr)
	case "deploy", "deploydoctor", "release":
		err = deploydoctor.Run(os.Args[2:], os.Stdout, os.Stderr)
	case "log", "logdoctor", "logs":
		err = logdoctor.Run(os.Args[2:], os.Stdout, os.Stderr)
	case "api", "apidoctor", "backend":
		err = apidoctor.Run(os.Args[2:], os.Stdout, os.Stderr)
	case "bot", "botdoctor", "telegram", "discord", "vk":
		err = botdoctor.Run(os.Args[2:], os.Stdout, os.Stderr)
	default:
		if _, ok := version.ProductByAlias(productAlias); !ok {
			fmt.Fprintln(os.Stderr, "Ошибка: неизвестный продукт DoctorTools:", os.Args[1])
			os.Exit(64)
		}
		fmt.Fprintln(os.Stderr, "Ошибка: продукт зарегистрирован, но не подключён к CLI:", os.Args[1])
		os.Exit(70)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		os.Exit(1)
	}
}

func printVersion(args []string) {
	asJSON := len(args) > 0 && args[0] == "--json"
	meta := struct {
		Name       string            `json:"name"`
		Version    string            `json:"version"`
		Author     string            `json:"author"`
		License    string            `json:"license"`
		ModulePath string            `json:"module_path"`
		Wiki       string            `json:"wiki"`
		Products   []version.Product `json:"products"`
	}{
		Name:       version.PlatformName,
		Version:    version.PlatformVersion,
		Author:     version.Author,
		License:    version.License,
		ModulePath: version.ModulePath,
		Wiki:       version.Wiki,
		Products:   version.ListProducts(),
	}
	if asJSON {
		data, _ := json.MarshalIndent(meta, "", "  ")
		fmt.Println(string(data))
		return
	}
	fmt.Printf("%s %s\n", version.PlatformName, version.PlatformVersion)
	fmt.Printf("DoctorCore %s\n", version.PlatformVersion)
	for _, p := range version.ListProducts() {
		fmt.Printf("%s %s (%s)\n", p.Name, p.Version, p.Status)
	}
}

func printHealth(args []string) {
	asJSON := len(args) > 0 && args[0] == "--json"
	checks := version.ValidateRegistry()
	if asJSON {
		data, _ := json.MarshalIndent(checks, "", "  ")
		fmt.Println(string(data))
		return
	}
	fmt.Println("Проверка DoctorTools registry:")
	for _, check := range checks {
		fmt.Printf("  %-16s %-4s %s\n", check.ID, check.Status, check.Message)
	}
}

func printProducts(args []string) {
	asJSON := len(args) > 0 && args[0] == "--json"
	items := version.ListProducts()
	if asJSON {
		data, _ := json.MarshalIndent(items, "", "  ")
		fmt.Println(string(data))
		return
	}
	fmt.Println("Продукты DoctorTools:")
	for _, p := range items {
		fmt.Printf("  %-13s %-9s %-16s %s\n", p.Name, p.Version, p.Status, p.Description)
	}
}

func printHelp() {
	fmt.Printf(`%s %s
Монорепозиторий русскоязычных CLI-инструментов диагностики и аудита.

Использование:
  doctortools <продукт> <команда> [путь]
  doctortools products [--json]
  doctortools doctor list [--json]
  doctortools doctor health [--json]
  doctortools version [--json]

Продукты:
  craft    CraftDoctor — Minecraft/server diagnostics
  stack    StackDoctor — full-stack diagnostics
  dep      DepDoctor — dependency/supply-chain audit
  config   ConfigDoctor — Linux/Nginx/Docker config audit
  deploy   DeployDoctor — pre-deploy checks
  api      APIDoctor — backend/API diagnostics
  bot      BotDoctor — bot diagnostics
  log      LogDoctor — log analysis

Примеры:
  doctortools craft scan /srv/minecraft
  doctortools dep audit . --json
  doctortools deploy check .
`, version.PlatformName, version.PlatformVersion)
}
