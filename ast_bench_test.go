package katschema_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/birdie-ai/katschema"
	"github.com/birdie-ai/katschema/ks"
	"github.com/birdie-ai/katschema/parser"
)

type astValidationBenchmarkInput struct {
	name   string
	schema katschema.Type
	source string
	valid  bool
}

func astValidationBenchmarkInputs(b *testing.B) []astValidationBenchmarkInput {
	b.Helper()
	core := compile(b, katschema.NewCompiler(), ks.Object(
		ks.Field("id", ks.String()),
		ks.Field("custom_fields", ks.Optional(ks.Object())),
	))
	patch := compile(b, katschema.NewCompiler(), ks.Object(
		ks.Field("custom_fields", ks.Optional(ks.Object(
			ks.Field("priority", ks.Int()),
			ks.Field("tags", ks.List(ks.String())),
		))),
	))
	overlay, err := katschema.Overlay(core, patch)
	if err != nil {
		b.Fatal(err)
	}
	return []astValidationBenchmarkInput{
		{"Core", core, `{"id":"one"}`, true},
		{"Overlay", overlay, `{"id":"one","custom_fields":{"priority":3,"tags":["high","new"]}}`, true},
		{"OverlayInvalid", overlay, `{"id":"one","custom_fields":{"priority":"wrong","tags":[]}}`, false},
	}
}

// BenchmarkValidateAST excludes parsing and schema compilation, measuring the
// direct validation path used when a caller already owns a parsed assignment.
func BenchmarkValidateAST(b *testing.B) {
	for _, input := range astValidationBenchmarkInputs(b) {
		b.Run(input.name, func(b *testing.B) {
			tree, root, err := parser.Parse(input.source)
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			for b.Loop() {
				if err := input.schema.ValidateAST(tree, root); (err == nil) != input.valid {
					b.Fatalf("ValidateAST = %v, want valid=%v", err, input.valid)
				}
			}
		})
	}
}

func BenchmarkValidateASTAnyObject(b *testing.B) {
	schema := compile(b, katschema.NewCompiler(), ks.Any())
	for _, count := range []int{1, 8, 16, 64} {
		b.Run(fmt.Sprintf("Fields%d", count), func(b *testing.B) {
			fields := make([]ks.Member, count)
			for i := range fields {
				fields[i] = ks.Field(fmt.Sprintf("field%03d", i), ks.LitInt(int64(i)))
			}
			tree, root, err := ks.Build(ks.Object(fields...))
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			for b.Loop() {
				if err := schema.ValidateAST(tree, root); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkValidateType uses the same fixtures as BenchmarkValidateAST, with
// value compilation outside the timed loop. Values use a separate compiler
// from their schemas, as they would in a request-scoped value arena.
func BenchmarkValidateType(b *testing.B) {
	for _, input := range astValidationBenchmarkInputs(b) {
		b.Run(input.name, func(b *testing.B) {
			tree, root, err := parser.Parse(input.source)
			if err != nil {
				b.Fatal(err)
			}
			value, err := katschema.CompileAST(b.Context(), tree, root)
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			for b.Loop() {
				if err := input.schema.ValidateType(value); (err == nil) != input.valid {
					b.Fatalf("ValidateType = %v, want valid=%v", err, input.valid)
				}
			}
		})
	}
}

func BenchmarkCompileAST(b *testing.B) {
	tree, root, err := parser.Parse(`{"id":"one","custom_fields":{"priority":3,"tags":["high","new"]}}`)
	if err != nil {
		b.Fatal(err)
	}
	b.Run("FreshCompiler", func(b *testing.B) {
		ctx := b.Context()
		b.ReportAllocs()
		for b.Loop() {
			value, err := katschema.CompileAST(ctx, tree, root)
			if err != nil || value.Kind() != katschema.Object {
				b.Fatalf("CompileAST = %v, %v; want object", value.Kind(), err)
			}
		}
	})
	b.Run("SharedCompiler", func(b *testing.B) {
		ctx := b.Context()
		compiler := katschema.NewCompiler()
		// Repeated literals can reuse canonical types. Unique values and a
		// bounded compiler lifetime are measured by BenchmarkASTBatch.
		if _, err := compiler.CompileAST(ctx, tree, root); err != nil {
			b.Fatal(err)
		}
		b.ReportAllocs()
		for b.Loop() {
			value, err := compiler.CompileAST(ctx, tree, root)
			if err != nil || value.Kind() != katschema.Object {
				b.Fatalf("CompileAST = %v, %v; want object", value.Kind(), err)
			}
		}
	})
}

// BenchmarkASTBatch models a DML payload containing independent assignments.
// One operation parses and validates an entire batch; the compiled path also
// compiles each object into one new compiler per batch. Unique IDs, strings,
// and numbers prevent repeated objects from hiding compilation costs.
func BenchmarkASTBatch(b *testing.B) {
	schema := compile(b, katschema.NewCompiler(), ks.Object(
		ks.Field("id", ks.String()),
		ks.Field("priority", ks.Where(ks.Int(), ks.Binary(ks.X(), ks.Gt, ks.IntExpr(0)))),
		ks.Field("message", ks.String()),
		ks.Field("active", ks.Bool()),
		ks.Field("tags", ks.List(ks.String())),
		ks.Field("details", ks.Object(
			ks.Field("source", ks.String()),
			ks.Field("score", ks.Real()),
		)),
	))
	for _, count := range []int{1, 1000} {
		source := bechmarkFixture(count)
		if !json.Valid([]byte(source)) {
			b.Fatal("invalid JSON batch fixture")
		}
		b.Run(fmt.Sprintf("Objects%d", count), func(b *testing.B) {
			b.Run("ParseValidateAST", func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(source)))
				for b.Loop() {
					tree, root, err := parser.Parse(source)
					if err != nil {
						b.Fatal(err)
					}
					objects := tree.List(root)
					if len(objects) != count {
						b.Fatalf("parsed %d objects, want %d", len(objects), count)
					}
					for _, object := range objects {
						if err := schema.ValidateAST(tree, object); err != nil {
							b.Fatal(err)
						}
					}
				}
				b.ReportMetric(float64(count), "objects/op")
			})
			b.Run("ParseCompileASTValidateType", func(b *testing.B) {
				ctx := b.Context()
				b.ReportAllocs()
				b.SetBytes(int64(len(source)))
				for b.Loop() {
					tree, root, err := parser.Parse(source)
					if err != nil {
						b.Fatal(err)
					}
					objects := tree.List(root)
					if len(objects) != count {
						b.Fatalf("parsed %d objects, want %d", len(objects), count)
					}
					compiler := katschema.NewCompiler()
					for _, object := range objects {
						value, err := compiler.CompileAST(ctx, tree, object)
						if err != nil {
							b.Fatal(err)
						}
						if err := schema.ValidateType(value); err != nil {
							b.Fatal(err)
						}
					}
				}
				b.ReportMetric(float64(count), "objects/op")
			})
		})
	}
}

func bechmarkFixture(count int) string {
	var src strings.Builder
	src.WriteByte('[')
	for i := range count {
		if i > 0 {
			src.WriteByte(',')
		}
		fmt.Fprintf(&src, `{"id":"feedback-%d","priority":%d,"message":"Feedback message %d","active":true,"tags":["new","tag-%d"],"details":{"source":"source-%d","score":%d.25}}`, i, i+1, i, i, i, i)
	}
	src.WriteByte(']')
	return src.String()
}
