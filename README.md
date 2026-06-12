# Open Physical AI Dojo

Open Physical AI Dojo is an educational VLA development environment for learning perception, Japanese instruction understanding, planning, and physical robot execution.

The current implementation is Phase 7 initial:

- Gin backend orchestrator
- React Task Runner
- Vision Viewer
- Python Perception Service
- Perception Service status API
- Three.js 3D Simulator Viewer with continuous pose animation and orbit camera
- Dogzilla Status Viewer
- Dogzilla Runtime health/state/stop API
- Benchmark Viewer
- Simulator evaluation API
- History Viewer
- JSONL persistence for episodes and evaluations
- Dogzilla Runtime mock
- Safety Guard for real-robot commands
- SSE execution monitor

## Architecture

```text
frontend React
  -> backend Gin
    -> Perception Service
    -> Dogzilla Runtime mock
```

The Dogzilla Runtime mock keeps the HTTP contract that the real ROS2/Python Dogzilla process should implement later. The Perception Service keeps the detector boundary that YOLO, SAM, OpenCV, or another model-backed detector should implement later.

## Run With Docker Compose

```bash
docker compose up
```

Open:

- Frontend: http://localhost:5173
- Backend health: http://localhost:8080/api/health
- Perception service health: http://localhost:8070/health
- Dogzilla mock health: http://localhost:8090/health

## Run Locally

Terminal 1:

```bash
cd ai_robotics/robot/dogzilla_runtime
python3 app.py
```

Terminal 2:

```bash
cd ai_robotics/perception_service
python3 app.py
```

Terminal 3:

```bash
cd backend
DATA_DIR=../data \
DOGZILLA_RUNTIME_URL=http://localhost:8090 \
PERCEPTION_SERVICE_URL=http://localhost:8070 \
go run ./cmd/server
```

Terminal 4:

```bash
cd frontend
npm install
npm run dev
```

## API Smoke Test

```bash
curl -s http://localhost:8080/api/health

curl -s -X POST http://localhost:8080/api/tasks \
  -H 'Content-Type: application/json' \
  -d '{"instruction":"赤いブロックの近くまで移動して止まって","environment":"dogzilla"}'

curl -s -X POST http://localhost:8080/api/perception \
  -H 'Content-Type: application/json' \
  -d '{"source":"sample_workbench","instruction":"赤いブロックを見つけて"}'

curl -s http://localhost:8080/api/perception/status

TASK_ID=$(curl -s -X POST http://localhost:8080/api/tasks \
  -H 'Content-Type: application/json' \
  -d '{"instruction":"赤いブロックの近くまで移動して止まって","environment":"simulator"}' \
  | node -e 'let s="";process.stdin.on("data",d=>s+=d);process.stdin.on("end",()=>console.log(JSON.parse(s).id))')

curl -s -X POST http://localhost:8080/api/actions/execute \
  -H 'Content-Type: application/json' \
  -d "{\"task_id\":\"$TASK_ID\"}"

curl -s http://localhost:8080/api/robot/dogzilla

curl -s -X POST http://localhost:8080/api/robot/dogzilla/stop \
  -H 'Content-Type: application/json' \
  -d '{}'

curl -s -X POST http://localhost:8080/api/evaluations/run \
  -H 'Content-Type: application/json' \
  -d '{"suite":"simulator_basic"}'

curl -s http://localhost:8080/api/tasks

curl -s http://localhost:8080/api/episodes

curl -s http://localhost:8080/api/evaluations
```

## Current Dogzilla Contract

- `GET /health`
- `GET /state`
- `GET /camera/frame`
- `POST /motion/stand`
- `POST /motion/sit`
- `POST /motion/move`
- `POST /stop`

## Next Implementation Targets

1. Replace Perception Service mock internals with a model-backed detector.
2. Replace rule-based planning with a Python language/planner service.
3. Replace JSONL persistence with SQLite or Postgres.
4. Replace Dogzilla mock internals with ROS2 Humble nodes on Raspberry Pi 5.
5. Add benchmark history charts.
