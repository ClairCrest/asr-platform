MIGRATIONS_DIR   := api/internal/store/migrations
DATABASE_URL     ?= postgres://asr:asr@localhost:5432/asr?sslmode=disable
KIND_CLUSTER     := asr-platform
K8S_MIGRATIONS   := deploy/k8s/base/migrations
INGRESS_NGINX_URL := https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.13.0/deploy/static/provider/kind/deploy.yaml
KEDA_URL         := https://github.com/kedacore/keda/releases/download/v2.17.0/keda-2.17.0.yaml

.PHONY: up down logs migrate-up migrate-down sqlc test lint \
	kind-up kind-down k8s-migrations k8s-build grafana prometheus load-test

up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f

migrate-up:
	migrate -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" down 1

sqlc:
	cd api && sqlc generate

test:
	cd api && go test -race -cover ./...
	cd worker && uv run pytest
	cd web && npm test --if-present

lint:
	cd api && golangci-lint run
	cd worker && uv run ruff check .
	cd web && npm run lint

# Stages a CRLF-stripped copy of the migration files under
# deploy/k8s/base/, since kustomize's configMapGenerator refuses to read
# sources from outside its own root. api/internal/store/migrations stays
# the one source of truth; this directory is gitignored and rebuilt here
# every time, never edited by hand.
k8s-migrations:
	mkdir -p $(K8S_MIGRATIONS)
	cp $(MIGRATIONS_DIR)/*.sql $(K8S_MIGRATIONS)/
	sed -i 's/\r$$//' $(K8S_MIGRATIONS)/*.sql

# Builds all three images with the tags the local kustomize overlay
# expects (imagePullPolicy: Never — see overlays/local) and loads them
# directly into the kind cluster's node, bypassing any registry.
k8s-build:
	docker build -t asr-api:local -f api/Dockerfile api/
	docker build -t asr-worker:local -f worker/Dockerfile worker/
	docker build -t asr-web:local -f web/Dockerfile web/
	kind load docker-image asr-api:local asr-worker:local asr-web:local --name $(KIND_CLUSTER)

# Cold start to a working platform at http://localhost:8080. Safe to
# re-run: creating an existing cluster/ingress-nginx install is a no-op,
# and `kubectl apply -k` is idempotent.
kind-up:
	kind get clusters 2>/dev/null | grep -qx $(KIND_CLUSTER) || \
		kind create cluster --name $(KIND_CLUSTER) --config deploy/kind-cluster.yaml
	kubectl apply -f $(INGRESS_NGINX_URL)
	kubectl apply --server-side -f $(KEDA_URL)
	kubectl wait --namespace ingress-nginx \
		--for=condition=ready pod \
		--selector=app.kubernetes.io/component=controller \
		--timeout=120s
	kubectl wait --namespace keda \
		--for=condition=available deployment/keda-operator \
		--timeout=120s
	$(MAKE) k8s-build
	$(MAKE) k8s-migrations
	kubectl apply -k deploy/k8s/overlays/local
	kubectl -n asr-platform rollout status statefulset/postgres --timeout=120s
	kubectl -n asr-platform rollout status statefulset/minio --timeout=120s
	kubectl -n asr-platform rollout status deployment/redis --timeout=60s
	kubectl -n asr-platform rollout status deployment/api --timeout=180s
	kubectl -n asr-platform rollout status deployment/worker --timeout=300s
	kubectl -n asr-platform rollout status deployment/web --timeout=60s
	kubectl -n asr-platform rollout status deployment/prometheus --timeout=60s
	kubectl -n asr-platform rollout status deployment/grafana --timeout=60s
	@echo ""
	@echo "Ready at http://localhost:8080"
	@echo "make grafana / make prometheus to reach the observability stack"

kind-down:
	kind delete cluster --name $(KIND_CLUSTER)

grafana:
	@echo "http://localhost:3000 (admin/admin, or anonymous viewer access)"
	kubectl -n asr-platform port-forward svc/grafana 3000:3000

prometheus:
	@echo "http://localhost:9090"
	kubectl -n asr-platform port-forward svc/prometheus 9090:9090

load-test:
	k6 run deploy/load-test/k6/load-test.js
