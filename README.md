# proc-lifegame

![Go](https://img.shields.io/badge/Go-1.24-00ADD8?logo=go)
![License](https://img.shields.io/badge/License-MIT-blue)

## 概要

`proc-lifegame` は、Goで実装したLinuxプロセス版の分散ライフゲームです。  
1つのセルを1つのLinuxプロセスとして扱い、`proc-coordinator` が親プロセスとして複数の `proc-cell` を起動します。

各 `proc-cell` は自分の座標、生死状態、世代番号を保持し、`proc-coordinator` とTCP localhost + JSONで通信します。  
`coordinator` は各世代ごとに近傍状態を配布し、全セルの応答が揃ってから次の世代へ進みます。

これは効率の良いライフゲーム実装を目指したものではありません。  
目的は、プロセス管理、IPC、同期制御、Coordinator-Worker構成、TUI表示といったインフラ寄りの技術を、観察しやすい題材で学ぶことです。

## 特徴

- 1セル = 1Linuxプロセス
- TCP localhost + JSONによるプロセス間通信
- Coordinator-Worker構成
- バリア同期による世代管理
- Bubble TeaによるTUI表示
- CUIモードによる軽量表示
- 盤面サイズ・世代数・更新間隔をCLIオプションで変更可能
- ランダム配置や `glider` / `beacon` / `glider-gun` などの初期配置テンプレート

## デモ

![proc-lifegame demo](docs/demo.gif)

## システム構成

<img src="docs/architecture.png" alt="architecture" width="50%">

`proc-coordinator` が親プロセスとして動作し、盤面サイズに応じて複数の `proc-cell` を起動します。  
表示は `internal/ui` が担当しますが、UI は `cell` に直接通信せず、`coordinator` が生成した盤面スナップショットだけを見ます。

<img src="docs/coordinator.png" alt="coordinator" width="50%">

プロセスモデル・バリア同期・IPCの詳細は [docs/reference.md](docs/reference.md) を参照してください。

## クイックスタート

```bash
go run ./cmd/proc-coordinator \
  --width 8 \
  --height 8 \
  --generations 100 \
  --pattern random \
  --interval 1s \
  --ui tui
```

動作要件、CUIモード、`--debug` オプション、全CLIオプション、初期配置テンプレート一覧、TUIの操作方法は [docs/reference.md](docs/reference.md) にまとめています。

## ドキュメント

- [docs/design.md](docs/design.md) — 設計判断ログ（1プロセス=1セル、TCP/JSONの採用理由、バリア同期、UI/cellの責務分離など）
- [docs/reference.md](docs/reference.md) — アーキテクチャ詳細、ディレクトリ構成、全CLIオプション、初期配置テンプレート、操作方法

## ライセンス

[LICENSE](LICENSE) を参照してください。
