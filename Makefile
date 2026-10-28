SHELL:=/bin/bash
BUILDTAGS:="osusergo,netgo"

.PHONY: help clean dist e2e run

help: ## list available targets
	@# Derived from Gomega's Makefile (github.com/onsi/gomega) under MIT License
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-16s\033[0m %s\n", $$1, $$2}'

dist: ## build snapshot csharg binary packages+archives in dist/
	@goreleaser release --snapshot --clean
	@ls -lh dist/csharg_*
	@echo "🏁  done"

e2e: ## runs the opt-in end-to-end tests
	go test -v -race -p 1 ./e2e/... -ginkgo.label-filter=e2e

clean: ## cleans up build and testing artefacts
	rm -rf dist
	find . -name __debug_bin -delete
	rm -f coverage.html coverage.out coverage.txt
