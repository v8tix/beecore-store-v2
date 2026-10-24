// Package common holds wire-message DTOs shared across multiple microservices' HTTP APIs.
package common

type (
	// ShippingAddressV1 is a shipping address as carried on the wire.
	ShippingAddressV1 struct {
		Country         string `json:"country,omitempty"`
		State           string `json:"state,omitempty"`
		City            string `json:"city,omitempty"`
		PostalCode      string `json:"postal_code,omitempty"`
		MainStreet      string `json:"main_street,omitempty"`
		SecondaryStreet string `json:"secondary_street,omitempty"`
		Numeration      string `json:"numeration,omitempty"`
		Phone           string `json:"phone,omitempty"`
	}
	// StoreV1 is a store snapshot as carried on the wire.
	StoreV1 struct {
		ID    string `json:"id,omitempty"`
		Owner string `json:"owner,omitempty"`
		Name  string `json:"name,omitempty"`
		Email string `json:"email,omitempty"`
	}

	// ProductV1 is a product snapshot as carried on the wire.
	ProductV1 struct {
		ID          string  `json:"id,omitempty"`
		Name        string  `json:"name,omitempty"`
		Description string  `json:"description,omitempty"`
		Img         string  `json:"img,omitempty"`
		Price       float64 `json:"price,omitempty"`
		Subtotal    float64 `json:"subtotal,omitempty"`
		Quantity    int     `json:"quantity,omitempty"`
	}

	// FinancialItemV1 is a store's financial breakdown within an order, as carried on the wire.
	FinancialItemV1 struct {
		Store          StoreV1     `json:"store,omitempty"`
		Products       []ProductV1 `json:"products,omitempty"`
		ShippingAmount float64     `json:"shipping_amount,omitempty"`
		Taxes          string      `json:"taxes,omitempty"`
		TaxesAmount    float64     `json:"taxes_amount,omitempty"`
		Subtotal       float64     `json:"subtotal,omitempty"`
		Total          float64     `json:"total,omitempty"`
	}
)
