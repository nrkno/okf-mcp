package linkextract

import (
	"reflect"
	"testing"
)

func TestExtract(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		docDir     string
		corpusRoot string
		want       []ExtractedLink
	}{
		{
			name:       "multiple links under one heading",
			body:       "# Architecture\n\nSee [Tools](/docs/tools.md) and [Config](../config.md).\n",
			docDir:     "docs",
			corpusRoot: ".",
			want: []ExtractedLink{
				{
					Target:        "docs/tools.md",
					RawTarget:     "/docs/tools.md",
					Heading:       "Architecture",
					HeadingNormal: "architecture",
					Line:          3,
				},
				{
					Target:        "config.md",
					RawTarget:     "../config.md",
					Heading:       "Architecture",
					HeadingNormal: "architecture",
					Line:          3,
				},
			},
		},
		{
			name:       "link before any heading",
			body:       "Read [Overview](overview.md) first.\n\n# Intro\n\nSee [Detail](detail.md).\n",
			docDir:     "docs",
			corpusRoot: ".",
			want: []ExtractedLink{
				{
					Target:        "docs/overview.md",
					RawTarget:     "overview.md",
					Heading:       "",
					HeadingNormal: "",
					Line:          1,
				},
				{
					Target:        "docs/detail.md",
					RawTarget:     "detail.md",
					Heading:       "Intro",
					HeadingNormal: "intro",
					Line:          5,
				},
			},
		},
		{
			name:       "empty body",
			body:       "",
			docDir:     "docs",
			corpusRoot: ".",
			want:       nil,
		},
		{
			name:       "no links",
			body:       "# Just a heading\n\nSome text without links.\n",
			docDir:     "docs",
			corpusRoot: ".",
			want:       nil,
		},
		{
			name:       "no headings",
			body:       "See [Target](target.md) here.\n",
			docDir:     "docs/sub",
			corpusRoot: ".",
			want: []ExtractedLink{
				{
					Target:        "docs/sub/target.md",
					RawTarget:     "target.md",
					Heading:       "",
					HeadingNormal: "",
					Line:          1,
				},
			},
		},
		{
			name:       "fenced code block skip",
			body:       "# Dependencies\n\n```\n[skip](skip.md)\n```\n\n[keep](keep.md)\n",
			docDir:     "docs",
			corpusRoot: ".",
			want: []ExtractedLink{
				{
					Target:        "docs/keep.md",
					RawTarget:     "keep.md",
					Heading:       "Dependencies",
					HeadingNormal: "dependencies",
					Line:          7,
				},
			},
		},
		{
			name:       "indented code block skip",
			body:       "# Dependencies\n\n    [skip](skip.md)\n\n[keep](keep.md)\n",
			docDir:     "docs",
			corpusRoot: ".",
			want: []ExtractedLink{
				{
					Target:        "docs/keep.md",
					RawTarget:     "keep.md",
					Heading:       "Dependencies",
					HeadingNormal: "dependencies",
					Line:          5,
				},
			},
		},
		{
			name:       "code span skip",
			body:       "# Dependencies\n\nUse `[skip](skip.md)` inline.\n\n[keep](keep.md)\n",
			docDir:     "docs",
			corpusRoot: ".",
			want: []ExtractedLink{
				{
					Target:        "docs/keep.md",
					RawTarget:     "keep.md",
					Heading:       "Dependencies",
					HeadingNormal: "dependencies",
					Line:          5,
				},
			},
		},
		{
			name:       "html block skip",
			body:       "# Dependencies\n\n<div>\n<a href=\"skip.md\">skip</a>\n</div>\n\n[keep](keep.md)\n",
			docDir:     "docs",
			corpusRoot: ".",
			want: []ExtractedLink{
				{
					Target:        "docs/keep.md",
					RawTarget:     "keep.md",
					Heading:       "Dependencies",
					HeadingNormal: "dependencies",
					Line:          7,
				},
			},
		},
		{
			name:       "escaped link skip",
			body:       "# Dependencies\n\n\\[skip](skip.md) and [keep](keep.md).\n",
			docDir:     "docs",
			corpusRoot: ".",
			want: []ExtractedLink{
				{
					Target:        "docs/keep.md",
					RawTarget:     "keep.md",
					Heading:       "Dependencies",
					HeadingNormal: "dependencies",
					Line:          3,
				},
			},
		},
		{
			name:       "reference style extraction",
			body:       "# Dependencies\n\nSee [Tools][tools-ref] and [Config][cfg].\n\n[tools-ref]: /docs/tools.md\n[cfg]: ../config.md\n",
			docDir:     "docs/sub",
			corpusRoot: ".",
			want: []ExtractedLink{
				{
					Target:        "docs/tools.md",
					RawTarget:     "/docs/tools.md",
					Heading:       "Dependencies",
					HeadingNormal: "dependencies",
					Line:          3,
				},
				{
					Target:        "docs/config.md",
					RawTarget:     "../config.md",
					Heading:       "Dependencies",
					HeadingNormal: "dependencies",
					Line:          3,
				},
			},
		},
		{
			name:       "http mailto and anchor skip",
			body:       "# Links\n\n[web](http://example.com), [mail](mailto:a@b.com), [section](#sec), [file](file.md).\n",
			docDir:     "docs",
			corpusRoot: ".",
			want: []ExtractedLink{
				{
					Target:        "docs/file.md",
					RawTarget:     "file.md",
					Heading:       "Links",
					HeadingNormal: "links",
					Line:          3,
				},
			},
		},
		{
			name:       "path traversal escape dropped",
			body:       "# Bad\n\n[escape](../../../../etc/passwd).\n",
			docDir:     "docs/sub/deep",
			corpusRoot: ".",
			want:       nil,
		},
		{
			name:       "boundary path kept",
			body:       "# Root\n\n[root](../../../root.md).\n",
			docDir:     "docs/sub/deep",
			corpusRoot: ".",
			want: []ExtractedLink{
				{
					Target:        "root.md",
					RawTarget:     "../../../root.md",
					Heading:       "Root",
					HeadingNormal: "root",
					Line:          3,
				},
			},
		},
		{
			name:       "heading normalization",
			body:       "## *Derived* from  `Source`\n\nSee [Target](target.md).\n",
			docDir:     "docs",
			corpusRoot: ".",
			want: []ExtractedLink{
				{
					Target:        "docs/target.md",
					RawTarget:     "target.md",
					Heading:       "Derived from  Source",
					HeadingNormal: "derived from source",
					Line:          3,
				},
			},
		},
		{
			name:       "heading depth stack",
			body:       "# H1\n\n[one](one.md)\n\n## H2\n\n[two](two.md)\n\n### H3\n\n[three](three.md)\n\n## H2b\n\n[four](four.md)\n",
			docDir:     "docs",
			corpusRoot: ".",
			want: []ExtractedLink{
				{Target: "docs/one.md", RawTarget: "one.md", Heading: "H1", HeadingNormal: "h1", Line: 3},
				{Target: "docs/two.md", RawTarget: "two.md", Heading: "H2", HeadingNormal: "h2", Line: 7},
				{Target: "docs/three.md", RawTarget: "three.md", Heading: "H3", HeadingNormal: "h3", Line: 11},
				{Target: "docs/four.md", RawTarget: "four.md", Heading: "H2b", HeadingNormal: "h2b", Line: 15},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Extract(tt.body, tt.docDir, tt.corpusRoot)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Extract() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestExtractEmptyDocDir(t *testing.T) {
	body := "# Top\n\n[Link](other.md)\n"
	got := Extract(body, ".", ".")
	want := []ExtractedLink{
		{
			Target:        "other.md",
			RawTarget:     "other.md",
			Heading:       "Top",
			HeadingNormal: "top",
			Line:          3,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Extract() = %#v, want %#v", got, want)
	}
}
