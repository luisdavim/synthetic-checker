.DEFAULT_GOAL := help
SHELL := bash
OS := $(shell uname -s)

# Make is verbose in Linux. Make it silent.
MAKEFLAGS += --silent

-include .env
export HELM_EXPERIMENTAL_OCI=1
export FILENAME_FORMAT={kind}-{group}-{version}

ifneq ($(CI),true)
	ifneq ($(OS),Windows_NT)
		# Before we start test that we have the mandatory executables available
		EXECUTABLES = jq yq go gocov gofumpt goreleaser svu goimports golangci-lint hadolint yamllint shellcheck
		OK := $(foreach exec,$(EXECUTABLES),\
			$(if $(shell which $(exec)),some string,$(error "No $(exec) in PATH, please install $(exec)")))
	endif
endif

PROJECTNAME := $(shell basename "$(PWD)")
REPO := $(shell git config --get remote.origin.url | sed -re 's/.*(\/\/|@)([^ ]*\/[^.]*).*\.git/\2/' -e 's/:/\//')
BINARY_NAME ?= $(PROJECTNAME)
VERSION ?= $(shell git describe --tags --always --dirty --match='v*' 2> /dev/null | grep -E '^v?[0-9]+\.[0-9]+(\.[0-9]+)?(-[a-z0-9]+)?$$' || cat $(PWD)/.version 2> /dev/null || echo v0.0.0)
NEXT_VERSION ?= $(shell svu next)

GOCMD  := go
GOTEST := $(GOCMD) test
GOVET  := $(GOCMD) vet
GOFILES = $(shell find . -type f -name '*.go' -not -path './vendor/*')

SHELLFILES := $(shell find . -type f -not -path './.git/*' -exec file --mime-type {} + | awk -F':' '{ if ($$2 ~ /^\s+text\/x-shellscript/) print $$1 }'\;)
CHANGELOG_FILE := CHANGELOG.md

SERVICE_PORT?=8080
DOCKER_REGISTRY?= #if set it should end with /
EXPORT_RESULT?=false # for CI please set EXPORT_RESULT to true

K8S_VERSION = 1.21.2
KUBECONFORM_FLGS := -summary -verbose -kubernetes-version=$(K8S_VERSION) -exit-on-error -schema-location default -schema-location 'k8s_schemas/{{ .ResourceKind }}{{ .KindSuffix }}.json'
PROMETHEUS_VERSION = main

GREEN  := $(shell tput -Txterm setaf 2)
YELLOW := $(shell tput -Txterm setaf 3)
WHITE  := $(shell tput -Txterm setaf 7)
CYAN   := $(shell tput -Txterm setaf 6)
RESET  := $(shell tput -Txterm sgr0)

.PHONY: all test fmt vet build vendor deps update-deps imports release tag

all: fmt lint build

