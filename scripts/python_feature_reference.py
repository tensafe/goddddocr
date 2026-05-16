#!/usr/bin/env python3
"""Export Python ddddocr detection and slide references for Go parity checks."""

from __future__ import annotations

import argparse
import base64
import json
import sys
import time
from pathlib import Path
from typing import Any


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Run Python ddddocr for object detection or slide features and "
            "write a JSON reference that can be copied into Go golden fixtures."
        )
    )
    parser.add_argument(
        "-mode",
        required=True,
        choices=("det", "slide-comparison", "slide-match"),
        help="feature to run",
    )
    parser.add_argument("-image", type=Path, help="image path for -mode det")
    parser.add_argument("-target", type=Path, help="target image for slide modes")
    parser.add_argument("-background", type=Path, help="background image for slide modes")
    parser.add_argument("-simple-target", action="store_true", help="use simple slide template matching")
    parser.add_argument("-out", type=Path, help="optional JSON output path")
    parser.add_argument("-data-url", action="store_true", help="include base64 data URLs in the report")
    return parser.parse_args(argv)


def main(argv: list[str]) -> int:
    args = parse_args(argv)
    try:
        report = run_reference(args)
    except Exception as exc:
        print(f"python_feature_reference: {exc}", file=sys.stderr)
        return 1

    encoded = json.dumps(report, ensure_ascii=False, indent=2)
    if args.out:
        args.out.parent.mkdir(parents=True, exist_ok=True)
        args.out.write_text(encoded + "\n", encoding="utf-8")
    print(encoded)
    return 0


def run_reference(args: argparse.Namespace) -> dict[str, Any]:
    ddddocr = import_ddddocr()
    started = time.time()

    if args.mode == "det":
        if not args.image:
            raise ValueError("-image is required for -mode det")
        image_bytes = args.image.read_bytes()
        engine = ddddocr.DdddOcr(ocr=False, det=True, show_ad=False)
        result = engine.detection(image_bytes)
        return compact_report(
            {
                "mode": "det",
                "image": str(args.image),
                "result": result,
                "processing_time": time.time() - started,
                "source": "python_ddddocr",
            },
            args,
        )

    if not args.target or not args.background:
        raise ValueError("-target and -background are required for slide modes")
    target_bytes = args.target.read_bytes()
    background_bytes = args.background.read_bytes()
    engine = ddddocr.DdddOcr(ocr=False, det=False, show_ad=False)
    if args.mode == "slide-comparison":
        result = engine.slide_comparison(target_bytes, background_bytes)
    else:
        result = engine.slide_match(target_bytes, background_bytes, simple_target=args.simple_target)
    return compact_report(
        {
            "mode": args.mode,
            "target_image": str(args.target),
            "background_image": str(args.background),
            "simple_target": bool(args.simple_target),
            "result": result,
            "processing_time": time.time() - started,
            "source": "python_ddddocr",
        },
        args,
        target_bytes=target_bytes,
        background_bytes=background_bytes,
    )


def import_ddddocr() -> Any:
    try:
        import ddddocr  # type: ignore
    except ModuleNotFoundError as exc:
        raise RuntimeError(
            "ddddocr and its Python dependencies are required. "
            "Example: python3 -m pip install -e /path/to/ddddocr numpy Pillow opencv-python onnxruntime"
        ) from exc
    return ddddocr


def compact_report(
    report: dict[str, Any],
    args: argparse.Namespace,
    target_bytes: bytes | None = None,
    background_bytes: bytes | None = None,
) -> dict[str, Any]:
    if args.data_url:
        if args.mode == "det" and args.image:
            report["image_data_url"] = data_url(args.image)
        if target_bytes is not None and args.target:
            report["target_data_url"] = data_url(args.target, target_bytes)
        if background_bytes is not None and args.background:
            report["background_data_url"] = data_url(args.background, background_bytes)
    return {key: value for key, value in report.items() if value not in ("", None)}


def data_url(path: Path, data: bytes | None = None) -> str:
    if data is None:
        data = path.read_bytes()
    suffix = path.suffix.lower()
    mime = {
        ".jpg": "image/jpeg",
        ".jpeg": "image/jpeg",
        ".png": "image/png",
        ".gif": "image/gif",
        ".webp": "image/webp",
    }.get(suffix, "application/octet-stream")
    return f"data:{mime};base64,{base64.b64encode(data).decode('ascii')}"


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
