#!/usr/bin/env bash
set -euo pipefail

for file in ./bin/doctortools ./bin/craftdoctor ./bin/stackdoctor ./bin/depdoctor ./bin/configdoctor ./bin/deploydoctor ./bin/apidoctor ./bin/botdoctor ./bin/logdoctor; do
  if [[ ! -x "$file" ]]; then
    echo "Бинарник не найден или не исполняемый: $file" >&2
    echo "Запустите: make build-all" >&2
    exit 1
  fi
done

for file in ./scripts/smoke.sh ./scripts/release.sh; do
  if [[ ! -x "$file" ]]; then
    echo "Shell-скрипт не исполняемый: $file" >&2
    echo "Запустите: make ensure-scripts-executable" >&2
    exit 1
  fi
done
./bin/doctortools version | grep -q "DoctorTools 3.0.1"
./bin/doctortools products | grep -q "CraftDoctor"
./bin/doctortools doctor list --json >/tmp/doctortools-products-smoke.json
./bin/doctortools doctor health --json >/tmp/doctortools-health-smoke.json
./bin/craftdoctor version | grep -q "CraftDoctor 3.0.1"
./bin/stackdoctor version | grep -q "StackDoctor 1.0.1"
./bin/depdoctor version | grep -q "DepDoctor 1.0.1"
./bin/configdoctor version | grep -q "ConfigDoctor 1.0.1"
./bin/deploydoctor version | grep -q "DeployDoctor 1.0.1"
./bin/apidoctor version | grep -q "APIDoctor 1.0.1"
./bin/botdoctor version | grep -q "BotDoctor 1.0.1"
./bin/logdoctor version | grep -q "LogDoctor 1.0.1"
./bin/doctortools stack scan . --json >/tmp/doctortools-stack-smoke.json
./bin/stackdoctor structure check . --json >/tmp/stackdoctor-structure-smoke.json
./bin/stackdoctor production check . --json >/tmp/stackdoctor-production-smoke.json
./bin/doctortools dep audit . --json >/tmp/doctortools-dep-smoke.json
./bin/doctortools config audit . --json >/tmp/doctortools-config-smoke.json
./bin/configdoctor nginx audit . --json >/tmp/configdoctor-nginx-smoke.json
./bin/doctortools deploy check . --json >/tmp/doctortools-deploy-smoke.json
./bin/deploydoctor archive check . --json >/tmp/deploydoctor-archive-smoke.json
./bin/doctortools log analyze . --json >/tmp/doctortools-log-smoke.json
./bin/doctortools api scan . --json >/tmp/doctortools-api-smoke.json
./bin/doctortools bot scan . --json >/tmp/doctortools-bot-smoke.json
./bin/botdoctor env audit . --json >/tmp/botdoctor-env-smoke.json
./bin/botdoctor bothost check . --json >/tmp/botdoctor-bothost-smoke.json
./bin/apidoctor openapi validate . --json >/tmp/apidoctor-openapi-smoke.json
./bin/craftdoctor java flags examples/craftdoctor --format json >/tmp/craftdoctor-java-smoke.json
./bin/craftdoctor world audit examples/craftdoctor --format json >/tmp/craftdoctor-world-smoke.json
./bin/craftdoctor production gate examples/craftdoctor --format json >/tmp/craftdoctor-gate-smoke.json
echo "Smoke-проверка DoctorTools 3.0.1 завершена."
