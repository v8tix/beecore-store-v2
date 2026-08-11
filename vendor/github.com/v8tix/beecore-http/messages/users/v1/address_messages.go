package users

import (
	"time"

	"github.com/v8tix/beecore-http/validator"
)

type (
	Address struct {
		ID              string    `json:"id,omitempty"`
		UserID          string    `json:"user_id,omitempty"`
		Type            string    `json:"type,omitempty"`
		Country         string    `json:"country,omitempty"`
		State           string    `json:"state,omitempty"`
		City            string    `json:"city,omitempty"`
		PostalCode      string    `json:"postal_code,omitempty"`
		MainStreet      string    `json:"main_street,omitempty"`
		SecondaryStreet string    `json:"secondary_street,omitempty"`
		Numeration      string    `json:"numeration,omitempty"`
		Phone           string    `json:"phone,omitempty"`
		CreatedAt       time.Time `json:"created_at,omitempty"`
		UpdatedAt       time.Time `json:"updated_at,omitempty"`
	}
)

type (
	RegisterAddressV1Req struct {
		Type            string `json:"type,omitempty"`
		Country         string `json:"country,omitempty"`
		State           string `json:"state,omitempty"`
		City            string `json:"city,omitempty"`
		PostalCode      string `json:"postal_code,omitempty"`
		MainStreet      string `json:"main_street,omitempty"`
		SecondaryStreet string `json:"secondary_street,omitempty"`
		Numeration      string `json:"numeration,omitempty"`
		Phone           string `json:"phone,omitempty"`
	}

	RegisterAddressV1Res struct {
		ID string `json:"address_id,omitempty"`
	}

	RegisterAddressEnvV1Res struct {
		Address RegisterAddressV1Res `json:"address,omitempty"`
	}
	GetAddressesV1EnvV1Res struct {
		Addresses []Address `json:"addresses,omitempty"`
	}

	UpdateAddressV1Req struct {
		Type            string `json:"type,omitempty"`
		Country         string `json:"country,omitempty"`
		State           string `json:"state,omitempty"`
		City            string `json:"city,omitempty"`
		PostalCode      string `json:"postal_code,omitempty"`
		MainStreet      string `json:"main_street,omitempty"`
		SecondaryStreet string `json:"secondary_street,omitempty"`
		Numeration      string `json:"numeration,omitempty"`
		Phone           string `json:"phone,omitempty"`
	}
)

func (r RegisterAddressV1Req) Req() {}

func (r RegisterAddressV1Req) Validate(v *validator.Validator) {
	v.Check(len(r.Type) != 0, "type", "must be provided")
	v.Check(len(r.Country) != 0, "country", "must be provided")
	v.Check(len(r.State) != 0, "state", "must be provided")
	v.Check(len(r.City) != 0, "city", "must be provided")
	v.Check(len(r.PostalCode) != 0, "postal_code", "must be provided")
	v.Check(len(r.MainStreet) != 0, "main_street", "must be provided")
	v.Check(len(r.SecondaryStreet) != 0, "secondary_street", "must be provided")
	v.Check(len(r.Numeration) != 0, "numeration", "must be provided")
	v.Check(len(r.Phone) != 0, "phone", "must be provided")
}

func (r RegisterAddressEnvV1Res) Res() {}

func (g GetAddressesV1EnvV1Res) Res() {}

func (u UpdateAddressV1Req) Req() {}

func (u UpdateAddressV1Req) Validate(v *validator.Validator) {
	v.Check(len(u.Type) != 0, "type", "must be provided")
	v.Check(len(u.Country) != 0, "country", "must be provided")
	v.Check(len(u.State) != 0, "state", "must be provided")
	v.Check(len(u.City) != 0, "city", "must be provided")
	v.Check(len(u.PostalCode) != 0, "postal_code", "must be provided")
	v.Check(len(u.MainStreet) != 0, "main_street", "must be provided")
	v.Check(len(u.SecondaryStreet) != 0, "secondary_street", "must be provided")
	v.Check(len(u.Numeration) != 0, "numeration", "must be provided")
	v.Check(len(u.Phone) != 0, "phone", "must be provided")
}
