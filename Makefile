# Log a message from the abc subsystem
.PHONY: log-abc
log-abc:
	@echo "This is a log message from the log-abc target.";

# Log a message from the xyz subsystem
.PHONY: log-xyz
log-xyz:
	@echo "This is a log message from the log-xyz target.";

# Run all log targets
.PHONY: log-all
log-all: log-abc log-xyz
	@echo "All logs complete.";

# Compile the mkm binary
.PHONY: build
build:
	go build -o mkm;

# Build mkm directly to its install path. The `rm -f` is load-bearing on
# macOS: the kernel caches Gatekeeper verdicts keyed on inode, so overwriting
# the bytes in place can leave a prior "rejected" verdict cached and the new
# binary gets SIGKILLed on launch. Removing the old file first forces a new
# inode and a fresh assessment.
.PHONY: install
install:
	rm -f ~/go/bin/mkm
	go build -o ~/go/bin/mkm

# Bump patch version and release (0.2.1 -> 0.2.2)
.PHONY: release-patch
release-patch:
	@v=$$(git tag -l 'v*' --sort=-v:refname | head -1 | sed 's/v//'); \
	major=$$(echo $$v | cut -d. -f1); \
	minor=$$(echo $$v | cut -d. -f2); \
	patch=$$(echo $$v | cut -d. -f3); \
	next="$$major.$$minor.$$((patch + 1))"; \
	echo "v$$v -> v$$next"; \
	git tag "v$$next" && git push origin "v$$next" && echo "Released v$$next"

# Bump minor version and release (0.2.1 -> 0.3.0)
.PHONY: release-minor
release-minor:
	@v=$$(git tag -l 'v*' --sort=-v:refname | head -1 | sed 's/v//'); \
	major=$$(echo $$v | cut -d. -f1); \
	minor=$$(echo $$v | cut -d. -f2); \
	next="$$major.$$((minor + 1)).0"; \
	echo "v$$v -> v$$next"; \
	git tag "v$$next" && git push origin "v$$next" && echo "Released v$$next"

# Bump major version and release (0.2.1 -> 1.0.0)
.PHONY: release-major
release-major:
	@v=$$(git tag -l 'v*' --sort=-v:refname | head -1 | sed 's/v//'); \
	major=$$(echo $$v | cut -d. -f1); \
	next="$$((major + 1)).0.0"; \
	echo "v$$v -> v$$next"; \
	git tag "v$$next" && git push origin "v$$next" && echo "Released v$$next"

# Package the binary into a tarball for distribution
.PHONY: tar
tar: build
	tar -czvf mkm.tar.gz mkm;

# This is a deliberately long description intended to stress-test how the preview pane wraps and clips text when a user hovers over a target whose metadata would otherwise spill beyond the visible width of the right-hand panel; if rendering is correct, it should word-wrap inside the preview and never bleed into the list on the left or past the panel border.
.PHONY: this-is-an-absurdly-long-target-name-intended-to-stress-test-the-left-pane-truncation-logic
this-is-an-absurdly-long-target-name-intended-to-stress-test-the-left-pane-truncation-logic: build log-abc log-xyz log-all install tar release-patch release-minor release-major this-is-another-moderately-long-dependency-name another-dep-name-for-good-measure yet-another-dep one-more-for-the-road final-dep
	@echo "If you can read this entire line without any line-wrap or visible truncation marker then the preview pane recipe rendering is definitely not clipping correctly and something is very wrong with the layout math inside renderPreview;"
	@echo "short line";
	@echo "Here is another absurdly long recipe line that contains no spaces for the first large chunk: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa end";

# This description contains supercalifragilisticexpialidociousunbreakablewordthatislongerthananyreasonablepanelwidthtoforcethewordwrappathwherea single word alone exceeds the wrap width.
.PHONY: unbreakable-word-target
unbreakable-word-target:
	@echo "tests wordWrap behavior with a single unbreakable word in the description";

# A target whose deps list is long enough to require clipping on its own single line in the preview pane
.PHONY: many-deps
many-deps: dep-alpha dep-bravo dep-charlie dep-delta dep-echo dep-foxtrot dep-golf dep-hotel dep-india dep-juliet dep-kilo dep-lima dep-mike dep-november dep-oscar dep-papa dep-quebec dep-romeo dep-sierra dep-tango dep-uniform dep-victor dep-whiskey dep-xray dep-yankee dep-zulu
	@echo "done"

.PHONY: docs-inline
docs-inline: ## Start the docs server on :8080
	@echo "docs"

.PHONY: test-unit
test-unit: build ## Run unit tests with race detector enabled
	@echo "tests"

