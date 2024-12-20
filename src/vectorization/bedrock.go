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
package vectorization

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"asaintsever/vectorproxy/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const defaultRegion = "us-west-2"

var (
	brc *bedrockruntime.Client
)

// type BedrockRequest struct {
// 	InputText  string `json:"inputText"`
// 	Dimensions int    `json:"dimensions"` //  Only for Titan Embeddings Text V2 Request
// }

type BedrockResponse struct {
	Embedding           []float32 `json:"embedding"` // Bedrock vectors are float64 arrays but we are converting them to float32, default type used by OpenSearch
	InputTextTokenCount int       `json:"inputTextTokenCount"`
}

func init() {
	aws_region := os.Getenv("AWS_REGION")
	if aws_region == "" {
		aws_region = defaultRegion
	}

	cfg, err := awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion(aws_region))
	if err != nil {
		log.Fatal(err)
	}

	brc = bedrockruntime.NewFromConfig(cfg)
}

func Vectorize(doc []byte, array []gjson.Result, path string) ([]byte, error) {
	var err error
	for value_indx, value := range array {
		// Replace first '#' character found in path with value_indx as sjson does not support '#' syntax
		fieldToVectorizePath := strings.Replace(path, "#", fmt.Sprintf("%d", value_indx), 1)

		if config.DryRun {
			log.Printf("Value: %s [Path: %s]", value.String(), fieldToVectorizePath)
		}

		// Deal with nested arrays (e.g. with path like "indication_parents.#.indication_names.#.name")
		if value.IsArray() {
			doc, err = Vectorize(doc, value.Array(), fieldToVectorizePath)
			if err != nil {
				return nil, err
			}
			continue
		}

		/* See https://docs.aws.amazon.com/bedrock/latest/userguide/model-parameters-titan-embed-text.html

		Titan Embeddings Text G1 Request:

		{
			"inputText": string
		}

		Titan Embeddings Text V2 Request:

		{
			"inputText": string,
			"dimensions": int,      - (optional) The number of dimensions the output embeddings should have. The following values are accepted: 1024 (default), 512, 256.
			"normalize": boolean,   – (optional) Flag indicating whether or not to normalize the output embeddings. Defaults to true.
			"embeddingTypes": list  - (optional) Accepts a list containing "float", "binary", or both. Defaults to float.
		}
		*/

		// Use sjson to create the payload for the Bedrock request
		// instead of using a struct and marshalling it to JSON as it is more efficient
		bedrockPayload, _ := sjson.Set("", "inputText", value.String())

		if config.EmbeddingModelID == "amazon.titan-embed-text-v2:0" {
			bedrockPayload, _ = sjson.Set(bedrockPayload, "dimensions", config.EmbeddingsDimension)
		}

		//-- Code below shows how to do the same using a struct and marshalling it to JSON
		// bedrockPayload := BedrockRequest{
		// 	InputText: value.String(),
		// }

		// if config.EmbeddingModelID == "amazon.titan-embed-text-v2:0" {
		// 	bedrockPayload.Dimensions = embeddingsDimension
		// }

		// payloadBytes, err := json.Marshal(bedrockPayload)
		// if err != nil {
		// 	return nil, err
		// }

		output, err := brc.InvokeModel(context.Background(), &bedrockruntime.InvokeModelInput{
			Body:        []byte(bedrockPayload),
			ModelId:     aws.String(config.EmbeddingModelID),
			ContentType: aws.String("application/json"),
		})

		if err != nil {
			//TODO catch too many requests (HTTP 429) error and retry with exponential backoff
			// "Bedrock Runtime: InvokeModel, exceeded maximum number of attempts, 3, https response error StatusCode: 429, RequestID: xxx, ThrottlingException: Too many requests, please wait before trying again. You have sent too many requests.  Wait before trying again."
			return nil, err
		}

		// Here we do not use gjson to extract the embedding as we want to convert it to float32
		// (as Bedrock vectors are float64 arrays)
		var resp BedrockResponse
		err = json.Unmarshal(output.Body, &resp)
		if err != nil {
			return nil, err
		}

		// Add the new field with "_embedding" suffix using sjson
		doc, err = sjson.SetBytes(doc, fieldToVectorizePath+"_embedding", resp.Embedding)
		if err != nil {
			return nil, err
		}
	}

	return doc, nil
}
