package domain

// Store represents the merchant store a basket/order line item belongs
// to, as embedded in the downstream beecore-baskets and beecore-orders
// wire responses (beecore-http's common.StoreV1 — see
// internal/business/core/repository/baskets_repository.go and
// orders_repository.go in the source repo). Unlike beecore-admin-v2's
// Store entity, beecore-store never calls a dedicated store-lookup
// endpoint directly — every Store value it handles arrives embedded
// inside a basket item or a financial line item, so this mirrors that
// narrower wire shape rather than the fuller CRUD-oriented
// stores.GetStoreV1Res (id/owner/name/email vs.
// id/name/participating/user_id/address_id/img_url/description/website).
type Store struct {
	ID    string
	Owner string
	Name  string
	Email string
}
