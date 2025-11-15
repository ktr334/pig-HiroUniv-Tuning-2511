-- このファイルに記述されたSQLコマンドが、マイグレーション時に実行されます。

-- ユーザー別の注文履歴を日時順で素早く取り出すための複合インデックス
CREATE INDEX IF NOT EXISTS idx_orders_user_created_id ON orders(user_id, created_at, order_id);