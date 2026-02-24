package lsp

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testServer creates a server connected to pipes for testing.
func testServer(t *testing.T) (*Server, *transport) {
	t.Helper()
	sr, cw := io.Pipe()
	cr, sw := io.Pipe()
	server := NewServer(sr, sw)
	client := newTransport(cr, cw)
	go func() {
		_ = server.Run()
	}()
	t.Cleanup(func() {
		cw.Close()
		sw.Close()
	})
	return server, client
}

// testProjectDir creates a temp directory with a dawn.toml so the LSP can open a project.
func testProjectDir(t *testing.T) (dir string, uri string) {
	t.Helper()
	dir = t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "dawn.toml"), []byte("[project]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir, "file://" + dir
}

// initAndOpenProject creates a project dir, initializes the server, opens a BUILD.dawn, and drains diagnostics.
func initAndOpenProject(t *testing.T, client *transport, text string) (dir string, uri string) {
	t.Helper()
	dir, rootURI := testProjectDir(t)
	buildPath := filepath.Join(dir, "BUILD.dawn")
	if err := os.WriteFile(buildPath, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	uri = "file://" + buildPath
	_ = sendRequest(t, client, 1, "initialize", InitializeParams{
		RootURI: rootURI,
	})
	sendNotification(t, client, "textDocument/didOpen", DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        uri,
			LanguageID: "starlark",
			Version:    1,
			Text:       text,
		},
	})
	_, _ = client.read() // drain diagnostics
	return dir, uri
}

func sendRequest(t *testing.T, client *transport, id int, method string, params interface{}) *jsonrpcMessage {
	t.Helper()
	data, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	rawID := json.RawMessage(mustMarshal(t, id))
	err = client.write(&jsonrpcMessage{
		ID:     &rawID,
		Method: method,
		Params: data,
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.read()
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func sendNotification(t *testing.T, client *transport, method string, params interface{}) {
	t.Helper()
	data, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	err = client.write(&jsonrpcMessage{
		Method: method,
		Params: data,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func mustMarshal(t *testing.T, v interface{}) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestInitialize(t *testing.T) {
	t.Parallel()
	_, client := testServer(t)

	resp := sendRequest(t, client, 1, "initialize", InitializeParams{
		RootURI: "file:///tmp/test",
	})

	if resp.Error != nil {
		t.Fatalf("initialize failed: %s", resp.Error.Message)
	}

	var result InitializeResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}

	if result.Capabilities.TextDocumentSync == nil {
		t.Fatal("expected TextDocumentSync capability")
	}
	if !result.Capabilities.HoverProvider {
		t.Error("expected hover provider")
	}
	if !result.Capabilities.DefinitionProvider {
		t.Error("expected definition provider")
	}
	if result.Capabilities.CompletionProvider == nil {
		t.Error("expected completion provider")
	}
	if result.ServerInfo == nil || result.ServerInfo.Name != "dawn-lsp" {
		t.Error("expected server info")
	}
}

func TestDiagnostics(t *testing.T) {
	t.Parallel()
	_, client := testServer(t)

	// Initialize
	_ = sendRequest(t, client, 1, "initialize", InitializeParams{
		RootURI: "file:///tmp/test",
	})

	// Open a document with a parse error
	sendNotification(t, client, "textDocument/didOpen", DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        "file:///tmp/test/BUILD.dawn",
			LanguageID: "starlark",
			Version:    1,
			Text:       "def foo(:\n  pass\n",
		},
	})

	// Read the diagnostics notification
	msg, err := client.read()
	if err != nil {
		t.Fatal(err)
	}
	if msg.Method != "textDocument/publishDiagnostics" {
		t.Fatalf("expected publishDiagnostics, got %s", msg.Method)
	}

	var diags PublishDiagnosticsParams
	if err := json.Unmarshal(msg.Params, &diags); err != nil {
		t.Fatal(err)
	}

	if len(diags.Diagnostics) == 0 {
		t.Error("expected diagnostics for parse error")
	}
}

func TestDiagnosticsResolveError(t *testing.T) {
	t.Parallel()
	_, client := testServer(t)

	dir, rootURI := testProjectDir(t)
	buildPath := filepath.Join(dir, "BUILD.dawn")
	text := "x = undefined_name\n"
	if err := os.WriteFile(buildPath, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}

	_ = sendRequest(t, client, 1, "initialize", InitializeParams{
		RootURI: rootURI,
	})

	// Open a document with an undefined name
	sendNotification(t, client, "textDocument/didOpen", DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        "file://" + buildPath,
			LanguageID: "starlark",
			Version:    1,
			Text:       text,
		},
	})

	msg, err := client.read()
	if err != nil {
		t.Fatal(err)
	}

	var diags PublishDiagnosticsParams
	if err := json.Unmarshal(msg.Params, &diags); err != nil {
		t.Fatal(err)
	}

	if len(diags.Diagnostics) == 0 {
		t.Error("expected diagnostics for undefined name")
	}
}

func TestNoDiagnosticsForValidFile(t *testing.T) {
	t.Parallel()
	_, client := testServer(t)

	dir, rootURI := testProjectDir(t)
	buildPath := filepath.Join(dir, "BUILD.dawn")
	text := "x = glob([\"*.go\"])\ny = path(\":foo\")\n"
	if err := os.WriteFile(buildPath, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}

	_ = sendRequest(t, client, 1, "initialize", InitializeParams{
		RootURI: rootURI,
	})

	// Open a valid document that uses Dawn builtins
	sendNotification(t, client, "textDocument/didOpen", DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        "file://" + buildPath,
			LanguageID: "starlark",
			Version:    1,
			Text:       text,
		},
	})

	msg, err := client.read()
	if err != nil {
		t.Fatal(err)
	}

	var diags PublishDiagnosticsParams
	if err := json.Unmarshal(msg.Params, &diags); err != nil {
		t.Fatal(err)
	}

	if len(diags.Diagnostics) != 0 {
		t.Errorf("expected no diagnostics for valid file, got %d: %v", len(diags.Diagnostics), diags.Diagnostics)
	}
}