# Leading comment that should be shadowed by the inline ## description
.PHONY: inline-wins
inline-wins: ## Inline description wins over leading comment
	@echo "inline wins"

# --- bulk fixtures for scrolling ---

.PHONY: db-migrate db-rollback db-seed db-reset db-dump db-restore db-shell db-status db-create db-drop
db-migrate: ## Apply all pending migrations
	@echo "migrate"
db-rollback: ## Roll back the most recent migration
	@echo "rollback"
db-seed: db-migrate ## Seed the dev database with sample data
	@echo "seed"
db-reset: db-drop db-create db-migrate db-seed ## Full reset: drop, create, migrate, seed
	@echo "reset"
db-dump: ## Dump the current DB to a timestamped SQL file
	@echo "dump"
db-restore: ## Restore from the latest dump file
	@echo "restore"
db-shell: ## Open a psql shell against the dev DB
	@echo "psql"
db-status: ## Show migration status
	@echo "status"
db-create: ## Create the dev database
	@echo "createdb"
db-drop: ## Drop the dev database (destructive!)
	@echo "dropdb"

.PHONY: docker-build docker-push docker-run docker-clean docker-logs docker-shell docker-ps docker-stop
docker-build: ## Build all container images
	@echo "docker build"
docker-push: docker-build ## Push images to the registry
	@echo "push"
docker-run: ## Run the main container locally
	@echo "run"
docker-clean: ## Remove dangling images and stopped containers
	@echo "clean"
docker-logs: ## Tail logs from the running container
	@echo "logs"
docker-shell: ## Exec a shell inside the running container
	@echo "shell"
docker-ps: ## List running containers
	@echo "ps"
docker-stop: ## Stop all project containers
	@echo "stop"

.PHONY: ci-lint ci-test ci-fmt ci-vet ci-all ci-coverage ci-bench ci-race
ci-lint: ## Run linters (golangci-lint, vet)
	@echo "lint"
ci-test: ## Run the full test suite
	@echo "test"
ci-fmt: ## Check formatting (gofmt, goimports)
	@echo "fmt"
ci-vet: ## Run go vet
	@echo "vet"
ci-all: ci-lint ci-fmt ci-vet ci-test ## Run everything CI runs
	@echo "all"
ci-coverage: ## Generate and open a coverage report
	@echo "coverage"
ci-bench: ## Run benchmarks
	@echo "bench"
ci-race: ## Run tests with the race detector
	@echo "race"

.PHONY: fmt-go fmt-proto fmt-yaml fmt-md fmt-all
fmt-go: ## Format all Go source files
	@echo "gofmt"
fmt-proto: ## Format .proto files with buf
	@echo "buf fmt"
fmt-yaml: ## Format YAML files with yq
	@echo "yamlfmt"
fmt-md: ## Format Markdown with prettier
	@echo "mdfmt"
fmt-all: fmt-go fmt-proto fmt-yaml fmt-md ## Format everything
	@echo "fmt all"

.PHONY: gen-proto gen-mocks gen-openapi gen-sqlc gen-all
gen-proto: ## Regenerate protobuf bindings
	@echo "buf generate"
gen-mocks: ## Regenerate mocks via mockery
	@echo "mockery"
gen-openapi: ## Regenerate OpenAPI clients
	@echo "oapi-codegen"
gen-sqlc: ## Regenerate sqlc query bindings
	@echo "sqlc generate"
gen-all: gen-proto gen-mocks gen-openapi gen-sqlc ## Regenerate all code
	@echo "gen all"

.PHONY: clean clean-build clean-cache clean-deps clean-all
clean: clean-build ## Remove built artifacts
	@echo "clean"
clean-build: ## Remove ./dist and ./bin
	@echo "rm build"
clean-cache: ## Clear Go build and test caches
	@echo "go clean -cache -testcache"
clean-deps: ## Remove node_modules and vendored deps
	@echo "rm deps"
clean-all: clean-build clean-cache clean-deps ## Nuclear clean
	@echo "clean all"

.PHONY: deps-tidy deps-upgrade deps-audit deps-graph
deps-tidy: ## go mod tidy
	@echo "tidy"
deps-upgrade: ## Bump all direct deps to latest
	@echo "upgrade"
deps-audit: ## Scan dependencies for known CVEs
	@echo "govulncheck"
deps-graph: ## Render a dependency graph to deps.svg
	@echo "graph"

.PHONY: k8s-apply k8s-diff k8s-delete k8s-logs k8s-exec k8s-port-forward k8s-rollout k8s-scale k8s-describe k8s-get-pods k8s-get-svc k8s-get-ingress k8s-top k8s-events k8s-restart k8s-drain k8s-cordon k8s-uncordon
k8s-apply: ## Apply all manifests in ./k8s
	@echo "kubectl apply"
