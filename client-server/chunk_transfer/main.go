package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/quic-go/quic-go/internal/testdata"
	"github.com/quic-go/quic-go/internal/utils"
	"github.com/quic-go/quic-go/logging"
	"github.com/quic-go/quic-go/qlog"
)

const quic_addr = "https://localhost:6121"
const tcp_addr = "http://localhost:6060"

func RunClient(hclient *http.Client, logger utils.Logger, urls []string, quiet *bool, byteStart int, byteEnd int) *http.Response {
	// var wg sync.WaitGroup
	// wg.Add(len(urls))
	var rsp *http.Response
	for _, addr := range urls {
		logger.Infof("GET %s", addr)
		// go func(addr string) {
		req, err := http.NewRequest("GET", addr, nil)
		if err != nil {
			log.Fatal(err)
		}
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", byteStart, byteEnd))
		rsp, err = hclient.Do(req)
		if err != nil {
			log.Fatal(err)
		}
		logger.Infof("Got response for %s: %#v", addr, rsp)

		body := &bytes.Buffer{}
		_, err = io.Copy(body, rsp.Body)
		if err != nil {
			log.Fatal(err)
		}
		if *quiet {
			logger.Infof("Response Body: %d bytes", body.Len())
		} else {
			logger.Infof("Response Body length: %d bytes", body.Len())
			logger.Infof("Response Body:")
			logger.Infof("%s", body.Bytes())
		}
		// wg.Done()
		// }(addr)
	}
	// wg.Wait()
	// return
	return rsp
}

func main() {
	verbose := flag.Bool("v", false, "verbose")
	quiet := flag.Bool("q", false, "don't print the data")
	keyLogFile := flag.String("keylog", "", "key log file")
	insecure := flag.Bool("insecure", false, "skip certificate verification")
	enableQlog := flag.Bool("qlog", false, "output a qlog (in the same directory)")
	flag.Parse()
	filenames := flag.Args()

	logger := utils.DefaultLogger

	if *verbose {
		logger.SetLogLevel(utils.LogLevelDebug)
	} else {
		logger.SetLogLevel(utils.LogLevelInfo)
	}
	logger.SetLogTimeFormat("")

	var keyLog io.Writer
	if len(*keyLogFile) > 0 {
		f, err := os.Create(*keyLogFile)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		keyLog = f
	}

	pool, err := x509.SystemCertPool()
	if err != nil {
		log.Fatal(err)
	}
	testdata.AddRootCA(pool)

	var qconf quic.Config
	if *enableQlog {
		qconf.Tracer = func(ctx context.Context, p logging.Perspective, connID quic.ConnectionID) logging.ConnectionTracer {
			filename := fmt.Sprintf("client_%x.qlog", connID)
			f, err := os.Create(filename)
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("Creating qlog file %s.\n", filename)
			return qlog.NewConnectionTracer(utils.NewBufferedWriteCloser(bufio.NewWriter(f), f), p, connID)
		}
	}

	roundTripper := &http3.RoundTripper{
		TLSClientConfig: &tls.Config{
			RootCAs:            pool,
			InsecureSkipVerify: *insecure,
			KeyLogWriter:       keyLog,
		},
		QuicConfig: &qconf,
	}
	defer roundTripper.Close()

	quic_client := &http.Client{
		Transport: roundTripper,
	}

	tcp_client := &http.Client{
		Transport: &http.Transport{},
	}

	tcp_urls := []string{}
	quic_urls := []string{}
	for _, filename := range filenames {
		quic_urls = append(quic_urls, quic_addr+"/"+filename)
		tcp_urls = append(tcp_urls, tcp_addr+"/"+filename)
	}

	var choice int
	byteStart := 0
	chunkSize := 100
	choice = 1
	for {
		byteEnd := byteStart + chunkSize - 1
		var rsp *http.Response
		if choice == 1 {
			rsp = RunClient(quic_client, logger, quic_urls, quiet, byteStart, byteEnd)
			choice = 2
		} else if choice == 2 {
			rsp = RunClient(tcp_client, logger, tcp_urls, quiet, byteStart, byteEnd)
			choice = 1
		}

		contentLength, _ := strconv.Atoi(rsp.Header.Get("Content-Length"))
		if contentLength < chunkSize {
			break
		}
		time.Sleep(1 * time.Second)
		byteStart += chunkSize
	}

	// for {
	// fmt.Print("You have 2 Choices\n1. QUIC\n2. TCP\nPress anything else to quit!\nEnter your choice: ")
	// fmt.Scanln(&choice)

	// else if choice == 2 {
	// RunClient(tcp_client, logger, tcp_urls, quiet)
	// } else {
	// break
	// }
	// }
}
