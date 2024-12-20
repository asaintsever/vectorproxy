package main

import (
	"flag"
	"log"
	"net/http"
	"strings"
)

var (
	vectorStoreURL        string
	gjsonPaths            []string
	dryRun                bool
	proxyPort             string
	titanEmbeddingModelID string
	embeddingsDimension   int
	maxParallel           int
)

func init() {
	flag.StringVar(&vectorStoreURL, "url", "https://localhost:9200", "Vector Store URL")
	paths := flag.String("gjson-paths", "", "Comma-separated list of GJSON Path strings")
	flag.BoolVar(&dryRun, "dry-run", false, "If set, do not send the request to Vector Store")
	flag.StringVar(&proxyPort, "port", "8080", "Port for the proxy server")
	flag.StringVar(&titanEmbeddingModelID, "titan-embedding-model", "amazon.titan-embed-text-v2:0", "Titan Embedding Model ID")
	flag.IntVar(&embeddingsDimension, "embeddings-dimension", 1024, "Embeddings Dimension")
	flag.IntVar(&maxParallel, "parallel", 10, "Maximum number of parallel processing")
	flag.Parse()

	if *paths != "" {
		gjsonPaths = strings.Split(*paths, ",")
	}
}

func main() {
	http.HandleFunc("/opensearch/_bulk", openSearchBulkHandler) // Intercept bulk requests
	http.HandleFunc("/opensearch/", proxyHandler)               // Forward all other requests
	log.Printf("Starting vector proxy on :%s", proxyPort)
	log.Fatal(http.ListenAndServe(":"+proxyPort, nil))
}
