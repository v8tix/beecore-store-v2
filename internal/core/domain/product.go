package domain

import "time"

// Product represents a store's product listing, as exchanged with the
// downstream beecore-stores service (see
// internal/business/core/repository/products_repository.go in the source
// repo, stores.GetProductV1Res).
type Product struct {
	ID                  string
	StoreID             string
	Name                string
	Description         string
	SKU                 string
	Price               float64
	Category            string
	ImgURL              string
	VideoURL            string
	Status              string
	Quantity            int
	Weight              float64
	Width               float64
	Height              float64
	Depth               float64
	DiscountPercentage  int
	DiscountDescription string
	Subtotal            float64
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// ProductPage is a page of a store's products together with the total
// match count, as returned by BaseRepositoryImpl.FindPaginatedProducts in
// products_repository.go in the source repo — it queries the Stores API's
// full-text search-count endpoint and search endpoint separately (no
// Link-header pagination, unlike beecore-admin-v2's product listing) and
// pairs the results into a single ([]stores.GetProductV1Res, int64)
// return.
type ProductPage struct {
	Products []Product
	Total    int64
}
