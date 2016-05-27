package grpcutils

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"strings"

	"github.com/gorilla/websocket"
	"golang.org/x/net/context"
)

var MethodOverrideParam = "method"

// WebsocketProxy attempts to expose the underlying handler as a bidi websocket stream with newline-delimited
// JSON as the content encoding.
//
// The HTTP Authorization header is populated from the Sec-Websocket-Protocol field
//
// example:
//   Sec-Websocket-Protocol: Bearer, foobar
// is converted to:
//   Authorization: Bearer foobar
//
// Method can be overwritten with the MethodOverrideParam get parameter in the requested URL
func WebsocketProxy(h http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !websocket.IsWebSocketUpgrade(r) {
			h.ServeHTTP(w, r)
			return
		}
		websocketProxy(w, r, h)
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func websocketProxy(w http.ResponseWriter, r *http.Request, h http.Handler) {
	req, err := httputil.DumpRequest(r, true)
	fmt.Println(err)
	fmt.Println(string(req))
	conn, err := upgrader.Upgrade(w, r, http.Header{
		"Sec-WebSocket-Protocol": []string{"Bearer"},
	})
	if err != nil {
		log.Println("error upgrading websocket:", err)
		return
	}
	defer conn.Close()

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()

	requestBodyR, requestBodyW := io.Pipe()
	request, err := http.NewRequest(r.Method, r.URL.String(), requestBodyR)
	if err != nil {
		log.Println("error preparing request:", err)
		return
	}
	request.Header.Set("Authorization", strings.Replace(r.Header.Get("Sec-WebSocket-Protocol"), "Bearer, ", "Bearer ", 1))
	if m := r.URL.Query().Get(MethodOverrideParam); m != "" {
		request.Method = m
	}

	responseBodyR, responseBodyW := io.Pipe()
	go func() {
		<-ctx.Done()
		responseBodyW.CloseWithError(io.EOF)
		responseBodyR.CloseWithError(io.EOF)
		requestBodyR.Close()
		requestBodyW.Close()
	}()

	response := newInMemoryResponseWriter(responseBodyW)
	go func() {
		log.Println("calling ServeHTTP")
		h.ServeHTTP(response, request)
		log.Println("done calling ServeHTTP")
		cancelFn()
	}()

	// read loop -- take messages from websocket and write to http request
	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Println("read loop done")
				return
			default:
			}
			log.Println("reading from socket.")
			_, p, err := conn.ReadMessage()
			if err != nil {
				log.Println("error reading websocket message:", err)
				return
			}
			log.Println("read payload:", string(p))
			log.Println("writing to requestBody:")
			n, err := requestBodyW.Write(p)
			log.Println("wrote to requestBody", n)
			requestBodyW.Write([]byte("\n"))
			log.Println("wrote newline to requestBody")
			if err != nil {
				log.Println("error writing message to upstream http server:", err)
				return
			}
		}
	}()
	// write loop -- take messages from response and write to websocket
	scanner := bufio.NewScanner(responseBodyR)
	for scanner.Scan() {
		if len(scanner.Bytes()) == 0 {
			log.Println("empty scan", scanner.Err())
			continue
		}
		log.Println("scanned", scanner.Text())
		if err = conn.WriteMessage(websocket.TextMessage, scanner.Bytes()); err != nil {
			log.Println("error writing websocket message:", err)
			return
		}
	}
	if err := scanner.Err(); err != nil {
		log.Println("scanner err:", err)
	}
}

type inMemoryResponseWriter struct {
	io.Writer
	header http.Header
	code   int
}

func newInMemoryResponseWriter(w io.Writer) *inMemoryResponseWriter {
	return &inMemoryResponseWriter{
		Writer: w,
		header: http.Header{},
	}
}

func (w *inMemoryResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}
func (w *inMemoryResponseWriter) Header() http.Header {
	return w.header
}
func (w *inMemoryResponseWriter) WriteHeader(code int) {
	w.code = code
}
func (w *inMemoryResponseWriter) Flush() {}
