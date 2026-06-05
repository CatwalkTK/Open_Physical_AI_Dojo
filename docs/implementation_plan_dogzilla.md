# Open Physical AI Dojo 実装計画 Dogzilla版

作成日: 2026-06-05

## 1. 方針

Open Physical AI Dojoは、学生が「見る、理解する、判断する、動かす」を一気通貫で学ぶための教育開発環境として実装する。既存docsの方針に従い、GinはOrchestrator、Reactは学習UI、AI/RobotはPython/ROS2側の疎結合サービスとして扱う。

実機ロボットはYahboom DOGZILLA S1/S2を前提にする。DogzillaはRaspberry Pi 5、ROS2 Humble、Pythonプログラミング、カメラ、IMU、サーボ角フィードバックを使える教育向け四足ロボットで、S2ではLiDARと音声モジュールも扱える。このため、初期実装から以下を固定方針にする。

- GinからDogzillaを直接制御しない
- Dogzilla制御は`ai_robotics/robot/dogzilla_runtime`に分離する
- GinとDogzilla RuntimeはHTTPまたはROS2 Bridge経由で接続する
- 実機実行前にSafety Guardを必ず通す
- Episodeログには画像、認識結果、計画、実行コマンド、ロボット状態、成功失敗を保存する

## 2. 全体アーキテクチャ

```text
frontend/ React
  |
  | REST / SSE
  v
backend/ Gin Orchestrator
  |-- DB: users, tasks, episodes, evaluations
  |-- Safety Guard
  |-- AI Adapter
  |-- Simulator Adapter
  `-- Robot Adapter
        |
        | HTTP / CLI / ROS2 bridge
        v
ai_robotics/
  |-- perception service
  |-- language service
  |-- planner service
  |-- simulator service
  `-- robot/dogzilla_runtime
        |-- ROS2 Humble nodes
        |-- camera
        |-- IMU / servo state
        |-- gait / pose / navigation commands
        `-- emergency stop
```

## 3. 実装モジュール

### 3.1 Backend Gin

```text
backend/
  cmd/server/
  internal/api/
  internal/domain/
  internal/service/
  internal/repository/
  internal/integration/ai/
  internal/integration/simulator/
  internal/integration/robot/
    dogzilla_client.go
  internal/safety/
  internal/evaluation/
  internal/stream/
```

主な責務:

- Reactからのタスク要求受付
- 日本語指示、画像、実行環境の管理
- AI/Planner/Simulator/Dogzilla Runtimeの呼び出し
- Safety Guardの最終判定
- SSEによる実行状態配信
- Episodeログ保存
- 評価ベンチマーク管理

### 3.2 Frontend React

```text
frontend/
  src/
    app/
    components/
    features/tasks/
    features/perception/
    features/plans/
    features/execution/
    features/episodes/
    features/evaluations/
    features/lessons/
    lib/api/
```

主な画面:

- Dashboard: 学習状況、最近のタスク、評価結果
- Task Runner: 日本語指示、画像入力、sim/real選択、開始/停止
- Vision Viewer: カメラ画像と検出枠
- Plan Viewer: 構造化指示と行動ステップ
- Execution Monitor: 実行ログ、Dogzilla状態、緊急停止
- Episode Browser: 成功/失敗ログ、再実行
- Benchmark: 認識、計画、実行の評価
- Lessons: 演習教材

### 3.3 AI / Robotics

```text
ai_robotics/
  perception/
  language/
  planner/
  simulator/
  robot/
    dogzilla_runtime/
      app.py
      ros_nodes/
      adapters/
      safety/
      config/
  calibration/
  datasets/
  benchmarks/