func TestCompletion(t *testing.T) {
	t.Parallel()
	_, client := testServer(t)

	uri := initAndOpen(t, client, "x = \n")

	resp := sendRequest(t, client, 2, "textDocument/completion", CompletionParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     Position{Line: 0, Character: 4},
		},
	})

	if resp.Error != nil {
		t.Fatalf("completion failed: %s", resp.Error.Message)
	}

	var result CompletionList
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}

	if len(result.Items) == 0 {
		t.Error("expected completion items")
	}

	// Check that Dawn builtins are present
	found := map[string]bool{}
	for _, item := range result.Items {
		found[item.Label] = true
	}
	for _, name := range []string{"target", "glob", "path", "sh", "os", "json"} {
		if !found[name] {
			t.Errorf("expected completion item %q", name)
		}
	}
}

func TestHover(t *testing.T) {
	t.Parallel()
	_, client := testServer(t)

	_, uri := initAndOpenProject(t, client, "x = glob([\"*.go\"])\n")

	resp := sendRequest(t, client, 2, "textDocument/hover", TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 0, Character: 5}, // on "glob"
	})

	if resp.Error != nil {
		t.Fatalf("hover failed: %s", resp.Error.Message)
	}

	var result Hover
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}

	if result.Contents.Value == "" {
		t.Error("expected hover contents")
	}
}

func TestDefinition(t *testing.T) {
	t.Parallel()
	_, client := testServer(t)

	_, uri := initAndOpenProject(t, client, "def foo():\n  pass\n\nbar = foo\n")

	// Go to definition of "foo" on line 3 (the reference)
	resp := sendRequest(t, client, 2, "textDocument/definition", TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 3, Character: 6}, // on "foo" in "bar = foo"
	})

	if resp.Error != nil {
		t.Fatalf("definition failed: %s", resp.Error.Message)
	}

	var result Location
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}

	if result.Range.Start.Line != 0 {
		t.Errorf("expected definition on line 0, got %d", result.Range.Start.Line)
	}
}

func TestDocumentSymbols(t *testing.T) {
	t.Parallel()
	_, client := testServer(t)

	_, uri := initAndOpenProject(t, client, "@target\ndef build():\n  pass\n\ndef helper():\n  pass\n\nVERSION = \"1.0\"\n")

	resp := sendRequest(t, client, 2, "textDocument/documentSymbol", DocumentSymbolParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
	})

	if resp.Error != nil {
		t.Fatalf("document symbols failed: %s", resp.Error.Message)
	}

	var result []DocumentSymbol
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}

	if len(result) < 3 {
		t.Fatalf("expected at least 3 symbols, got %d", len(result))
	}

	// Check types: build should be Class (target), helper should be Function, VERSION should be Variable
	symbolMap := map[string]DocumentSymbol{}
	for _, s := range result {
		symbolMap[s.Name] = s
	}

	if sym, ok := symbolMap["build"]; ok {
		if sym.Kind != symbolKindClass {
			t.Errorf("expected build to be Class kind, got %d", sym.Kind)
		}
	} else {
		t.Error("expected build symbol")
	}

	if sym, ok := symbolMap["helper"]; ok {
		if sym.Kind != symbolKindFunction {
			t.Errorf("expected helper to be Function kind, got %d", sym.Kind)
		}
	} else {
		t.Error("expected helper symbol")
	}

	if sym, ok := symbolMap["VERSION"]; ok {
		if sym.Kind != symbolKindVariable {
			t.Errorf("expected VERSION to be Variable kind, got %d", sym.Kind)
		}
	} else {
		t.Error("expected VERSION symbol")
	}
}

func TestSemanticTokens(t *testing.T) {
	t.Parallel()
	_, client := testServer(t)

	_, uri := initAndOpenProject(t, client, "@target\ndef build():\n  sh.exec(\"go build\")\n")

	resp := sendRequest(t, client, 2, "textDocument/semanticTokens/full", SemanticTokensParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
	})

	if resp.Error != nil {
		t.Fatalf("semantic tokens failed: %s", resp.Error.Message)
	}

	var result SemanticTokensResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}

	if len(result.Data) == 0 {
		t.Error("expected semantic token data")
	}
	if len(result.Data)%5 != 0 {
		t.Errorf("semantic token data length %d not divisible by 5", len(result.Data))
	}
}

