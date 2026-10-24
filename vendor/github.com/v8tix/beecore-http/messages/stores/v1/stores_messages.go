package stores

import (
	"time"

	"github.com/v8tix/beecore-http/validator"
)

type (
	// CreateStoreV1Req is the request body for creating a store.
	CreateStoreV1Req struct {
		Name        string `json:"name,omitempty"`
		UserID      string `json:"user_id,omitempty"`
		AddressID   string `json:"address_id,omitempty"`
		ImgURL      string `json:"img_url,omitempty"`
		Description string `json:"description,omitempty"`
		Website     string `json:"website,omitempty"`
	}

	// CreateStoreV1Res is the response body returned after creating a store.
	CreateStoreV1Res struct {
		StoreID string `json:"store_id,omitempty"`
	}

	// CreateStoreEnvV1Res envelopes CreateStoreV1Res.
	CreateStoreEnvV1Res struct {
		Store CreateStoreV1Res `json:"store,omitempty"`
	}

	// RebrandStoreV1Req is the request body for renaming a store.
	RebrandStoreV1Req struct {
		Name string `json:"name,omitempty"`
	}

	// GetStoreV1Res is the response body for a single store.
	GetStoreV1Res struct {
		ID            string    `json:"id,omitempty"`
		Name          string    `json:"name,omitempty"`
		Participating bool      `json:"participating,omitempty"`
		UserID        string    `json:"user_id,omitempty"`
		AddressID     string    `json:"address_id,omitempty"`
		ImgURL        string    `json:"img_url,omitempty"`
		Description   string    `json:"description,omitempty"`
		Website       string    `json:"website,omitempty"`
		CreatedAt     time.Time `json:"created_at,omitempty"`
		UpdatedAt     time.Time `json:"updated_at,omitempty"`
	}

	// GetStoreEnvV1Res envelopes GetStoreV1Res.
	GetStoreEnvV1Res struct {
		Store GetStoreV1Res `json:"store"`
	}
)

// Req marks CreateStoreV1Req as a request payload.
func (c CreateStoreV1Req) Req() {}

// Validate checks that CreateStoreV1Req has all required fields.
func (c CreateStoreV1Req) Validate(v *validator.Validator) {
	v.Check(len(c.UserID) != 0, "user_id", "must be provided")
	v.Check(len(c.AddressID) != 0, "address_id", "must be provided")
	v.Check(len(c.Name) != 0, "name", "must be provided")
	v.Check(len(c.Description) != 0, "description", "must be provided")
}

// Req marks RebrandStoreV1Req as a request payload.
func (r RebrandStoreV1Req) Req() {}

// Validate checks that RebrandStoreV1Req has all required fields.
func (r RebrandStoreV1Req) Validate(v *validator.Validator) {
	v.Check(len(r.Name) != 0, "name", "must be provided")
}

// Res marks CreateStoreEnvV1Res as a response payload.
func (c CreateStoreEnvV1Res) Res() {}

// Res marks GetStoreEnvV1Res as a response payload.
func (g GetStoreEnvV1Res) Res() {}
