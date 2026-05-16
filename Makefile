.PHONY: test server ort doctor smoke docker-smoke bench-workers

test:
	go test ./...

server:
	go run ./cmd/goddddocr-server -addr :8088

ort:
	go run ./cmd/ortfetch

doctor:
	go run ./cmd/ocrdoctor -image samples/yzm1.png -expect 3n3d

smoke:
	scripts/smoke.sh

docker-smoke:
	scripts/docker_smoke.sh

bench-workers:
	scripts/bench_workers.sh