func TestReferences(t *testing.T) {
	t.Parallel()
	_, client := testServer(t)

	_, uri := initAndOpenProject(t, client, "def foo():\n  pass\n\nx = foo\ny = foo()\n")

	resp := sendRequest(t, client, 2, "textDocument/references", ReferenceParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     Position{Line: 0, Character: 4}, // on "foo" in def
		},
	})

	if resp.Error != nil {
		t.Fatalf("references failed: %s", resp.Error.Message)
	}

	var result []Location
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}

	// Should find at least: def foo, foo in "x = foo", foo in "y = foo()"
	if len(result) < 3 {
		t.Errorf("expected at least 3 references, got %d", len(result))
	}
}

func TestSignatureHelp(t *testing.T) {
	t.Parallel()
	_, client := testServer(t)

	_, uri := initAndOpenProject(t, client, "x = glob([\"*.go\"], )\n")

	resp := sendRequest(t, client, 2, "textDocument/signatureHelp", SignatureHelpParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     Position{Line: 0, Character: 19}, // inside glob()
		},
	})

	if resp.Error != nil {
		t.Fatalf("signature help failed: %s", resp.Error.Message)
	}

	// May be null if cursor position doesn't match
	if string(resp.Result) != "null" {
		var result SignatureHelp
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatal(err)
		}
		if len(result.Signatures) > 0 && result.Signatures[0].Label == "" {
			t.Error("expected signature label")
		}
	}
}

// --- Comprehensive tests for all LSP features ---

// initAndOpen is a test helper that initializes the server with a project, opens a document, and drains diagnostics.
func initAndOpen(t *testing.T, client *transport, text string) string {
	t.Helper()
	_, uri := initAndOpenProject(t, client, text)
	return uri
}

func TestDefinitionOfVariable(t *testing.T) {
	t.Parallel()
	_, client := testServer(t)

	// version is defined on line 0, used on line 1
	uri := initAndOpen(t, client, "version = \"1.0\"\nldflags = version\n")

	resp := sendRequest(t, client, 2, "textDocument/definition", TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 1, Character: 10}, // on "version" in "ldflags = version"
	})
	if resp.Error != nil {
		t.Fatalf("definition failed: %s", resp.Error.Message)
	}

	var result Location
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}
	if result.Range.Start.Line != 0 || result.Range.Start.Character != 0 {
		t.Errorf("expected definition at (0,0), got (%d,%d)", result.Range.Start.Line, result.Range.Start.Character)
	}
}

func TestDefinitionOfParameter(t *testing.T) {
	t.Parallel()
	_, client := testServer(t)

	// Parameter "name" defined in def signature, used in body
	uri := initAndOpen(t, client, "def greet(name):\n  x = name\n")

	resp := sendRequest(t, client, 2, "textDocument/definition", TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 1, Character: 6}, // on "name" in "x = name"
	})
	if resp.Error != nil {
		t.Fatalf("definition failed: %s", resp.Error.Message)
	}

	var result Location
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}
	// Parameter "name" is at line 0, character 10
	if result.Range.Start.Line != 0 || result.Range.Start.Character != 10 {
		t.Errorf("expected parameter definition at (0,10), got (%d,%d)", result.Range.Start.Line, result.Range.Start.Character)
	}
}

func TestDefinitionInDecorator(t *testing.T) {
	t.Parallel()
	_, client := testServer(t)

	// "all_go_sources" is defined on line 0, referenced inside the decorator on line 1
	uri := initAndOpen(t, client, "all_go_sources = glob([\"**/*.go\"])\n@target(sources=all_go_sources)\ndef build():\n  pass\n")

	// Go to definition of "all_go_sources" inside the decorator expression
	resp := sendRequest(t, client, 2, "textDocument/definition", TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 1, Character: 17}, // on "all_go_sources" in @target(sources=all_go_sources)
	})
	if resp.Error != nil {
		t.Fatalf("definition failed: %s", resp.Error.Message)
	}

	if string(resp.Result) == "null" {
		t.Fatal("expected non-null definition for identifier in decorator")
	}

	var result Location
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}
	if result.Range.Start.Line != 0 || result.Range.Start.Character != 0 {
		t.Errorf("expected definition at (0,0), got (%d,%d)", result.Range.Start.Line, result.Range.Start.Character)
	}
}

func TestReferencesInDecorator(t *testing.T) {
	t.Parallel()
	_, client := testServer(t)

	// "all_go_sources" used in decorator and in a body
	uri := initAndOpen(t, client, "all_go_sources = glob([\"**/*.go\"])\n@target(sources=all_go_sources)\ndef build():\n  x = all_go_sources\n")

	// Find references of "all_go_sources" from its definition
	resp := sendRequest(t, client, 2, "textDocument/references", ReferenceParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     Position{Line: 0, Character: 0}, // on "all_go_sources" definition
		},
	})
	if resp.Error != nil {
		t.Fatalf("references failed: %s", resp.Error.Message)
	}

	var result []Location
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}

	// Should find: definition on line 0, decorator on line 1, body on line 3
	if len(result) < 3 {
		t.Errorf("expected at least 3 references (def + decorator + body), got %d", len(result))
		for i, loc := range result {
			t.Logf("  ref[%d]: line=%d, char=%d", i, loc.Range.Start.Line, loc.Range.Start.Character)
		}
	}
}

