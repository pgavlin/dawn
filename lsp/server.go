package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"

	"github.com/pgavlin/dawn"
	"github.com/pgavlin/starlark-go/typecheck"
)

// Server is the Dawn LSP server.
type Server struct {
	mu        sync.RWMutex
	documents map[string]*Document
	project   *ProjectContext
	dawnProj  *dawn.Project
	env       *typecheck.Env
	transport *transport
	log       *log.Logger
}

// NewServer creates a new LSP server.
func NewServer(r io.Reader, w io.Writer) *Server {
	return &Server{
		documents: make(map[string]*Document),
		transport: newTransport(r, w),
		log:       log.New(os.Stderr, "[dawn-lsp] ", log.LstdFlags),
	}
}

// Run starts the server main loop, reading requests and dispatching them.
func (s *Server) Run() error {
	for {
		msg, err := s.transport.read()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("read error: %w", err)
		}

		if err := s.dispatch(msg); err != nil {
			s.log.Printf("dispatch error: %v", err)
		}
	}
}

func (s *Server) dispatch(msg *jsonrpcMessage) error {
	switch msg.Method {
	case "initialize":
		return s.handleInitialize(msg)
	case "initialized":
		return nil
	case "shutdown":
		return s.handleShutdown(msg)
	case "exit":
		os.Exit(0)
		return nil
	case "textDocument/didOpen":
		return s.handleDidOpen(msg)
	case "textDocument/didChange":
		return s.handleDidChange(msg)
	case "textDocument/didClose":
		return s.handleDidClose(msg)
	case "textDocument/didSave":
		return s.handleDidSave(msg)
	case "textDocument/completion":
		return s.handleCompletion(msg)
	case "textDocument/hover":
		return s.handleHover(msg)
	case "textDocument/definition":
		return s.handleDefinition(msg)
	case "textDocument/references":
		return s.handleReferences(msg)
	case "textDocument/documentSymbol":
		return s.handleDocumentSymbol(msg)
	case "textDocument/semanticTokens/full":
		return s.handleSemanticTokensFull(msg)
	case "textDocument/signatureHelp":
		return s.handleSignatureHelp(msg)
	default:
		// Unknown method. If it has an ID, respond with method not found.
		if msg.ID != nil {
			return s.respondError(msg.ID, -32601, "method not found: "+msg.Method)
		}
		return nil
	}
}

func (s *Server) handleInitialize(msg *jsonrpcMessage) error {
	var params InitializeParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return s.respondError(msg.ID, -32602, "invalid params")
	}

	s.log.Printf("initialize: rootUri=%s", params.RootURI)

	rootURI := params.RootURI
	if rootURI == "" {
		rootURI = "file://" + params.RootPath
	}

	s.project = newProjectContext(rootURI)

	// Try to open the project for type-checking.
	if s.project != nil && s.project.Root != "" {
		ctx := context.Background()
		proj, err := dawn.Open(ctx, s.project.Root, nil)
		if err != nil {
			s.log.Printf("failed to open project: %v", err)
		} else {
			s.dawnProj = proj
			s.env = proj.BaseEnv()
		}
	}

	result := InitializeResult{
		Capabilities: ServerCapabilities{
			TextDocumentSync: &TextDocumentSyncOptions{
				OpenClose: true,
				Change:    1, // Full
			},
			CompletionProvider: &CompletionOptions{
				TriggerCharacters: []string{".", ":", "/", "\"", "@"},
			},
			HoverProvider:      true,
			DefinitionProvider: true,
			ReferencesProvider: true,
			SignatureHelpProvider: &SignatureHelpOptions{
				TriggerCharacters: []string{"(", ","},
			},
			DocumentSymbolProvider: true,
			SemanticTokensProvider: &SemanticTokensOptions{
				Legend: SemanticTokensLegend{
					TokenTypes:     SemanticTokenTypes,
					TokenModifiers: SemanticTokenModifiers,
				},
				Full: true,
			},
		},
		ServerInfo: &ServerInfo{
			Name:    "dawn-lsp",
			Version: "0.1.0",
		},
	}

	return s.respond(msg.ID, result)
}

func (s *Server) handleShutdown(msg *jsonrpcMessage) error {
	return s.respond(msg.ID, nil)
}

func (s *Server) handleDidOpen(msg *jsonrpcMessage) error {
	var params DidOpenTextDocumentParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return nil
	}

	path := uriToPath(params.TextDocument.URI)

	doc := &Document{
		URI:     params.TextDocument.URI,
		Path:    path,
		Version: params.TextDocument.Version,
		Content: params.TextDocument.Text,
	}

	// Try to initialize from project module data.
	if s.dawnProj != nil {
		if info, ok := s.dawnProj.ModuleForFile(path); ok {
			doc.initFromModule(info)
		} else {
			doc.analyze(s.env)
		}
	} else {
		doc.analyze(nil)
	}

	s.mu.Lock()
	s.documents[params.TextDocument.URI] = doc
	s.mu.Unlock()

	return s.publishDiagnostics(doc)
}

