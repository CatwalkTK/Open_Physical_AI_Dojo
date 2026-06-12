#!/usr/bin/env python3
from __future__ import annotations

import json
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any

import detector


MODEL_NAME = "opencv-color-detector-v1"
MAX_BODY_BYTES = 12 * 1024 * 1024


class Handler(BaseHTTPRequestHandler):
    def do_GET(self) -> None:
        if self.path == "/health":
            self.respond({
                "status": "ok",
                "service": "perception",
                "model": MODEL_NAME,
                "updated_at": utc_now(),
            })
            return
        self.respond({"error": "not found"}, status=404)

    def do_POST(self) -> None:
        if self.path == "/perception":
            payload = self.read_json()
            if payload is None:
                self.respond({"error": "request body too large"}, status=413)
                return
            self.respond(run_perception(payload))
            return
        self.respond({"error": "not found"}, status=404)

    def read_json(self) -> dict[str, Any] | None:
        length = int(self.headers.get("Content-Length", "0"))
        if length > MAX_BODY_BYTES:
            return None
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
    image_base64 = payload.get("image_base64") or ""

    image = None
    if image_base64:
        image = detector.decode_base64_image(str(image_base64))
        if image is None:
            return {
                "source": source,
                "image_size": {"width": 0, "height": 0},
                "objects": [],
                "summary": "image_base64 could not be decoded",
            }
    if image is None:
        image = detector.generate_sample_scene()
        source = source or "sample_workbench"

    height, width = image.shape[:2]
    objects = detector.detect(image)
    return {
        "source": source,
        "image_size": {"width": int(width), "height": int(height)},
        "objects": objects,
        "image_base64": detector.encode_image_base64(image),
        "summary": f"{MODEL_NAME} detected {len(objects)} objects",
    }


def utc_now() -> str:
    return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())


def main() -> None:
    server = ThreadingHTTPServer(("0.0.0.0", 8070), Handler)
    print(f"Perception Service ({MODEL_NAME}) listening on :8070")
    server.serve_forever()


if __name__ == "__main__":
    main()