func TestDefinitionOfForLoopVariable(t *testing.T) {
	t.Parallel()
	_, client := testServer(t)

	// "item" defined in for loop, used in body
	uri := initAndOpen(t, client, "def process():\n  for item in [1, 2, 3]:\n    x = item\n")

	resp := sendRequest(t, client, 2, "textDocument/definition", TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 2, Character: 8}, // on "item" in "x = item"
	})
	if resp.Error != nil {
		t.Fatalf("definition failed: %s", resp.Error.Message)
	}

	var result Location
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}
	// "item" is defined in the for clause on line 1
	if result.Range.Start.Line != 1 {
		t.Errorf("expected for-loop variable definition on line 1, got %d", result.Range.Start.Line)
	}
}

func TestDefinitionCrossFile(t *testing.T) {
	t.Parallel()

	// Create a temp directory with two BUILD.dawn files
	tmpDir := t.TempDir()
	libDir := filepath.Join(tmpDir, "lib")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write dawn.toml so project root is discovered
	if err := os.WriteFile(filepath.Join(tmpDir, "dawn.toml"), []byte("[project]\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Write lib/BUILD.dawn with a function definition
	libBuild := "def helper(x, y):\n  return x + y\n"
	if err := os.WriteFile(filepath.Join(libDir, "BUILD.dawn"), []byte(libBuild), 0o600); err != nil {
		t.Fatal(err)
	}

	// The main BUILD.dawn loads from lib
	mainBuild := "load(\"//lib\", \"helper\")\nresult = helper(1, 2)\n"
	mainPath := filepath.Join(tmpDir, "BUILD.dawn")
	if err := os.WriteFile(mainPath, []byte(mainBuild), 0o600); err != nil {
		t.Fatal(err)
	}

	_, client := testServer(t)
	rootURI := "file://" + tmpDir
	_ = sendRequest(t, client, 1, "initialize", InitializeParams{
		RootURI: rootURI,
	})

	mainURI := "file://" + mainPath
	sendNotification(t, client, "textDocument/didOpen", DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        mainURI,
			LanguageID: "starlark",
			Version:    1,
			Text:       mainBuild,
		},
	})
	_, _ = client.read() // diagnostics

	// Go to definition of "helper" on line 1 (the usage)
	resp := sendRequest(t, client, 2, "textDocument/definition", TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: mainURI},
		Position:     Position{Line: 1, Character: 9}, // on "helper" in "result = helper(1, 2)"
	})
	if resp.Error != nil {
		t.Fatalf("definition failed: %s", resp.Error.Message)
	}

	if string(resp.Result) == "null" {
		t.Fatal("expected non-null definition result for loaded name")
	}

	var result Location
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}

	// Should jump to lib/BUILD.dawn line 0 (def helper)
	expectedURI := "file://" + filepath.Join(libDir, "BUILD.dawn")
	if result.URI != expectedURI {
		t.Errorf("expected definition in %s, got %s", expectedURI, result.URI)
	}
	if result.Range.Start.Line != 0 {
		t.Errorf("expected definition on line 0, got %d", result.Range.Start.Line)
	}
}

func TestDefinitionCrossFileGlobalVar(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	libDir := filepath.Join(tmpDir, "lib")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "dawn.toml"), []byte("[project]\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// lib/BUILD.dawn has a global variable (not a function)
	libBuild := "my_var = \"hello\"\n"
	if err := os.WriteFile(filepath.Join(libDir, "BUILD.dawn"), []byte(libBuild), 0o600); err != nil {
		t.Fatal(err)
	}

	mainBuild := "load(\"//lib\", \"my_var\")\nx = my_var\n"
	mainPath := filepath.Join(tmpDir, "BUILD.dawn")
	if err := os.WriteFile(mainPath, []byte(mainBuild), 0o600); err != nil {
		t.Fatal(err)
	}

	_, client := testServer(t)
	_ = sendRequest(t, client, 1, "initialize", InitializeParams{
		RootURI: "file://" + tmpDir,
	})

	mainURI := "file://" + mainPath
	sendNotification(t, client, "textDocument/didOpen", DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        mainURI,
			LanguageID: "starlark",
			Version:    1,
			Text:       mainBuild,
		},
	})
	_, _ = client.read()

	// Go to definition of "my_var" on line 1
	resp := sendRequest(t, client, 2, "textDocument/definition", TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: mainURI},
		Position:     Position{Line: 1, Character: 4}, // on "my_var"
	})
	if resp.Error != nil {
		t.Fatalf("definition failed: %s", resp.Error.Message)
	}

	if string(resp.Result) == "null" {
		t.Fatal("expected non-null definition for loaded global variable")
	}

	var result Location
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}

	expectedURI := "file://" + filepath.Join(libDir, "BUILD.dawn")
	if result.URI != expectedURI {
		t.Errorf("expected definition in %s, got %s", expectedURI, result.URI)
	}
	if result.Range.Start.Line != 0 {
		t.Errorf("expected definition on line 0, got %d", result.Range.Start.Line)
	}
}

func TestReferencesVariable(t *testing.T) {
	t.Parallel()
	_, client := testServer(t)

	// "version" is assigned on line 0 and used on lines 1 and 2
	uri := initAndOpen(t, client,
"version = \"1.0\"\nldflags = version\nx = version\n")

	resp := sendRequest(t, client, 2, "textDocument/references", ReferenceParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     Position{Line: 0, Character: 0}, // on "version" definition
		},
	})
	if resp.Error != nil {
		t.Fatalf("references failed: %s", resp.Error.Message)
	}

	var result []Location
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}

	if len(result) < 3 {
		t.Errorf("expected at least 3 references for 'version', got %d", len(result))
	}
}

