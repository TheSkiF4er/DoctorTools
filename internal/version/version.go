package version

import "sort"

const (
	PlatformName    = "DoctorTools"
	PlatformVersion = "3.0.1"
	Author          = "Дмитрий \"SkiF4er\" Ефимов"
	License         = "Apache-2.0"
	ModulePath      = "gitflic.ru/skif4er/doctortools"
	Wiki            = "GitFlic Wiki"
)

const (
	// Совместимость со старым кодом CraftDoctor.
	Name    = "CraftDoctor"
	Version = "3.0.1"
)

type Product struct {
	ID          string   `json:"id"`
	Alias       string   `json:"alias"`
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Description string   `json:"description"`
	Profile     string   `json:"profile"`
	Commands    []string `json:"commands"`
	Status      string   `json:"status"`
}

type RegistryCheck struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

var Products = map[string]Product{
	"craft":  {ID: "craftdoctor", Alias: "craft", Name: "CraftDoctor", Version: "3.0.1", Description: "Диагностика Minecraft-серверных проектов.", Profile: "minecraft", Commands: []string{"scan", "ci", "rules", "config", "plugins", "logs", "performance", "java flags", "world audit", "proxy", "security", "production", "version"}, Status: "stable"},
	"stack":  {ID: "stackdoctor", Alias: "stack", Name: "StackDoctor", Version: "1.0.1", Description: "Диагностика full-stack проектов.", Profile: "fullstack", Commands: []string{"scan", "structure check", "frontend scan", "backend scan", "env audit", "production check", "version"}, Status: "stable"},
	"dep":    {ID: "depdoctor", Alias: "dep", Name: "DepDoctor", Version: "1.0.1", Description: "Offline-аудит зависимостей и supply-chain рисков.", Profile: "dependencies", Commands: []string{"audit", "lock check", "scripts audit", "licenses", "outdated", "supply-chain scan", "version"}, Status: "stable"},
	"config": {ID: "configdoctor", Alias: "config", Name: "ConfigDoctor", Version: "1.0.1", Description: "Аудит Linux/Nginx/Docker/systemd конфигураций.", Profile: "config", Commands: []string{"audit", "nginx audit", "docker audit", "systemd audit", "env audit", "permissions check", "version"}, Status: "stable"},
	"deploy": {ID: "deploydoctor", Alias: "deploy", Name: "DeployDoctor", Version: "1.0.1", Description: "Pre-deploy проверки перед релизом.", Profile: "deploy", Commands: []string{"check", "release check", "ci check", "env check", "docker check", "archive check", "version"}, Status: "stable"},
	"api":    {ID: "apidoctor", Alias: "api", Name: "APIDoctor", Version: "1.0.1", Description: "Диагностика backend/API проектов.", Profile: "api", Commands: []string{"scan", "openapi validate", "routes check", "security audit", "health check", "probe", "version"}, Status: "stable"},
	"bot":    {ID: "botdoctor", Alias: "bot", Name: "BotDoctor", Version: "1.0.1", Description: "Offline-диагностика Telegram/VK/Discord ботов.", Profile: "bot", Commands: []string{"scan", "telegram check", "discord check", "vk check", "env audit", "webhook check", "bothost check", "security audit", "version"}, Status: "stable"},
	"log":    {ID: "logdoctor", Alias: "log", Name: "LogDoctor", Version: "1.0.1", Description: "Универсальный анализ логов.", Profile: "logs", Commands: []string{"analyze", "summarize", "root-cause", "grep", "scan", "version"}, Status: "stable"},
}

func ListProducts() []Product {
	items := make([]Product, 0, len(Products))
	for _, p := range Products {
		items = append(items, p)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Alias < items[j].Alias })
	return items
}

func ProductByAlias(alias string) (Product, bool) {
	p, ok := Products[alias]
	return p, ok
}

func ValidateRegistry() []RegistryCheck {
	checks := []RegistryCheck{}
	aliases := map[string]bool{}
	ids := map[string]bool{}
	required := []string{"craft", "stack", "dep", "config", "deploy", "api", "bot", "log"}
	for _, alias := range required {
		p, ok := Products[alias]
		if !ok {
			checks = append(checks, RegistryCheck{ID: "registry." + alias, Status: "fail", Message: "Продукт не зарегистрирован"})
			continue
		}
		status := "ok"
		message := p.Name + " зарегистрирован"
		if p.ID == "" || p.Name == "" || p.Version == "" || p.Profile == "" || len(p.Commands) == 0 {
			status = "fail"
			message = "Неполная карточка продукта"
		}
		if aliases[p.Alias] || ids[p.ID] {
			status = "fail"
			message = "Дублирующийся alias или id"
		}
		aliases[p.Alias] = true
		ids[p.ID] = true
		checks = append(checks, RegistryCheck{ID: "registry." + alias, Status: status, Message: message})
	}
	return checks
}
