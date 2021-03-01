# trading-bot

仮想通貨の自動売買を行うボットです。

## 対応する取引所

* Coincheck

## 環境

* docker
* vscode
    * 拡張機能 Remote-Containers を導入しておくこと

## 動かし方

1. ソースコードをclone
2. cloneしたディレクトリを VSCode の Remote-Containers で開く
3. configs/bot-follow-uptrend.toml を開き、下記を編集
    * access_key
    * secret_key
4. VSCode でターミナルを開き下記コマンドでボットを起動
    ```
    make run
    ```
5. 停止するときは `ctrl-c`

### その他

* DBに各種データを保存してます。
    * 接続先情報: `build/docker-compose.yml` を参照
    * 注文履歴: `orders`
    * 約定履歴: `contracts`
    * 保有ポジション: `positions`
    * 利益: `profits`
* Webブラウザで下記にアクセスすると各種GUIツールが使えます。
    * DB管理ツール: `localhost:8080`
    * ダッシュボードツール: `localhost:3000`
