package domain

import "time"

// OrderShippingAddress is the destination address recorded on an Order,
// as exchanged with the downstream beecore-orders service (beecore-http's
// common.ShippingAddressV1 — see
// internal/business/core/repository/orders_repository.go in the source
// repo). Named distinctly from AddressType's ShippingAddress constant
// (address.go) — that enumerates a User's registered address kind, this
// is the actual address payload embedded in an Order.
type OrderShippingAddress struct {
	Country         string
	State           string
	City            string
	PostalCode      string
	MainStreet      string
	SecondaryStreet string
	Numeration      string
	Phone           string
}

// Order represents a placed order, as exchanged with the downstream
// beecore-orders service (see
// internal/business/core/repository/orders_repository.go in the source
// repo, orders.GetOrderV1Res / CreateOrderV1Req).
type Order struct {
	ID              string
	UserID          string
	PaymentID       string
	BasketID        string
	Status          string
	ShippingAddress OrderShippingAddress
	FinancialItem   FinancialLineItem
	UpdatedAt       time.Time
}
