.PHONY: test server ort doctor smoke docker-smoke bench-workers eval-samples prep-sample

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

eval-samples:
	go run ./cmd/ocreval -manifest fixtures/ocr_golden.json

prep-sample:
	go run ./cmd/ocrprep -image samples/yzm1.png -out /tmp/goddddocr-preprocess.png -json /tmp/goddddocr-preprocess.json -matrix-csv /tmp/goddddocr-preprocess.csv
