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

// Task 10 finding — no port/service.Store, port/resource.StoreRemote,
// core/store use-case, or infrastructure/http/storeremote adapter exist in
// this repo, despite the phase-2 plan listing them as that task's expected
// deliverables. Confirmed by exhaustive search, not assumption:
//
//   - internal/business/core/repository/base_repository.go's ~35-method
//     BaseRepository interface (the full downstream call surface of the
//     source repo) has no store-entity lookup/search/pagination method —
//     only FindProduct and FindPaginatedProducts, which hit the
//     beecore-stores microservice's *product* endpoints (they return
//     stores.GetProductV1Res, not stores.GetStoreV1Res).
//   - No stores_repository.go (or similarly named file) exists anywhere in
//     internal/business/core/repository.
//   - cmd/web/routes.go has zero routes under "/store" or "/stores"; no
//     cmd/web/store_handler.go exists.
//   - The only ".Store" field accesses anywhere in cmd/web/*.go are log
//     fields read off an already-embedded Basket/Order FinancialItem.Store
//     (cmd/web/payphone_handlers.go) — never a dedicated lookup call.
//   - stores.GetStoreV1Res/GetStoreEnvV1Res/CreateStoreV1Req do exist in
//     the shared beecore-http library (beecore-admin, a different app,
//     uses them to manage stores) but are never imported anywhere in this
//     source repo.
//
// This is the "don't fabricate a new route; document instead" situation
// the migration plan's rules anticipated, taken to its conclusion: there
// is no reachable downstream call to wrap, not even indirectly through an
// existing handler, so there is nothing for a Store port/use-case/adapter
// to do. Store's only real role in this repo is the plain value type
// above, embedded in Basket/FinancialLineItem/Order (see domain/basket.go,
// domain/order.go) and already fully usable via the Basket/Order slices
// once they're ported (Tasks 13-16) — no adapter of its own required.