func TestReferencesParameter(t *testing.T) {
	t.Parallel()
	_, client := testServer(t)

	// "x" is a parameter used twice in body
	uri := initAndOpen(t, client,
"def add(x, y):\n  z = x + y\n  return x\n")

	// Find references of "x" at parameter definition
	resp := sendRequest(t, client, 2, "textDocument/references", ReferenceParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     Position{Line: 0, Character: 8}, // on "x" parameter
		},
	})
	if resp.Error != nil {
		t.Fatalf("references failed: %s", resp.Error.Message)
	}

	var result []Location
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}

	// Should find: param "x", "x" in "x + y", "x" in "return x" = 3 references
	if len(result) < 3 {
		t.Errorf("expected at least 3 references for parameter 'x', got %d", len(result))
	}
}

func TestReferencesBuiltin(t *testing.T) {
	t.Parallel()
	_, client := testServer(t)

	uri := initAndOpen(t, client,
"x = glob([\"*.go\"])\ny = glob([\"*.py\"])\n")

	resp := sendRequest(t, client, 2, "textDocument/references", ReferenceParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     Position{Line: 0, Character: 4}, // on "glob"
		},
	})
	if resp.Error != nil {
		t.Fatalf("references failed: %s", resp.Error.Message)
	}

	var result []Location
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}

	if len(result) < 2 {
		t.Errorf("expected at least 2 references for 'glob', got %d", len(result))
	}
}

func TestSignatureHelpBuiltin(t *testing.T) {
	t.Parallel()
	_, client := testServer(t)

	// Cursor inside target() call
	uri := initAndOpen(t, client,
"x = target(name=\"foo\", )\n")

	resp := sendRequest(t, client, 2, "textDocument/signatureHelp", SignatureHelpParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     Position{Line: 0, Character: 22}, // after comma inside target()
		},
	})
	if resp.Error != nil {
		t.Fatalf("signature help failed: %s", resp.Error.Message)
	}

	if string(resp.Result) == "null" {
		t.Fatal("expected non-null signature help for target()")
	}

	var result SignatureHelp
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}

	if len(result.Signatures) == 0 {
		t.Fatal("expected at least one signature")
	}
	if !strings.Contains(result.Signatures[0].Label, "target") {
		t.Errorf("expected signature label to contain 'target', got %q", result.Signatures[0].Label)
	}
	if len(result.Signatures[0].Parameters) == 0 {
		t.Error("expected parameters in signature")
	}
}

func TestSignatureHelpUserDefined(t *testing.T) {
	t.Parallel()
	_, client := testServer(t)

	uri := initAndOpen(t, client,
"def my_func(a, b, c=None):\n  pass\n\nresult = my_func(1, )\n")

	resp := sendRequest(t, client, 2, "textDocument/signatureHelp", SignatureHelpParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     Position{Line: 3, Character: 20}, // inside my_func()
		},
	})
	if resp.Error != nil {
		t.Fatalf("signature help failed: %s", resp.Error.Message)
	}

	if string(resp.Result) == "null" {
		t.Fatal("expected non-null signature help for user function")
	}

	var result SignatureHelp
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}

	if len(result.Signatures) == 0 {
		t.Fatal("expected at least one signature")
	}
	if !strings.Contains(result.Signatures[0].Label, "my_func") {
		t.Errorf("expected signature label to contain 'my_func', got %q", result.Signatures[0].Label)
	}
	if len(result.Signatures[0].Parameters) != 3 {
		t.Errorf("expected 3 parameters, got %d", len(result.Signatures[0].Parameters))
	}
}

func TestSignatureHelpCrossFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	libDir := filepath.Join(tmpDir, "lib")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "dawn.toml"), []byte("[project]\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	libBuild := "def build_thing(src, out, flags=None):\n  pass\n"
	if err := os.WriteFile(filepath.Join(libDir, "BUILD.dawn"), []byte(libBuild), 0o600); err != nil {
		t.Fatal(err)
	}

	mainBuild := "load(\"//lib\", \"build_thing\")\nresult = build_thing(\"main.go\", )\n"
	mainPath := filepath.Join(tmpDir, "BUILD.dawn")
	if err := os.WriteFile(mainPath, []byte(mainBuild), 0o600); err != nil {
		t.Fatal(err)
	}

	_, client := testServer(t)
	_ = sendRequest(t, client, 1, "initialize", InitializeParams{
		RootURI: "file://" + tmpDir,
	})

	mainURI := "file://" + mainPath
	sendNotification(t, client, "textDocument/didOpen", DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        mainURI,
			LanguageID: "starlark",
			Version:    1,
			Text:       mainBuild,
		},
	})
	_, _ = client.read()

	resp := sendRequest(t, client, 2, "textDocument/signatureHelp", SignatureHelpParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: mainURI},
			Position:     Position{Line: 1, Character: 32}, // inside build_thing()
		},
	})
	if resp.Error != nil {
		t.Fatalf("signature help failed: %s", resp.Error.Message)
	}

	if string(resp.Result) == "null" {
		t.Fatal("expected non-null signature help for cross-file function")
	}

	var result SignatureHelp
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}

	if len(result.Signatures) == 0 {
		t.Fatal("expected at least one signature")
	}
	if !strings.Contains(result.Signatures[0].Label, "build_thing") {
		t.Errorf("expected signature label to contain 'build_thing', got %q", result.Signatures[0].Label)
	}
	if len(result.Signatures[0].Parameters) != 3 {
		t.Errorf("expected 3 parameters (src, out, flags), got %d", len(result.Signatures[0].Parameters))
	}
}

