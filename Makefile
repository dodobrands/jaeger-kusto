GREEN  := $(shell tput -Txterm setaf 2)
YELLOW := $(shell tput -Txterm setaf 3)
WHITE  := $(shell tput -Txterm setaf 7)
RESET  := $(shell tput -Txterm sgr0)

.PHONY: all
all: help

.PHONY: lint
lint:
	golangci-lint run

.PHONY: tidy
tidy:
	go mod tidy -v

.PHONY: prepare
prepare: tidy lint

.PHONY: build
build:
	go build -v -o jaeger-kusto

.PHONY: test
test:
	@echo "Running tests under test folder"
	@go test -v \
		--tags=integration \
		-timeout 300s ./test/... | \
	sed "/PASS/s//$(printf "\033[32mPASS\033[0m")/" | \
	sed "/FAIL/s//$(printf "\033[31mFAIL\033[0m")/"

.PHONY: help
help:
	@echo ''
	@echo 'Usage:'
	@echo '  ${YELLOW}make${RESET} ${GREEN}<target>${RESET}'
	@echo ''
	@echo 'Targets:'
	@echo "  ${YELLOW}lint                   ${RESET} Run linters via golangci-lint"
	@echo "  ${YELLOW}tidy                   ${RESET} Run tidy for go module to remove unused dependencies"
	@echo "  ${YELLOW}prepare                ${RESET} Run all available checks"
	@echo "  ${YELLOW}build                  ${RESET} Setup local environment. Create kind cluster"
	@echo "  ${YELLOW}test                   ${RESET} Run integration tests"
