package gauche

import (
	"errors"
	"fmt"
	"log"
	"reflect"
	// "bytes"
	// "crypto/md5"
)

const ()

var (
	EMPTY_HASH16_ARRAY *[16]byte               = new([16]byte)
	MEMORY_STORE       map[[16]byte]*CacheItem = make(map[[16]byte]*CacheItem)
)

type MemStorage struct {
}

func (m MemStorage) IsValidHash(hash [16]byte) bool {
	return string(hash[:]) != string((*EMPTY_HASH16_ARRAY)[:])
}
func IsZero(thing reflect.Value) bool {
	return reflect.Zero(reflect.TypeOf(thing)).Interface() == thing.Interface()
}
func (m MemStorage) StoreItem(c *CacheItem) error {
	if !c.IsCacheable() {
		return errors.New(fmt.Sprintf("CacheItem is not cacheable. Private: %v No-Cache: %v No-Store: %v Expired: %v", c.IsPrivate(), c.IsNoCache(), c.IsNoStore(), c.IsExpired()))
	}
	if m.IsValidHash(c.Hash) {
		go func() {
			log.Printf("Storing item: %s", c.Hash)

			MEMORY_STORE[c.Hash] = c
		}()
	} else {
		return errors.New("CacheItem does not have a valid hash")
	}
	return nil
}
func (m MemStorage) GetItem(key [16]byte) *CacheItem {
	return MEMORY_STORE[key]
}
func (m MemStorage) HasItem(key [16]byte) bool { return MEMORY_STORE[key] != nil }
func (m MemStorage) PurgeItem(key [16]byte) {
	MEMORY_STORE[key] = nil
	delete(MEMORY_STORE, key)
}
func (m MemStorage) Purge() error {
	MEMORY_STORE = make(map[[16]byte]*CacheItem)
	return nil
}
