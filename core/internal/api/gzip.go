package api

import (
	"compress/gzip"
	"net/http"
	"strings"
	"sync"
)

// gzipTypes son los tipos de contenido que se comprimen (texto: la interfaz, sus estilos y las respuestas JSON).
var gzipTypes = []string{"text/html", "text/css", "text/javascript", "application/javascript", "application/json", "image/svg+xml", "text/plain"}

// gzipPool reutiliza los compresores (crear uno cuesta más que comprimir una respuesta pequeña).
var gzipPool = sync.Pool{New: func() any {
	writer, _ := gzip.NewWriterLevel(nil, gzip.BestSpeed)
	return writer
}}

// gzipWriter comprime la respuesta si resulta ser de un tipo de texto.
type gzipWriter struct {
	http.ResponseWriter
	writer  *gzip.Writer
	decided bool
}

// compressible indica si un tipo de contenido se comprime.
func compressible(contentType string) bool {
	for _, kind := range gzipTypes {
		if strings.HasPrefix(contentType, kind) {
			return true
		}
	}
	return false
}

// * WriteHeader decide si se comprime según el tipo que declaró el manejador (y nunca con una respuesta ya comprimida).
func (g *gzipWriter) WriteHeader(status int) {
	if !g.decided {
		g.decided = true
		header := g.Header()
		if status == http.StatusOK && header.Get("Content-Encoding") == "" && compressible(header.Get("Content-Type")) {
			header.Del("Content-Length")
			header.Set("Content-Encoding", "gzip")
			header.Add("Vary", "Accept-Encoding")
			g.writer = gzipPool.Get().(*gzip.Writer)
			g.writer.Reset(g.ResponseWriter)
		}
	}
	g.ResponseWriter.WriteHeader(status)
}

// * Write escribe los datos, comprimidos si toca.
func (g *gzipWriter) Write(data []byte) (int, error) {
	if !g.decided {
		if g.Header().Get("Content-Type") == "" {
			g.Header().Set("Content-Type", http.DetectContentType(data))
		}
		g.WriteHeader(http.StatusOK)
	}
	if g.writer != nil {
		return g.writer.Write(data)
	}
	return g.ResponseWriter.Write(data)
}

// * finish cierra el compresor y lo devuelve al conjunto.
func (g *gzipWriter) finish() {
	if g.writer != nil {
		g.writer.Close()
		gzipPool.Put(g.writer)
	}
}

// * withGzip comprime las respuestas de texto cuando el navegador lo acepta. Va por la red local, pero ahorra bytes que leer y
// * parsear a la interfaz y quita el aviso de "activar compresión" de las auditorías (Lighthouse).
func withGzip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") || r.Header.Get("Range") != "" {
			next.ServeHTTP(w, r)
			return
		}
		wrapped := &gzipWriter{ResponseWriter: w}
		defer wrapped.finish()
		next.ServeHTTP(wrapped, r)
	})
}
