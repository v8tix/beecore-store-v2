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

// CreateOrderRequest is the input to resource.OrderRemote.CreateOrder — one
// store's order, as built by the source repo's dto.toCreateOrderV1Req
// (internal/business/core/dto/order_dto.go) from a single
// common.FinancialItemV1 entry of a checked-out basket's financial
// breakdown (orders.CreateOrderV1Req). The source repo's
// cmd/web/payphone_handlers.go PayPhoneConfirm handler builds one of these
// per store (dto.ToCreateOrdersV1Req) and calls CreateOrder once per
// store — that orchestration is a Payment-slice concern (plan Task
// 17/18), not reproduced here; this type only carries the shape CreateOrder
// needs.
type CreateOrderRequest struct {
	UserID            string
	PaymentID         string
	BasketID          string
	ShippingAddressID string
	FinancialItem     FinancialLineItem
}
