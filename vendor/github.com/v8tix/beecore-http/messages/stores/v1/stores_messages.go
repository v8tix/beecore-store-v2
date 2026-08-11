package stores

import (
	"time"

	"github.com/v8tix/beecore-http/validator"
)

type (
	CreateStoreV1Req struct {
		Name        string `json:"name,omitempty"`
		UserID      string `json:"user_id,omitempty"`
		AddressID   string `json:"address_id,omitempty"`
		ImgURL      string `json:"img_url,omitempty"`
		Description string `json:"description,omitempty"`
		Website     string `json:"website,omitempty"`
	}

	CreateStoreV1Res struct {
		StoreID string `json:"store_id,omitempty"`
	}

	CreateStoreEnvV1Res struct {
		Store CreateStoreV1Res `json:"store,omitempty"`
	}

	RebrandStoreV1Req struct {
		Name string `json:"name,omitempty"`
	}

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

	GetStoreEnvV1Res struct {
		Store GetStoreV1Res `json:"store"`
	}
)

func (c CreateStoreV1Req) Req() {}

func (c CreateStoreV1Req) Validate(v *validator.Validator) {
	v.Check(len(c.UserID) != 0, "user_id", "must be provided")
	v.Check(len(c.AddressID) != 0, "address_id", "must be provided")
	v.Check(len(c.Name) != 0, "name", "must be provided")
	v.Check(len(c.Description) != 0, "description", "must be provided")
}

func (r RebrandStoreV1Req) Req() {}

func (r RebrandStoreV1Req) Validate(v *validator.Validator) {
	v.Check(len(r.Name) != 0, "name", "must be provided")
}

func (c CreateStoreEnvV1Res) Res() {}

func (g GetStoreEnvV1Res) Res() {}