k8s-diff: ## Diff local manifests against cluster state
	@echo "kubectl diff"
k8s-delete: ## Delete all project resources
	@echo "kubectl delete"
k8s-logs: ## Tail logs for the current app pod
	@echo "kubectl logs"
k8s-exec: ## Exec a shell in the current app pod
	@echo "kubectl exec"
k8s-port-forward: ## Forward local ports to the app service
	@echo "kubectl port-forward"
k8s-rollout: ## Trigger a rolling restart of the app deployment
	@echo "kubectl rollout restart"
k8s-scale: ## Scale the app deployment to N replicas
	@echo "kubectl scale"
k8s-describe: ## Describe the app deployment
	@echo "kubectl describe"
k8s-get-pods: ## List all pods in the namespace
	@echo "kubectl get pods"
k8s-get-svc: ## List all services in the namespace
	@echo "kubectl get svc"
k8s-get-ingress: ## List all ingresses in the namespace
	@echo "kubectl get ingress"
k8s-top: ## Show pod resource usage
	@echo "kubectl top"
k8s-events: ## Show recent cluster events
	@echo "kubectl get events"
k8s-restart: ## Restart all pods for the app
	@echo "kubectl rollout restart"
k8s-drain: ## Drain a node for maintenance
	@echo "kubectl drain"
k8s-cordon: ## Cordon a node to prevent new pods
	@echo "kubectl cordon"
k8s-uncordon: ## Uncordon a node
	@echo "kubectl uncordon"

.PHONY: aws-login aws-whoami aws-s3-ls aws-s3-sync aws-s3-cp aws-ec2-ls aws-ec2-ssh aws-logs aws-metrics aws-iam-whoami aws-sts aws-ssm aws-secrets aws-rds-snapshot aws-rds-restore aws-cdn-invalidate
aws-login: ## Refresh AWS SSO credentials
	@echo "aws sso login"
aws-whoami: ## Show the current AWS identity
	@echo "aws sts get-caller-identity"
aws-s3-ls: ## List the project S3 bucket
	@echo "aws s3 ls"
aws-s3-sync: ## Sync local ./public with the CDN bucket
	@echo "aws s3 sync"
aws-s3-cp: ## Copy a file to the project bucket
	@echo "aws s3 cp"
aws-ec2-ls: ## List project EC2 instances
	@echo "aws ec2 describe-instances"
aws-ec2-ssh: ## SSH into the primary EC2 instance
	@echo "aws ssm start-session"
aws-logs: ## Tail CloudWatch logs for the main service
	@echo "aws logs tail"
aws-metrics: ## Open the CloudWatch metrics dashboard
	@echo "open metrics"
aws-iam-whoami: ## Show the effective IAM identity
	@echo "aws iam get-user"
aws-sts: ## Print current STS credentials
	@echo "aws sts"
aws-ssm: ## Start an SSM session on the primary host
	@echo "ssm"
aws-secrets: ## List secrets in AWS Secrets Manager
	@echo "secrets"
aws-rds-snapshot: ## Take an RDS snapshot
	@echo "snapshot"
aws-rds-restore: ## Restore RDS from the latest snapshot
	@echo "restore"
aws-cdn-invalidate: ## Invalidate the CDN cache
	@echo "invalidate"

.PHONY: api-run api-test api-lint api-docs api-schema api-mock api-bench api-perf api-smoke api-e2e
api-run: ## Start the API server in dev mode
	@echo "api run"
api-test: ## Run API unit tests
	@echo "api test"
api-lint: ## Lint the API package
	@echo "api lint"
api-docs: ## Serve API docs on :8080
	@echo "api docs"
api-schema: ## Regenerate the OpenAPI schema
	@echo "api schema"
api-mock: ## Run a mock API server for frontend dev
	@echo "api mock"
api-bench: ## Run API benchmarks
	@echo "api bench"
api-perf: ## Run a perf suite against the API
	@echo "api perf"
api-smoke: ## Smoke test the deployed API
	@echo "api smoke"
api-e2e: api-run ## Run end-to-end API tests
	@echo "api e2e"

.PHONY: web-dev web-build web-preview web-test web-lint web-typecheck web-format web-bundle-analyze web-lighthouse web-i18n-extract web-i18n-sync
web-dev: ## Start the frontend dev server
	@echo "web dev"
web-build: ## Build the production frontend bundle
	@echo "web build"
web-preview: web-build ## Serve the built bundle for preview
	@echo "web preview"
