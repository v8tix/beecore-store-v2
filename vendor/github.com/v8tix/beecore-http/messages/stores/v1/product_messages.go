// Package stores holds the HTTP wire messages (requests and responses) for the stores service.
package stores

import (
	"time"

	"github.com/v8tix/beecore-http/validator"
)

type (
	// AddProductV1Req is the request body for adding a product.
	AddProductV1Req struct {
		Name                string  `json:"name,omitempty"`
		Description         string  `json:"description,omitempty"`
		SKU                 string  `json:"sku,omitempty"`
		Price               float64 `json:"price,omitempty"`
		CategoryID          string  `json:"category,omitempty"`
		ImgURL              string  `json:"img_url,omitempty"`
		VideoURL            string  `json:"video_url,omitempty"`
		Status              string  `json:"status,omitempty"`
		Quantity            int     `json:"quantity,omitempty"`
		Weight              float64 `json:"weight,omitempty"`
		Width               float64 `json:"width,omitempty"`
		Height              float64 `json:"height,omitempty"`
		Depth               float64 `json:"depth,omitempty"`
		DiscountPercentage  int     `json:"discount_percentage,omitempty"`
		DiscountDescription string  `json:"discount_description,omitempty"`
	}

	// UpdateProductV1Req is the request body for updating a product.
	UpdateProductV1Req struct {
		StoreID             string  `json:"store_id,omitempty"`
		Name                string  `json:"name,omitempty"`
		Description         string  `json:"description,omitempty"`
		SKU                 string  `json:"sku,omitempty"`
		Price               float64 `json:"price,omitempty"`
		CategoryID          string  `json:"category,omitempty"`
		ImgURL              string  `json:"img_url,omitempty"`
		VideoURL            string  `json:"video_url,omitempty"`
		Status              string  `json:"status,omitempty"`
		Quantity            int     `json:"quantity,omitempty"`
		Weight              float64 `json:"weight,omitempty"`
		Width               float64 `json:"width,omitempty"`
		Height              float64 `json:"height,omitempty"`
		Depth               float64 `json:"depth,omitempty"`
		DiscountPercentage  int     `json:"discount_percentage,omitempty"`
		DiscountDescription string  `json:"discount_description,omitempty"`
	}

	// AddProductV1Res is the response body returned after adding a product.
	AddProductV1Res struct {
		ProductID string `json:"product_id,omitempty"`
	}

	// AddProductEnvV1Res envelopes AddProductV1Res.
	AddProductEnvV1Res struct {
		Product AddProductV1Res `json:"product,omitempty"`
	}

	// RebrandProductV1Req is the request body for renaming/redescribing a product.
	RebrandProductV1Req struct {
		Name        string `json:"name,omitempty"`
		Description string `json:"description,omitempty"`
	}

	// IncreaseProductPriceV1Req is the request body for increasing a product's price.
	IncreaseProductPriceV1Req struct {
		Price float64 `json:"price,omitempty"`
	}

	// DecreaseProductPriceV1Req is the request body for decreasing a product's price.
	DecreaseProductPriceV1Req struct {
		Price float64 `json:"price,omitempty"`
	}

	// GetProductV1Res is the response body for a single product.
	GetProductV1Res struct {
		ID                  string    `json:"id,omitempty"`
		StoreID             string    `json:"store_id,omitempty"`
		Name                string    `json:"name,omitempty"`
		Description         string    `json:"description,omitempty"`
		SKU                 string    `json:"sku,omitempty"`
		Price               float64   `json:"price,omitempty"`
		CategoryID          string    `json:"category,omitempty"`
		CategoryName        string    `json:"category_name,omitempty"`
		ImgURL              string    `json:"img_url,omitempty"`
		VideoURL            string    `json:"video_url,omitempty"`
		Status              string    `json:"status,omitempty"`
		Quantity            int       `json:"quantity,omitempty"`
		Weight              float64   `json:"weight,omitempty"`
		Width               float64   `json:"width,omitempty"`
		Height              float64   `json:"height,omitempty"`
		Depth               float64   `json:"depth,omitempty"`
		DiscountPercentage  int       `json:"discount_percentage,omitempty"`
		DiscountDescription string    `json:"discount_description,omitempty"`
		Subtotal            float64   `json:"subtotal,omitempty"`
		CreatedAt           time.Time `json:"created_at,omitempty"`
		UpdatedAt           time.Time `json:"updated_at,omitempty"`
	}

	// GetProductEnvV1Res envelopes GetProductV1Res.
	GetProductEnvV1Res struct {
		Product GetProductV1Res `json:"product"`
	}

	// GetProductsEnvV1Res envelopes a list of products.
	GetProductsEnvV1Res struct {
		Products []GetProductV1Res `json:"products"`
	}

	// GetProductsSearchCountEnvV1Res is the response body for a product search's result count.
	GetProductsSearchCountEnvV1Res struct {
		SearchTerm string `json:"search,omitempty"`
		Total      int    `json:"total,omitempty"`
	}
)

