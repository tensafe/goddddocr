.PHONY: test server ort doctor smoke docker-smoke bench-workers eval-samples prep-sample python-prep-reference python-feature-reference prep-compare

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

python-prep-reference:
	python3 scripts/python_preprocess_reference.py -image samples/yzm1.png -out /tmp/python-preprocess.png -json /tmp/python-preprocess.json -matrix-csv /tmp/python-preprocess.csv

python-feature-reference:
	python3 scripts/python_feature_reference.py -mode det -image samples/yzm2.jpeg -out /tmp/python-detection-reference.json

prep-compare:
	scripts/preprocess_compare.sh samples/yzm1.png
