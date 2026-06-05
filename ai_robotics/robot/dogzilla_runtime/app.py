#!/usr/bin/env python3
from __future__ import annotations

import json
import time
from dataclasses import asdict, dataclass, field
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any


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
                "format": "mock",
                "message": "camera frame endpoint placeholder",
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
