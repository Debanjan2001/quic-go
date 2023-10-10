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

func RunClient(ctx context.Context, hclient *http.Client, logger utils.Logger, urls []string, quiet *bool) {
	// var wg sync.WaitGroup

	// wg.Add(len(urls))
	done := make(chan bool)
	for _, addr := range urls {
		logger.Infof("GET %s", addr)
		go func(addr string) {
			rsp, err := hclient.Get(addr)
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
				logger.Infof("Response Body:")
				logger.Infof("%s", body.Bytes())
			}
			done <- true
			// wg.Done()
		}(addr)
	}
	// wg.Wait()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("Timeout! Exiting gracefully")
			return
		case <-done:
			fmt.Println("Done!")
			return
		default:
			time.Sleep(time.Millisecond)
		}
	}
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

	ctx, cancelFunc := context.WithCancel(context.Background())
	go func() {
		time.Sleep(3 * time.Second)
		cancelFunc()
		fmt.Println("Quitting QUIC Client")
	}()

	RunClient(ctx, quic_client, logger, quic_urls, quiet)

	time.Sleep(10 * time.Second)
	ctx, cancelFunc = context.WithCancel(context.Background())
	RunClient(ctx, tcp_client, logger, tcp_urls, quiet)
}
