default: build

.PHONY: build install test fmt lint docs

build:
	go build -v ./...

install:
	go install -v ./...

test:
	go test -v -cover ./internal/...

testacc:
	TF_ACC=1 go test -v -timeout 120m ./internal/...

fmt:
	gofmt -w .
	terraform fmt -recursive ./examples/

docs:
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate

# Point the module path at your own GitHub org before publishing, e.g.
#   make rename OWNER=my-org
rename:
	@test -n "$(OWNER)" || (echo "usage: make rename OWNER=<github-org>" && exit 1)
	grep -rl 'ashima-omni/terraform-provider-omni' --include='*.go' --include='go.mod' --include='*.md' . \
		| xargs sed -i.bak 's|ashima-omni/terraform-provider-omni|$(OWNER)/terraform-provider-omni|g'
	sed -i.bak 's|registry.terraform.io/ashima-omni/omni|registry.terraform.io/$(OWNER)/omni|g' main.go
	find . -name '*.bak' -delete
