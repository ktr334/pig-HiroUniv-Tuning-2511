package repository

import (
	"backend/internal/model"
	"context"

	// N+1問題を解消する過程でsql.NullTimeなどの型を使わなくなったため
	//"database/sql"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
)

type OrderRepository struct {
	db DBTX
}

func NewOrderRepository(db DBTX) *OrderRepository {
	return &OrderRepository{db: db}
}

// 注文を作成し、生成された注文IDを返す
func (r *OrderRepository) Create(ctx context.Context, order *model.Order) (string, error) {
	query := `INSERT INTO orders (user_id, product_id, shipped_status, created_at) VALUES (?, ?, 'shipping', NOW())`
	result, err := r.db.ExecContext(ctx, query, order.UserID, order.ProductID)
	if err != nil {
		return "", err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", id), nil
}

// 複数の注文IDのステータスを一括で更新
// 主に配送ロボットが注文を引き受けた際に一括更新をするために使用
func (r *OrderRepository) UpdateStatuses(ctx context.Context, orderIDs []int64, newStatus string) error {
	if len(orderIDs) == 0 {
		return nil
	}
	query, args, err := sqlx.In("UPDATE orders SET shipped_status = ? WHERE order_id IN (?)", newStatus, orderIDs)
	if err != nil {
		return err
	}
	query = r.db.Rebind(query)
	_, err = r.db.ExecContext(ctx, query, args...)
	return err
}

// 配送中(shipped_status:shipping)の注文一覧を取得
func (r *OrderRepository) GetShippingOrders(ctx context.Context) ([]model.Order, error) {
	var orders []model.Order
	query := `
        SELECT
            o.order_id,
            p.weight,
            p.value
        FROM orders o
        JOIN products p ON o.product_id = p.product_id
        WHERE o.shipped_status = 'shipping'
    `
	err := r.db.SelectContext(ctx, &orders, query)
	return orders, err
}

// 注文履歴一覧を取得
func (r *OrderRepository) ListOrders(ctx context.Context, userID int, req model.ListRequest) ([]model.Order, int, error) {
	// ベースクエリ
	baseQuery := `FROM orders o JOIN products p ON o.product_id = p.product_id`
	whereClause := `WHERE o.user_id = ?`
	args := []interface{}{userID}

	// 検索条件の組み立て
	if req.Search != "" {
		whereClause += " AND p.name LIKE ?"
		if req.Type == "prefix" {
			args = append(args, req.Search+"%")
		} else {
			args = append(args, "%"+req.Search+"%")
		}
	}


	// ソート条件の組み立て
	sortField := "o.order_id"
	switch req.SortField {
	case "product_name":
		sortField = "p.name"
	case "created_at":
		sortField = "o.created_at"
	case "shipped_status":
		sortField = "o.shipped_status"
	case "arrived_at":
		sortField = "o.arrived_at"
	}
	sortOrder := "DESC"
	if strings.ToUpper(req.SortOrder) == "ASC" {
		sortOrder = "ASC"
	}
	orderByClause := fmt.Sprintf("ORDER BY %s %s, o.order_id ASC", sortField, sortOrder)

	// ページネーション
	limitClause := "LIMIT ?"
	offsetClause := "OFFSET ?"
	args = append(args, req.PageSize, req.Offset)

	// データを取得するクエリ (ウィンドウ関数 COUNT(*) OVER() を追加)
	dataQuery := `
		SELECT
			o.order_id,
			o.product_id,
			p.name as product_name,
			o.shipped_status,
			o.created_at,
			o.arrived_at,
			COUNT(*) OVER() as total_count 
		` + baseQuery + " " + whereClause + " " + orderByClause + " " + limitClause + " " + offsetClause

	// 一時的にクエリ結果を受け取るための内部構造体
	// model.Order を埋め込み、total_count を追加
	type orderWithTotal struct {
		model.Order
		TotalCount int `db:"total_count"`
	}

	var results []orderWithTotal
	// SelectContext の宛先を &results に変更
	if err := r.db.SelectContext(ctx, &results, dataQuery, args...); err != nil {
		return nil, 0, err
	}

	// 最終的に返すスライスと総件数を準備
	var orders []model.Order
	var total int = 0

	// 取得した結果が1件以上あれば、総件数をセット
	if len(results) > 0 {
		// total_count は全行で同じ値なので、最初の行から取得
		total = results[0].TotalCount
	}

	// 取得した results から、model.Order の部分だけを orders スライスに詰め替える
	for _, res := range results {
		orders = append(orders, res.Order)
	}

	return orders, total, nil
}