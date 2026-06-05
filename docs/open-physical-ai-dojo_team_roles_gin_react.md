# Open Physical AI Dojo 3名開発チーム役割分担 Gin + React版

## 1. 前提

本プロジェクトは、バックエンドにGoのGin、フロントエンドにReactとNode.jsを採用する。

この構成では、Ginを単なるWeb APIではなく、AIモデル、シミュレータ、実機ロボット、学習ログ、評価ベンチマークをつなぐOrchestratorとして設計する。AIモデルやロボット制御は、PythonやROS 2を併用する可能性が高いため、GinはそれらをHTTP、gRPC、CLI、またはローカルプロセス経由で制御する統合層として扱う。

## 2. 推奨する3名体制

| 担当者 | 役割 | 主担当 | 一言でいう役割 |
|---|---|---|---|
| 大串 | Backend / Integration Lead | Go Gin、API、DB、AI・ロボット連携、全体統合 | 全部をつなぐ司令塔 |
| 谷口 | Frontend / UX / Education Lead | React、Node.js、UI、教材、デモ体験 | 学生が使える形にする |
| 伊豆味 | AI / Robotics / Data Lead | 画像認識、指示理解、シミュレータ、実機、データ | AIが見て動く部分を作る |

3人だけで進める場合、PM専任を置くより、大串がBackend / Integration LeadとしてPM兼アーキテクトを兼ねるのが現実的である。

## 3. 役割1: Backend / Integration Lead（担当: 大串）

### ミッション

Ginを中心に、フロントエンド、AIモデル、シミュレータ、実機ロボット、ログ保存、評価実行をつなぐ。

### 主な担当

- Go GinによるAPIサーバー開発
- API設計とOpenAPI仕様の作成
- 認証・ユーザー管理の最小実装
- タスク実行APIの実装
- AI推論サービスとの接続
- シミュレータ実行サービスとの接続
- 実機ロボット制御サービスとの接続
- Episodeログ保存
- 評価ベンチマーク実行API
- DB設計
- Docker構成
- CI/CDの基本整備
- 全体スケジュール管理

### 担当モジュール

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
  internal/safety/
  internal/evaluation/
```

### 主なAPI

| API | 内容 |
|---|---|
| POST /api/tasks | 日本語指示を受け取りタスクを開始する |
| GET /api/tasks/:id | タスク状態を取得する |
| POST /api/perception | 画像認識を実行する |
| POST /api/plans | 行動計画を生成する |
| POST /api/actions/execute | シミュレータまたは実機で実行する |
| GET /api/episodes | 実行ログを取得する |
| POST /api/evaluations/run | 評価ベンチマークを実行する |
| GET /api/stream/tasks/:id | 実行状態をリアルタイム配信する |

### 成果物

- Gin APIサーバー
- API仕様書
- DBスキーマ
- AI/Robot/Simulator連携アダプタ
- 実行ログ保存機能
- 評価API
- Docker Compose
- READMEの開発環境手順

## 4. 役割2: Frontend / UX / Education Lead（担当: 谷口）

### ミッション

ReactとNode.jsで、学生が迷わず使える学習・実験・評価UIを作る。単なる管理画面ではなく、フィジカルAIの学習体験を成立させる。

### 主な担当

- Reactフロントエンド開発
- Node.jsによるフロント開発環境整備
- 学生向けダッシュボード
- タスク実行画面
- カメラ画像・認識結果オーバーレイ表示
- 行動計画の可視化
- シミュレータ・実機実行ログ表示
- 評価結果画面
- 教材ページ
- 教員向け進捗確認画面
- デモ動画用の見せ方設計
- UIから見たAPI要件の整理

### 担当モジュール

```text
frontend/
  src/
    app/
    components/
    features/tasks/
    features/perception/
    features/plans/
    features/episodes/
    features/evaluations/
    features/lessons/
    lib/api/
