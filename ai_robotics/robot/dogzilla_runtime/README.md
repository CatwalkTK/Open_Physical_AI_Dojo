# Dogzilla Runtime Mock

This is the HTTP boundary used by the Gin orchestrator. The current file is a mock that can run without ROS2 or a physical Dogzilla.

Run locally:

```bash
python3 app.py
```

Important endpoints:

- `GET /health`
- `GET /state`
- `GET /camera/frame`
- `POST /motion/stand`
- `POST /motion/sit`
- `POST /motion/move`
- `POST /stop`

The real Dogzilla implementation should keep this HTTP contract and move ROS2 Humble, camera, IMU, servo state, and emergency stop logic behind it.
