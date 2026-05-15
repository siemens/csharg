SHELL:=/bin/bash
GOGEN:=go generate .
BUILDTAGS:="osusergo,netgo"

.PHONY: help clean dist run

help: ## list available targets
	@# Derived from Gomega's Makefile (github.com/onsi/gomega) under MIT License
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-16s\033[0m %s\n", $$1, $$2}'

dist: ## build snapshot csharg binary packages+archives in dist/
# gorelease will run go generate anyway
	@scripts/goreleaser.sh --snapshot --clean
	@ls -lh dist/csharg_*
	@echo "🏁  done"

clean: ## cleans up build and testing artefacts
	rm -rf dist
	find . -name __debug_bin -delete
	rm -f coverage.html coverage.out coverage.txt

run: ## runs csharg with optional ARGS=
	go run -v -tags $(BUILDTAGS) ./cmd/csharg $(ARGS)