func TestCompletionDotAccess(t *testing.T) {
	t.Parallel()
	_, client := testServer(t)

	uri := initAndOpen(t, client,
"x = sh.\n")

	resp := sendRequest(t, client, 2, "textDocument/completion", CompletionParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     Position{Line: 0, Character: 7}, // after "sh."
		},
	})
	if resp.Error != nil {
		t.Fatalf("completion failed: %s", resp.Error.Message)
	}

	var result CompletionList
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}

	found := map[string]bool{}
	for _, item := range result.Items {
		found[item.Label] = true
	}
	for _, name := range []string{"exec", "output"} {
		if !found[name] {
			t.Errorf("expected completion item %q for sh.", name)
		}
	}
}

func TestCompletionNestedDotAccess(t *testing.T) {
	t.Parallel()
	_, client := testServer(t)

	uri := initAndOpen(t, client,
"x = os.path.\n")

	resp := sendRequest(t, client, 2, "textDocument/completion", CompletionParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     Position{Line: 0, Character: 12}, // after "os.path."
		},
	})
	if resp.Error != nil {
		t.Fatalf("completion failed: %s", resp.Error.Message)
	}

	var result CompletionList
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}

	found := map[string]bool{}
	for _, item := range result.Items {
		found[item.Label] = true
	}
	for _, name := range []string{"join", "dir", "base", "ext"} {
		if !found[name] {
			t.Errorf("expected completion item %q for os.path.", name)
		}
	}
}

func TestCompletionKeywordArgs(t *testing.T) {
	t.Parallel()
	_, client := testServer(t)

	// Cursor inside target() call, after the opening paren
	uri := initAndOpen(t, client,
"x = target( )\n")

	resp := sendRequest(t, client, 2, "textDocument/completion", CompletionParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     Position{Line: 0, Character: 12}, // inside target()
		},
	})
	if resp.Error != nil {
		t.Fatalf("completion failed: %s", resp.Error.Message)
	}

	var result CompletionList
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}

	found := map[string]bool{}
	for _, item := range result.Items {
		found[item.Label] = true
	}

	// Should include keyword arg completions for target()
	for _, name := range []string{"name", "deps", "sources"} {
		if !found[name] {
			t.Errorf("expected keyword argument completion %q for target()", name)
		}
	}
}

func TestCompletionKeywordArgsFilterUsed(t *testing.T) {
	t.Parallel()
	_, client := testServer(t)

	// target() call with name= already used
	uri := initAndOpen(t, client,
"x = target(name=\"foo\", )\n")

	resp := sendRequest(t, client, 2, "textDocument/completion", CompletionParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     Position{Line: 0, Character: 22}, // after comma
		},
	})
	if resp.Error != nil {
		t.Fatalf("completion failed: %s", resp.Error.Message)
	}

	var result CompletionList
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}

	// "name" should NOT appear since it's already used
	for _, item := range result.Items {
		if item.Label == "name" && item.Detail == "parameter" {
			t.Error("'name' keyword arg should be filtered out since it's already used")
		}
	}
}

func TestHoverDotExpr(t *testing.T) {
	t.Parallel()
	_, client := testServer(t)

	uri := initAndOpen(t, client,
"x = sh.exec(\"ls\")\n")

	resp := sendRequest(t, client, 2, "textDocument/hover", TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 0, Character: 8}, // on "exec" in "sh.exec"
	})
	if resp.Error != nil {
		t.Fatalf("hover failed: %s", resp.Error.Message)
	}

	if string(resp.Result) == "null" {
		t.Fatal("expected non-null hover for sh.exec")
	}

	var result Hover
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Contents.Value, "sh.exec") {
		t.Errorf("expected hover to contain 'sh.exec', got %q", result.Contents.Value)
	}
}

func TestHoverNestedDotExpr(t *testing.T) {
	t.Parallel()
	_, client := testServer(t)

	uri := initAndOpen(t, client,
"x = os.path.join(\"a\", \"b\")\n")

	resp := sendRequest(t, client, 2, "textDocument/hover", TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 0, Character: 13}, // on "join" in "os.path.join"
	})
	if resp.Error != nil {
		t.Fatalf("hover failed: %s", resp.Error.Message)
	}

	if string(resp.Result) == "null" {
		t.Fatal("expected non-null hover for os.path.join")
	}

	var result Hover
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Contents.Value, "os.path.join") {
		t.Errorf("expected hover to contain 'os.path.join', got %q", result.Contents.Value)
	}
}

