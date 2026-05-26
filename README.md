# ai-examples

My from-scratch reimplementations of the [Ardan Labs "Ultimate AI"](https://github.com/ardanlabs/ai-training)
Go curriculum — built by first principles to learn AI engineering, not by copying the reference.

Each example is rebuilt from the underlying concept (vectors, embeddings, RAG, agents,
security, multimodal) rather than from Ardan's source. Datasets and extra techniques
often differ from the originals on purpose.

## Examples

| Example | Concept | Run |
|---------|---------|-----|
| example01-vectors | Hand-crafted vectors, cosine similarity, top-K retrieval, vector arithmetic | `go run ./example01-vectors` |

## Notes

- Module: `github.com/veerLakshya/ai-examples`
- Go 1.26+
- Standalone — does not depend on the Ardan `foundation/` packages.
