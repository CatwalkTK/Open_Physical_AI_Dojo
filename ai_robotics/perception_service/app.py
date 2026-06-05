#!/usr/bin/env python3
from __future__ import annotations

import json
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any


IMAGE_SIZE = {"width": 480, "height": 320}


class Handler(BaseHTTPRequestHandler):
    def do_GET(self) -> None:
        if self.path == "/health":
            self.respond({
                "status": "ok",
                "service": "perception",
                "model": "mock-workbench-detector",
                "updated_at": utc_now(),
            })
            return
        self.respond({"error": "not found"}, status=404)

    def do_POST(self) -> None:
        if self.path == "/perception":
            payload = self.read_json()
            self.respond(run_perception(payload))
            return
        self.respond({"error": "not found"}, status=404)

    def read_json(self) -> dict[str, Any]:
        length = int(self.headers.get("Content-Length", "0"))
        if length == 0:
            return {}
        raw = self.rfile.read(length)
        try:
            return json.loads(raw.decode("utf-8"))
        except json.JSONDecodeError:
            return {}

    def respond(self, payload: dict[str, Any], status: int = 200) -> None:
        body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, fmt: str, *args: Any) -> None:
        print("[%s] %s" % (utc_now(), fmt % args))


def run_perception(payload: dict[str, Any]) -> dict[str, Any]:
    source = str(payload.get("source") or "sample_workbench")
    instruction = str(payload.get("instruction") or "")
    lower_instruction = instruction.lower()

    objects = [
        {
            "label": "red_block",
            "display_name": "赤いブロック",
            "confidence": 0.94,
            "bbox": [94, 78, 205, 172],
            "position_hint": "front_left",
        },
        {
            "label": "blue_marker",
            "display_name": "青い目印",
            "confidence": 0.88,
            "bbox": [286, 112, 382, 202],
            "position_hint": "front_right",
        },
        {
            "label": "table_edge",
            "display_name": "机の端",
            "confidence": 0.81,
            "bbox": [0, 256, 480, 294],
            "position_hint": "near_front",
        },
    ]

    if "赤" in instruction or "red" in lower_instruction:
        objects = [obj for obj in objects if obj["label"] == "red_block"]
    elif "青" in instruction or "blue" in lower_instruction:
        objects = [obj for obj in objects if obj["label"] == "blue_marker"]

    return {
        "source": source,
        "image_size": IMAGE_SIZE,
        "objects": objects,
        "summary": "perception service mock completed; replace detector internals with YOLO/SAM/etc. later",
    }


def utc_now() -> str:
    return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())


def main() -> None:
    server = ThreadingHTTPServer(("0.0.0.0", 8070), Handler)
    print("Perception Service listening on :8070")
    server.serve_forever()


if __name__ == "__main__":
    main()
