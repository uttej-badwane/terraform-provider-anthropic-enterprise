PROVIDER_NAME := anthropic-enterprise
NAMESPACE := uttej-badwane
GOBIN ?= $(shell go env GOPATH)/bin
DEV_TFRC := $(CURDIR)/dev.tfrc

default: fmt lint install generate

build:
	go build -v ./...

install: build
	go install -v ./...

lint:
	golangci-lint run

generate:
	cd tools; go generate ./...
	./scripts/set-subcategories.sh

fmt:
	gofmt -s -w -e .
	terraform fmt -recursive examples/

test:
	go test -v -cover -timeout=120s -parallel=10 ./...

# Acceptance tests against the in-process mock server (default).
testacc:
	TF_ACC=1 go test -v -cover -timeout 30m ./internal/provider/

# Acceptance tests against a live organization. Read-only plus tf-acc- prefixed
# workspace / RBAC group lifecycle only. Requires ANTHROPIC_ADMIN_API_KEY and
# optionally ANTHROPIC_ENTERPRISE_API_KEY / ANTHROPIC_AUTH_TOKEN in the environment.
testacc-live:
	ANTHROPIC_ACC_LIVE=1 TF_ACC=1 go test -v -timeout 30m -run "TestAccLive" ./internal/provider/

# Write-tier live tests for a DEVELOPMENT organization only: invites, members,
# service accounts and federation objects are created and destroyed. Also set
# ANTHROPIC_ACC_INVITE_EMAIL and ANTHROPIC_ACC_MEMBER_USER_ID as needed.
testacc-live-write:
	ANTHROPIC_ACC_LIVE=1 ANTHROPIC_ACC_LIVE_WRITE=1 TF_ACC=1 go test -v -timeout 30m -run "TestAccLive" ./internal/provider/

# Write dev.tfrc pointing dev_overrides at the installed binary.
dev-override:
	@printf 'provider_installation {\n  dev_overrides {\n    "%s/%s" = "%s"\n  }\n  direct {}\n}\n' $(NAMESPACE) $(PROVIDER_NAME) $(GOBIN) > $(DEV_TFRC)
	@echo "export TF_CLI_CONFIG_FILE=$(DEV_TFRC)"

# Fail if any string from an external denylist appears in the tree.
publish-check:
	./scripts/publish-check.sh

docs-validate:
	cd tools; go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs validate --provider-dir .. -provider-name anthropic

snapshot:
	goreleaser release --snapshot --clean --skip=sign

.PHONY: default build install lint generate fmt test testacc testacc-live testacc-live-write dev-override publish-check docs-validate snapshot
