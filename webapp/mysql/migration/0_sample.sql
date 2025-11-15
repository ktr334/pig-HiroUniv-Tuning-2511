-- このファイルに記述されたSQLコマンドが、マイグレーション時に実行されます。

-- MySQL 5.7 互換のため IF NOT EXISTS は付けずに作成
CREATE INDEX idx_orders_user_created_id ON orders(user_id, created_at, order_id);

-- 検索用インデックス
CREATE INDEX idx_products_name ON products(name);