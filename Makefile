# Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

REGISTRY                                   := eu.gcr.io/gardener-project/gardener
APISERVER_IMAGE_REPOSITORY                 := $(REGISTRY)/apiserver
CONTROLLER_MANAGER_IMAGE_REPOSITORY        := $(REGISTRY)/controller-manager
NODE_AGENT_IMAGE_REPOSITORY                := $(REGISTRY)/node-agent
SCHEDULER_IMAGE_REPOSITORY                 := $(REGISTRY)/scheduler
ADMISSION_IMAGE_REPOSITORY                 := $(REGISTRY)/admission-controller
RESOURCE_MANAGER_IMAGE_REPOSITORY          := $(REGISTRY)/resource-manager
OPERATOR_IMAGE_REPOSITORY                  := $(REGISTRY)/operator
GARDENLET_IMAGE_REPOSITORY                 := $(REGISTRY)/gardenlet
EXTENSION_PROVIDER_LOCAL_IMAGE_REPOSITORY  := $(REGISTRY)/extensions/provider-local
PUSH_LATEST_TAG                            := false
VERSION                                    := $(shell cat VERSION)
EFFECTIVE_VERSION                          := $(VERSION)-$(shell git rev-parse HEAD)
BUILD_DATE                                 := $(shell date '+%Y-%m-%dT%H:%M:%S%z' | sed 's/\([0-9][0-9]\)$$/:\1/g')
REPO_ROOT                                  := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
GARDENER_LOCAL_KUBECONFIG                  := $(REPO_ROOT)/example/gardener-local/kind/local/kubeconfig
GARDENER_LOCAL2_KUBECONFIG                 := $(REPO_ROOT)/example/gardener-local/kind/local2/kubeconfig
GARDENER_EXTENSIONS_KUBECONFIG             := $(REPO_ROOT)/example/gardener-local/kind/extensions/kubeconfig
GARDENER_LOCAL_HA_SINGLE_ZONE_KUBECONFIG   := $(REPO_ROOT)/example/gardener-local/kind/ha-single-zone/kubeconfig
GARDENER_LOCAL2_HA_SINGLE_ZONE_KUBECONFIG  := $(REPO_ROOT)/example/gardener-local/kind/ha-single-zone2/kubeconfig
GARDENER_LOCAL_HA_MULTI_ZONE_KUBECONFIG    := $(REPO_ROOT)/example/gardener-local/kind/ha-multi-zone/kubeconfig
GARDENER_LOCAL_OPERATOR_KUBECONFIG         := $(REPO_ROOT)/example/gardener-local/kind/operator/kubeconfig
GARDENER_PREVIOUS_RELEASE                  := ""
GARDENER_NEXT_RELEASE                      := $(VERSION)
LOCAL_GARDEN_LABEL                         := local-garden
REMOTE_GARDEN_LABEL                        := remote-garden
ACTIVATE_SEEDAUTHORIZER                    := false
SEED_NAME                                  := provider-extensions
SEED_KUBECONFIG                            := $(REPO_ROOT)/example/provider-extensions/seed/kubeconfig
DEV_SETUP_WITH_WEBHOOKS                    := false
IPFAMILY                                   := ipv4
PARALLEL_E2E_TESTS                         := 15
GARDENER_RELEASE_DOWNLOAD_PATH             := $(REPO_ROOT)/dev

ifneq ($(SEED_NAME),provider-extensions)
	SEED_KUBECONFIG := $(REPO_ROOT)/example/provider-extensions/seed/kubeconfig-$(SEED_NAME)
endif

ifndef ARTIFACTS
	export ARTIFACTS=/tmp/artifacts
endif

ifneq ($(strip $(shell git status --porcelain 2>/dev/null)),)
	EFFECTIVE_VERSION := $(EFFECTIVE_VERSION)-dirty
endif

SHELL=/usr/bin/env bash -o pipefail

#########################################
# Tools                                 #
#########################################

TOOLS_DIR := hack/tools
include hack/tools.mk

LOGCHECK_DIR := $(TOOLS_DIR)/logcheck
GOMEGACHECK_DIR := $(TOOLS_DIR)/gomegacheck

#########################################
# Rules for local development scenarios #
#########################################

register-local-env: export IPFAMILY := $(IPFAMILY)

ENVTEST_TYPE ?= kubernetes

.PHONY: start-envtest
start-envtest: $(SETUP_ENVTEST)
	@./hack/start-envtest.sh --environment-type=$(ENVTEST_TYPE)

#################################################################
# Rules related to binary build, Docker image build and release #
#################################################################

.PHONY: install
install:
	@EFFECTIVE_VERSION=$(EFFECTIVE_VERSION) ./hack/install.sh ./...

