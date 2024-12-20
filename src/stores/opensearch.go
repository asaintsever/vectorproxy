package stores

import (
	"bytes"
	"crypto/tls"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"golang.org/x/net/http2"
)

func openSearchBulkHandler(w http.ResponseWriter, r *http.Request) {
	proxyURL := vectorStoreURL + r.RequestURI

	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	// Measure the time taken for JSON processing
	startTime := time.Now()

	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	// Split the NDJSON into individual JSON documents
	documents := bytes.Split(body, []byte("\n"))

	// Process each document in parallel with a limit on the number of goroutines
	var buffer bytes.Buffer
	var wg sync.WaitGroup
	var mu sync.Mutex
	sem := make(chan struct{}, maxParallel)

	for i := 0; i < len(documents); i += 2 {
		// Skip empty lines
		if len(documents[i]) == 0 {
			continue
		}

		metadata := documents[i]
		doc := documents[i+1]

		wg.Add(1)
		sem <- struct{}{}
		go func(metadata, doc []byte) {
			defer wg.Done()
			defer func() { <-sem }()
			for _, path := range gjsonPaths {
				// Extract the field using gjson
				values := gjson.GetBytes(doc, path).Array()
				if len(values) > 0 {
					var err error
					doc, err = vectorize(doc, values, path)
					if err != nil {
						// Only log the error as we don't want to stop the processing
						// Ideally, we should store those errors and add them in the
						// final response received from OpenSearch (setting "errors" to true)
						log.Print("Failed to vectorize field - ", err)
					}
				}
			}

			if doc != nil {
				mu.Lock()
				buffer.Write(metadata)
				buffer.WriteByte('\n')
				buffer.Write(doc)
				buffer.WriteByte('\n')
				mu.Unlock()
			}
		}(metadata, doc)
	}

	wg.Wait()

	processingTime := time.Since(startTime).Milliseconds()

	// Create the request to Elasticsearch/OpenSearch
	req, err := http.NewRequest("POST", proxyURL, &buffer)
	if err != nil {
		http.Error(w, "Failed to create request to Elasticsearch/OpenSearch", http.StatusInternalServerError)
		return
	}

	// Copy headers from the original request
	for name, values := range r.Header {
		for _, value := range values {
			req.Header.Add(name, value)

			if dryRun {
				if name == "Content-Type" || name == "Authorization" {
					w.Header().Add(name, value)
				}
			}
		}
	}

	if dryRun {
		log.Printf("Sending request to: %s", proxyURL)

		// If dry-run is enabled, return the buffer without sending it
		w.WriteHeader(http.StatusOK)
		w.Write(buffer.Bytes())
		return
	}

	// Configure HTTP client to use HTTP/2 and skip TLS certificate validation
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	http2.ConfigureTransport(client.Transport.(*http.Transport))

	// Perform the request
	resp, err := client.Do(req)
	if err != nil {
		log.Print(err.Error())
		http.Error(w, "Failed to forward request to Elasticsearch/OpenSearch", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Read the response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Failed to read response from Elasticsearch/OpenSearch", http.StatusInternalServerError)
		return
	}

	// Update the ingest time field in the response
	updatedRespBody, err := sjson.SetBytes(respBody, "ingest_took", processingTime)
	if err != nil {
		http.Error(w, "Failed to update ingest_took field in response", http.StatusInternalServerError)
		return
	}

	// Write the updated response back to the client
	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	w.Write(updatedRespBody)
}
