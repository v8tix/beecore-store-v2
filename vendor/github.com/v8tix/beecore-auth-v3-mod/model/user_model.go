package model

import "time"

type User struct {
	ID        string
	DNI       *string
	FirstName string
	LastName  string
	Email     string
	Birthday  *string
	Genre     *string
	Phone     *string
	ImgURL    *string
	Website   *string
	Enabled   bool
	Password  Password
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Role struct {
	ID   string
	Name string
}

type Permission struct {
	ID   string
	Name string
}

type Resource struct {
	ID   string
	Name string
}