// Req marks AddProductV1Req as a request payload.
func (a AddProductV1Req) Req() {}

// Req marks UpdateProductV1Req as a request payload.
func (u UpdateProductV1Req) Req() {}

// Res marks AddProductV1Res as a response payload.
func (a AddProductV1Res) Res() {}

// Validate checks that AddProductV1Req has all required fields.
func (a AddProductV1Req) Validate(v *validator.Validator) {
	v.Check(len(a.Name) != 0, "name", "must be provided")
	v.Check(len(a.Description) != 0, "description", "must be provided")
	v.Check(a.Price > 0, "price", "price must be greater than zero")
	v.Check(len(a.ImgURL) != 0, "img_url", "must be provided")
	v.Check(len(a.CategoryID) != 0, "category", "must be provided")
	v.Check(len(a.Status) != 0, "status", "must be provided")
	v.Check(a.Quantity >= 0, "quantity", "quantity must be greater than or equal to zero")

	if a.DiscountPercentage > 0 {
		v.Check(len(a.DiscountDescription) != 0, "discount_description", "must be provided")
	}
}

// Validate checks that UpdateProductV1Req has all required fields.
func (u UpdateProductV1Req) Validate(v *validator.Validator) {
	v.Check(len(u.Name) != 0, "name", "must be provided")
	v.Check(len(u.Description) != 0, "description", "must be provided")
	v.Check(u.Price > 0, "price", "price must be greater than zero")
	v.Check(len(u.ImgURL) != 0, "img_url", "must be provided")
	v.Check(len(u.CategoryID) != 0, "category", "must be provided")
	v.Check(len(u.Status) != 0, "status", "must be provided")
	v.Check(u.Quantity >= 0, "quantity", "quantity must be greater than or equal to zero")

	if u.DiscountPercentage > 0 {
		v.Check(len(u.DiscountDescription) != 0, "discount_description", "must be provided")
	}
}

// Req marks RebrandProductV1Req as a request payload.
func (r RebrandProductV1Req) Req() {}

// Validate checks that RebrandProductV1Req has all required fields.
func (r RebrandProductV1Req) Validate(v *validator.Validator) {
	v.Check(r.Name != "", "name", "must be provided")
	v.Check(r.Description != "", "description", "must be provided")
}

// Req marks IncreaseProductPriceV1Req as a request payload.
func (i IncreaseProductPriceV1Req) Req() {}

// Validate checks that IncreaseProductPriceV1Req has all required fields.
func (i IncreaseProductPriceV1Req) Validate(v *validator.Validator) {
	v.Check(i.Price > 0, "price", "price must be greater than zero")
}

// Req marks DecreaseProductPriceV1Req as a request payload.
func (d DecreaseProductPriceV1Req) Req() {}

// Validate checks that DecreaseProductPriceV1Req has all required fields.
func (d DecreaseProductPriceV1Req) Validate(v *validator.Validator) {
	v.Check(d.Price > 0, "price", "price must be greater than zero")
}

// Res marks GetProductEnvV1Res as a response payload.
func (g GetProductEnvV1Res) Res() {}

// Res marks AddProductEnvV1Res as a response payload.
func (a AddProductEnvV1Res) Res() {}

// Res marks GetProductsEnvV1Res as a response payload.
func (g GetProductsEnvV1Res) Res() {}

// Res marks GetProductsSearchCountEnvV1Res as a response payload.
func (g GetProductsSearchCountEnvV1Res) Res() {}