func TestHoverUserFunction(t *testing.T) {
	t.Parallel()
	_, client := testServer(t)

	uri := initAndOpen(t, client,
"def my_func(a, b):\n  \"\"\"Does something useful.\"\"\"\n  pass\n\nx = my_func\n")

	resp := sendRequest(t, client, 2, "textDocument/hover", TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 4, Character: 5}, // on "my_func" in "x = my_func"
	})
	if resp.Error != nil {
		t.Fatalf("hover failed: %s", resp.Error.Message)
	}

	if string(resp.Result) == "null" {
		t.Fatal("expected non-null hover for user function")
	}

	var result Hover
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Contents.Value, "my_func") {
		t.Errorf("expected hover to mention 'my_func', got %q", result.Contents.Value)
	}
	if !strings.Contains(result.Contents.Value, "Does something useful") {
		t.Errorf("expected hover to contain docstring, got %q", result.Contents.Value)
	}
}

func TestDocumentSymbolsComprehensive(t *testing.T) {
	t.Parallel()
	_, client := testServer(t)

	uri := initAndOpen(t, client,
"load(\"//lib\", \"helper\")\n\nversion = \"1.0\"\n\n@target\ndef build():\n  pass\n\ndef helper_fn():\n  pass\n")

	resp := sendRequest(t, client, 2, "textDocument/documentSymbol", DocumentSymbolParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
	})
	if resp.Error != nil {
		t.Fatalf("document symbols failed: %s", resp.Error.Message)
	}

	var result []DocumentSymbol
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}

	symbolMap := map[string]DocumentSymbol{}
	for _, s := range result {
		symbolMap[s.Name] = s
	}

	// load statement should appear as module
	if _, ok := symbolMap["//lib"]; !ok {
		t.Error("expected load statement as module symbol")
	}

	// version should appear as variable
	if sym, ok := symbolMap["version"]; ok {
		if sym.Kind != symbolKindVariable {
			t.Errorf("expected 'version' to be Variable kind, got %d", sym.Kind)
		}
	} else {
		t.Error("expected 'version' symbol")
	}

	// build should appear as class (target)
	if sym, ok := symbolMap["build"]; ok {
		if sym.Kind != symbolKindClass {
			t.Errorf("expected 'build' to be Class kind, got %d", sym.Kind)
		}
	} else {
		t.Error("expected 'build' symbol")
	}

	// helper_fn should appear as function
	if sym, ok := symbolMap["helper_fn"]; ok {
		if sym.Kind != symbolKindFunction {
			t.Errorf("expected 'helper_fn' to be Function kind, got %d", sym.Kind)
		}
	} else {
		t.Error("expected 'helper_fn' symbol")
	}
}

func TestDiagnosticsMultipleErrors(t *testing.T) {
	t.Parallel()
	_, client := testServer(t)

	dir, rootURI := testProjectDir(t)
	buildPath := filepath.Join(dir, "BUILD.dawn")
	text := "x = undefined1\ny = undefined2\nz = undefined3\n"
	if err := os.WriteFile(buildPath, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}

	_ = sendRequest(t, client, 1, "initialize", InitializeParams{
		RootURI: rootURI,
	})

	// File with multiple undefined names
	sendNotification(t, client, "textDocument/didOpen", DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        "file://" + buildPath,
			LanguageID: "starlark",
			Version:    1,
			Text:       text,
		},
	})

	msg, err := client.read()
	if err != nil {
		t.Fatal(err)
	}

	var diags PublishDiagnosticsParams
	if err := json.Unmarshal(msg.Params, &diags); err != nil {
		t.Fatal(err)
	}

	if len(diags.Diagnostics) < 3 {
		t.Errorf("expected at least 3 diagnostics, got %d", len(diags.Diagnostics))
	}

	// Each diagnostic should have correct line numbers
	for i, d := range diags.Diagnostics {
		if d.Range.Start.Line != i {
			t.Errorf("diagnostic %d: expected line %d, got %d", i, i, d.Range.Start.Line)
		}
	}
}

func TestDiagnosticsClearedOnClose(t *testing.T) {
	t.Parallel()
	_, client := testServer(t)

	dir, rootURI := testProjectDir(t)
	buildPath := filepath.Join(dir, "BUILD.dawn")
	text := "x = undefined\n"
	if err := os.WriteFile(buildPath, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}

	_ = sendRequest(t, client, 1, "initialize", InitializeParams{
		RootURI: rootURI,
	})

	uri := "file://" + buildPath
	sendNotification(t, client, "textDocument/didOpen", DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        uri,
			LanguageID: "starlark",
			Version:    1,
			Text:       text,
		},
	})
	_, _ = client.read() // read diagnostics (has errors)

	// Close the document
	sendNotification(t, client, "textDocument/didClose", DidCloseTextDocumentParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
	})

	// Should receive empty diagnostics
	msg, err := client.read()
	if err != nil {
		t.Fatal(err)
	}

	var diags PublishDiagnosticsParams
	if err := json.Unmarshal(msg.Params, &diags); err != nil {
		t.Fatal(err)
	}

	if len(diags.Diagnostics) != 0 {
		t.Errorf("expected 0 diagnostics after close, got %d", len(diags.Diagnostics))
	}
}

