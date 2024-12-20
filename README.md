# Vector Proxy (VP)

VP is a service that acts as a proxy between an ingest script/pipeline and the vector store. It is responsible for computing or delegating vectorization and then sending the vectorized data to the vector store.

> [!NOTE]
> VP proxies all calls to the vector store. Only indexing calls are modified to include the vectorized data in the payload, other calls being directly forwarded to the vector store.

## Usage

```bash
vectp -h
```

E.g. to compute vectors from a sample NDJSON file:

- Start proxy first (in dry mode here just to see the computed vectors):

    ```bash
    vectp -gjson-paths indication_names.#.name,icd_codes.#.icd_code_description,indication_parents.#.indication_names.#.name -dry-run
    ```

- Then send the NDJSON data to the proxy, the same way as you would send it to OpenSearch:

    ```bash
    curl -v -k -u "<USER>:<PWD>" -XPOST "http://localhost:8080/opensearch/_bulk" -H 'Content-Type: application/x-ndjson' --data-binary "@./samples/opensearch/test.ndjson"
    ```

## Features

| Proxy Vector Store API | Delegated Vectorization | Local Vectorization |
|------------------------|-------------------------|---------------------|
| OpenSearch/Elasticsearch Bulk API | Amazon Bedrock | - |

## GJSON Path Syntax

The service uses the GJSON Path syntax to extract the data from the JSON payload. The syntax is similar to the JSON Path syntax but with some differences.

See the [GJSON Path syntax documentation](https://github.com/tidwall/gjson/blob/master/SYNTAX.md) for more information and [GJSON playground](https://gjson.dev/) to experiment with the syntax.
