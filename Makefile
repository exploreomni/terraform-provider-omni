default: build

.PHONY: build install test testacc sweep fmt lint docs

build:
	go build -v ./...

install:
	go install -v ./...

test:
	go test -v -cover ./internal/...

# Acceptance tests. These create, update and destroy real objects, so point
# OMNI_BASE_URL at a throwaway instance. -parallel 1 because they share one
# instance and the API allows 60 requests a minute.
testacc:
	TF_ACC=1 go test -v -parallel 1 -timeout 120m ./internal/provider/

# Delete objects left behind by a failed acceptance run. Only ones named
# tfacc-*, so this is safe to run against an instance that has other things on
# it - though it should still only ever be the test instance.
sweep:
	go test -v -sweep=all -timeout 20m ./internal/provider/

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
