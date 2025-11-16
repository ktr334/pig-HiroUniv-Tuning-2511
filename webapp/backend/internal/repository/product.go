package repository

import (
	"backend/internal/model"
	"context"
)

type ProductRepository struct {
	db DBTX
}

func NewProductRepository(db DBTX) *ProductRepository {
	return &ProductRepository{db: db}
}

// SQL でページングして高速化したバージョン
func (r *ProductRepository) ListProducts(ctx context.Context, userID int, req model.ListRequest) ([]model.Product, int, error) {

	// ---- 1. 件数取得（COUNT） ----
	countQuery := `
		SELECT COUNT(*)
		FROM products
	`
	countArgs := []interface{}{}
	searchClause := ""

	if req.Search != "" {
		searchClause = " WHERE (name LIKE ? OR description LIKE ?)"
		searchPattern := "%" + req.Search + "%"
		countArgs = append(countArgs, searchPattern, searchPattern)
	}

	var total int
	if err := r.db.GetContext(ctx, &total, countQuery+searchClause, countArgs...); err != nil {
		return nil, 0, err
	}

	// ---- 2. ページング付きデータ取得 ----
	baseQuery := `
        SELECT product_id, name, value, weight, image, description
        FROM products
    `
	args := []interface{}{}

	if req.Search != "" {
		baseQuery += " WHERE (name LIKE ? OR description LIKE ?)"
		searchPattern := "%" + req.Search + "%"
		args = append(args, searchPattern, searchPattern)
	}

	baseQuery += " ORDER BY " + req.SortField + " " + req.SortOrder + " , product_id ASC"
	baseQuery += " LIMIT ? OFFSET ?"

	args = append(args, req.PageSize, req.Offset)

	var pagedProducts []model.Product
	if err := r.db.SelectContext(ctx, &pagedProducts, baseQuery, args...); err != nil {
		return nil, 0, err
	}

	// ---- 3. ページ分のデータと total を返す ----
	return pagedProducts, total, nil
}
