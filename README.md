# HMI Service using Synerex for Harmoware-WES project 

This is a HMI Service Synerex Provider repository for Harmoware-WES.


# 使い方
hmi-servece.goをビルドして起動します
## オプション
- `nosx`: synerexを使わずに起動、`assets/wms_order_demo.csv`をテスト用オーダとして実行する
- `log`: `log/`にログを残す
- `wsaddr`: websocketのアドレス、デフォルトは`localhost:10090`、hololens2と通信する際は`0.0.0.0:10090`などを指定する

## コマンド一覧
websocket接続後に以下のコマンドを送信できる

- `id:<id>`: userのidを設定 (最初に送る必要がある)
- `cmd:start`: 新しいバッチを開始する
- `cmd:next`: 次のアイテムに以降
- `cmd:robot`: ロボットの位置を受信(未実装)
- `cmd:status`: バッチの状況を受信する

- `send:<message>`; 他のwebsocketクライアントに<message>を送信する
- `echo:<message>`: <message>をechoする

## synerexを使った他のシステムとの連動
### 手順
1. node_server, synerex_serverを起動
