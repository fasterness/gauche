package gauche

import (
	"bytes"
	"net/http"
)

type CachedResponseWriter struct {
	Buffer bytes.Buffer
	writer http.ResponseWriter
}

func (c *CachedResponseWriter) Header() http.Header {
	return c.writer.Header()
}
func (c *CachedResponseWriter) Write(b []byte) (int, error) {
	c.Buffer.Write(b)
	return len(c.Buffer.Bytes()), nil
}
func (c *CachedResponseWriter) WriteHeader(i int) {
	c.writer.WriteHeader(i)
}
