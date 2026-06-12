#!/usr/bin/env python3
from __future__ import annotations

import base64
import json
import struct
import time
import zlib
from dataclasses import asdict, dataclass, field
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any

CAMERA_WIDTH = 480
CAMERA_HEIGHT = 320


def render_camera_scene(width: int = CAMERA_WIDTH, height: int = CAMERA_HEIGHT) -> bytes:
    """Render a synthetic robot camera view as PNG using only the stdlib.

    The real runtime will replace this with actual camera frames, so the
    mock intentionally avoids third-party imaging dependencies.
    """
    rows = []
    for y in range(height):
        row = bytearray([0])  # PNG filter type 0 (None) per scanline
        floor_shade = 90 + int(50 * y / height)
        for x in range(width):
            r, g, b = floor_shade, floor_shade + 8, floor_shade + 14
            if 200 <= x <= 320 and 150 <= y <= 250:
                r, g, b = 205, 40, 40  # red block ahead
            elif (x - 96) ** 2 + (y - 210) ** 2 <= 40 ** 2:
                r, g, b = 30, 110, 190  # blue marker, front left
            elif y >= 290:
                r, g, b = 56, 52, 48  # floor edge
            row += bytes((r, g, b))
        rows.append(bytes(row))

    def chunk(kind: bytes, data: bytes) -> bytes:
        return struct.pack(">I", len(data)) + kind + data + struct.pack(
            ">I", zlib.crc32(kind + data) & 0xFFFFFFFF
        )

    ihdr = struct.pack(">IIBBBBB", width, height, 8, 2, 0, 0, 0)
    idat = zlib.compress(b"".join(rows), level=6)
    return (
        b"\x89PNG\r\n\x1a\n"
        + chunk(b"IHDR", ihdr)
        + chunk(b"IDAT", idat)
        + chunk(b"IEND", b"")
    )


CAMERA_FRAME_BASE64 = base64.b64encode(render_camera_scene()).decode("ascii")


@dataclass
class RuntimeState:
    mode: str = "idle"
    battery: float = 92.0
    roll: float = 0.0
    pitch: float = 0.0
    yaw: float = 0.0
    servo_angles: list[float] = field(default_factory=lambda: [0.0] * 12)
    updated_at: str = ""

    def to_payload(self) -> dict[str, Any]:
        self.updated_at = utc_now()
        return asdict(self)


state = RuntimeState()


class Handler(BaseHTTPRequestHandler):
    def do_GET(self) -> None:
        if self.path == "/health":
            self.respond({"status": "ok", "robot": "dogzilla", "mode": state.mode})
            return
        if self.path == "/state":
            self.respond(state.to_payload())
            return
        if self.path == "/camera/frame":
            self.respond({
                "format": "png",
                "width": CAMERA_WIDTH,
                "height": CAMERA_HEIGHT,
                "image_base64": CAMERA_FRAME_BASE64,
                "updated_at": utc_now(),
            })
            return
        self.respond({"error": "not found"}, status=404)

    def do_POST(self) -> None:
        payload = self.read_json()
        if self.path == "/motion/stand":
            state.mode = "standing"
            self.respond({"ok": True, "mode": state.mode, "updated_at": utc_now()})
            return
        if self.path == "/motion/sit":
            state.mode = "sitting"
            self.respond({"ok": True, "mode": state.mode, "updated_at": utc_now()})
            return
        if self.path == "/motion/move":
            duration_ms = int(payload.get("duration_ms", 300))
            if duration_ms > 1800:
                self.respond({"error": "duration_ms exceeds mock safety limit"}, status=400)
                return
            state.mode = "moving"
            state.yaw += float(payload.get("yaw_deg", 0.0))
            state.pitch = min(5.0, abs(float(payload.get("linear_x", 0.0))) * 20)
            time.sleep(max(duration_ms, 100) / 1000)
            state.mode = "standing"
            state.pitch = 0.0
            self.respond({"ok": True, "mode": state.mode, "updated_at": utc_now()})
            return
        if self.path == "/motion/pose":
            state.mode = "pose"
            self.respond({"ok": True, "mode": state.mode, "updated_at": utc_now()})
            return
        if self.path == "/motion/gait":
            state.mode = "gait:" + str(payload.get("name", "default"))
            self.respond({"ok": True, "mode": state.mode, "updated_at": utc_now()})
            return
        if self.path == "/nav/goal":
            state.mode = "navigating"
            self.respond({"ok": True, "mode": state.mode, "updated_at": utc_now()})
            return
        if self.path == "/stop":
            state.mode = "stopped"
            state.pitch = 0.0
            state.roll = 0.0
            self.respond({"ok": True, "mode": state.mode, "updated_at": utc_now()})
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
        body = json.dumps(payload).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, fmt: str, *args: Any) -> None:
        print("[%s] %s" % (utc_now(), fmt % args))


def utc_now() -> str:
    return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())


def main() -> None:
    server = ThreadingHTTPServer(("0.0.0.0", 8090), Handler)
    print("Dogzilla Runtime mock listening on :8090")
    server.serve_forever()


if __name__ == "__main__":
    main()