web-test: ## Run frontend unit tests
	@echo "web test"
web-lint: ## Lint the frontend code
	@echo "web lint"
web-typecheck: ## Run the TypeScript compiler in check mode
	@echo "web typecheck"
web-format: ## Format the frontend code with prettier
	@echo "web format"
web-bundle-analyze: web-build ## Open the bundle analyzer
	@echo "analyze"
web-lighthouse: web-preview ## Run Lighthouse against the preview server
	@echo "lighthouse"
web-i18n-extract: ## Extract translation strings
	@echo "extract"
web-i18n-sync: ## Sync translations with the remote service
	@echo "sync"

.PHONY: backup-create backup-list backup-restore backup-verify backup-prune backup-encrypt backup-decrypt backup-offsite
backup-create: ## Create a full backup of the dev env
	@echo "backup create"
backup-list: ## List available backups
	@echo "backup list"
backup-restore: ## Restore from a named backup
	@echo "backup restore"
backup-verify: ## Verify checksums of all backups
	@echo "backup verify"
backup-prune: ## Prune old backups per retention policy
	@echo "backup prune"
backup-encrypt: ## Encrypt an existing backup
	@echo "backup encrypt"
backup-decrypt: ## Decrypt a backup file
	@echo "backup decrypt"
backup-offsite: ## Ship backups to the offsite bucket
	@echo "backup offsite"

.PHONY: monitor-up monitor-down monitor-logs monitor-alerts monitor-dashboards monitor-status
monitor-up: ## Start the local monitoring stack (prom + grafana)
	@echo "monitor up"
monitor-down: ## Stop the local monitoring stack
	@echo "monitor down"
monitor-logs: ## Tail logs from the monitoring stack
	@echo "monitor logs"
monitor-alerts: ## List currently firing alerts
	@echo "monitor alerts"
monitor-dashboards: ## Open the Grafana dashboards folder
	@echo "monitor dashboards"
monitor-status: ## Show monitoring stack health
	@echo "monitor status"

.PHONY: cache-warm cache-purge cache-inspect cache-stats cache-keys cache-flushdb
cache-warm: ## Pre-populate the cache from DB
	@echo "cache warm"
cache-purge: ## Purge all cache entries
	@echo "cache purge"
cache-inspect: ## Inspect a specific cache key
	@echo "cache inspect"
cache-stats: ## Show cache hit/miss statistics
	@echo "cache stats"
cache-keys: ## List all cache keys (dev only)
	@echo "cache keys"
cache-flushdb: ## Flush the entire Redis DB (destructive!)
	@echo "cache flushdb"

.PHONY: queue-run queue-pause queue-resume queue-stats queue-retry queue-dlq queue-purge queue-list
queue-run: ## Start worker processes
	@echo "queue run"
queue-pause: ## Pause all queue workers
	@echo "queue pause"
queue-resume: ## Resume paused queue workers
	@echo "queue resume"
queue-stats: ## Show queue depth and throughput
	@echo "queue stats"
queue-retry: ## Retry all failed jobs
	@echo "queue retry"
queue-dlq: ## Show dead-letter queue contents
	@echo "queue dlq"
queue-purge: ## Purge the entire queue
	@echo "queue purge"
queue-list: ## List active queues
	@echo "queue list"

.PHONY: auth-keygen auth-rotate auth-revoke auth-list-sessions auth-force-logout auth-token-verify auth-oauth-refresh auth-permissions
auth-keygen: ## Generate a new JWT signing key
	@echo "auth keygen"
auth-rotate: ## Rotate JWT signing keys
	@echo "auth rotate"
auth-revoke: ## Revoke a user's active sessions
	@echo "auth revoke"
auth-list-sessions: ## List active sessions for a user
	@echo "auth list"
auth-force-logout: ## Force logout all users (emergency)
	@echo "auth force logout"
auth-token-verify: ## Verify a JWT token
	@echo "auth verify"
auth-oauth-refresh: ## Refresh OAuth tokens
	@echo "auth refresh"
auth-permissions: ## Dump the current RBAC matrix
	@echo "auth perms"

.PHONY: user-create user-delete user-list user-impersonate user-export user-import user-lock user-unlock user-audit
user-create: ## Create a new user (admin CLI)
	@echo "user create"
user-delete: ## Soft-delete a user
	@echo "user delete"
user-list: ## List users
	@echo "user list"
user-impersonate: ## Impersonate a user for debugging
	@echo "user impersonate"
user-export: ## Export a user's data (GDPR)
	@echo "user export"
user-import: ## Bulk-import users from CSV
	@echo "user import"