func (s *Server) handleDidChange(msg *jsonrpcMessage) error {
	var params DidChangeTextDocumentParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return nil
	}

	s.mu.Lock()
	doc, ok := s.documents[params.TextDocument.URI]
	if ok && len(params.ContentChanges) > 0 {
		doc.update(params.TextDocument.Version, params.ContentChanges[len(params.ContentChanges)-1].Text, s.env)
	}
	s.mu.Unlock()

	if ok {
		return s.publishDiagnostics(doc)
	}
	return nil
}

func (s *Server) handleDidClose(msg *jsonrpcMessage) error {
	var params DidCloseTextDocumentParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return nil
	}

	s.mu.Lock()
	delete(s.documents, params.TextDocument.URI)
	s.mu.Unlock()

	// Clear diagnostics for the closed document.
	return s.notify("textDocument/publishDiagnostics", PublishDiagnosticsParams{
		URI:         params.TextDocument.URI,
		Diagnostics: []Diagnostic{},
	})
}

func (s *Server) handleDidSave(msg *jsonrpcMessage) error {
	// Re-publish diagnostics on save.
	var params DidSaveTextDocumentParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return nil
	}

	s.mu.RLock()
	doc, ok := s.documents[params.TextDocument.URI]
	s.mu.RUnlock()
	if ok {
		return s.publishDiagnostics(doc)
	}
	return nil
}

func (s *Server) handleCompletion(msg *jsonrpcMessage) error {
	var params CompletionParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return s.respondError(msg.ID, -32602, "invalid params")
	}
	result := s.completion(&params)
	return s.respond(msg.ID, result)
}

func (s *Server) handleHover(msg *jsonrpcMessage) error {
	var params TextDocumentPositionParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return s.respondError(msg.ID, -32602, "invalid params")
	}
	result := s.hover(&params)
	return s.respond(msg.ID, result)
}

func (s *Server) handleDefinition(msg *jsonrpcMessage) error {
	var params TextDocumentPositionParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return s.respondError(msg.ID, -32602, "invalid params")
	}
	result := s.definition(&params)
	return s.respond(msg.ID, result)
}

func (s *Server) handleReferences(msg *jsonrpcMessage) error {
	var params ReferenceParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return s.respondError(msg.ID, -32602, "invalid params")
	}
	result := s.references(&params)
	return s.respond(msg.ID, result)
}

func (s *Server) handleDocumentSymbol(msg *jsonrpcMessage) error {
	var params DocumentSymbolParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return s.respondError(msg.ID, -32602, "invalid params")
	}
	result := s.documentSymbols(&params)
	return s.respond(msg.ID, result)
}

func (s *Server) handleSemanticTokensFull(msg *jsonrpcMessage) error {
	var params SemanticTokensParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return s.respondError(msg.ID, -32602, "invalid params")
	}
	result := s.semanticTokens(&params)
	return s.respond(msg.ID, result)
}

func (s *Server) handleSignatureHelp(msg *jsonrpcMessage) error {
	var params SignatureHelpParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return s.respondError(msg.ID, -32602, "invalid params")
	}
	result := s.signatureHelp(&params)
	return s.respond(msg.ID, result)
}

// publishDiagnostics sends diagnostics for a document.
func (s *Server) publishDiagnostics(doc *Document) error {
	diags := doc.collectDiagnostics()
	if diags == nil {
		diags = []Diagnostic{}
	}
	return s.notify("textDocument/publishDiagnostics", PublishDiagnosticsParams{
		URI:         doc.URI,
		Diagnostics: diags,
	})
}

// respond sends a successful response.
func (s *Server) respond(id *json.RawMessage, result interface{}) error {
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return s.transport.write(&jsonrpcMessage{
		ID:     id,
		Result: data,
	})
}

// respondError sends an error response.
func (s *Server) respondError(id *json.RawMessage, code int, message string) error {
	return s.transport.write(&jsonrpcMessage{
		ID: id,
		Error: &jsonrpcError{
			Code:    code,
			Message: message,
		},
	})
}

// notify sends a notification (no ID, no response expected).
func (s *Server) notify(method string, params interface{}) error {
	data, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return s.transport.write(&jsonrpcMessage{
		Method: method,
		Params: data,
	})
}