.PHONY: docker-images
docker-images:
	@echo "Building docker images with version and tag $(EFFECTIVE_VERSION)"
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(APISERVER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)                -t $(APISERVER_IMAGE_REPOSITORY):latest                -f Dockerfile --target apiserver .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(CONTROLLER_MANAGER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)       -t $(CONTROLLER_MANAGER_IMAGE_REPOSITORY):latest       -f Dockerfile --target controller-manager .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(NODE_AGENT_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)               -t $(NODE_AGENT_IMAGE_REPOSITORY):latest               -f Dockerfile --target node-agent .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(SCHEDULER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)                -t $(SCHEDULER_IMAGE_REPOSITORY):latest                -f Dockerfile --target scheduler .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(ADMISSION_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)                -t $(ADMISSION_IMAGE_REPOSITORY):latest                -f Dockerfile --target admission-controller .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(RESOURCE_MANAGER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)         -t $(RESOURCE_MANAGER_IMAGE_REPOSITORY):latest         -f Dockerfile --target resource-manager .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(OPERATOR_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)                 -t $(OPERATOR_IMAGE_REPOSITORY):latest                 -f Dockerfile --target operator .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(GARDENLET_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)                -t $(GARDENLET_IMAGE_REPOSITORY):latest                -f Dockerfile --target gardenlet .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(EXTENSION_PROVIDER_LOCAL_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION) -t $(EXTENSION_PROVIDER_LOCAL_IMAGE_REPOSITORY):latest -f Dockerfile --target gardener-extension-provider-local .

