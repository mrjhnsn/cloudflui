package backend

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
)

// RPC error codes.
const (
	codeParseError     = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInternalError  = -32603
)

// Request is a single JSON-RPC request.
type Request struct {
	ID     int             `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

// Response is a single JSON-RPC response.
type Response struct {
	ID     int       `json:"id"`
	Result any       `json:"result,omitempty"`
	Error  *RPCError `json:"error,omitempty"`
}

// RPCError is a JSON-RPC error object.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Server dispatches RPC requests to a Backend.
type Server struct {
	b *Backend
}

// NewServer builds a server over a ready backend.
func NewServer(b *Backend) *Server {
	return &Server{b: b}
}

// Serve reads newline-delimited requests from r and writes responses to w.
// It returns when the input is exhausted (EOF) or a write fails.
func (s *Server) Serve(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	// Record lists can be large; allow generous lines so a single response
	// or request never trips the scanner's default 64 KiB buffer.
	scanner.Buffer(make([]byte, 0, 64*1024), 64*1024*1024)
	out := bufio.NewWriter(w)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		resp := s.handle(line)
		if err := writeResponse(out, resp); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// handle dispatches a single request to its method and returns the response.
func (s *Server) handle(req []byte) Response {
	var r Request
	if err := json.Unmarshal(req, &r); err != nil {
		return Response{ID: 0, Error: &RPCError{Code: codeParseError, Message: "parse error: " + err.Error()}}
	}
	if r.Method == "" {
		return Response{ID: r.ID, Error: &RPCError{Code: codeInvalidRequest, Message: "missing method"}}
	}
	result, err := s.dispatch(r.Method, r.Params)
	if err != nil {
		if _, ok := err.(methodNotFoundError); ok {
			return Response{ID: r.ID, Error: &RPCError{Code: codeMethodNotFound, Message: err.Error()}}
		}
		return Response{ID: r.ID, Error: &RPCError{Code: codeInternalError, Message: err.Error()}}
	}
	return Response{ID: r.ID, Result: result}
}

// methodNotFoundError marks a call to an unknown method.
type methodNotFoundError struct{ method string }

func (e methodNotFoundError) Error() string {
	return fmt.Sprintf("method not found: %s", e.method)
}

// dispatch runs a method with its raw params and returns its JSON result.
func (s *Server) dispatch(method string, params json.RawMessage) (any, error) {
	ctx := context.Background()
	switch method {
	case "ping":
		return map[string]bool{"ok": true}, nil
	case "get_config":
		return s.b.GetConfig(), nil

	case "list_zones":
		return s.b.ListZones(ctx)

	case "list_records":
		var p struct {
			ZoneID string `json:"zone_id"`
		}
		if err := decode(params, &p); err != nil {
			return nil, err
		}
		return s.b.ListRecords(ctx, p.ZoneID)

	case "create_record":
		var p recordParams
		if err := decode(params, &p); err != nil {
			return nil, err
		}
		if err := s.b.CreateRecord(ctx, p.ZoneID, p.Name, p.Type, p.Content, p.TTL, p.Proxied, p.Comment); err != nil {
			return nil, err
		}
		return ok(), nil

	case "update_record":
		var p recordParams
		if err := decode(params, &p); err != nil {
			return nil, err
		}
		if err := s.b.UpdateRecord(ctx, p.ZoneID, p.RecordID, p.Name, p.Type, p.Content, p.TTL, p.Proxied, p.Comment); err != nil {
			return nil, err
		}
		return ok(), nil

	case "delete_record":
		var p struct {
			ZoneID   string `json:"zone_id"`
			RecordID string `json:"record_id"`
		}
		if err := decode(params, &p); err != nil {
			return nil, err
		}
		if err := s.b.DeleteRecord(ctx, p.ZoneID, p.RecordID); err != nil {
			return nil, err
		}
		return ok(), nil

	case "list_workers":
		var p struct {
			AccountID string `json:"account_id"`
		}
		if err := decode(params, &p); err != nil {
			return nil, err
		}
		return s.b.ListWorkers(ctx, p.AccountID)

	case "list_deployments":
		var p struct {
			AccountID string `json:"account_id"`
			Script    string `json:"script"`
		}
		if err := decode(params, &p); err != nil {
			return nil, err
		}
		return s.b.ListDeployments(ctx, p.AccountID, p.Script)

	case "save_profile":
		var p struct {
			Name      string `json:"name"`
			APIKey    string `json:"api_key"`
			AccountID string `json:"account_id"`
		}
		if err := decode(params, &p); err != nil {
			return nil, err
		}
		return s.b.SaveProfile(p.Name, p.APIKey, p.AccountID)

	case "delete_profile":
		var p struct {
			Name string `json:"name"`
		}
		if err := decode(params, &p); err != nil {
			return nil, err
		}
		return s.b.DeleteProfile(p.Name)

	case "activate_profile":
		var p struct {
			Name string `json:"name"`
		}
		if err := decode(params, &p); err != nil {
			return nil, err
		}
		return s.b.ActivateProfile(p.Name)

	case "set_default_profile":
		var p struct {
			Name string `json:"name"`
		}
		if err := decode(params, &p); err != nil {
			return nil, err
		}
		return s.b.SetDefaultProfile(p.Name)

	default:
		return nil, methodNotFoundError{method: method}
	}
}

// recordParams is the payload shared by create_record and update_record.
type recordParams struct {
	ZoneID   string `json:"zone_id"`
	RecordID string `json:"record_id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Content  string `json:"content"`
	TTL      int    `json:"ttl"`
	Proxied  bool   `json:"proxied"`
	Comment  string `json:"comment"`
}

// ok returns the empty-object result used for void methods.
func ok() any {
	return struct{}{}
}

// decode unmarshals params, tolerating a missing/empty payload.
func decode(params json.RawMessage, dst any) error {
	if len(params) == 0 {
		return nil
	}
	return json.Unmarshal(params, dst)
}

// writeResponse encodes a response as one JSON line and flushes it.
func writeResponse(w *bufio.Writer, resp Response) error {
	data, err := json.Marshal(resp)
	if err != nil {
		// Fall back to a well-formed internal error; this should never happen
		// because the response shapes are all marshalable.
		data = []byte(`{"id":0,"error":{"code":-32603,"message":"marshal error"}}`)
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	if err := w.WriteByte('\n'); err != nil {
		return err
	}
	return w.Flush()
}
