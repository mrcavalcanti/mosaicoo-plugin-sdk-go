package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/har"
	"github.com/google/uuid"
	"github.com/grafana/grafana-plugin-sdk-go/experimental/e2e/utils"
)

// HAR is a Storage implementation that stores requests and responses in HAR format on disk.
type HAR struct {
	lock        sync.RWMutex
	path        string
	har         *har.HAR
	currentTime func() time.Time
	newUUID     func() string
}

// NewHARStorage creates a new HARStorage.
func NewHARStorage(path string) *HAR {
	storage := &HAR{
		path:        path,
		har:         &har.HAR{},
		currentTime: time.Now,
		newUUID:     newUUID,
	}
	storage.Init()
	return storage
}

// WithCurrentTimeOverride replaces the default s.currentTime() with the given function.
func (s *HAR) WithCurrentTimeOverride(fn func() time.Time) {
	s.currentTime = fn
	s.Init()
}

// WithUUIDOverride replaces the default s.newUUID() with the given function.
func (s *HAR) WithUUIDOverride(fn func() string) {
	s.newUUID = fn
	s.Init()
}

func (s *HAR) Init() {
	if err := s.Load(); err == nil {
		return
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	s.har.Log = &har.Log{
		Version: "1.2",
		Creator: &har.Creator{
			Name:    "grafana-plugin-sdk-go",
			Version: "experimental",
		},
		Entries: make([]*har.Entry, 0),
		Pages: []*har.Page{{
			StartedDateTime: s.currentTime().Format(time.RFC3339),
			Title:           "Grafana E2E",
			ID:              s.newUUID(),
			PageTimings:     &har.PageTimings{},
		}},
	}
}

// Add converts the http.Request and http.Response to a har.Entry and adds it to the Fixture.
func (s *HAR) Add(req *http.Request, res *http.Response) {
	var (
		err     error
		reqBody []byte
		resBody []byte
	)

	reqHeaders := make([]*har.NameValuePair, 0)
	for name, value := range req.Header.Clone() {
		reqHeaders = append(reqHeaders, &har.NameValuePair{Name: name, Value: value[0]})
	}

	resHeaders := make([]*har.NameValuePair, 0)
	for name, value := range res.Header.Clone() {
		resHeaders = append(resHeaders, &har.NameValuePair{Name: name, Value: value[0]})
	}

	queryString := make([]*har.NameValuePair, 0)
	for name, value := range req.URL.Query() {
		queryString = append(queryString, &har.NameValuePair{Name: name, Value: value[0]})
	}

	if req.Body != nil {
		reqBody, err = utils.ReadRequestBody(req)
		if err != nil {
			fmt.Println("Failed to read request body", "err", err)
		}
	}

	if res.Body != nil {
		resBody, err = io.ReadAll(res.Body)
		if err != nil {
			fmt.Println("Failed to read response body", "err", err)
		}
		res.Body = ioutil.NopCloser(bytes.NewReader(resBody))
	}

	reqCookies := make([]*har.Cookie, 0)
	for _, cookie := range req.Cookies() {
		reqCookies = append(reqCookies, &har.Cookie{Name: cookie.Name, Value: cookie.Value})
	}

	resCookies := make([]*har.Cookie, 0)
	for _, cookie := range res.Cookies() {
		resCookies = append(resCookies, &har.Cookie{Name: cookie.Name, Value: cookie.Value})
	}

	s.lock.Lock()
	defer s.lock.Unlock()
	s.har.Log.Entries = append(s.har.Log.Entries, &har.Entry{
		StartedDateTime: s.currentTime().Format(time.RFC3339),
		Time:            0.0,
		Comment:         s.newUUID(),
		Cache: &har.Cache{
			Comment: "Not cached",
		},
		Timings: &har.Timings{
			Send:    0.0,
			Wait:    0.0,
			Receive: 0.0,
		},
		Request: &har.Request{
			Method:      req.Method,
			HTTPVersion: req.Proto,
			URL:         req.URL.String(),
			Headers:     reqHeaders,
			QueryString: queryString,
			Cookies:     reqCookies,
			BodySize:    int64(len(reqBody)),
			PostData: &har.PostData{
				MimeType: req.Header.Get("Content-Type"),
				Text:     string(reqBody),
			},
		},
		Response: &har.Response{
			Status:      int64(res.StatusCode),
			StatusText:  res.Status,
			HTTPVersion: res.Proto,
			Headers:     resHeaders,
			HeadersSize: -1,
			BodySize:    int64(len(resBody)),
			Cookies:     resCookies,
			RedirectURL: res.Header.Get("Location"),
			Content: &har.Content{
				Size:     int64(len(resBody)),
				MimeType: res.Header.Get("Content-Type"),
				Text:     string(resBody),
			},
		},
	})
}

// Entries converts HAR entries to a slice of Entry (http.Request and http.Response pairs).
func (s *HAR) Entries() []*Entry {
	s.lock.RLock()
	defer s.lock.RUnlock()

	entries := make([]*Entry, len(s.har.Log.Entries))
	for i, e := range s.har.Log.Entries {
		postData := ""
		if e.Request.PostData != nil {
			postData = e.Request.PostData.Text
		}
		req, err := http.NewRequest(e.Request.Method, e.Request.URL, nil)
		if err != nil {
			fmt.Println("Failed to create request", "err", err)
			continue
		}
		req.Body = ioutil.NopCloser(strings.NewReader(postData))
		req.ContentLength = e.Request.BodySize
		req.Header = make(http.Header)
		for _, header := range e.Request.Headers {
			req.Header.Add(header.Name, header.Value)
		}

		bodyReq := req.Clone(context.Background())
		bodyReq.Body = ioutil.NopCloser(strings.NewReader(postData))
		res := &http.Response{
			StatusCode:    int(e.Response.Status),
			Status:        e.Response.StatusText,
			Proto:         e.Response.HTTPVersion,
			Header:        make(http.Header),
			Body:          ioutil.NopCloser(strings.NewReader(e.Response.Content.Text)),
			ContentLength: int64(len(e.Response.Content.Text)),
			Request:       bodyReq,
		}

		for _, header := range e.Response.Headers {
			res.Header.Add(header.Name, header.Value)
		}

		// use the HAR entry's comment field to store the ID of the entry
		if e.Comment == "" {
			e.Comment = newUUID()
		}

		entries[i] = &Entry{
			Request:  req,
			Response: res,
		}
	}

	return entries
}

// Delete removes the HAR entry matching the given Request.
func (s *HAR) Delete(req *http.Request) bool {
	if i, entry := s.findEntry(req); entry != nil {
		s.lock.Lock()
		s.har.Log.Entries = append(s.har.Log.Entries[:i], s.har.Log.Entries[i+1:]...)
		s.lock.Unlock()
		return true
	}
	return false
}

// Save writes the HAR to disk.
func (s *HAR) Save() error {
	s.lock.Lock()
	defer s.lock.Unlock()
	err := os.MkdirAll(filepath.Dir(s.path), os.ModePerm)
	if err != nil {
		return err
	}
	raw, err := s.har.MarshalJSON()
	if err != nil {
		return err
	}
	return ioutil.WriteFile(s.path, raw, 0600)
}

// Load reads the HAR from disk.
func (s *HAR) Load() error {
	s.lock.Lock()
	defer s.lock.Unlock()
	raw, err := ioutil.ReadFile(s.path)
	if err != nil {
		return err
	}
	return s.har.UnmarshalJSON(raw)
}

// Match returns the stored http.Response for the given request.
func (s *HAR) Match(req *http.Request) *http.Response {
	s.lock.RLock()
	defer s.lock.RUnlock()
	if _, entry := s.findEntry(req); entry != nil {
		return entry.Response
	}
	return nil
}

// findEntry returns them matching entry index and entry for the given request.
func (s *HAR) findEntry(req *http.Request) (int, *Entry) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	for i, entry := range s.Entries() {
		if res := entry.Match(req); res != nil {
			res.Body.Close()
			return i, entry
		}
	}
	return -1, nil
}

func newUUID() string {
	return uuid.New().String()
}