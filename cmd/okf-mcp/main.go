package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/nrkno/plattform-okf-mcp/internal/index"
	"github.com/nrkno/plattform-okf-mcp/internal/logparser"
	"github.com/nrkno/plattform-okf-mcp/internal/matcher"
	"github.com/nrkno/plattform-okf-mcp/internal/parser"
	"github.com/nrkno/plattform-okf-mcp/internal/scanner"
	"github.com/nrkno/plattform-okf-mcp/internal/validator"
)

// idx is the shared index. Wave 5 tests reassign and restore this variable
// directly to achieve test isolation.
// Tests using newFixtureServer must NOT call t.Parallel() — the global mutation is sequential-only.
var idx *index.Index

// Tool definitions — package-level so main_test.go uses the exact same schema.
// NEVER reconstruct mcp.NewTool inline inside main() — always use these vars.
var (
	listTagsTool = mcp.NewTool("list_tags",
		mcp.WithDescription("List all tags across all indexed documents"),
	)
	listDocsTool = mcp.NewTool("list_docs",
		mcp.WithDescription("List all indexed documents with their metadata (no content)"),
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

// validateDocTool validates document conformance.
var validateDocTool = mcp.NewTool("validate_doc",
	mcp.WithDescription("Validate document conformance and report errors, warnings, and notifications"),
	mcp.WithString("file_path",
		mcp.Description("Optional: relative path of a single file to validate. If omitted, validates entire bundle."),
	),
	mcp.WithArray("known_types",
		mcp.Description("Optional: list of known type values for W3 warnings"),
	),
)

// getIndexTool returns the bundle tree structure.
var getIndexTool = mcp.NewTool("get_index",
	mcp.WithDescription("Return the bundle tree showing all documents and their directory structure"),
	mcp.WithString("path",
		mcp.Description("Optional: relative path to a subtree root. If omitted, returns full tree."),
	),
)

// getLogTool returns structured log entries from the documentation change log.
var getLogTool = mcp.NewTool("get_log",
	mcp.WithDescription("Return structured log entries from the documentation change log"),
	mcp.WithString("since",
		mcp.Description("Optional: only return entries on or after this date (YYYY-MM-DD)"),
	),
	mcp.WithString("action",
		mcp.Description("Optional: filter by action type (e.g. 'Creation', 'Update')"),
	),
	mcp.WithNumber("limit",
		mcp.Description("Optional: maximum number of entries to return (default: all)"),
	),
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
			"bundle":      doc.Bundle,
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
				"index is empty: no markdown documents found in cwd",
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
		"bundle":      doc.Bundle,
	}
	out, err := json.Marshal(payload)
	if err != nil {
		return mcp.NewToolResultError("failed to marshal doc response: " + err.Error()), nil
	}
	return mcp.NewToolResultText(string(out)), nil
}

// severityLabel maps a validator severity to a human-readable string.
func severityLabel(s validator.Severity) string {
	switch s {
	case validator.SeverityError:
		return "error"
	case validator.SeverityWarning:
		return "warning"
	case validator.SeverityNotification:
		return "notification"
	default:
		return "unknown"
	}
}

// validateDocHandler validates a single file or the entire bundle.
func validateDocHandler(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	filePath, _ := args["file_path"].(string)

	if err := idx.Rebuild(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	var result validator.Result
	if filePath == "" {
		result = validator.ValidateBundle(idx)
	} else {
		absPath := filepath.Join(idx.Dir(), filePath)
		knownTypes := defaultKnownTypes()
		if rawKT, ok := args["known_types"]; ok && rawKT != nil {
			if arr, ok := rawKT.([]interface{}); ok {
				knownTypes = make([]string, 0, len(arr))
				for _, v := range arr {
					if s, ok := v.(string); ok {
						knownTypes = append(knownTypes, s)
					}
				}
			}
		}
		if reservedBasename(filepath.Base(filePath)) {
			fs, err := validator.ValidateReserved(absPath, filePath)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			result = validator.Result{Findings: fs, Summary: buildSummary(fs, 1)}
		} else {
			fs, err := validator.ValidateDoc(absPath, knownTypes)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			result = validator.Result{Findings: fs, Summary: buildSummary(fs, 1)}
		}
	}

	type findingJSON struct {
		Code     string `json:"code"`
		Severity string `json:"severity"`
		File     string `json:"file"`
		Line     int    `json:"line,omitempty"`
		Message  string `json:"message"`
	}
	type validateResponse struct {
		Summary  validator.Summary `json:"summary"`
		Findings []findingJSON     `json:"findings"`
	}

	findings := make([]findingJSON, len(result.Findings))
	for i, f := range result.Findings {
		findings[i] = findingJSON{
			Code:     f.Code,
			Severity: severityLabel(f.Severity),
			File:     f.File,
			Line:     f.Line,
			Message:  f.Message,
		}
	}
	out, err := json.Marshal(validateResponse{Summary: result.Summary, Findings: findings})
	if err != nil {
		return mcp.NewToolResultError("failed to marshal validation result: " + err.Error()), nil
	}
	return mcp.NewToolResultText(string(out)), nil
}

// buildSummary computes a Summary from a finding slice.
func buildSummary(findings []validator.Finding, files int) validator.Summary {
	s := validator.Summary{Files: files}
	for _, f := range findings {
		switch f.Severity {
		case validator.SeverityError:
			s.Errors++
		case validator.SeverityWarning:
			s.Warnings++
		case validator.SeverityNotification:
			s.Notifications++
		}
	}
	return s
}

// reservedBasename reports whether the basename marks a reserved file.
func reservedBasename(name string) bool {
	return name == "index.md" || name == "log.md"
}

// defaultKnownTypes returns the standard OKF type vocabulary.
func defaultKnownTypes() []string {
	return []string{"Architecture", "Playbook", "Configuration", "API Reference", "Metrics Reference", "Log"}
}

// getIndexHandler returns the bundle tree or a subtree.
func getIndexHandler(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	subPath, _ := args["path"].(string)

	if err := idx.Rebuild(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	tree := idx.Tree()
	if subPath != "" {
		sub := findSubtree(&tree, subPath)
		if sub == nil {
			return mcp.NewToolResultError(
				fmt.Sprintf("path %q not found in bundle tree", subPath),
			), nil
		}
		tree = *sub
	}

	out, err := json.Marshal(tree)
	if err != nil {
		return mcp.NewToolResultError("failed to marshal tree: " + err.Error()), nil
	}
	return mcp.NewToolResultText(string(out)), nil
}

// findSubtree walks the tree to find the node whose Name matches the given
// relative path. Returns nil if no match is found.
func findSubtree(root *index.TreeNode, path string) *index.TreeNode {
	if path == "" || path == "." {
		return root
	}
	segments := strings.Split(path, string(filepath.Separator))
	current := root
	for _, seg := range segments {
		if current == nil || current.Type != "directory" {
			return nil
		}
		found := false
		for i := range current.Children {
			if current.Children[i].Name == seg {
				current = &current.Children[i]
				found = true
				break
			}
		}
		if !found {
			return nil
		}
	}
	return current
}

// getLogHandler returns parsed log.md entries from ALL log.md files in the
// index, merged in reverse-chronological order. Each entry is tagged with the
// relative path of its source log.md. Filters (since, action, limit) apply to
// the merged list. See design §4 and invariant I-12.
func getLogHandler(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	since, _ := args["since"].(string)
	action, _ := args["action"].(string)
	var limit int
	if v, ok := args["limit"]; ok {
		if n, ok := v.(float64); ok {
			limit = int(n)
		}
	}

	if err := idx.Rebuild(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Collect all log.md files from reserved list (one per bundle).
	var logFiles []string
	for _, r := range idx.Reserved() {
		if filepath.Base(r.FilePath) == "log.md" {
			logFiles = append(logFiles, r.FilePath)
		}
	}
	if len(logFiles) == 0 {
		return marshalLogResult(nil, "no log.md found")
	}

	// I-2: live read each log.md from disk. Strip frontmatter, parse, and
	// tag every entry with its source log.md's relative path.
	var entries []logEntryJSON
	note := ""
	for _, rel := range logFiles {
		absPath := filepath.Join(idx.Dir(), rel)
		data, err := os.ReadFile(absPath)
		if err != nil {
			return marshalLogResult(nil, fmt.Sprintf("failed to read %s: %v", rel, err))
		}
		content := string(data)
		fmInfo := parser.DetectFrontmatter(content)
		body := content
		if fmInfo.HasFrontmatter {
			body = content[fmInfo.BodyOffset:]
		}
		parsed := logparser.Parse(body)
		if parsed == nil && len(strings.TrimSpace(body)) > 0 {
			note = "log.md has malformed entries"
		}
		for _, e := range parsed {
			entries = append(entries, logEntryJSON{
				Date:   e.Date,
				Action: e.Action,
				Target: e.Target,
				Detail: e.Detail,
				Source: rel,
			})
		}
	}

	// Multi-key sort: date desc (newest first), source asc as tiebreak.
	// SliceStable preserves document order for entries sharing both keys.
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Date != entries[j].Date {
			return entries[i].Date > entries[j].Date
		}
		return entries[i].Source < entries[j].Source
	})

	// Apply filters.
	if since != "" {
		filtered := entries[:0]
		for _, e := range entries {
			if e.Date >= since {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}
	if action != "" {
		filtered := entries[:0]
		for _, e := range entries {
			if e.Action == action {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}

	if entries == nil {
		entries = []logEntryJSON{}
	}

	return marshalLogResult(entries, note)
}

// logEntryJSON is one entry in the get_log response. Source is the relative
// path of the log.md file the entry came from (multi-bundle aggregation).
type logEntryJSON struct {
	Date   string `json:"date"`
	Action string `json:"action"`
	Target string `json:"target"`
	Detail string `json:"detail,omitempty"`
	Source string `json:"source"`
}

// logResult is the JSON response for get_log. Top-level Source is removed —
// the per-entry Source field is the source of truth in multi-bundle repos.
type logResult struct {
	Entries []logEntryJSON `json:"entries"`
	Note    string         `json:"note,omitempty"`
}

// marshalLogResult builds the get_log JSON response.
func marshalLogResult(entries []logEntryJSON, note string) (*mcp.CallToolResult, error) {
	if entries == nil {
		entries = []logEntryJSON{}
	}
	resp := logResult{Entries: entries, Note: note}
	out, err := json.Marshal(resp)
	if err != nil {
		return mcp.NewToolResultError("failed to marshal log result: " + err.Error()), nil
	}
	return mcp.NewToolResultText(string(out)), nil
}

func main() {
	validateFlag := flag.Bool("validate", false, "Validate document conformance and exit (no MCP server)")
	validatePath := flag.String("path", ".", "Path to validate (relative to cwd)")
	enableHidden := flag.Bool("enable-hidden", false, "Traverse hidden directories (except .git, .hg, .svn)")
	flag.Parse()

	if *validateFlag {
		runValidate(*validatePath, *enableHidden)
		return
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "okf-mcp: failed to get working directory: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "okf-mcp: serving %s\n", cwd)

	idx = index.New(cwd, scanner.ScanOptions{EnableHidden: *enableHidden})

	s := server.NewMCPServer("okf-mcp", "1.0.0",
	server.WithInstructions(
		"Use this server to query the repository's markdown documentation. "+
			"Call list_tags first to discover available topics and tags. "+
			"Then use get_doc(topic) to retrieve the relevant document. "+
			"Use validate_doc to check document conformance. "+
			"Use get_index to browse the documentation tree structure. "+
			"Use get_log to access structured change log entries. "+
			"Prefer these tools over reading files directly when looking for documentation.",
	),
	)
	s.AddTool(listTagsTool, listTagsHandler)
	s.AddTool(listDocsTool, listDocsHandler)
	s.AddTool(getDocTool, getDocHandler)
	s.AddTool(validateDocTool, validateDocHandler)
	s.AddTool(getIndexTool, getIndexHandler)
	s.AddTool(getLogTool, getLogHandler)

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "okf-mcp: %v\n", err)
		os.Exit(1)
	}
}

// runValidate validates OKF docs at the given path and prints findings to stderr.
func runValidate(path string, enableHidden bool) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "okf-mcp: invalid path: %v\n", err)
		os.Exit(2)
	}
	localIdx := index.New(absPath, scanner.ScanOptions{EnableHidden: enableHidden})
	if err := localIdx.Rebuild(); err != nil {
		fmt.Fprintf(os.Stderr, "okf-mcp: scan error: %v\n", err)
		os.Exit(2)
	}
	result := validator.ValidateBundle(localIdx)
	for _, f := range result.Findings {
		fmt.Fprintf(os.Stderr, "%s: [%s] %s: %s\n",
			f.File, f.Code, severityLabel(f.Severity), f.Message)
	}
	fmt.Fprintf(os.Stderr, "\n%d files: %d errors, %d warnings, %d notifications\n",
		result.Summary.Files, result.Summary.Errors,
		result.Summary.Warnings, result.Summary.Notifications)
	if result.Summary.Errors > 0 {
		os.Exit(1)
	}
}