```

### 主要画面

| 画面 | 内容 |
|---|---|
| Dashboard | 学生の演習状況、最近のタスク、評価結果 |
| Task Runner | 日本語指示入力、実行環境選択、開始・停止 |
| Vision Viewer | カメラ画像、物体検出枠、セグメント表示 |
| Plan Viewer | AIが生成した行動ステップの表示 |
| Execution Monitor | 実行中ログ、成功・失敗、停止状態 |
| Episode Browser | 過去の実行ログ、失敗理由、再実行 |
| Benchmark | 評価ベンチマークの実行結果 |
| Lessons | 学生向け教材と演習課題 |

### 成果物

- Reactアプリ
- APIクライアント
- 学生向けUI
- 教員向けUI
- 教材5本以上
- デモ用画面
- UI操作手順書

## 5. 役割3: AI / Robotics / Data Lead（担当: 伊豆味）

### ミッション

画像認識、日本語指示理解、シミュレータ、実機ロボット、データセットを担当し、AIが物理タスクを実行できる状態を作る。

### 主な担当

- 物体検出モデルの選定・微調整
- セグメンテーションモデルの利用
- 日本語指示の構造化
- タスク計画の初期ロジック作成
- シミュレータ環境の構築
- Pick & Placeタスクの実装
- 実機ロボット接続
- カメラキャリブレーション
- 作業台座標とロボット座標の変換
- Safety Guardロジックの原案作成
- データセット作成
- 評価ベンチマーク作成

### 担当モジュール

```text
ai_robotics/
  perception/
  language/
  planner/
  simulator/
  robot/
  calibration/
  datasets/
  benchmarks/