func TestSemanticTokensComprehensive(t *testing.T) {
	t.Parallel()
	_, client := testServer(t)

	uri := initAndOpen(t, client,
"@target\ndef build():\n  x = glob([\"*.go\"])\n  sh.exec(\"go build\")\n")

	resp := sendRequest(t, client, 2, "textDocument/semanticTokens/full", SemanticTokensParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
	})
	if resp.Error != nil {
		t.Fatalf("semantic tokens failed: %s", resp.Error.Message)
	}

	var result SemanticTokensResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}

	if len(result.Data) == 0 {
		t.Fatal("expected semantic token data")
	}
	if len(result.Data)%5 != 0 {
		t.Fatalf("semantic token data length %d not divisible by 5", len(result.Data))
	}

	// Decode tokens and verify we have various types
	type decodedToken struct {
		line, col, length, tokenType, modifiers uint32
	}
	var tokens []decodedToken
	var prevLine, prevCol uint32
	for i := 0; i < len(result.Data); i += 5 {
		dl := result.Data[i]
		dc := result.Data[i+1]
		line := prevLine + dl
		col := dc
		if dl == 0 {
			col = prevCol + dc
		}
		tokens = append(tokens, decodedToken{
			line:      line,
			col:       col,
			length:    result.Data[i+2],
			tokenType: result.Data[i+3],
			modifiers: result.Data[i+4],
		})
		prevLine = line
		prevCol = col
	}

	// Should have tokens for: @(decorator), target(macro), build(function), glob(macro), sh(macro)
	hasDecorator := false
	hasMacro := false
	hasFunction := false
	for _, tok := range tokens {
		switch tok.tokenType {
		case tokenTypeDecorator:
			hasDecorator = true
		case tokenTypeMacro:
			hasMacro = true
		case tokenTypeFunction:
			hasFunction = true
		}
	}

	if !hasDecorator {
		t.Error("expected at least one decorator token")
	}
	if !hasMacro {
		t.Error("expected at least one macro token (for builtins)")
	}
	if !hasFunction {
		t.Error("expected at least one function token")
	}
}

func TestCompletionIncludesLoadedNames(t *testing.T) {
	t.Parallel()
	_, client := testServer(t)

	uri := initAndOpen(t, client,
"load(\"//lib\", \"helper\")\n\ndef build():\n  pass\n\nx = None\n")

	resp := sendRequest(t, client, 2, "textDocument/completion", CompletionParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     Position{Line: 5, Character: 4}, // on "None" in "x = None"
		},
	})
	if resp.Error != nil {
		t.Fatalf("completion failed: %s", resp.Error.Message)
	}

	var result CompletionList
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}

	found := map[string]bool{}
	for _, item := range result.Items {
		found[item.Label] = true
	}

	// Should include loaded names
	if !found["helper"] {
		t.Error("expected 'helper' (loaded name) in completions")
	}
	// Should include user-defined functions
	if !found["build"] {
		t.Error("expected 'build' (user function) in completions")
	}
	// Should include builtins
	if !found["glob"] {
		t.Error("expected 'glob' (builtin) in completions")
	}
	// Should include keywords
	if !found["def"] {
		t.Error("expected 'def' (keyword) in completions")
	}
}

func TestDocumentChange(t *testing.T) {
	t.Parallel()
	_, client := testServer(t)

	dir, rootURI := testProjectDir(t)
	buildPath := filepath.Join(dir, "BUILD.dawn")
	text := "x = undefined\n"
	if err := os.WriteFile(buildPath, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}

	uri := "file://" + buildPath
	_ = sendRequest(t, client, 1, "initialize", InitializeParams{
		RootURI: rootURI,
	})

	// Open with error
	sendNotification(t, client, "textDocument/didOpen", DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        uri,
			LanguageID: "starlark",
			Version:    1,
			Text:       text,
		},
	})
	msg, _ := client.read() // diagnostics with error
	var diags1 PublishDiagnosticsParams
	_ = json.Unmarshal(msg.Params, &diags1)
	if len(diags1.Diagnostics) == 0 {
		t.Error("expected diagnostics for undefined name")
	}

	// Fix the error
	sendNotification(t, client, "textDocument/didChange", DidChangeTextDocumentParams{
		TextDocument: VersionedTextDocumentIdentifier{URI: uri, Version: 2},
		ContentChanges: []TextDocumentContentChangeEvent{
			{Text: "x = glob([\"*.go\"])\n"},
		},
	})
	msg, _ = client.read() // new diagnostics
	var diags2 PublishDiagnosticsParams
	_ = json.Unmarshal(msg.Params, &diags2)
	if len(diags2.Diagnostics) != 0 {
		t.Errorf("expected 0 diagnostics after fix, got %d", len(diags2.Diagnostics))
	}
}

func TestCompletionKeywordArgsUserFunc(t *testing.T) {
	t.Parallel()
	_, client := testServer(t)

	// User-defined function with parameters; cursor inside a call to it
	uri := initAndOpen(t, client,
"def my_build(src, out, debug=False):\n  pass\n\nresult = my_build( )\n")

	resp := sendRequest(t, client, 2, "textDocument/completion", CompletionParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     Position{Line: 3, Character: 19}, // inside my_build()
		},
	})
	if resp.Error != nil {
		t.Fatalf("completion failed: %s", resp.Error.Message)
	}

	var result CompletionList
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}

	found := map[string]bool{}
	for _, item := range result.Items {
		found[item.Label] = true
	}

	for _, name := range []string{"src", "out", "debug"} {
		if !found[name] {
			t.Errorf("expected keyword argument completion %q for my_build()", name)
		}
	}
}
