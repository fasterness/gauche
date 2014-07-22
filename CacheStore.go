package gauche

type CacheStore interface {
	StoreItem(c *CacheItem) error
	GetItem(key [16]byte) *CacheItem
	HasItem(key [16]byte) bool
	PurgeItem(key [16]byte)
	Purge() error
}
