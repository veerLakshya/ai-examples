package main

type ChunkOpts struct {
	Size    int // target chars per chunk (or tokens, we'll use chars)
	Overlap int // chars of overlap between consecutive chunks
}

type Chunk struct {
	Source string // which doc this came from
	Index  int    // position within doc
	Text   string
}

type Chunker func(text string, opts ChunkOpts) []Chunker
