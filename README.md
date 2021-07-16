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
- `cmd:finish`: バッチの作業終了
- `cmd:status`: バッチの状況を受信する
- `cmd:robot`: ロボットの位置を受信(実装中)
- `cmd:call <x> <y>`: (x,y)の地点にロボットを呼ぶ（実装中）

- `send:<message>`: 他のwebsocketクライアントに`<message>`を送信する
- `echo:<message>`: `<message>`をechoする

## synerexを使ったUnityシミュレータ群との連動
trusco_field以外は`https://github.com/fukurin00/HMI-Services.git`のsubmoduleにあります
### 手順
1. node_server, synerex_serverを起動
2. proxy_provider起動
3. trusco_field起動
4. hmi-serviceを起動
5. cli-providerでsetState, wmsCsvを実行(シミュレータのセットアップ(人の配置))
6. websocketで接続
7. `id`コマンドでユーザidを指定
8. `cmd:start`でバッチ作業開始
9. `cmd:next`で次のアイテムに以降
10. `cmd:finish`でバッチの出荷完了

# メッセージ形式
`{"type":"<メッセージの種類>", "payload":<メッセージ>`
## メッセージの種類
- item
  - `{"name":商品名, "position":{"x":x座標, "y":y座標}, "shelf":棚のID, "id":商品ID, "batch_id":バッチのID, "pick_time":ピッキング時刻}`
- error
  - `"エラーメッセージ"`
- finish
  - `"finish"`
- batch
  - `{"id":作業中バッチのid, "worker_id":作業者のid, "floor":階(int), "ship_pos":{"x":出荷場所のx座標, "y":出荷場所のy座標}, "items":商品の数分の`cmd:start`と同じ形式の配列, "start_time":作業中バッチの作業開始時刻}`
- robot
  - 実装中
  - `テキスト`

- 時刻は `<Year>-<Month>-<Day>T<Hour>:<Minute>:<Second>Z`の形式で出力
- id系は整数(不明の場合は`-1`)

### cmd:start
- 出力: 次のピッキングアイテム
- 形式: item  
### cmd:next
- 出力: 次のピッキングアイテム、すべてピッキングされていたら出荷場所を指定
- 形式: item
### cmd:finish
- 出力: finish

### cmd:status
- 出力: 現在のバッチの作業状況
- 形式: batch