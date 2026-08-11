package model

import "github.com/v8tix/beecore-hash-mod"

type Password struct {
	PlainText *string
	Hash      string
}

func (p *Password) Set(plainTextPassword string) error {
	result, err := hash.Argon2idHash(plainTextPassword)
	if err != nil {
		return err
	}

	p.PlainText = &plainTextPassword
	p.Hash = result
	return nil
}

func (p *Password) Matches(plaintextPassword string) error {
	if err := hash.CheckHash(p.Hash, plaintextPassword); err != nil {
		return err
	}

	return nil
}