## Help:
help: ## Show this help.
	@echo ''
	@echo 'Usage:'
	@echo '  ${YELLOW}make${RESET} ${GREEN}<target>${RESET}'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?##\\s*"} { \
		if (/^[a-zA-Z%_-]+:.*?##.*$$/) {printf "    ${YELLOW}%-20s${GREEN}%s${RESET}\n", $$1, $$2} \
		else if (/^## .*$$/) {printf "  ${CYAN}%s${RESET}\n", substr($$1,4)} \
		}' $(MAKEFILE_LIST)

## Build:
build: test ## Build your project and put the output binary in out/bin/
	mkdir -p bin
	GO111MODULE=on $(GOCMD) build -o bin/$(BINARY_NAME) .

build-release: fmt lint-go ## Build your project for all target platforms and put the output binaries in dist/
	goreleaser build --snapshot --rm-dist

tag: deps fmt lint test ## Bump the previous tag and push the new one
	git tag $(NEXT_VERSION)
	git push --tags

release: test tag ## Release your project and put the output binaries in dist/
	goreleaser release --rm-dist

release-local: deps test ## Release your project and put the output binaries in dist/
	goreleaser release --snapshot --skip-publish --rm-dist
	./scripts/publish.sh

clean: ## Remove build related file
	rm -fr ./bin
	rm -fr ./dist
	rm -fr ./out
	rm -fr ./k8s_schemas
	rm -f ./openapi2jsonschema.*
	rm -f ./junit-report.xml checkstyle-report.xml ./coverage.xml ./profile.cov yamllint-checkstyle.xml

deps: ## Fetch dependencies
	$(GOCMD) get
	$(GOCMD) mod tidy

update-deps: ## Update depdendencies
	$(GOCMD) get -u
	$(GOCMD) mod tidy

vendor: deps ## Copy of all packages needed to support builds and tests in the vendor directory
	$(GOCMD) mod vendor

watch: ## Run the code with cosmtrek/air to have automatic reload on changes
	$(eval PACKAGE_NAME=$(shell head -n 1 go.mod | cut -d ' ' -f2))
	docker run -it --rm -w /go/src/$(PACKAGE_NAME) -v $(shell pwd):/go/src/$(PACKAGE_NAME) -p $(SERVICE_PORT):$(SERVICE_PORT) cosmtrek/air

## Test:
test: deps vet fmt lint-go ## Run the tests of the project
ifeq ($(EXPORT_RESULT), true)
	GO111MODULE=off go get -u github.com/jstemmer/go-junit-report
	$(eval OUTPUT_OPTIONS = | tee /dev/tty | go-junit-report -set-exit-code > junit-report.xml)
endif
	$(GOTEST) -v -race -bench=. -cover ./... $(OUTPUT_OPTIONS)

update-tests: deps vet fmt ## run tests with the -update flag
	$(GOTEST) ./... -update

vet: deps ## Run go vet against code
	$(GOVET) ./...

coverage: ## Run the tests of the project and export the coverage
	$(GOTEST) -cover -covermode=count -coverprofile=profile.cov ./...
	$(GOCMD) tool cover -func profile.cov
ifeq ($(EXPORT_RESULT), true)
	GO111MODULE=off go get -u github.com/AlekSi/gocov-xml
	GO111MODULE=off go get -u github.com/axw/gocov/gocov
	gocov convert profile.cov | gocov-xml > coverage.xml
endif

uncover-%: ## Generates colorized coverage report to stdout of uncovered funcs. Source originates from the golang cover tool.
	$(GOTEST) -cover -coverprofile -coverprofile=profile.cov
	uncover profile.cov $(*)

uncover: ## Generates colorized coverage report to stdout of uncovered funcs. Source originates from the golang cover tool.
	$(GOTEST) -cover -coverprofile -coverprofile=profile.cov
	uncover profile.cov

## Lint:
fmt: imports ## Format the code
	# $(GOCMD) fmt ./...
	# gofmt -s -w .
	gofumpt -w $(GOFILES)

imports: ## Organise imports
	goimports -local $(shell dirname ${REPO}) -w $(GOFILES)

lint: lint-go lint-dockerfile lint-helm lint-yaml lint-shell ## Run all available linters

lint-go: deps ## Use golintci-lint on your project
	$(eval OUTPUT_OPTIONS = $(shell [ "${EXPORT_RESULT}" == "true" ] && echo "--out-format checkstyle ./... | tee /dev/tty > checkstyle-report.xml" || echo "" ))
	golangci-lint run --deadline=65s $(OUTPUT_OPTIONS)

lint-yaml: ## Use yamllint on the yaml file of your projects
ifeq ($(EXPORT_RESULT), true)
	GO111MODULE=off go get -u github.com/thomaspoignant/yamllint-checkstyle
	$(eval OUTPUT_OPTIONS = | tee /dev/tty | yamllint-checkstyle > yamllint-checkstyle.xml)
endif
	yamllint -f parsable $(shell git ls-files '*.yml' '*.yaml' | grep -v "helm/$(PROJECTNAME)") $(OUTPUT_OPTIONS)

lint-dockerfile: ## Lint your Dockerfile
# If dockerfile is present we lint it.
ifeq ($(shell test -e ./Dockerfile && echo -n yes),yes)
	$(eval OUTPUT_OPTIONS = $(shell [ "${EXPORT_RESULT}" == "true" ] && echo "--format checkstyle" || echo "" ))
	$(eval OUTPUT_FILE = $(shell [ "${EXPORT_RESULT}" == "true" ] && echo "| tee /dev/tty > checkstyle-report.xml" || echo "" ))
	hadolint $(OUTPUT_OPTIONS) - < ./Dockerfile $(OUTPUT_FILE)
endif

lint-shell: ## Lint all shell scripts
ifneq ($(SHELLFILES),)
	shellcheck $(SHELLFILES)
endif

lint-helm: ## Lint the helm chart
ifeq ($(shell test -e ./helm/$(PROJECTNAME) && echo -n yes),yes)
	helm lint ./helm/$(PROJECTNAME)
endif

## Helm:
k8s_schemas: ## Get json schemas from CRDs to validate helm templates
	@curl -sLO https://raw.githubusercontent.com/yannh/kubeconform/master/scripts/openapi2jsonschema.py
	@curl -sLO https://raw.githubusercontent.com/yannh/kubeconform/master/scripts/requirements.txt
	@pip3 install -r requirements.txt
	@mkdir -p k8s_schemas
	@chmod +x openapi2jsonschema.py

	# Extend "kind" check to Crossplane's CompositeResourceDefinition and use claimNames when available.
	@sed -i'.original' 's/if y\["kind"\] != "CustomResourceDefinition":/if y\["kind"\] not in \["CustomResourceDefinition", "CompositeResourceDefinition"\]:/' openapi2jsonschema.py
	@sed -i'.original' 's/kind=y\["spec"\]\["names"\]\["kind"\]/kind=y\["spec"\]\["claimNames"\]\["kind"\] if "claimNames" in y\["spec"\] else y\["spec"\]\["names"\]\["kind"\]/' openapi2jsonschema.py

	@cd k8s_schemas && ../openapi2jsonschema.py https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/$(PROMETHEUS_VERSION)/example/prometheus-operator-crd/monitoring.coreos.com_alertmanagerconfigs.yaml
	@cd k8s_schemas && ../openapi2jsonschema.py https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/$(PROMETHEUS_VERSION)/example/prometheus-operator-crd/monitoring.coreos.com_alertmanagers.yaml
	@cd k8s_schemas && ../openapi2jsonschema.py https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/$(PROMETHEUS_VERSION)/example/prometheus-operator-crd/monitoring.coreos.com_podmonitors.yaml
	@cd k8s_schemas && ../openapi2jsonschema.py https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/$(PROMETHEUS_VERSION)/example/prometheus-operator-crd/monitoring.coreos.com_probes.yaml
	@cd k8s_schemas && ../openapi2jsonschema.py https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/$(PROMETHEUS_VERSION)/example/prometheus-operator-crd/monitoring.coreos.com_prometheuses.yaml
	@cd k8s_schemas && ../openapi2jsonschema.py https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/$(PROMETHEUS_VERSION)/example/prometheus-operator-crd/monitoring.coreos.com_prometheusrules.yaml
	@cd k8s_schemas && ../openapi2jsonschema.py https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/$(PROMETHEUS_VERSION)/example/prometheus-operator-crd/monitoring.coreos.com_servicemonitors.yaml
	@cd k8s_schemas && ../openapi2jsonschema.py https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/$(PROMETHEUS_VERSION)/example/prometheus-operator-crd/monitoring.coreos.com_thanosrulers.yaml
	@[ -e openapi2jsonschema.py.original ] && rm openapi2jsonschema.py.original

helm-test: k8s_schemas ## Test the helm chart
	helm template ./helm/$(PROJECTNAME)  | kubeconform  $(KUBECONFORM_FLGS)
	@set -eo pipefail; \
		for values in ./helm/$(PROJECTNAME)/ci/*.yaml; do \
		  echo -e "\n$(YELLOW)>>>$(RESET) Testing $(GREEN)Chart$(RESET) with $(CYAN)$$(basename $$values)$(RESET)"; \
		  helm template ./helm/$(PROJECTNAME) --namespace dev --values $$values | kubeconform $(KUBECONFORM_FLGS); \
		done

helm-docs: ## Generate helm chart docs
	helm-docs -o README.md

## Docker:
docker-build: ## Use the dockerfile to build the container
	docker build --rm --tag $(BINARY_NAME) .

docker-release: ## Release the container with tag latest and version
	docker tag $(BINARY_NAME) $(DOCKER_REGISTRY)$(BINARY_NAME):latest
	docker tag $(BINARY_NAME) $(DOCKER_REGISTRY)$(BINARY_NAME):$(VERSION)
	# Push the docker images
	docker push $(DOCKER_REGISTRY)$(BINARY_NAME):latest
	docker push $(DOCKER_REGISTRY)$(BINARY_NAME):$(VERSION)

## Documentation:
gomarkdoc: ## Use gomarkdoc to generate documentation for eval package
	gomarkdoc --output '{{.Dir}}/README.md' ./...

changelog: ## Generate changelog file
	git-chglog --no-case -o $(CHANGELOG_FILE) --next-tag $(NEXT_VERSION)

codestats: ## Display code statistics
	tokei --hidden
