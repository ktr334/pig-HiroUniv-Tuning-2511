// SQLのLIMITとOFFSET句を使うことで、必要な分だけのデータをデータベースから取得するように変更
package repository

import (
	"backend/internal/model"
	"context"
	"fmt"
)

type ProductRepository struct {
	db DBTX
}

func NewProductRepository(db DBTX) *ProductRepository {
	return &ProductRepository{db: db}
}

// 商品一覧を取得（ページネーション対応）
func (r *ProductRepository) ListProducts(ctx context.Context, userID int, req model.ListRequest) ([]model.Product, int, error) {
	// ベースクエリ
	baseQuery := "FROM products"
	whereClause := ""
	args := []interface{}{}
	countArgs := []interface{}{}

	// 検索条件
	if req.Search != "" {
		whereClause = " WHERE (name LIKE ? OR description LIKE ?)"
		searchPattern := "%" + req.Search + "%"
		args = append(args, searchPattern, searchPattern)
		countArgs = append(countArgs, searchPattern, searchPattern)
	}

	// 総件数を取得
	countQuery := "SELECT COUNT(*) " + baseQuery + whereClause
	var total int
	if err := r.db.GetContext(ctx, &total, countQuery, countArgs...); err != nil {
		return nil, 0, err
	}

	// データ取得クエリ
	sortClause := fmt.Sprintf("ORDER BY %s %s, product_id ASC", req.SortField, req.SortOrder)
	limitClause := "LIMIT ?"
	offsetClause := "OFFSET ?"
	args = append(args, req.PageSize, req.Offset)

	query := "SELECT product_id, name, value, weight, image, description " +
		baseQuery + " " +
		whereClause + " " +
		sortClause + " " +
		limitClause + " " +
		offsetClause

	var products []model.Product
	err := r.db.SelectContext(ctx, &products, query, args...)
	if err != nil {
		return nil, 0, err
	}

	return products, total, nil
}
