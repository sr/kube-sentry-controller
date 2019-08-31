.PHONY: install test lint apply generate

GO ?= go

all: install test lint generate

install:
	$(GO) install ./

test:
	$(GO) test ./...

lint:
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint
	golangci-lint run ./...

# Install CRDs into a cluster
apply: generate
	kubectl apply -f config/crds

# Generate code and manifests  (e.g. CRD, RBAC, etc)
generate:
	$(GO) install sigs.k8s.io/controller-tools/cmd/controller-gen
	controller-gen object crd paths=./pkg/apis/... output:crd:dir=config/crds
