package gauche

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

type CacheHandler struct {
	Handler http.Handler
	Store   CacheStore
}

func New(handler http.Handler) *CacheHandler {
	store := MemStorage{}
	return &CacheHandler{Handler: handler, Store: store}
}
func (c *CacheHandler) ExamineHeaders(headers map[string][]string) {
	for k, h := range headers {
		log.Printf("%s:\n", k)
		for _, v := range h {
			log.Printf("\t%s", v)
		}

	}

}

func (c *CacheHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	crw := &CachedResponseWriter{writer: w}

	//examine request headers
	log.Printf("REQUEST")
	c.ExamineHeaders(req.Header)
	ci := NewCacheItemFromRequest(req)
	log.Printf("%s:\n%v", "KEY", string(ci.Hash[:]))
	//retrieve cached response if desired, allowed, and available
	ci_cache := c.Store.GetItem(ci.Hash)
	if ci_cache != nil {
		log.Println("Cache hit!")
		//caculate freshness and appropriateness of cached response
		//TODO: Add methods to CachedResponseWriter to override response headers
		if crw.Header().Get("Expires") == "" {
			crw.Header().Add("Expires", ci_cache.Expires.Format(EXPIRES_FORMAT))
		}
		if crw.Header().Get("Last-Modified") == "" {
			crw.Header().Add("Last-Modified", ci_cache.LastModified.Format(EXPIRES_FORMAT))
		}
		crw.Header().Add("Cache-Control", fmt.Sprintf("max-age=%d", ci_cache.MaxAge()))
		c.ExamineHeaders(crw.Header())
		crw.writer.Write(ci_cache.Content)
		return
	}
	log.Println("Cache miss")
	//retrieve response from upstream
	c.Handler.ServeHTTP(crw, req)
	if crw.Header().Get("Expires") == "" {
		crw.Header().Add("Expires", time.Now().Add(time.Hour).Format(EXPIRES_FORMAT))
	}
	if crw.Header().Get("Last-Modified") == "" {
		crw.Header().Add("Last-Modified", time.Now().Format(EXPIRES_FORMAT))
	}
	go func() {
		//examine response headers from upstream
		log.Printf("RESPONSE")

		// c.ExamineHeaders(crw.Header())
		//Cache the response if allowed
		ci_response := NewCacheItemFromResponse(req.URL, crw)

		log.Printf("Cache Key:\t%s", ci_response.CreateKey([]byte(req.URL.String())))
		log.Printf("%s:\t%v", "Private?", ci_response.IsPrivate())
		log.Printf("%s:\t%v", "No Cache?", ci_response.IsNoCache())
		log.Printf("%s:\t%v", "No Store?", ci_response.IsNoStore())
		log.Printf("%s:\t%v", "Expires", ci_response.Expires)
		if err := c.Store.StoreItem(ci_response); err != nil {
			log.Printf("ERROR: %v", err)
		}

	}()
	log.Printf("%s", crw.Buffer.Bytes())
	crw.writer.Write(crw.Buffer.Bytes())
}
