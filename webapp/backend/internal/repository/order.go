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

// 複数の注文を一度にデータベースへ挿入する
func (r *OrderRepository) BulkCreate(ctx context.Context, orders []*model.Order) error {
	if len(orders) == 0 {
		return nil
	}

	// VALUES (?, ?, 'shipping', NOW()), (?, ?, 'shipping', NOW()), ... の部分を動的に生成
	valueStrings := make([]string, 0, len(orders))
	valueArgs := make([]interface{}, 0, len(orders)*2)
	for _, o := range orders {
		valueStrings = append(valueStrings, "(?, ?, 'shipping', NOW())")
		valueArgs = append(valueArgs, o.UserID, o.ProductID)
	}

	query := fmt.Sprintf("INSERT INTO orders (user_id, product_id, shipped_status, created_at) VALUES %s",
		strings.Join(valueStrings, ","))

	_, err := r.db.ExecContext(ctx, query, valueArgs...)
	return err
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

	// 総件数を取得（ページング用）
	// 検索がない場合はordersのみでカウントし、不要なJOINを避けて高速化する
	var total int
	if req.Search == "" {
		countQuery := "SELECT COUNT(*) FROM orders WHERE user_id = ?"
		if err := r.db.GetContext(ctx, &total, countQuery, userID); err != nil {
			return nil, 0, err
		}
	} else {
		countQuery := "SELECT COUNT(*) " + baseQuery + " " + whereClause
		countArgs := append([]interface{}{}, args...)
		if err := r.db.GetContext(ctx, &total, countQuery, countArgs...); err != nil {
			return nil, 0, err
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

	// データを取得するクエリ
	dataQuery := `
		SELECT
			o.order_id,
			o.product_id,
			p.name as product_name,
			o.shipped_status,
			o.created_at,
			o.arrived_at
		` + baseQuery + " " + whereClause + " " + orderByClause + " " + limitClause + " " + offsetClause

	var orders []model.Order
	if err := r.db.SelectContext(ctx, &orders, dataQuery, args...); err != nil {
		return nil, 0, err
	}

	return orders, total, nil
}
