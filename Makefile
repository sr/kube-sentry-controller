.PHONY: install test apply generate tools

export GO111MODULE = on

install:
	go install -v ./

test:
	go test -v ./...

lint:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint
	golangci-lint run -v ./...

# Install CRDs into a cluster
apply: generate
	kubectl apply -f config/crds

# Generate code and manifests  (e.g. CRD, RBAC, etc)
generate: tools
	go generate ./pkg/...
	controller-gen all

tools:
	go install -v k8s.io/code-generator/cmd/client-gen \
		k8s.io/code-generator/cmd/deepcopy-gen \
		sigs.k8s.io/controller-tools/cmd/controller-gen
