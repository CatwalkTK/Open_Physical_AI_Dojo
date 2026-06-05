# Perception Service

HTTP service boundary for visual perception.

Current implementation is a mock workbench detector. Keep the HTTP contract stable when replacing internals with YOLO, SAM, OpenCV, or another model-backed detector.

Run locally:

```bash
python3 app.py
```

Endpoints:

- `GET /health`
- `POST /perception`

Example:

```bash
curl -s -X POST http://localhost:8070/perception \
  -H 'Content-Type: application/json' \
  -d '{"source":"sample_workbench","instruction":"赤いブロックを見つけて"}'
```