.PHONY: docker-push
docker-push:
	@if ! docker images $(APISERVER_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(APISERVER_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(CONTROLLER_MANAGER_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(CONTROLLER_MANAGER_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(NODE_AGENT_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(NODE_AGENT_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(SCHEDULER_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(SCHEDULER_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(ADMISSION_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(ADMISSION_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(RESOURCE_MANAGER_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(RESOURCE_MANAGER_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(GARDENLET_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(GARDENLET_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(EXTENSION_PROVIDER_LOCAL_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(EXTENSION_PROVIDER_LOCAL_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@docker push $(APISERVER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)
	@if [[ "$(PUSH_LATEST_TAG)" == "true" ]]; then docker push $(APISERVER_IMAGE_REPOSITORY):latest; fi
	@docker push $(CONTROLLER_MANAGER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)
	@if [[ "$(PUSH_LATEST_TAG)" == "true" ]]; then docker push $(CONTROLLER_MANAGER_IMAGE_REPOSITORY):latest; fi
	@docker push $(SCHEDULER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)
	@if [[ "$(PUSH_LATEST_TAG)" == "true" ]]; then docker push $(SCHEDULER_IMAGE_REPOSITORY):latest; fi
	@docker push $(ADMISSION_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)
	@if [[ "$(PUSH_LATEST_TAG)" == "true" ]]; then docker push $(ADMISSION_IMAGE_REPOSITORY):latest; fi
	@docker push $(RESOURCE_MANAGER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)
	@if [[ "$(PUSH_LATEST_TAG)" == "true" ]]; then docker push $(RESOURCE_MANAGER_IMAGE_REPOSITORY):latest; fi
	@docker push $(GARDENLET_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)
	@if [[ "$(PUSH_LATEST_TAG)" == "true" ]]; then docker push $(GARDENLET_IMAGE_REPOSITORY):latest; fi
	@docker push $(EXTENSION_PROVIDER_LOCAL_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)
	@if [[ "$(PUSH_LATEST_TAG)" == "true" ]]; then docker push $(EXTENSION_PROVIDER_LOCAL_IMAGE_REPOSITORY):latest; fi

#####################################################################
# Rules for verification, formatting, linting, testing and cleaning #
#####################################################################

.PHONY: revendor
revendor:
	@GO111MODULE=on go mod tidy
	@GO111MODULE=on go mod vendor
	@cd $(LOGCHECK_DIR); go mod tidy
	@cd $(GOMEGACHECK_DIR); go mod tidy

.PHONY: clean
clean:
	@hack/clean.sh ./cmd/... ./extensions/... ./pkg/... ./plugin/... ./test/...

.PHONY: add-license-headers
add-license-headers: $(GO_ADD_LICENSE)
	@./hack/add-license-header.sh

.PHONY: check-generate
check-generate:
	@hack/check-generate.sh $(REPO_ROOT)

.PHONY: check
check: $(GO_ADD_LICENSE) $(GOIMPORTS) $(GOLANGCI_LINT) $(HELM) $(IMPORT_BOSS) $(LOGCHECK) $(GOMEGACHECK) $(YQ)
	@hack/check.sh --golangci-lint-config=./.golangci.yaml ./cmd/... ./extensions/... ./pkg/... ./plugin/... ./test/...
	@hack/check-imports.sh ./charts/... ./cmd/... ./extensions/... ./pkg/... ./plugin/... ./test/... ./third_party/...

	@echo "> Check $(LOGCHECK_DIR)"
	@cd $(LOGCHECK_DIR); $(abspath $(GOLANGCI_LINT)) run -c $(REPO_ROOT)/.golangci.yaml --timeout 10m ./...
	@cd $(LOGCHECK_DIR); go vet ./...
	@cd $(LOGCHECK_DIR); $(abspath $(GOIMPORTS)) -l .

	@echo "> Check $(GOMEGACHECK_DIR)"
	@cd $(GOMEGACHECK_DIR); $(abspath $(GOLANGCI_LINT)) run -c $(REPO_ROOT)/.golangci.yaml --timeout 10m ./...
	@cd $(GOMEGACHECK_DIR); go vet ./...
	@cd $(GOMEGACHECK_DIR); $(abspath $(GOIMPORTS)) -l .

	@hack/check-charts.sh ./charts
	@hack/check-license-header.sh
	@hack/check-skaffold-deps.sh

tools-for-generate: $(CONTROLLER_GEN) $(GEN_CRD_API_REFERENCE_DOCS) $(GOIMPORTS) $(GO_TO_PROTOBUF) $(HELM) $(MOCKGEN) $(OPENAPI_GEN) $(PROTOC_GEN_GOGO) $(YAML2JSON) $(YQ)

.PHONY: generate
generate: tools-for-generate
	@hack/update-protobuf.sh
	@hack/update-codegen.sh
	@hack/generate-parallel.sh charts cmd example extensions pkg plugin test
	@cd $(LOGCHECK_DIR); go generate ./...
	@cd $(GOMEGACHECK_DIR); go generate ./...
	@hack/generate-monitoring-docs.sh
	$(MAKE) format

.PHONY: generate-sequential
generate-sequential: tools-for-generate
	@hack/update-protobuf.sh
	@hack/update-codegen.sh
	@hack/generate.sh ./charts/... ./cmd/... ./example/... ./extensions/... ./pkg/... ./plugin/... ./test/...
	@cd $(LOGCHECK_DIR); go generate ./...
	@cd $(GOMEGACHECK_DIR); go generate ./...
	@hack/generate-monitoring-docs.sh
	$(MAKE) format

.PHONY: format
format: $(GOIMPORTS) $(GOIMPORTSREVISER)
	@./hack/format.sh ./cmd ./extensions ./pkg ./plugin ./test ./hack
	@cd $(LOGCHECK_DIR); $(abspath $(GOIMPORTS)) -l -w .
	@cd $(GOMEGACHECK_DIR); $(abspath $(GOIMPORTS)) -l -w .

.PHONY: test
test: $(REPORT_COLLECTOR) $(PROMTOOL)
	@./hack/test.sh ./cmd/... ./extensions/pkg/... ./pkg/... ./plugin/...
	@cd $(LOGCHECK_DIR); go test -race -timeout=2m ./... | grep -v 'no test files'
	@cd $(GOMEGACHECK_DIR); go test -race -timeout=2m ./... | grep -v 'no test files'

.PHONY: test-integration
test-integration: $(REPORT_COLLECTOR) $(SETUP_ENVTEST)
	@./hack/test-integration.sh ./test/integration/...

.PHONY: test-cov
test-cov: $(PROMTOOL)
	@./hack/test-cover.sh ./cmd/... ./extensions/pkg/... ./pkg/... ./plugin/...

.PHONY: test-cov-clean
test-cov-clean:
	@./hack/test-cover-clean.sh

.PHONY: check-apidiff
check-apidiff: $(GO_APIDIFF)
	@./hack/check-apidiff.sh

.PHONY: check-vulnerabilities
check-vulnerabilities: $(GO_VULN_CHECK)
	$(GO_VULN_CHECK) ./...

.PHONY: test-prometheus
test-prometheus: $(PROMTOOL)
	@./hack/test-prometheus.sh

.PHONY: check-docforge
check-docforge: $(DOCFORGE)
	@./hack/check-docforge.sh

.PHONY: verify
verify: check format test test-integration test-prometheus

.PHONY: verify-extended
verify-extended: check-generate check format test-cov test-cov-clean test-integration test-prometheus

#####################################################################
# Rules for local environment                                       #
#####################################################################

kind-% kind2-% gardener-%: export IPFAMILY := $(IPFAMILY)
# KUBECONFIG
kind-up kind-down gardener-up gardener-dev gardener-debug gardener-down register-local-env tear-down-local-env register-kind2-env tear-down-kind2-env: export KUBECONFIG = $(GARDENER_LOCAL_KUBECONFIG)
test-e2e-local-simple test-e2e-local-migration test-e2e-local-workerless test-e2e-local ci-e2e-kind ci-e2e-kind-upgrade: export KUBECONFIG = $(GARDENER_LOCAL_KUBECONFIG)
kind2-up kind2-down gardenlet-kind2-up gardenlet-kind2-dev gardenlet-kind2-debug gardenlet-kind2-down: export KUBECONFIG = $(GARDENER_LOCAL2_KUBECONFIG)
kind-extensions-up kind-extensions-down gardener-extensions-up gardener-extensions-down: export KUBECONFIG = $(GARDENER_EXTENSIONS_KUBECONFIG)
kind-ha-single-zone-up kind-ha-single-zone-down gardener-ha-single-zone-up gardener-ha-single-zone-down register-kind-ha-single-zone-env tear-down-kind-ha-single-zone-env register-kind2-ha-single-zone-env tear-down-kind2-ha-single-zone-env: export KUBECONFIG = $(GARDENER_LOCAL_HA_SINGLE_ZONE_KUBECONFIG)
test-e2e-local-ha-single-zone test-e2e-local-migration-ha-single-zone ci-e2e-kind-ha-single-zone ci-e2e-kind-ha-single-zone-upgrade: export KUBECONFIG = $(GARDENER_LOCAL_HA_SINGLE_ZONE_KUBECONFIG)
kind2-ha-single-zone-up kind2-ha-single-zone-down gardenlet-kind2-ha-single-zone-up gardenlet-kind2-ha-single-zone-dev gardenlet-kind2-ha-single-zone-debug gardenlet-kind2-ha-single-zone-down: export KUBECONFIG = $(GARDENER_LOCAL2_HA_SINGLE_ZONE_KUBECONFIG)
kind-ha-multi-zone-up kind-ha-multi-zone-down gardener-ha-multi-zone-up register-kind-ha-multi-zone-env tear-down-kind-ha-multi-zone-env: export KUBECONFIG = $(GARDENER_LOCAL_HA_MULTI_ZONE_KUBECONFIG)
test-e2e-local-ha-multi-zone ci-e2e-kind-ha-multi-zone ci-e2e-kind-ha-multi-zone-upgrade: export KUBECONFIG = $(GARDENER_LOCAL_HA_MULTI_ZONE_KUBECONFIG)
kind-operator-up kind-operator-down operator-up operator-dev operator-debug operator-down test-e2e-local-operator ci-e2e-kind-operator: export KUBECONFIG = $(GARDENER_LOCAL_OPERATOR_KUBECONFIG)
# CLUSTER_NAME
kind-up kind-down: export CLUSTER_NAME = gardener-local
kind2-up kind2-down: export CLUSTER_NAME = gardener-local2
kind-ha-single-zone-up kind-ha-single-zone-down: export CLUSTER_NAME = gardener-local-ha-single-zone
kind2-ha-single-zone-up kind2-ha-single-zone-down: export CLUSTER_NAME = gardener-local2-ha-single-zone
kind-ha-multi-zone-up kind-ha-multi-zone-down: export CLUSTER_NAME = gardener-local-ha-multi-zone
# KIND_KUBECONFIG
kind-up kind-down: export KIND_KUBECONFIG = $(REPO_ROOT)/example/provider-local/seed-kind/base/kubeconfig
kind2-up kind2-down: export KIND_KUBECONFIG = $(REPO_ROOT)/example/provider-local/seed-kind2/base/kubeconfig
kind-ha-single-zone-up kind-ha-single-zone-down: export KIND_KUBECONFIG = $(REPO_ROOT)/example/provider-local/seed-kind-ha-single-zone/base/kubeconfig
kind2-ha-single-zone-up kind2-ha-single-zone-down: export KIND_KUBECONFIG = $(REPO_ROOT)/example/provider-local/seed-kind2-ha-single-zone/base/kubeconfig
kind-ha-multi-zone-up kind-ha-multi-zone-down: export KIND_KUBECONFIG = $(REPO_ROOT)/example/provider-local/seed-kind-ha-multi-zone/base/kubeconfig
# CLUSTER_VALUES
kind-up kind-down: export CLUSTER_VALUES = $(REPO_ROOT)/example/gardener-local/kind/local/values.yaml
kind2-up kind2-down: export CLUSTER_VALUES = $(REPO_ROOT)/example/gardener-local/kind/local2/values.yaml
kind-ha-single-zone-up kind-ha-single-zone-down: export CLUSTER_VALUES = $(REPO_ROOT)/example/gardener-local/kind/ha-single-zone/values.yaml
kind2-ha-single-zone-up kind2-ha-single-zone-down: export CLUSTER_VALUES = $(REPO_ROOT)/example/gardener-local/kind/ha-single-zone2/values.yaml
kind-ha-multi-zone-up kind-ha-multi-zone-down: export CLUSTER_VALUES = $(REPO_ROOT)/example/gardener-local/kind/ha-multi-zone/values.yaml
# ADDITIONAL_PARAMETERS
kind2-up kind2-ha-single-zone-up: export ADDITIONAL_PARAMETERS = --skip-registry
kind2-down: export ADDITIONAL_PARAMETERS = --keep-backupbuckets-dir
kind-ha-multi-zone-up: export ADDITIONAL_PARAMETERS = --multi-zonal

kind-up kind2-up kind-ha-single-zone-up kind2-ha-single-zone-up kind-ha-multi-zone-up: $(KIND) $(KUBECTL) $(HELM) $(YQ)
	./hack/kind-up.sh --cluster-name $(CLUSTER_NAME) --path-kubeconfig $(KIND_KUBECONFIG) --path-cluster-values $(CLUSTER_VALUES) $(ADDITIONAL_PARAMETERS)
kind-down kind2-down kind-ha-single-zone-down kind2-ha-single-zone-down kind-ha-multi-zone-down: $(KIND)
	./hack/kind-down.sh --cluster-name $(CLUSTER_NAME) --path-kubeconfig $(KIND_KUBECONFIG) $(ADDITIONAL_PARAMETERS)

kind-extensions-up: $(KIND) $(KUBECTL)
	REPO_ROOT=$(REPO_ROOT) ./hack/kind-extensions-up.sh
kind-extensions-down: $(KIND)
	docker stop gardener-extensions-control-plane
kind-extensions-clean:
	./hack/kind-down.sh --cluster-name gardener-extensions --path-kubeconfig $(REPO_ROOT)/example/provider-extensions/garden/kubeconfig

kind-operator-up: $(KIND) $(KUBECTL) $(HELM) $(YQ)
	./hack/kind-up.sh --cluster-name gardener-operator-local --path-kubeconfig $(REPO_ROOT)/example/gardener-local/kind/operator/kubeconfig --path-cluster-values $(REPO_ROOT)/example/gardener-local/kind/operator/values.yaml
	mkdir -p $(REPO_ROOT)/dev/local-backupbuckets/gardener-operator
kind-operator-down: $(KIND)
	./hack/kind-down.sh --cluster-name gardener-operator-local --path-kubeconfig $(REPO_ROOT)/example/gardener-local/kind/operator/kubeconfig
	# We need root privileges to clean the backup bucket directory, see https://github.com/gardener/gardener/issues/6752
	docker run --user root:root -v $(REPO_ROOT)/dev/local-backupbuckets:/dev/local-backupbuckets alpine rm -rf /dev/local-backupbuckets/gardener-operator

# speed-up skaffold deployments by building all images concurrently
export SKAFFOLD_BUILD_CONCURRENCY = 0
gardener%up gardener%dev gardener%debug gardenlet%up gardenlet%dev gardenlet%debug operator-up operator-dev operator-debug: export SKAFFOLD_DEFAULT_REPO = localhost:5001
gardener%up gardener%dev gardener%debug gardenlet%up gardenlet%dev gardenlet%debug operator-up operator-dev operator-debug: export SKAFFOLD_PUSH = true
gardener%up gardener%dev gardener%debug gardenlet%up gardenlet%dev gardenlet%debug operator-up operator-dev operator-debug: export SOURCE_DATE_EPOCH = $(shell date -d $(BUILD_DATE) +%s)
# use static label for skaffold to prevent rolling all gardener components on every `skaffold` invocation
gardener%up gardener%dev gardener%debug gardener%down gardenlet%up gardenlet%dev gardenlet%debug gardenlet%down: export SKAFFOLD_LABEL = skaffold.dev/run-id=gardener-local
# set ldflags for skaffold
gardener%up gardener%dev gardener%debug gardenlet%up gardenlet%dev gardenlet%debug operator-up operator-dev operator-debug: export LD_FLAGS = $(shell $(REPO_ROOT)/hack/get-build-ld-flags.sh k8s.io/component-base $(REPO_ROOT)/VERSION Gardener $(BUILD_DATE))
# skaffold dev and debug clean up deployed modules by default, disable this
gardener%dev gardener%debug gardenlet%dev gardenlet%debug operator-dev operator-debug: export SKAFFOLD_CLEANUP = false
# skaffold dev triggers new builds and deployments immediately on file changes by default,
# this is too heavy in a large project like gardener, so trigger new builds and deployments manually instead.
gardener%dev gardenlet%dev operator-dev: export SKAFFOLD_TRIGGER = manual
# Artifacts might be already built when you decide to start debugging.
# However, these artifacts do not include the gcflags which `skaffold debug` sets automatically, so delve would not work.
# Disabling the skaffold cache for debugging ensures that you run artifacts with gcflags required for debugging.
gardener%debug gardenlet%debug operator-debug: export SKAFFOLD_CACHE_ARTIFACTS = false

gardener-ha-single-zone%: export SKAFFOLD_PROFILE=ha-single-zone
gardener-ha-multi-zone%: export SKAFFOLD_PROFILE=ha-multi-zone

gardener-up gardener-ha-single-zone-up gardener-ha-multi-zone-up: $(SKAFFOLD) $(HELM) $(KUBECTL) $(YQ)
	$(SKAFFOLD) run
gardener-dev gardener-ha-single-zone-dev gardener-ha-multi-zone-dev: $(SKAFFOLD) $(HELM) $(KUBECTL) $(YQ)
	$(SKAFFOLD) dev
gardener-debug gardener-ha-single-zone-debug gardener-ha-multi-zone-debug: $(SKAFFOLD) $(HELM) $(KUBECTL) $(YQ)
	$(SKAFFOLD) debug
gardener-down gardener-ha-single-zone-down gardener-ha-multi-zone-down: $(SKAFFOLD) $(HELM) $(KUBECTL)
	./hack/gardener-down.sh

gardener-extensions-%: export SKAFFOLD_LABEL = skaffold.dev/run-id=gardener-extensions

gardener-extensions-up: $(SKAFFOLD) $(HELM) $(KUBECTL) $(YQ)
	./hack/gardener-extensions-up.sh --path-garden-kubeconfig $(REPO_ROOT)/example/provider-extensions/garden/kubeconfig --path-seed-kubeconfig $(SEED_KUBECONFIG) --seed-name $(SEED_NAME)
gardener-extensions-down: $(SKAFFOLD) $(HELM) $(KUBECTL)
	./hack/gardener-extensions-down.sh --path-garden-kubeconfig $(REPO_ROOT)/example/provider-extensions/garden/kubeconfig --path-seed-kubeconfig $(SEED_KUBECONFIG) --seed-name $(SEED_NAME)

register-local-env tear-down-local-env: export SEED_FOLDER = seed-kind
register-kind-ha-single-zone-env tear-down-kind-ha-single-zone-env: export SEED_FOLDER = seed-kind-ha-single-zone
register-kind-ha-multi-zone-env tear-down-kind-ha-multi-zone-env: export SEED_FOLDER = seed-kind-ha-multi-zone

register-local-env register-kind-ha-single-zone-env register-kind-ha-multi-zone-env: $(KUBECTL)
	$(KUBECTL) apply -k $(REPO_ROOT)/example/provider-local/garden/local
	@if [[ "$(IPFAMILY)" != "ipv6" ]]; then $(KUBECTL) apply -k $(REPO_ROOT)/example/provider-local/$(SEED_FOLDER)/local; else $(KUBECTL) apply -k $(REPO_ROOT)/example/provider-local/$(SEED_FOLDER)/local-ipv6; fi

tear-down-local-env tear-down-kind-ha-single-zone-env tear-down-kind-ha-multi-zone-env: $(KUBECTL)
	$(KUBECTL) annotate project local confirmation.gardener.cloud/deletion=true
	$(KUBECTL) delete -k $(REPO_ROOT)/example/provider-local/$(SEED_FOLDER)/local
	$(KUBECTL) delete -k $(REPO_ROOT)/example/provider-local/garden/local

gardenlet-kind2-up gardenlet-kind2-dev gardenlet-kind2-debug gardenlet-kind2-down register-kind2-env tear-down-kind2-env: export SKAFFOLD_PREFIX_NAME = kind2
gardenlet-kind2-ha-single-zone-up gardenlet-kind2-ha-single-zone-dev gardenlet-kind2-ha-single-zone-debug gardenlet-kind2-ha-single-zone-down register-kind2-ha-single-zone-env tear-down-kind2-ha-single-zone-env: export SKAFFOLD_PREFIX_NAME = kind2-ha-single-zone
gardenlet-kind2-up gardenlet-kind2-dev gardenlet-kind2-debug gardenlet-kind2-down register-kind2-env tear-down-kind2-env: export SKAFFOLD_COMMAND_KUBECONFIG := $(GARDENER_LOCAL_KUBECONFIG)
gardenlet-kind2-ha-single-zone-up gardenlet-kind2-ha-single-zone-dev gardenlet-kind2-ha-single-zone-debug gardenlet-kind2-ha-single-zone-down register-kind2-ha-single-zone-env tear-down-kind2-ha-single-zone-env: export SKAFFOLD_COMMAND_KUBECONFIG := $(GARDENER_LOCAL_HA_SINGLE_ZONE_KUBECONFIG)

gardenlet-kind2-up gardenlet-kind2-ha-single-zone-up: $(SKAFFOLD) $(HELM) $(KUBECTL)
	$(SKAFFOLD) deploy -m $(SKAFFOLD_PREFIX_NAME)-env -p $(SKAFFOLD_PREFIX_NAME) --kubeconfig=$(SKAFFOLD_COMMAND_KUBECONFIG)
	@# define GARDENER_LOCAL_KUBECONFIG so that it can be used by skaffold when checking whether the seed managed by this gardenlet is ready
	GARDENER_LOCAL_KUBECONFIG=$(SKAFFOLD_COMMAND_KUBECONFIG) $(SKAFFOLD) run -m gardenlet -p $(SKAFFOLD_PREFIX_NAME)
gardenlet-kind2-dev gardenlet-kind2-ha-single-zone-dev: $(SKAFFOLD) $(HELM) $(KUBECTL)
	$(SKAFFOLD) deploy -m $(SKAFFOLD_PREFIX_NAME)-env -p $(SKAFFOLD_PREFIX_NAME) --kubeconfig=$(SKAFFOLD_COMMAND_KUBECONFIG)
	@# define GARDENER_LOCAL_KUBECONFIG so that it can be used by skaffold when checking whether the seed managed by this gardenlet is ready
	GARDENER_LOCAL_KUBECONFIG=$(SKAFFOLD_COMMAND_KUBECONFIG) $(SKAFFOLD) dev -m gardenlet -p $(SKAFFOLD_PREFIX_NAME)
gardenlet-kind2-debug gardenlet-kind2-ha-single-zone-debug: $(SKAFFOLD) $(HELM)
	$(SKAFFOLD) deploy -m $(SKAFFOLD_PREFIX_NAME)-env -p $(SKAFFOLD_PREFIX_NAME) --kubeconfig=$(SKAFFOLD_COMMAND_KUBECONFIG)
	@# define GARDENER_LOCAL_KUBECONFIG so that it can be used by skaffold when checking whether the seed managed by this gardenlet is ready
	GARDENER_LOCAL_KUBECONFIG=$(SKAFFOLD_COMMAND_KUBECONFIG) $(SKAFFOLD) debug -m gardenlet -p $(SKAFFOLD_PREFIX_NAME)
gardenlet-kind2-down gardenlet-kind2-ha-single-zone-down: $(SKAFFOLD) $(HELM) $(KUBECTL)
	$(SKAFFOLD) delete -m $(SKAFFOLD_PREFIX_NAME)-env -p $(SKAFFOLD_PREFIX_NAME) --kubeconfig=$(SKAFFOLD_COMMAND_KUBECONFIG)
	$(SKAFFOLD) delete -m gardenlet,$(SKAFFOLD_PREFIX_NAME)-env -p $(SKAFFOLD_PREFIX_NAME)
register-kind2-env register-kind2-ha-single-zone-env: $(KUBECTL)
	$(KUBECTL) apply -k $(REPO_ROOT)/example/provider-local/seed-$(SKAFFOLD_PREFIX_NAME)/local
tear-down-kind2-env tear-down-kind2-ha-single-zone-env: $(KUBECTL)
	$(KUBECTL) delete -k $(REPO_ROOT)/example/provider-local/seed-$(SKAFFOLD_PREFIX_NAME)/local

operator-%: export SKAFFOLD_FILENAME = skaffold-operator.yaml

operator-up: $(SKAFFOLD) $(HELM) $(KUBECTL)
	$(SKAFFOLD) run
operator-dev: $(SKAFFOLD) $(HELM) $(KUBECTL)
	$(SKAFFOLD) dev
operator-debug: $(SKAFFOLD) $(HELM) $(KUBECTL)
	$(SKAFFOLD) debug
operator-down: $(SKAFFOLD) $(HELM) $(KUBECTL)
	$(KUBECTL) annotate garden --all confirmation.gardener.cloud/deletion=true
	$(KUBECTL) delete garden --all --ignore-not-found --wait --timeout 5m
	$(SKAFFOLD) delete

test-e2e-local: $(GINKGO)
	./hack/test-e2e-local.sh --procs=$(PARALLEL_E2E_TESTS) --label-filter="default" ./test/e2e/gardener/...
test-e2e-local-workerless: $(GINKGO)
	./hack/test-e2e-local.sh --procs=$(PARALLEL_E2E_TESTS) --label-filter="default && workerless" ./test/e2e/gardener/...
test-e2e-local-simple: $(GINKGO)
	./hack/test-e2e-local.sh --procs=$(PARALLEL_E2E_TESTS) --label-filter "Shoot && simple" ./test/e2e/gardener/...
test-e2e-local-migration: $(GINKGO)
	./hack/test-e2e-local.sh --procs=$(PARALLEL_E2E_TESTS) --label-filter "Shoot && control-plane-migration" ./test/e2e/gardener/...
test-e2e-local-migration-ha-single-zone: $(GINKGO)
	SHOOT_FAILURE_TOLERANCE_TYPE=node ./hack/test-e2e-local.sh --procs=$(PARALLEL_E2E_TESTS) --label-filter "Shoot && control-plane-migration" ./test/e2e/gardener/...
test-e2e-local-ha-single-zone: $(GINKGO)
	SHOOT_FAILURE_TOLERANCE_TYPE=node ./hack/test-e2e-local.sh --procs=$(PARALLEL_E2E_TESTS) --label-filter "simple || (high-availability && upgrade-to-node)" ./test/e2e/gardener/...
test-e2e-local-ha-multi-zone: $(GINKGO)
	SHOOT_FAILURE_TOLERANCE_TYPE=zone ./hack/test-e2e-local.sh --procs=$(PARALLEL_E2E_TESTS) --label-filter "simple || (high-availability && upgrade-to-zone)" ./test/e2e/gardener/...
test-e2e-local-operator: $(GINKGO)
	./hack/test-e2e-local.sh operator --procs=1 --label-filter="default" ./test/e2e/operator/...

test-non-ha-pre-upgrade: $(GINKGO)
	./hack/test-e2e-local.sh --procs=$(PARALLEL_E2E_TESTS) --label-filter="pre-upgrade && !high-availability" ./test/e2e/gardener/...
test-pre-upgrade: $(GINKGO)
	./hack/test-e2e-local.sh --procs=$(PARALLEL_E2E_TESTS) --label-filter="pre-upgrade" ./test/e2e/gardener/...

test-non-ha-post-upgrade: $(GINKGO)
	./hack/test-e2e-local.sh --procs=$(PARALLEL_E2E_TESTS) --label-filter="post-upgrade && !high-availability" ./test/e2e/gardener/...
test-post-upgrade: $(GINKGO)
	./hack/test-e2e-local.sh --procs=$(PARALLEL_E2E_TESTS) --label-filter="post-upgrade" ./test/e2e/gardener/...

ci-e2e-kind: $(KIND) $(YQ)
	./hack/ci-e2e-kind.sh
ci-e2e-kind-migration: $(KIND) $(YQ)
	GARDENER_LOCAL_KUBECONFIG=$(GARDENER_LOCAL_KUBECONFIG) GARDENER_LOCAL2_KUBECONFIG=$(GARDENER_LOCAL2_KUBECONFIG) ./hack/ci-e2e-kind-migration.sh
ci-e2e-kind-migration-ha-single-zone: $(KIND) $(YQ)
	GARDENER_LOCAL_KUBECONFIG=$(GARDENER_LOCAL_HA_SINGLE_ZONE_KUBECONFIG) GARDENER_LOCAL2_KUBECONFIG=$(GARDENER_LOCAL2_HA_SINGLE_ZONE_KUBECONFIG) SHOOT_FAILURE_TOLERANCE_TYPE=node ./hack/ci-e2e-kind-migration-ha-single-zone.sh
ci-e2e-kind-ha-single-zone: $(KIND) $(YQ)
	./hack/ci-e2e-kind-ha-single-zone.sh
ci-e2e-kind-ha-multi-zone: $(KIND) $(YQ)
	./hack/ci-e2e-kind-ha-multi-zone.sh
ci-e2e-kind-operator: $(KIND) $(YQ)
	./hack/ci-e2e-kind-operator.sh

ci-e2e-kind-upgrade: $(KIND) $(YQ)
	SHOOT_FAILURE_TOLERANCE_TYPE="" GARDENER_PREVIOUS_RELEASE=$(GARDENER_PREVIOUS_RELEASE) GARDENER_RELEASE_DOWNLOAD_PATH=$(GARDENER_RELEASE_DOWNLOAD_PATH) GARDENER_NEXT_RELEASE=$(GARDENER_NEXT_RELEASE) ./hack/ci-e2e-kind-upgrade.sh
ci-e2e-kind-ha-single-zone-upgrade: $(KIND) $(YQ)
	SHOOT_FAILURE_TOLERANCE_TYPE="node" GARDENER_PREVIOUS_RELEASE=$(GARDENER_PREVIOUS_RELEASE) GARDENER_RELEASE_DOWNLOAD_PATH=$(GARDENER_RELEASE_DOWNLOAD_PATH) GARDENER_NEXT_RELEASE=$(GARDENER_NEXT_RELEASE) ./hack/ci-e2e-kind-upgrade.sh
ci-e2e-kind-ha-multi-zone-upgrade: $(KIND) $(YQ)
	SHOOT_FAILURE_TOLERANCE_TYPE="zone" GARDENER_PREVIOUS_RELEASE=$(GARDENER_PREVIOUS_RELEASE) GARDENER_RELEASE_DOWNLOAD_PATH=$(GARDENER_RELEASE_DOWNLOAD_PATH) GARDENER_NEXT_RELEASE=$(GARDENER_NEXT_RELEASE) ./hack/ci-e2e-kind-upgrade.sh
