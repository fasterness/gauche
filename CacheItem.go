package gauche

import (
	"bytes"
	"crypto/md5"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	//Mon Jan 2 15:04:05 -0700 MST 2006
	EXPIRES_FORMAT         string = "Mon, 02 Jan 2006 15:04:05 MST"
	HEADER_PREFIX          string = "X-Gauche-"
	HEADER_OVERRIDE_PREFIX string = HEADER_PREFIX + "Override-"
)

var (
	CACHE_CONTROL_DIRECTIVES = []string{
		"max-age",      //TTL of cached response in seconds
		"no-cache",     //do not cache response
		"no-store",     //do not cache any part of the request or response
		"no-transform", //serve the content as-is.
		"cache-extension",
	}
	CACHE_REQUEST_DIRECTIVES = append(CACHE_CONTROL_DIRECTIVES, []string{
		"max-stale",
		"min-fresh",
		"only-if-cached",
	}...)
	CACHE_RESPONSE_DIRECTIVES = append(CACHE_CONTROL_DIRECTIVES, []string{
		"public",           //generally cacheable
		"private",          //user-specific and should not be cached
		"s-maxage",         //TTL for shared cache in seconds
		"must-revalidate",  //revalidate cache and do not serve stale content
		"proxy-revalidate", //revalidate shared cache and do not serve stale content
	}...)
	HEADERS = []string{
		"Cache-Control",
		"Etag",
		"Pragma",
		"Vary",
		"Expires",
		"Last-Modified",
	}
)

type CacheItem struct {
	Url          *url.URL
	Method       string
	Hash         [16]byte
	Content      []byte
	Headers      http.Header
	Expires      time.Time
	LastModified time.Time
	Etag         []byte
	Vary         []string
}

func (ci *CacheItem) SetDefaults() {
	if ci.LastModified.IsZero() {
		ci.LastModified = time.Now()
	}
	if ci.Expires.IsZero() {
		ci.Expires = time.Now().Add(time.Hour * 24)
	}
	if len(ci.Etag) == 16 {
		copy(ci.Etag[:15], ci.Hash[:])
	} else {
		ci.Hash = ci.CreateKeyFromConfig()
	}

}
func NewCacheItemFromRequest(req *http.Request) *CacheItem {
	ci := new(CacheItem)
	ci.Url = req.URL
	ci.Headers = req.Header
	ci.Etag = []byte(req.Header.Get("Etag"))
	ci.Vary = strings.Split(req.Header.Get("Etag"), ",")
	ci.Expires, _ = time.Parse(req.Header.Get("Expires"), EXPIRES_FORMAT)
	ci.LastModified, _ = time.Parse(req.Header.Get("Last-Modified"), EXPIRES_FORMAT)
	ci.SetDefaults()
	return ci
}
func NewCacheItemFromResponse(u *url.URL, r *CachedResponseWriter) *CacheItem {
	ci := new(CacheItem)
	ci.Url = u
	ci.Headers = r.writer.Header()
	ci.Content = r.Buffer.Bytes()
	ci.Etag = []byte(ci.Headers.Get("Etag"))
	ci.Vary = strings.Split(ci.Headers.Get("Etag"), ",")
	ci.Expires, _ = time.Parse(ci.Headers.Get("Expires"), EXPIRES_FORMAT)
	ci.LastModified, _ = time.Parse(ci.Headers.Get("Last-Modified"), EXPIRES_FORMAT)
	ci.SetDefaults()
	return ci
}
func (ci *CacheItem) CreateKeyFromConfig() [16]byte {
	var data [][]byte
	data = append(data, []byte(ci.Url.Path))
	for _, v := range ci.Vary {
		//TODO: set flags to ignore some or all vary items
		data = append(data, []byte(ci.Headers.Get(v)))
	}
	return ci.CreateKey(data...)
}
func (c *CacheItem) CreateKey(data ...[]byte) [16]byte {
	var hash bytes.Buffer
	for _, v := range data {
		hash.Write(v)
	}
	h := md5.Sum(hash.Bytes())
	return h
}
func (c *CacheItem) MaxAge() int {
	return int(c.Expires.Sub(time.Now()) / time.Second)
}
func (c *CacheItem) IsCacheable() bool {
	return !(c.IsExpired() || c.IsPrivate() || c.IsNoCache() || c.IsNoStore())
}
func (c *CacheItem) IsExpired() bool {
	return !c.Expires.IsZero() && c.Expires.Before(time.Now())
}
func (c *CacheItem) cacheControlDirectiveMatches(rx string) bool {
	// var k string
	var v []string
	v = c.Headers["Cache-Control"]
	for _, va := range v {
		b, err := regexp.MatchString(rx, string(va))
		if err == nil && b {
			return true
		}
	}
	return false
}
func (c *CacheItem) IsPrivate() bool {
	return c.cacheControlDirectiveMatches("private")
}
func (c *CacheItem) IsNoCache() bool {
	return c.cacheControlDirectiveMatches("no-cache")
}
func (c *CacheItem) IsNoStore() bool {
	return c.cacheControlDirectiveMatches("no-store")
}
