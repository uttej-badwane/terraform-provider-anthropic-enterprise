PROVIDER_NAME := anthropic-enterprise
NAMESPACE := uttej-badwane
GOBIN ?= $(shell go env GOPATH)/bin
DEV_TFRC := $(CURDIR)/dev.tfrc

# Pinned tool versions. CI uses the same ones (.github/workflows/test.yml), so
# `make lint` and `make vulncheck` here match what a pull request is held to.
GOLANGCI_LINT_VERSION := v2.13.2
GOVULNCHECK_VERSION := v1.8.0

default: fmt lint install generate

# Install the linters at the versions CI runs.
tools:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	go install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)

# Report vulnerabilities in dependencies that this code can actually reach.
vulncheck:
	go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

# `-diff` reports what `go mod tidy` would change and exits non-zero without
# touching go.mod/go.sum, so it is safe to run on a dirty worktree and needs
# no `git diff --exit-code` follow-up (Go 1.23+).
tidy-check:
	@go mod tidy -diff || \
		(echo; echo "go.mod/go.sum are not tidy. Run 'go mod tidy' and commit the result."; exit 1)

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

.PHONY: default tools vulncheck tidy-check build install lint generate fmt test testacc testacc-live testacc-live-write dev-override publish-check docs-validate snapshot
