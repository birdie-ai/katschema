COVERAGE_DIR     := .coverage
COVERAGE_PROFILE := $(COVERAGE_DIR)/coverage.out
COVERAGE_HTML    := $(COVERAGE_DIR)/coverage.html

all:

.PHONY: test
test:
	go test -race -shuffle on ./...

.PHONY: test/bench
test/bench:
	go test -bench=.  -benchmem -memprofile memprofile.out -cpuprofile profile.out ./...

.PHONY: check/grammar
check/grammar:
	node grammar/check.js grammar.ohm grammar/examples/*

.PHONY: test/coverage
test/coverage:
	@mkdir -p $(COVERAGE_DIR)
	go test ./... \
		-coverpkg=./... \
		-covermode=atomic \
	@echo
	go tool cover -func=$(COVERAGE_PROFILE)

.PHONY: test/coverage-html
test/coverage-html: test/coverage
	go tool cover \
		-html=$(COVERAGE_PROFILE) \
		-o $(COVERAGE_HTML)
	@echo "report: $(COVERAGE_HTML)"
