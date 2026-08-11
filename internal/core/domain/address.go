package domain

import "time"

// AddressType enumerates the kinds of address a User can register, as
// defined in internal/business/core/repository/address_repository.go in
// the source repo.
type AddressType string

const (
	PersonalAddress AddressType = "Personal"
	StoreAddress    AddressType = "Store"
	ShippingAddress AddressType = "Shipping"
)

// Address represents a physical address belonging to a User (personal,
// store, or shipping), as exchanged with the downstream beecore-customers
// user service (see
// internal/business/core/repository/address_repository.go in the source
// repo).
type Address struct {
	ID              string
	UserID          string
	Type            AddressType
	Country         string
	State           string
	City            string
	PostalCode      string
	MainStreet      string
	SecondaryStreet string
	Numeration      string
	Phone           string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
