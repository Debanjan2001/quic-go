package main

import (
	"net/http"

	"github.com/quic-go/quic-go/http3"
)

func main() {
	dirpath := "/home/bob/Desktop/mtp/DATA/"
	handler := http.FileServer(http.Dir(dirpath))
	http.Handle("/", handler)

	// certFile, keyFile := testdata.GetCertificatePaths()

	var err error

	go func() {
		err = http.ListenAndServe(
			"localhost:6060",
			handler,
		)
		if err != nil {
			panic(err)
		}
	}()

	err = http3.ListenAndServe(
		"localhost:6121",
		"/home/bob/Desktop/out/2048-sha256-root.pem",
		"/home/bob/Desktop/out/2048-sha256-root.key",
		handler,
	)
	// err = http3.ListenAndServeQUIC(
	// 	"localhost:6121",
	// 	"/home/bob/Desktop/out/2048-sha256-root.pem",
	// 	"/home/bob/Desktop/out/2048-sha256-root.key",
	// 	handler,
	// )
	// err = http3.ListenAndServeQUIC(
	// 	"localhost:6121",
	// 	"/home/bob/Desktop/out/leaf_cert.pem",
	// 	"/home/bob/Desktop/out/leaf_cert.key",
	// 	handler,
	// )

	if err != nil {
		panic(err)
	}
}
