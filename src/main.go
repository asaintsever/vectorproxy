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
package main

import (
	"asaintsever/vectorproxy/config"
	"asaintsever/vectorproxy/stores"
	"flag"
	"log"
	"net/http"
	"strings"
)

func init() {
	flag.StringVar(&config.VectorStoreURL, "url", "https://localhost:9200", "Vector Store URL")
	paths := flag.String("gjson-paths", "", "Comma-separated list of GJSON Path strings")
	flag.BoolVar(&config.DryRun, "dry-run", false, "If set, do not send the request to Vector Store")
	flag.StringVar(&config.ProxyPort, "port", "8080", "Port for the proxy server")
	flag.StringVar(&config.EmbeddingModelID, "embedding-model", "amazon.titan-embed-text-v2:0", "Text Embedding Model ID")
	flag.IntVar(&config.EmbeddingsDimension, "embeddings-dimension", 1024, "Embeddings Dimension")
	flag.IntVar(&config.MaxParallel, "parallel", 10, "Maximum number of parallel processing")
	flag.Parse()

	if *paths != "" {
		config.GjsonPaths = strings.Split(*paths, ",")
	}
}

func main() {
	http.HandleFunc("/opensearch/_bulk", stores.OpenSearchBulkHandler) // Intercept bulk requests
	http.HandleFunc("/opensearch/", stores.ProxyHandler)               // Forward all other requests
	log.Printf("Starting vector proxy on :%s", config.ProxyPort)
	log.Fatal(http.ListenAndServe(":"+config.ProxyPort, nil))
}
