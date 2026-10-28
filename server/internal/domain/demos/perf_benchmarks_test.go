package demos

// Topic 38 performance's real benchmark functions — the curated set the
// demo's bounded boot run measures and any reader can re-run with
// `go test -bench`. Everything measured is this module's own code or
// the stdlib: gzip levels × payload sizes over the deterministic
// posts-page document (PerfPostsPage), buffered json.Marshal versus
// the streaming Encoder writeJSON uses, and the in-memory posts
// repository's List — the fake mode's actual serving path, whose
// defensive copies are the allocation story the topic teaches. Run
// them yourself with real budgets:
//
//	go test -bench=^BenchmarkPerf -benchtime=1s -count=10 -benchmem -run='^$' ./internal/domain/demos/

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"

	postsdata "guide/internal/data/posts"
	"guide/internal/domain/posts"
)

// BenchmarkPerfGzipCompress measures the compression trade-off curve:
// gzip levels 1, 6 (the default), and 9 against the deterministic JSON
// document at 4KB, 64KB, and 512KB. ns/op is the CPU price,
// B/op + allocs/op the allocation price; the compressed SIZES for the
// same cells are what the demo's ratio column computes from the same
// bytes — CPU and ratio together are the whole decision.
func BenchmarkPerfGzipCompress(b *testing.B) {
	for _, size := range PerfPayloadSizes {
		payload, err := PerfPayloadJSON(size)
		if err != nil {
			b.Fatal(err)
		}
		for _, level := range PerfGzipLevels {
			b.Run(fmt.Sprintf("level=%d/size=%s", level, size.Label), func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					compressed, err := PerfGzipBytes(payload, level)
					if err != nil {
						b.Fatal(err)
					}
					if len(compressed) == 0 {
						b.Fatal("empty gzip output")
					}
				}
			})
		}
	}
}

// BenchmarkPerfJSONEncode measures the two ways this server's handlers
// could turn one posts page into bytes: buffered json.Marshal (one
// whole-document allocation handed to the caller) versus the streaming
// json.NewEncoder(w).Encode that writeJSON actually uses on every
// response. Same page, same 64KB shape GET /api/posts serves.
func BenchmarkPerfJSONEncode(b *testing.B) {
	page := PerfPostsPage(PerfPayloadSizes[1])
	b.Run("approach=marshal", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			raw, err := json.Marshal(page)
			if err != nil {
				b.Fatal(err)
			}
			if len(raw) == 0 {
				b.Fatal("empty marshal output")
			}
		}
	})
	b.Run("approach=stream", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if err := json.NewEncoder(io.Discard).Encode(page); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkPerfInMemoryPostsList measures the fake mode's REAL hot
// path: the seeded in-memory repository's List under its RWMutex,
// filtering, sorting, and copying every post it returns. The
// defensive copies are the discipline (callers can never mutate stored
// state) — and -benchmem states what that discipline costs per call,
// the allocation tax the escape-analysis section explains line by
// line.
func BenchmarkPerfInMemoryPostsList(b *testing.B) {
	repository := postsdata.NewInMemoryRepository()
	query := posts.ListQuery{Page: 1, Limit: 20, Sort: posts.SortNewest}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		page, err := repository.List(context.Background(), query)
		if err != nil {
			b.Fatal(err)
		}
		if len(page.Items) == 0 {
			b.Fatal("empty page from the seeded store")
		}
	}
}