```

### Ginとの接続方式

初期版では、AI/Robot側をGinに直接埋め込まない。以下のいずれかで疎結合にする。

| 接続方式 | 用途 |
|---|---|
| HTTP service | AI推論、シミュレータ実行 |
| CLI execution | 学習スクリプト、評価スクリプト |
| gRPC | 高速な制御連携が必要になった場合 |
| File/Queue | Episodeログ、学習データ連携 |

### 成果物

- 物体認識モデル
- 日本語指示理解処理
- シミュレータ環境
- 実機ロボット接続サンプル
- カメラキャリブレーション手順
- データセット仕様
- 評価ベンチマーク
- 実機デモ動画

## 6. 技術境界

### 6.1 Ginが担当すること

- UIからのリクエスト受付
- タスク状態管理
- AIサービス呼び出し
- シミュレータサービス呼び出し
- ロボット制御サービス呼び出し
- 安全判定の最終ゲート
- DB保存
- 評価結果管理
- APIとしての公開

### 6.2 React/Node.jsが担当すること

- 学生が操作する画面
- 教員が確認する画面
- カメラ画像・認識結果の表示
- 行動計画の可視化
- 実行ログのリアルタイム表示
- 教材・演習の提供
- APIクライアント

### 6.3 AI/Robot側が担当すること

- 画像認識
- 日本語指示理解
- タスク計画の候補生成
- シミュレータ実行
- 実機ロボット制御
- キャリブレーション
- 学習・評価
- データ生成

## 7. RACI表 Gin + React版

| 作業 | 大串: Backend | 谷口: Frontend | 伊豆味: AI/Robot |
|---|---|---|---|
| 全体設計 | A/R | C | C |
| API設計 | A/R | C | C |
| Gin API実装 | A/R | I | C |
| DB設計 | A/R | I | C |
| React UI実装 | C | A/R | I |
| Node.js開発環境 | I | A/R | I |
| 教材ページ | C | A/R | C |
| 物体認識 | C | I | A/R |
| 日本語指示理解 | C | C | A/R |
| シミュレータ | C | I | A/R |
| 実機ロボット制御 | C | I | A/R |
| Safety Guard | A | C | R |
| 評価ベンチマーク | A | C | R |
| デモ設計 | A | R | R |
| GitHub公開 | A/R | C | C |
| 応募資料 | A/R | C | C |

## 8. 月別分担

### 1か月目: 基盤設計

| 担当者 | 作業 |
|---|---|
| 大串 | Ginプロジェクト作成、API仕様、DBスキーマ、Docker構成 |
| 谷口 | Reactプロジェクト作成、画面設計、APIクライアント雛形 |
| 伊豆味 | モデル候補、シミュレータ候補、実機候補、データ仕様 |

完了条件:

- GinとReactが接続できる
- タスク実行APIのモックが動く
- AI/Robot側の入出力JSONが決まっている

### 2か月目: 認識とUIの初期実装

| 担当者 | 作業 |
|---|---|
| 大串 | /api/perception、/api/tasks、Episode保存 |
| 谷口 | Task Runner、Vision Viewer、Dashboard初期版 |
| 伊豆味 | 物体検出、合成データ生成、サンプル推論 |

完了条件:

- UIから画像認識を実行できる
- 検出結果を画面に重ねて表示できる
- Episodeログを保存できる

### 3か月目: 日本語指示理解と計画生成

| 担当者 | 作業 |
|---|---|
| 大串 | /api/plans、AIサービス連携、タスク状態管理 |
| 谷口 | Plan Viewer、Execution Monitor初期版 |
| 伊豆味 | 日本語指示理解、タスク計画、Pick & Placeシミュレータ |

完了条件:

- 日本語指示を構造化できる
- 行動計画をUIに表示できる
- シミュレータでPick & Placeを実行できる

### 4か月目: 実行制御と安全判定

| 担当者 | 作業 |
|---|---|
| 大串 | /api/actions/execute、Safety Guard、リアルタイム配信 |
| 谷口 | 実行中ログ、停止ボタン、成功失敗表示 |
| 伊豆味 | Action Policy、Safety Guard原案、シミュレータ評価 |

完了条件:

- UIからシミュレータ実行できる
- 実行状態をリアルタイム表示できる
- 危険または不確実な動作を止められる

### 5か月目: 実機連携

| 担当者 | 作業 |
|---|---|
| 大串 | Robot Runtime連携、実機タスク状態管理、ログ強化 |
| 谷口 | 実機モード、カメラ映像、失敗分析画面 |
| 伊豆味 | 実機ロボット接続、カメラキャリブレーション、実機Pick & Place |

完了条件:

- 実機で最低1タスクが動く
- UIから実機モードを操作できる
- 成功・失敗ログを保存できる

### 6か月目: 公開・評価・応募資料

| 担当者 | 作業 |
|---|---|
| 大串 | API整理、Docker、README、評価API |
| 谷口 | 教材5本、デモ画面、教員画面、操作手順 |
| 伊豆味 | モデルカード、データセットカード、評価結果、デモ動画 |

完了条件:

- GitHub公開できる
- デモ動画がある
- 定量評価がある
- 応募資料に成果物として記載できる

## 9. 優先順位の再設定

Gin + React構成では、最初から大規模モデル開発に寄せすぎず、UIからAIとロボットが一連で動くことを優先する。

### 最優先

- Gin API
- React Task Runner
- 物体認識のサンプル推論
- 日本語指示のJSON化
- シミュレータPick & Place
- Episodeログ保存
- 実行状態表示

### 次点

- 実機ロボット接続
- Safety Guard
- 評価ベンチマーク
- 教材5本
- デモ動画

### 後回し

- 本格的な強化学習
- 大規模VLAモデルの事前学習
- 複雑なUI
- 複数ロボット対応
- 高度な権限管理

## 10. 結論

Gin、React、Node.jsを採用する場合、3名の分担は以下が最も現実的である。

```text
大串: Go Ginで全体をつなぐ
谷口: React/Node.jsで学生が使える画面を作る
伊豆味: AI/ロボット/シミュレータで物理世界を動かす
```

この分担なら、3名でも「応募資料に見せられるWebデモ」「AI推論」「シミュレータ・実機動作」を並行して進められる。