user-lock: ## Lock a user account
	@echo "user lock"
user-unlock: ## Unlock a user account
	@echo "user unlock"
user-audit: ## Show audit log for a user
	@echo "user audit"

.PHONY: billing-invoice billing-charge billing-refund billing-reconcile billing-export billing-webhook-replay
billing-invoice: ## Generate the monthly invoice run
	@echo "billing invoice"
billing-charge: ## Charge a specific invoice
	@echo "billing charge"
billing-refund: ## Issue a refund
	@echo "billing refund"
billing-reconcile: ## Reconcile Stripe events with local state
	@echo "billing reconcile"
billing-export: ## Export billing data for accounting
	@echo "billing export"
billing-webhook-replay: ## Replay Stripe webhooks from the archive
	@echo "billing replay"

.PHONY: admin-shell admin-broadcast admin-feature-flag admin-maintenance-on admin-maintenance-off admin-backup-trigger
admin-shell: ## Open an admin REPL against prod
	@echo "admin shell"
admin-broadcast: ## Broadcast a message to all logged-in users
	@echo "admin broadcast"
admin-feature-flag: ## Toggle a feature flag
	@echo "admin flag"
admin-maintenance-on: ## Enable maintenance mode
	@echo "admin maint on"
admin-maintenance-off: ## Disable maintenance mode
	@echo "admin maint off"
admin-backup-trigger: ## Trigger an on-demand backup
	@echo "admin backup"

.PHONY: analytics-query analytics-export analytics-ingest-replay analytics-report-daily analytics-report-weekly analytics-report-monthly
analytics-query: ## Run an ad-hoc analytics query
	@echo "analytics query"
analytics-export: ## Export analytics data for BI tools
	@echo "analytics export"
analytics-ingest-replay: ## Replay ingested events from S3
	@echo "analytics replay"
analytics-report-daily: ## Generate the daily report
	@echo "analytics daily"
analytics-report-weekly: ## Generate the weekly report
	@echo "analytics weekly"
analytics-report-monthly: ## Generate the monthly report
	@echo "analytics monthly"

.PHONY: search-reindex search-rebuild search-optimize search-sync search-stats search-query-explain
search-reindex: ## Reindex changed documents
	@echo "search reindex"
search-rebuild: ## Rebuild the entire search index
	@echo "search rebuild"
search-optimize: ## Optimize the search index
	@echo "search optimize"
search-sync: ## Sync search data from the primary DB
	@echo "search sync"
search-stats: ## Show search index statistics
	@echo "search stats"
search-query-explain: ## Explain a search query plan
	@echo "search explain"

.PHONY: notify-test-email notify-test-sms notify-test-push notify-digest notify-dry-run
notify-test-email: ## Send a test email
	@echo "notify email"
notify-test-sms: ## Send a test SMS
	@echo "notify sms"
notify-test-push: ## Send a test push notification
	@echo "notify push"
notify-digest: ## Trigger the daily digest pipeline
	@echo "notify digest"
notify-dry-run: ## Render all notifications without sending
	@echo "notify dry"

.PHONY: perf-profile perf-trace perf-flamegraph perf-heap perf-goroutines perf-cpu
perf-profile: ## Capture a CPU profile from the running service
	@echo "perf profile"
perf-trace: ## Capture an execution trace
	@echo "perf trace"
perf-flamegraph: perf-profile ## Render a flamegraph from the latest profile
	@echo "perf flamegraph"
perf-heap: ## Capture a heap profile
	@echo "perf heap"
perf-goroutines: ## Dump all goroutine stacks
	@echo "perf goroutines"
perf-cpu: ## Show CPU usage by endpoint
	@echo "perf cpu"

.PHONY: secrets-list secrets-get secrets-set secrets-rotate secrets-audit
secrets-list: ## List secret names (not values)
	@echo "secrets list"
secrets-get: ## Fetch a secret value
	@echo "secrets get"
secrets-set: ## Set or update a secret value
	@echo "secrets set"
secrets-rotate: ## Rotate a specific secret
	@echo "secrets rotate"
secrets-audit: ## Show secret access audit log
	@echo "secrets audit"

.PHONY: feature-ls feature-enable feature-disable feature-status feature-export feature-import
feature-ls: ## List all feature flags
	@echo "feature ls"
feature-enable: ## Enable a feature flag
	@echo "feature enable"
feature-disable: ## Disable a feature flag
	@echo "feature disable"
feature-status: ## Show rollout status for a flag
	@echo "feature status"
feature-export: ## Export feature flag config
	@echo "feature export"
feature-import: ## Import feature flag config
	@echo "feature import"