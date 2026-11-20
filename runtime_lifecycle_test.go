package main

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"net"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestMeasuredHTTPBindFailureIsReportedSafely(t *testing.T) {
	buf := captureStartupLogs(t)
	failure := errors.New("private-listen-error-token")
	err := serveMeasuredHTTP(&http.Server{Addr: ":8080"}, func(network, addr string) (net.Listener, error) {
		if network != "tcp" || addr != ":8080" {
			t.Fatal(network, addr)
		}
		return nil, failure
	})
	if !errors.Is(err, failure) {
		t.Fatal("bind error lost")
	}
	if strings.Contains(buf.String(), "private-") || strings.Contains(buf.String(), "event=bound") {
		t.Fatal("bind failure logged unsafe content or false readiness")
	}
	if !strings.Contains(buf.String(), "stage=http.listen event=end duration_ms=") {
		t.Fatal("bind timing missing")
	}
}

type measuredListener struct{ closed bool }

func (*measuredListener) Accept() (net.Conn, error) { return nil, net.ErrClosed }
func (l *measuredListener) Close() error            { l.closed = true; return nil }
func (*measuredListener) Addr() net.Addr            { return &net.TCPAddr{} }

func TestMeasuredHTTPPreservesDefaultAddressAndClosesListener(t *testing.T) {
	buf := captureStartupLogs(t)
	listener := &measuredListener{}
	err := serveMeasuredHTTP(&http.Server{}, func(network, addr string) (net.Listener, error) {
		if network != "tcp" || addr != ":http" {
			t.Fatal(network, addr)
		}
		return listener, nil
	})
	if err == nil || !listener.closed {
		t.Fatal("Serve error or listener closure lost")
	}
	if !strings.Contains(buf.String(), "stage=http.listening event=bound") {
		t.Fatal("successful bind not logged")
	}
}

func startupFunctionCalls(t *testing.T, file, name string) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != name {
			continue
		}
		var calls []string
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				switch callee := call.Fun.(type) {
				case *ast.Ident:
					calls = append(calls, callee.Name)
				case *ast.SelectorExpr:
					calls = append(calls, callee.Sel.Name)
				}
			}
			return true
		})
		return calls
	}
	t.Fatal("startup function missing", name)
	return nil
}

func TestDiagnosticStartupPreservesPreparationAndWorkerOrder(t *testing.T) {
	calls := startupFunctionCalls(t, "main.go", "main")
	index := func(name string) int {
		for i, call := range calls {
			if call == name {
				return i
			}
		}
		t.Fatal("missing startup call", name)
		return -1
	}
	if index("prepareDatabase") >= index("HandleFunc") || index("prepareDatabase") >= index("serveUntilShutdown") {
		t.Fatal("HTTP registration or workers moved before database preparation")
	}
	var got []string
	for _, call := range startupFunctionCalls(t, "runtime_lifecycle.go", "serveUntilShutdown") {
		switch call {
		case "startNotificationWorkers", "startAcademyWorker", "startAcademyStorageWorker", "startAcademyCommunicationWorker", "startAcademyAnalyticsWorker", "serveMeasuredHTTP":
			got = append(got, call)
		}
	}
	want := []string{"startNotificationWorkers", "startAcademyWorker", "startAcademyStorageWorker", "startAcademyCommunicationWorker", "startAcademyAnalyticsWorker", "serveMeasuredHTTP"}
	if !reflect.DeepEqual(got, want) {
		t.Fatal("original worker/listener order changed", got)
	}
}
