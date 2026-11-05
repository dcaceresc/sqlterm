// Package credentials stores passwords outside connection profile files.
package credentials

import "github.com/zalando/go-keyring"

var ErrNotFound = keyring.ErrNotFound

type Store interface {
	Get(string) (string, error)
	Set(string, string) error
	Delete(string) error
}
type System struct{}

func (System) Get(id string) (string, error) { return keyring.Get("sqlterm", id) }
func (System) Set(id, password string) error { return keyring.Set("sqlterm", id, password) }
func (System) Delete(id string) error        { return keyring.Delete("sqlterm", id) }
