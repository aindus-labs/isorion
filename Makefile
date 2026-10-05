# ISORION — Universal Infrastructure Compiler
# Compatible GNU make (Linux/macOS/Windows-Git-Bash).

BIN     := isorion
CMD     := ./cmd/isorion
OUT_DIR := output

.PHONY: all build test vet fmt examples clean fuzz release-snapshot ci-check

all: build

build:
	go build -o bin/$(BIN)$(EXT) $(CMD)

test:
	go test ./... -count=1

vet:
	go vet ./...

fmt:
	gofmt -w cmd pkg

examples: build
	bin/$(BIN)$(EXT) fusion examples/app.isor -t docker-compose,kubernetes,yaml -o $(OUT_DIR)/app
	bin/$(BIN)$(EXT) fusion examples/multi-env.isor -t docker-compose -o $(OUT_DIR)/multi
	bin/$(BIN)$(EXT) fusion examples/app.isor -t huborion -o $(OUT_DIR)/huborion

fuzz:
	go test ./pkg/parse/ -run=Fuzz -fuzz=FuzzImport -fuzztime=60s

# Release pipeline (goreleaser — tag v* déclenche une vraie release)
release-snapshot:
	goreleaser release --snapshot --clean

# Mêmes gates que .github/workflows/ci.yml, en local
ci-check: vet test
	@test -z "$$(gofmt -l .)" || (echo "unformatted:"; gofmt -l .; exit 1)

clean:
	rm -rf bin $(OUT_DIR) dist
