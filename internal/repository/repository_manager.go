package repository

import "oss.nandlabs.io/golly/managers"

const (
	RestaurantsRepo = "restaurants"
)

var Manager = managers.NewItemManager[any]()

func Get[T any](id string) (v T) {
	item := Manager.Get(id)
	if item != nil {
		v = item.(T)
		return
	}
	return
}
