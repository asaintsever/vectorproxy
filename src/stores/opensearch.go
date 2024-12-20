// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package stores

import (
	"bytes"
	"crypto/tls"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"asaintsever/vectorproxy/config"
	"asaintsever/vectorproxy/vectorization"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"golang.org/x/net/http2"
)

func OpenSearchBulkHandler(w http.ResponseWriter, r *http.Request) {
	proxyURL := config.VectorStoreURL + r.RequestURI

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
	sem := make(chan struct{}, config.MaxParallel)

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
			for _, path := range config.GjsonPaths {
				// Extract the field using gjson
				values := gjson.GetBytes(doc, path).Array()
				if len(values) > 0 {
					var err error
					doc, err = vectorization.Vectorize(doc, values, path)
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

			if config.DryRun {
				if name == "Content-Type" || name == "Authorization" {
					w.Header().Add(name, value)
				}
			}
		}
	}

	if config.DryRun {
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
