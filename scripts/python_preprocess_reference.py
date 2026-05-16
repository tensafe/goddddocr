#!/usr/bin/env python3
"""Export Python/Pillow OCR preprocessing references for Go parity checks."""

from __future__ import annotations

import argparse
import csv
import hashlib
import json
import sys
from pathlib import Path
from typing import Any


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Export a Python/Pillow preprocessing reference compatible with "
            "cmd/ocrprep -compare-csv and -compare-png."
        )
    )
    parser.add_argument("-image", required=True, type=Path, help="image file to preprocess")
    parser.add_argument("-out", type=Path, help="optional grayscale PNG output path")
    parser.add_argument("-matrix-csv", type=Path, help="optional grayscale pixel matrix CSV output path")
    parser.add_argument("-json", type=Path, help="optional JSON report output path")
    parser.add_argument("-target-height", type=int, default=64, help="OCR target height; default: 64")
    parser.add_argument("-png-fix", action="store_true", help="composite RGBA PNGs over white before resizing")
    return parser.parse_args(argv)


def main(argv: list[str]) -> int:
    args = parse_args(argv)
    try:
        report = export_reference(args)
    except Exception as exc:
        print(f"python_preprocess_reference: {exc}", file=sys.stderr)
        return 1

    encoded = json.dumps(report, ensure_ascii=False, indent=2)
    if args.json:
        args.json.parent.mkdir(parents=True, exist_ok=True)
        args.json.write_text(encoded + "\n", encoding="utf-8")
    print(encoded)
    return 0


def export_reference(args: argparse.Namespace) -> dict[str, Any]:
    try:
        from PIL import Image
    except ModuleNotFoundError as exc:
        raise RuntimeError("Pillow is required: python3 -m pip install pillow") from exc

    if args.target_height <= 0:
        raise ValueError("target-height must be positive")

    opened = Image.open(args.image)
    image = opened
    try:
        original_mode = image.mode
        original_width, original_height = image.size
        if original_width <= 0 or original_height <= 0:
            raise ValueError("empty image")

        if args.png_fix and image.mode == "RGBA":
            background = Image.new("RGB", image.size, (255, 255, 255))
            background.paste(image, (0, 0), mask=image)
            image = background

        target_width = int(image.size[0] * (args.target_height / image.size[1]))
        if target_width <= 0:
            raise ValueError(
                f"target width {target_width} is invalid for {image.size[0]}x{image.size[1]}"
            )

        gray = image.resize((target_width, args.target_height), resample_filter()).convert("L")
        pixels = gray.tobytes()

        if args.out:
            args.out.parent.mkdir(parents=True, exist_ok=True)
            gray.save(args.out)
        if args.matrix_csv:
            args.matrix_csv.parent.mkdir(parents=True, exist_ok=True)
            write_matrix_csv(args.matrix_csv, pixels, gray.width, gray.height)

        return summarize(args, pixels, gray.width, gray.height, original_mode, original_width, original_height)
    finally:
        if image is not opened:
            image.close()
        opened.close()


def resample_filter() -> int:
    from PIL import Image

    if hasattr(Image, "Resampling"):
        return Image.Resampling.LANCZOS
    return Image.LANCZOS


def write_matrix_csv(path: Path, pixels: bytes, width: int, height: int) -> None:
    with path.open("w", newline="", encoding="utf-8") as file:
        writer = csv.writer(file)
        for y in range(height):
            start = y * width
            writer.writerow(pixels[start : start + width])


def summarize(
    args: argparse.Namespace,
    pixels: bytes,
    width: int,
    height: int,
    original_mode: str,
    original_width: int,
    original_height: int,
) -> dict[str, Any]:
    values = list(pixels)
    mean = round(sum(values) / len(values), 3) if values else 0
    report: dict[str, Any] = {
        "image_path": str(args.image),
        "output_path": str(args.out) if args.out else "",
        "matrix_csv_path": str(args.matrix_csv) if args.matrix_csv else "",
        "width": width,
        "height": height,
        "pixels": len(values),
        "min": min(values) if values else 0,
        "max": max(values) if values else 0,
        "mean": mean,
        "sha256": hashlib.sha256(pixels).hexdigest(),
        "png_fix": bool(args.png_fix),
        "target_height": args.target_height,
        "source": "python_pillow",
        "original_mode": original_mode,
        "original_width": original_width,
        "original_height": original_height,
    }
    return {key: value for key, value in report.items() if value != ""}


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
