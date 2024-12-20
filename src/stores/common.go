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

	"asaintsever/vectorproxy/config"

	"golang.org/x/net/http2"
)

func ProxyHandler(w http.ResponseWriter, r *http.Request) {
	proxyURL := config.VectorStoreURL + r.RequestURI

	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	// Create the request
	req, err := http.NewRequest(r.Method, proxyURL, bytes.NewReader(body))
	if err != nil {
		http.Error(w, "Failed to create request to vector store", http.StatusInternalServerError)
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
		// Log the request being forwarded
		log.Printf("Forwarding request to: %s", proxyURL)

		// If dry-run is enabled, return the request details without forwarding
		w.WriteHeader(http.StatusOK)
		w.Write(body)
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
		http.Error(w, "Failed to forward request to vector store", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Copy headers from the response
	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	// Write the response back to the client
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
