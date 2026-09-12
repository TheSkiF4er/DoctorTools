.PHONY: build build-all test vet fmt-check lint smoke release ensure-scripts-executable clean

BINS=doctortools craftdoctor stackdoctor depdoctor configdoctor deploydoctor apidoctor botdoctor logdoctor

build: build-all

build-all: ensure-scripts-executable
	@mkdir -p bin
	@for bin in $(BINS); do echo "Сборка $$bin"; go build -trimpath -o bin/$$bin ./cmd/$$bin; chmod 0755 bin/$$bin; done

test:
	go test ./...

vet:
	go vet ./...

fmt-check:
	@test -z "$$(gofmt -l $$(find . -name '*.go' -not -path './dist/*'))" || (echo 'Найдены Go-файлы без gofmt:' >&2; gofmt -l $$(find . -name '*.go' -not -path './dist/*') >&2; exit 1)

ensure-scripts-executable:
	@chmod 0755 scripts/*.sh

lint: fmt-check vet

smoke: build-all ensure-scripts-executable
	./scripts/smoke.sh

release: ensure-scripts-executable
	./scripts/release.sh

clean:
	rm -rf bin dist
	rm -f craftdoctor doctortools stackdoctor depdoctor configdoctor deploydoctor apidoctor botdoctor logdoctor *.exe
	rm -f *doctor-report.html *doctor-report.md *doctor-report.json craftdoctor-ci-report.json
	rm -f CHANGED_FILES_*.txt DELETED_FILES_*.txt
