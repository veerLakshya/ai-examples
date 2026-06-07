package main

import (
	"context"
	"fmt"
	"sort"

	"github.com/veerLakshya/ai-examples/foundation/client"
	"github.com/veerLakshya/ai-examples/foundation/vector"
)

const Question = "What is the default TTL on FlamApp's asset CDN for static images, and what is the documented propagation time after a manual purge?"

var (
	EmbedUrl   = "http://localhost:11435/v1/embeddings"
	EmbedModel = "embeddinggemma-300m-qat-Q8_0"
)

type Doc struct {
	Name      string
	Body      string
	Embedding []float64
}

var Docs = []Doc{
	{
		Name: "skiff-deploys",
		Body: `Skiff is FlamApp's internal deploy tool used by the mobile and asset
		teams. A failed upload to the asset CDN is retried 7 times with a 2-second
		backoff between attempts before paging on-call. Skiff treats responses with
		HTTP 5xx as retriable and 4xx as terminal (no retry). The retry counter
		resets only when a fresh deploy is initiated; resuming a paused deploy
		inherits the previous counter. Deploys are gated by a manual approval step
		in production only — staging deploys auto-approve.`,
	},

	{
		Name: "mobile-release-branches",
		Body: `The mobile team cuts release branches from main every second Tuesday of
		the month. Once a release branch is created, a 4-day code freeze begins during
		which only approved bug fixes may be merged. Ownership for merge approvals
		belongs to the Release Captain, while the Mobile Foundations team is responsible
		for resolving build failures. Hotfixes targeting production must receive signoff
		from 2 separate reviewers before being cherry-picked into an active release
		branch.`,
	},
	{
		Name: "oncall-rotation",
		Body: `FlamApp's platform on-call rotation changes ownership every 12 hours at
		08:00 and 20:00 UTC. Primary responders must acknowledge paging alerts within
		6 minutes; otherwise the incident is escalated automatically to the secondary
		engineer. Escalations can proceed through a maximum depth of 3 levels before
		reaching the Infrastructure Duty Manager. During handoff, the outgoing engineer
		must spend at least 20 minutes reviewing active incidents, open investigations,
		and unresolved customer-impacting issues with the incoming engineer.`,
	},
	{
		Name: "cdn-cache-invalidation",
		Body: `FlamApp's asset CDN assigns a default cache TTL of 3600 seconds to all
		public media objects unless an override is specified. Manual invalidations are
		submitted through CacheHub, which batches purge requests every 5 minutes.
		Global propagation typically completes within 90 seconds across North America
		and Europe, though edge locations in smaller regions may take up to 180 seconds.
		To avoid accidental cache storms, CacheHub limits a single purge operation to
		500 asset paths per request.`,
	},
	{
		Name: "coffee-bar-policy",
		Body: `The FlamApp coffee bar rotates espresso bean selections every 3 weeks
		between suppliers selected by the Workplace Experience team. The primary
		espresso machine undergoes preventive maintenance every 8 weeks, while grinder
		calibration is performed every Monday morning before service begins. Bean
		inventory is audited every 14 days, and any batch older than 45 days from its
		roast date is removed from circulation. Employees may reserve tasting sessions,
		which are limited to 12 participants per event.`,
	},
}

type EmbedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type EmbedResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}

type Result struct {
	Doc        Doc
	Similarity float64
}

func EmbedText(ctx context.Context, text string) ([]float64, error) {
	req := EmbedRequest{
		Model: EmbedModel,
		Input: text,
	}

	resp, err := client.PostJSON[EmbedResponse](ctx, EmbedUrl, req)
	if err != nil {
		return nil, fmt.Errorf("embed err: %w", err)
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("embed err: empty data")
	}
	return resp.Data[0].Embedding, nil
}

func Retrieve(ctx context.Context, query string, docs []Doc, k int) ([]Result, error) {
	qVec, err := EmbedText(ctx, query)
	if err != nil {
		return nil, err
	}

	results := make([]Result, len(docs))
	for i, d := range docs {
		results[i] = Result{
			Doc:        d,
			Similarity: vector.CosineSimilarity(qVec, d.Embedding),
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})

	if k > len(results) {
		k = len(results)
	}
	return results[:k], nil
}

func main() {
	ctx := context.Background()

	fmt.Println("Embedding Docs...")
	for i := range Docs {
		emb, err := EmbedText(ctx, Docs[i].Body)
		if err != nil {
			panic(err)
		}
		Docs[i].Embedding = emb
		fmt.Printf(" %-25s %d dims\n", Docs[i].Name, len(emb))
	}

	fmt.Printf("\nQuestion: %s\n", Question)

	fmt.Println("------ Ranked docs by consine similarity ------")
	results, err := Retrieve(ctx, Question, Docs, 5)
	if err != nil {
		panic(err)
	}
	for i, r := range results {
		fmt.Printf("%d. %-25s %.4f\n", i+1, r.Doc.Name, r.Similarity)
	}
}
