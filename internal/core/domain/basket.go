package domain

// BasketProduct is the product-with-quantity snapshot a basket line item
// or financial line item embeds (beecore-http's common.ProductV1) —
// narrower than the full Product entity (no SKU/category/dimensions/etc.),
// since a basket/order only ever carries the fields it needs to bill and
// display a line, not the full catalog record. See
// internal/business/core/repository/baskets_repository.go in the source
// repo (baskets.GetBasketItemV1Res, common.FinancialItemV1).
type BasketProduct struct {
	ID          string
	Name        string
	Description string
	Img         string
	Price       float64
	Subtotal    float64
	Quantity    int
}

// BasketItem is a single line item in a Basket — the store it belongs to
// and the product/quantity for that line, as returned by the downstream
// beecore-baskets service (see
// internal/business/core/repository/baskets_repository.go in the source
// repo, baskets.GetBasketItemV1Res).
type BasketItem struct {
	Store   Store
	Product BasketProduct
}

// Basket represents a shopping basket/cart, as exchanged with the
// downstream beecore-baskets service (see
// internal/business/core/repository/baskets_repository.go in the source
// repo, baskets.GetBasketV1Res / GetBasketByUserIDEnvV1Res).
type Basket struct {
	ID        string
	UserID    string
	PaymentID string
	Items     []BasketItem
	Status    string
}

// FinancialLineItem is one store's grouped products/charges within a
// Basket's or Order's financial breakdown (beecore-http's
// common.FinancialItemV1).
type FinancialLineItem struct {
	Store          Store
	Products       []BasketProduct
	ShippingAmount float64
	Taxes          string
	TaxesAmount    float64
	Subtotal       float64
	Total          float64
}

// Financials is the computed cost breakdown for a Basket, as returned by
// beecore-baskets' /baskets/{id}/financials endpoint (see
// BaseRepositoryImpl.ComputeFinancials in baskets_repository.go in the
// source repo, baskets.GetFinancialItemsV1Res).
type Financials struct {
	Items               []FinancialLineItem
	Subtotal            float64
	TotalShippingAmount float64
	TotalTaxesAmount    float64
	Total               float64
}