```

主な責務:

- 物体検出と認識
- 日本語指示のJSON化
- 行動計画生成
- シミュレータ実行
- Dogzilla ROS2制御
- カメラ、LiDAR、IMU、サーボ状態の取得
- キャリブレーション
- データセットと評価ベンチマーク

## 4. API設計

### 4.1 Gin公開API

| API | 内容 |
|---|---|
| `POST /api/tasks` | 日本語指示、画像、実行環境を受け取りタスク作成 |
| `GET /api/tasks/:id` | タスク状態取得 |
| `POST /api/perception` | 画像認識実行 |
| `POST /api/plans` | 構造化指示と行動計画生成 |
| `POST /api/actions/execute` | simまたはDogzillaで実行 |
| `POST /api/actions/stop` | 実行停止、Dogzilla emergency stop |
| `GET /api/episodes` | Episodeログ一覧 |
| `GET /api/episodes/:id` | Episode詳細 |
| `POST /api/evaluations/run` | 評価ベンチマーク実行 |
| `GET /api/stream/tasks/:id` | SSEで状態配信 |

### 4.2 Dogzilla Runtime API

Ginから呼び出す内部APIとして定義する。

| API | 内容 |
|---|---|
| `GET /health` | Dogzilla Runtimeの生存確認 |
| `GET /state` | 姿勢、バッテリー、IMU、サーボ角、モード |
| `GET /camera/frame` | 最新カメラフレーム取得 |
| `POST /motion/stand` | 起立 |
| `POST /motion/sit` | 着座または安全姿勢 |
| `POST /motion/move` | 前後左右移動、旋回 |
| `POST /motion/pose` | 姿勢制御 |
| `POST /motion/gait` | gait選択 |
| `POST /nav/goal` | S2のLiDAR/Nav2利用時の目標移動 |
| `POST /stop` | 即時停止 |

初期MVPでは`stand`、`sit`、`move`、`stop`、`state`、`camera/frame`を優先する。

## 5. データモデル

### Task

```json
{
  "id": "task_001",
  "instruction": "赤いブロックの近くまで移動して止まって",
  "environment": "simulator|dogzilla",
  "status": "queued|running|succeeded|failed|stopped",
  "created_at": "2026-06-05T00:00:00Z"
}
```

### PerceptionResult

```json
{
  "objects": [
    {
      "label": "red_block",
      "confidence": 0.92,
      "bbox": [120, 80, 220, 180],
      "position_hint": "front_left"
    }
  ]
}
```

### ActionPlan

```json
{
  "goal": "approach_target",
  "steps": [
    {"type": "stand"},
    {"type": "turn", "yaw_deg": -15},
    {"type": "move", "linear_x": 0.08, "duration_ms": 1200},
    {"type": "stop"}
  ],
  "risk_level": "low|medium|high"
}
```

### Episode

```json
{
  "task_id": "task_001",
  "instruction": "...",
  "perception": {},
  "plan": {},
  "commands": [],
  "robot_states": [],
  "result": "succeeded|failed|stopped",
  "failure_reason": null
}
```

## 6. Safety Guard

Dogzilla前提では、Safety GuardをMVPから必須にする。

### Gin側の安全判定

- `environment=dogzilla`では高リスク操作を拒否
- 速度、旋回角、実行時間に上限を設定
- 未認識対象への移動を禁止
- UIからの停止を常時受け付ける
- Runtimeの`/health`が失敗したら実行しない
- `risk_level=high`の計画は実機実行不可

### Dogzilla Runtime側の安全判定

- 起動直後は停止/安全姿勢
- コマンドタイムアウト時は停止
- IMU異常、転倒疑い、低バッテリー時は停止
- サーボ角が許容範囲外なら停止
- S2 LiDAR利用時は前方障害物で停止

## 7. 開発フェーズ

### Phase 0: リポジトリ基盤

期間: 1週間

- Gin、React、Pythonサービスのディレクトリ作成
- Docker Compose作成
- OpenAPI雛形作成
- Task/Plan/EpisodeのJSONスキーマ作成
- Dogzilla RuntimeのモックAPI作成

完了条件:

- `docker compose up`でGin、React、Dogzilla mockが起動する
- Reactから`POST /api/tasks`を呼べる

### Phase 1: WebデモMVP

期間: 2週間

- Task Runner実装
- Ginの`/api/tasks`、`/api/plans`、`/api/stream/tasks/:id`実装
- 日本語指示をルールベースでJSON化
- 行動計画をUI表示
- Episode保存

完了条件:

- 日本語指示を入力すると、構造化結果と行動計画が表示される
- 実行ログがEpisodeとして保存される

### Phase 2: 認識連携

期間: 2週間

- `POST /api/perception`実装
- サンプル画像の物体検出
- Vision Viewerで検出枠表示
- Dogzillaカメラフレーム取得APIのモック実装
- 認識結果を計画生成に反映

完了条件:

- UIから画像認識を実行できる
- 認識対象を含む行動計画を生成できる

### Phase 3: シミュレータ実行

期間: 3週間

- Simulator Adapter実装
- Pick/Placeではなく、Dogzillaに適した移動・接近・停止タスクを先に実装
- `POST /api/actions/execute`実装
- SSEで実行状態配信
- Benchmark初期版

完了条件:

- UIからシミュレータ上のDogzillaタスクを実行できる
- 成功/失敗がEpisodeに残る

### Phase 4: Dogzilla実機Bring-up

期間: 3週間

- Raspberry Pi 5上でROS2 Humble環境確認
- Yahboom Dogzillaサンプル起動
- カメラ、IMU、サーボ状態取得確認
- Dogzilla Runtimeの`/health`、`/state`、`/camera/frame`実装
- `stand`、`sit`、`move`、`stop`実装
- Gin Robot AdapterからRuntimeを呼び出す

完了条件:

- Gin経由でDogzillaの起立、短距離移動、停止ができる
- UIの停止ボタンで即時停止できる

### Phase 5: 実機VLAデモ

期間: 3週間

- 日本語指示からDogzilla行動計画生成
- カメラ認識結果に基づく接近/停止
- Safety Guard強化
- 失敗理由の分類
- 教材1から3本を実機対応

完了条件:

- 「赤い物体の近くまで移動して止まる」などの安全な実機タスクが動く
- 成功/失敗ログを教材と評価に使える

### Phase 6: 公開・教育コンテンツ

期間: 3週間

- 教材5本作成
- 教員向け運用ガイド作成
- 評価ベンチマーク整備
- README、モデルカード、データセットカード作成
- デモ動画撮影

完了条件:

- GitHub公開できる
- Webデモ、シミュレータデモ、Dogzilla実機デモが揃う

## 8. MVPで扱うタスク

Dogzillaは四足移動ロボットなので、最初のMVPではロボットアーム前提のPick & Placeを主タスクにしない。教育効果と実機安全性を優先し、以下から始める。

1. 色付き物体を認識する
2. 日本語指示から対象物と動作を抽出する
3. 対象方向へ短距離移動する
4. 指定距離または障害物手前で止まる
5. 実行ログから成功/失敗を振り返る

例:

- 「赤いブロックを見つけて、近くまで移動して止まって」
- 「前方の障害物に近づきすぎないように進んで」
- 「青い目印の方向を向いて」
- 「机の端に近づいたら止まって」

## 9. 教材計画

| 教材 | 内容 | 成果 |
|---|---|---|
| Lesson 1 | カメラ画像と物体検出 | Vision Viewerで検出枠を見る |
| Lesson 2 | 日本語指示のJSON化 | 指示から対象、動作、制約を抽出 |
| Lesson 3 | 行動計画 | stand/move/turn/stopの計画を作る |
| Lesson 4 | シミュレータ実行 | 安全に行動計画を試す |
| Lesson 5 | Dogzilla実機実行 | Safety Guardつきで短距離移動 |
| Lesson 6 | Episode分析 | 失敗理由を見て改善する |

## 10. 優先順位

### 最優先

- Dogzilla Runtime mock
- Gin Robot Adapter
- Safety Guard
- Task Runner
- Plan Viewer
- Execution Monitor
- Episodeログ
- Dogzilla実機の`stand/move/stop/state`

### 次点

- 物体検出の精度改善
- LiDAR/Nav2連携
- 教員向け進捗画面
- 評価ベンチマーク
- 教材拡充

### 後回し

- 本格的なVLA大規模事前学習
- 強化学習によるgait生成
- 複数ロボット対応
- 複雑な権限管理
- Pick & Place中心のタスク

## 11. リスクと対策

| リスク | 対策 |
|---|---|
| Dogzilla実機制御が不安定 | Runtime mockとシミュレータを先に作り、実機差分をAdapterに閉じ込める |
| ROS2とWeb APIの境界が複雑 | GinはHTTPだけを見て、ROS2はDogzilla Runtime内に閉じる |
| 学生が危険な指示を入力する | Safety Guardで速度、時間、距離、環境を制限する |
| Pick & Place要件とDogzillaが合わない | MVPは移動、接近、停止、探索に寄せる |
| データ収集が遅れる | EpisodeログをPhase 1から保存する |
| 実機が1台しかない | mock、sim、realの3モードを同じAPIで扱う |

## 12. 最初に作るべき成果物

1. `backend` Gin API雛形
2. `frontend` React Task Runner
3. `ai_robotics/robot/dogzilla_runtime` mock
4. OpenAPI仕様
5. Task/Plan/Episode JSONスキーマ
6. Safety Guard初期実装
7. Dogzilla Bring-up手順書

## 13. 参考資料

- docs/2026-06-05-AI-Open Physical AI Dojo 要件定義書.md
- docs/open-physical-ai-dojo_team_roles_gin_react.md
- Yahboom DOGZILLA S1/S2 study page: https://www.yahboom.net/study/DOGZILLA
- YahboomTechnology/DOGZILLA GitHub: https://github.com/YahboomTechnology/DOGZILLA
