package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"

	"github.com/amarin/logging"
)

const (
	vaultPath       = "/Users/asmarin/Documents/MD"
	gakoPrefix      = "Архив/Государственный Архив Калужской Области/"
	gakoFondsPrefix = "Архив/Государственный Архив Калужской Области/Фонды"

	warmupPage      = "https://archive.admoblkaluga.ru/"
	initialPage     = "https://archive.admoblkaluga.ru/gako/type/stocks"
	stripPagePrefix = "https://archive.admoblkaluga.ru"
	tmpPath         = ".data/tmp"
	proxyURL        = "socks5://127.0.0.1:8989"
)

var (
	logger logging.Logger
	client http.Client
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := logging.Init(
		logging.WithLevel(logging.LevelDebug),
		logging.WithFormat(logging.FormatText),
		logging.WithTarget(logging.StdOut),
	); err != nil {
		log.Fatal(err)
	}

	logger := logging.NewNamedLoggerCtx(ctx, "main").WithLevel(logging.LevelDebug)

	logger.Debug("setup client")
	if err := initClient(ctx); err != nil {
		logger.WithError(err).Warn("cant setup client")
		os.Exit(1)
	}

	if err := run(ctx, os.Args[1:]); err != nil {
		logger.WithError(err).Warn("run error")
		os.Exit(1)
	}
}

type rt struct {
	http.RoundTripper
	logger   logging.Logger
	accept   string
	referrer string
}

func (r *rt) RoundTrip(request *http.Request) (*http.Response, error) {
	//logger := logging.NewNamedLoggerCtx(request.Context(), "transport")

	request.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/152.0.0.0 Safari/537.36")
	request.Header.Set("Accept", r.accept)
	request.Header.Set("Sec-Fetch-Site", "same-origin")
	request.Header.Set("Sec-Fetch-Mode", "navigate")
	request.Header.Set("Sec-Fetch-User", "?1")
	request.Header.Set("Sec-Fetch-Dest", "document")
	request.Header.Set("Referer", r.referrer)
	request.Header.Set("Accept-Language", "Accept-Language: en-GB,en-US;q=0.9,en;q=0.8,ru;q=0.7")

	return r.RoundTripper.RoundTrip(request)
}

func initClient(ctx context.Context) (err error) {
	logger := logging.NewNamedLoggerCtx(ctx, "client").WithLevel(logging.LevelDebug)
	baseTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return fmt.Errorf("default transport is not *http.Transport")
	}
	baseTransport.Clone()

	useProxy, err := url.Parse(proxyURL)
	if err != nil {
		logger.WithError(err).Warn("setup proxy failed")
		return fmt.Errorf("setup proxy failed")
	}

	baseTransport.Proxy = http.ProxyURL(useProxy)
	client.Transport = &rt{
		RoundTripper: baseTransport,
		logger:       logger,
		accept:       "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
	}
	if client.Jar, err = cookiejar.New(nil); err != nil {
		return err
	}

	return nil
}

func savePage(ctx context.Context, urlString string, body io.ReadCloser) (savedPath string, err error) {
	var (
		fh         *os.File
		bytesTaken int
	)
	logger := logging.
		NewNamedLoggerCtx(ctx, "cache").
		WithKey("url", urlString).
		WithLevel(logging.LevelDebug)

	if urlString == warmupPage {
		logger.Debug("skip")
		return "", nil
	}

	pageName := path.Join(tmpPath, urlString[len(stripPagePrefix):]) + ".html"
	logger.Debugf("store as %s", pageName)

	basePath := path.Dir(pageName)
	logger.Debugf("create base path %s", basePath)

	if err = os.MkdirAll(basePath, 0750); err != nil {
		logger.WithError(err).Warn("create file base failed")
		return "", err
	}
	logger.Debug("open file for writing")
	if fh, err = os.Create(pageName); err != nil {
		logger.WithError(err).Warn("create file failed")
		return "", err
	}

	logger.Debug("writing file")
	repeatRead := true
	buffSize := 1024
	buff := make([]byte, buffSize)
	for {
		if !repeatRead {
			break
		}

		if bytesTaken, err = body.Read(buff); err != nil {
			logger.WithError(err).Warn("read %d bytes from response with error", bytesTaken)
		}
		if bytesTaken > 0 {
			fh.Write(buff[0:bytesTaken])
		}

		repeatRead = bytesTaken == buffSize
	}

	fh.Close()
	body.Close()

	logger.Infof("saved as %s", pageName)
	return savedPath, nil
}

func getPage(ctx context.Context, urlString string) (savedPath string, err error) {
	var (
		request  *http.Request
		response *http.Response
	)

	logger := logging.
		NewNamedLoggerCtx(ctx, "fetcher").
		WithKey("url", urlString).
		WithLevel(logging.LevelDebug)

	if !strings.HasPrefix(urlString, stripPagePrefix) {
		logger.Warnf("cant get %s: unexpected prefix", urlString)
		return "", fmt.Errorf("unexpected prefix: %s", urlString)
	}

	logger.Debug("build request")
	if request, err = http.NewRequestWithContext(ctx, http.MethodGet, urlString, nil); err != nil {
		return "", err
	}

	logger.Debug("execute request")
	if response, err = client.Do(request); err != nil {
		logger.WithError(err).Warn("execute request failed")
		return "", err
	}

	if response.StatusCode != 200 {
		logger.WithError(err).Warnf("unexpected response code %d", response.StatusCode)
		return "", fmt.Errorf("%d %s", response.StatusCode, response.Status)
	}

	return savePage(ctx, urlString, response.Body)

}

// run разбирает глобальный флаг --data и подкоманду. По умолчанию — serve
// (совместимость: genodex -p 9000). --data можно указывать и до, и после
// подкоманды: genodex --data X backup --to Y == genodex backup --data X --to Y.
func run(ctx context.Context, args []string) (err error) {
	var (
		initialPageSavedPath string
	)

	logger := logging.NewNamedLoggerCtx(ctx, "loader").WithLevel(logging.LevelDebug)

	logger.Info("warmup")
	if _, err = getPage(ctx, warmupPage); err != nil {
		logger.WithError(err).Warn("fetch failed")
		return err
	}

	if initialPageSavedPath, err = getPage(ctx, initialPage); err != nil {
		logger.WithError(err).Warn("fetch failed")
		return err
	}

	logger.Debugf("parse stored file %s", initialPageSavedPath)

	return nil
}
