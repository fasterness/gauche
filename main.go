package main

import (
	"bytes"
	"crypto/md5"
	"flag"
	"fmt"
	"github.com/fasterness/httpipe"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

//Store cached responses in a map. This is not optimal, but it's a start
var cacheTable map[[16]byte]*CacheItem

type CacheItem struct {
	Response *http.Response
	Body     []byte
}

func (c *CacheItem) Expired() int {
	t := GetExpires(&c.Response.Header)
	return int(time.Now().Unix() - t.Unix())
}

//decide whether or not to consider query params as part of the signature.
//If you have a dynamic app, such that the params change the response,
//then definitely consider them in the signature.
//If you're serving static content, ignore the params can help prevent
//cache-busting and other misuse
var ignoreQueryParams bool = false

type CacheReadWriter struct {
	buffer *bytes.Buffer
}

func (c *CacheReadWriter) Write(p []byte) (n int, err error) {
	return c.buffer.Write(p)
}
func (c CacheReadWriter) Read(p []byte) (n int, err error) {
	return c.buffer.Read(p)
}
func sumBytes(b ...[16]byte) [16]byte {
	var flat []byte
	for _, v := range b {
		flat = append(flat, v[:]...)
	}
	return md5.Sum(flat)
}
func GetBodyBuffer(body io.ReadCloser, buf *bytes.Buffer) io.ReadCloser {
	crw := CacheReadWriter{buffer: buf}
	io.Copy(&crw, body)
	body.Close()
	return ioutil.NopCloser(crw)

}
func SignatureFromRequest(req *http.Request, ignoreQueryParams bool) [16]byte {
	//We will collect request headers and attributes to create a request signature
	var signature []byte
	//add the path to the signature
	signature = append(signature, []byte(req.URL.Path)...)
	if !ignoreQueryParams {
		//add the query, if you so desire
		signature = append(signature, []byte(req.URL.RawQuery)...)
	}
	if req.Method == "POST" {
		//add the request body. If the action is Idempotent
		//a post of the same data to the same path should
		//produce the same result
		buf := bytes.Buffer{}
		req.Body = GetBodyBuffer(req.Body, &buf)
		signature = append(signature, buf.Bytes()...)
	}
	//hash the cumulitive signature for a consistent key size
	return md5.Sum(signature)
}
func CacheControlContains(h *http.Header, directive ...string) bool {
	ccHeader := h.Get("Cache-Control")
	for _, d := range directive {
		if strings.Contains(ccHeader, d) {
			return true
		}
	}
	return false
}
func GetMaxAge(h *http.Header) int {
	var maxage int
	ccHeader := h.Get("Cache-Control")
	if ccHeader != "" {

		fmt.Sscanf(ccHeader, "maxage=%d", maxage)
	}
	if maxage < 0 {
		return 0
	}
	return maxage
}
func GetExpires(h *http.Header) time.Time {
	var expires time.Time
	header := strings.Replace(h.Get("Expires"), "GMT", "UTC", -1)
	if header != "" {

		expires, _ = time.Parse(time.RFC1123, header)
	}
	if expires.IsZero() {
		return time.Now()
	} else {
		return expires
	}
}
func CacheRequestHandler(req *http.Request, ctx *httpipe.Context) (*http.Request, *http.Response) {
	//
	// The Rules are as follows:
	// 1. The request is a GET or POST
	if req.Method != "GET" && req.Method != "POST" {
		//allow other methods to pass, at least for now
		return req, nil
	}
	signature := SignatureFromRequest(req, ignoreQueryParams)
	//If we have the signature in our cache, return the cached response.
	//Otherwise, return a nil response and let the request continue upstream
	item := cacheTable[signature]
	//2. We have a cached representation of the response
	if item != nil {
		//3. Test for freshness
		if item.Expired() <= GetMaxAge(&req.Header) {
			//If it's been expired for less than the maximum requested by the client, return it
			return req, httpipe.NewResponse(req, &item.Response.Header, item.Response.StatusCode, item.Body)
		}
	}
	return req, nil
}
func CacheResponseHandler(resp *http.Response, ctx *httpipe.Context) *http.Response {
	if resp == nil {
		return resp
	}
	signature := SignatureFromRequest(ctx.Request, ignoreQueryParams)
	if !CacheControlContains(&resp.Header, "no-store", "no-cache") {
		c := &CacheItem{Response: resp}
		buf := new(bytes.Buffer)
		resp.Body = GetBodyBuffer(resp.Body, buf)
		c.Body = buf.Bytes()
		cacheTable[signature] = c
	}
	return resp
}

var (
	Upstream string
	Bind     string
	Port     int
)

func init() {
	flag.StringVar(&Upstream, "upstream", "http://localhost", "The upstream instance to serve from")
	flag.StringVar(&Bind, "bind", "127.0.0.1", "Address to bind to")
	flag.IntVar(&Port, "port", 31337, "The port to listen on")
	flag.Parse()
	cacheTable = make(map[[16]byte]*CacheItem)
}
func main() {

	server := httpipe.New(Upstream)
	server.AddRequestHandler(httpipe.RequestWrapper(CacheRequestHandler))
	server.AddResponseHandler(httpipe.ResponseWrapper(CacheResponseHandler))
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", Bind, Port), server))
}
