package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/nrkno/plattform-okf-mcp/internal/index"
	"github.com/nrkno/plattform-okf-mcp/internal/matcher"
)

// idx is the shared index. Wave 5 tests reassign and restore this variable
// directly to achieve test isolation.
// Tests using newFixtureServer must NOT call t.Parallel() — the global mutation is sequential-only.
var idx *index.Index

// Tool definitions — package-level so main_test.go uses the exact same schema.
// NEVER reconstruct mcp.NewTool inline inside main() — always use these vars.
var (
	listTagsTool = mcp.NewTool("list_tags",
		mcp.WithDescription("List all tags across all indexed OKF documents"),
	)
	listDocsTool = mcp.NewTool("list_docs",
		mcp.WithDescription("List all indexed OKF documents with their metadata (no content)"),
	)
	getDocTool = mcp.NewTool("get_doc",
		mcp.WithDescription("Retrieve a document by topic and optional tag filter"),
		mcp.WithString("topic",
			mcp.Required(),
			mcp.Description("Topic or title to search for"),
		),
		mcp.WithArray("tags",
			mcp.Description("Optional tag filter"),
		),
		mcp.WithString("match",
			mcp.Description(`Tag filter mode: "and" (all tags must match, default) or "or" (any tag matches)`),
		),
	)
)

// listTagsHandler rebuilds the index and returns all tags as a JSON array.
func listTagsHandler(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := idx.Rebuild(); err != nil {
		fmt.Fprintf(os.Stderr, "okf-mcp: ERROR: rebuild failed: %v\n", err)
		return mcp.NewToolResultError(err.Error()), nil
	}
	out, err := json.Marshal(idx.Tags())
	if err != nil {
		return mcp.NewToolResultError("failed to marshal tags: " + err.Error()), nil
	}
	return mcp.NewToolResultText(string(out)), nil
}

// listDocsHandler rebuilds the index and returns all document metadata as a JSON array.
func listDocsHandler(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := idx.Rebuild(); err != nil {
		fmt.Fprintf(os.Stderr, "okf-mcp: ERROR: rebuild failed: %v\n", err)
		return mcp.NewToolResultError(err.Error()), nil
	}
	docs := idx.Docs()
	entries := make([]map[string]any, len(docs))
	for i, doc := range docs {
		entries[i] = map[string]any{
			"title":       doc.Title,
			"description": doc.Description,
			"tags":        doc.Tags,
			"file_path":   doc.FilePath,
		}
	}
	out, err := json.Marshal(entries)
	if err != nil {
		return mcp.NewToolResultError("failed to marshal docs: " + err.Error()), nil
	}
	return mcp.NewToolResultText(string(out)), nil
}

// getDocHandler finds the best-matching document by topic and optional tag filter,
// then returns its full content and metadata as JSON.
func getDocHandler(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()

	// Extract and validate topic.
	topic, _ := args["topic"].(string)
	if topic == "" {
		return mcp.NewToolResultError("topic is required"), nil
	}

	// Extract and default match mode.
	matchMode, _ := args["match"].(string)
	if matchMode == "" {
		matchMode = "and"
	}

	// Validate match mode before hitting the index.
	if matchMode != "and" && matchMode != "or" {
		return mcp.NewToolResultError(
			fmt.Sprintf("invalid match value %q: must be \"and\" or \"or\"", matchMode),
		), nil
	}

	// Extract optional tag filter; reject scalar values explicitly.
	var filterTags []string
	if rawTags, exists := args["tags"]; exists && rawTags != nil {
		switch v := rawTags.(type) {
		case []interface{}:
			for _, t := range v {
				if s, ok := t.(string); ok {
					filterTags = append(filterTags, s)
				}
			}
		default:
			return mcp.NewToolResultError(`"tags" must be an array of strings, not a scalar value`), nil
		}
	}

	if err := idx.Rebuild(); err != nil {
		fmt.Fprintf(os.Stderr, "okf-mcp: ERROR: rebuild failed: %v\n", err)
		return mcp.NewToolResultError(err.Error()), nil
	}

	doc, found := matcher.FindBest(topic, filterTags, matchMode, idx.Docs())
	if !found {
		if len(idx.Docs()) == 0 {
			return mcp.NewToolResultError(
				"index is empty: no OKF-conformant markdown docs found in cwd",
			), nil
		}
		return mcp.NewToolResultError(
			fmt.Sprintf("no document matched topic %q with tags %v", topic, filterTags),
		), nil
	}

	// I-2: live read from disk. filepath.Join(idx.Dir(), doc.FilePath) resolves
	// correctly regardless of the process cwd at call time.
	absPath := filepath.Join(idx.Dir(), doc.FilePath)
	content, err := os.ReadFile(absPath)
	if err != nil {
		return mcp.NewToolResultError(
			"document found but file no longer exists: " + doc.FilePath,
		), nil
	}

	// Strip frontmatter: content already has title/description/tags as structured
	// fields — transmitting raw YAML in the body too is purely redundant.
	// doc.BodyOffset is the byte offset of the first character after "---\n".
	body := content
	if doc.BodyOffset <= len(content) {
		body = content[doc.BodyOffset:]
	}

	// I-1: file_path in the response is the relative path, not the joined absolute path.
	payload := map[string]any{
		"content":     string(body),
		"file_path":   doc.FilePath,
		"tags":        doc.Tags,
		"title":       doc.Title,
		"description": doc.Description,
	}
	out, err := json.Marshal(payload)
	if err != nil {
		return mcp.NewToolResultError("failed to marshal doc response: " + err.Error()), nil
	}
	return mcp.NewToolResultText(string(out)), nil
}

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "okf-mcp: failed to get working directory: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "okf-mcp: serving %s\n", cwd)

	idx = index.New(cwd)

	s := server.NewMCPServer("okf-mcp", "1.0.0",
		server.WithInstructions(
			"Use this server for OKF documentation lookups in the current repository. "+
				"Call list_tags first to discover available topics and tags. "+
				"Then use get_doc(topic) to retrieve the relevant document. "+
				"Prefer this over reading files directly when looking for platform or process documentation.",
		),
	)
	s.AddTool(listTagsTool, listTagsHandler)
	s.AddTool(listDocsTool, listDocsHandler)
	s.AddTool(getDocTool, getDocHandler)

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "okf-mcp: %v\n", err)
		os.Exit(1)
	}
}
