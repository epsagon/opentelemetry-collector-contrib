// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package jaegerexporter

import (
	"context"
	"net"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jaegertracing/jaeger/model"
	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "createExporter",
			config: Config{
				ExporterSettings: config.NewExporterSettings(component.NewID(typeStr)),
				GRPCClientSettings: configgrpc.GRPCClientSettings{
					Headers:     nil,
					Endpoint:    "foo.bar",
					Compression: "",
					TLSSetting: configtls.TLSClientSetting{
						Insecure: true,
					},
					Keepalive: nil,
				},
			},
		},
		{
			name: "createExporterWithHeaders",
			config: Config{
				ExporterSettings: config.NewExporterSettings(component.NewID(typeStr)),
				GRPCClientSettings: configgrpc.GRPCClientSettings{
					Headers:     map[string]string{"extra-header": "header-value"},
					Endpoint:    "foo.bar",
					Compression: "",
					Keepalive:   nil,
				},
			},
		},
		{
			name: "createBasicSecureExporter",
			config: Config{
				ExporterSettings: config.NewExporterSettings(component.NewID(typeStr)),
				GRPCClientSettings: configgrpc.GRPCClientSettings{
					Headers:     nil,
					Endpoint:    "foo.bar",
					Compression: "",
					Keepalive:   nil,
				},
			},
		},
		{
			name: "createSecureExporterWithClientTLS",
			config: Config{
				ExporterSettings: config.NewExporterSettings(component.NewID(typeStr)),
				GRPCClientSettings: configgrpc.GRPCClientSettings{
					Headers:     nil,
					Endpoint:    "foo.bar",
					Compression: "",
					TLSSetting: configtls.TLSClientSetting{
						TLSSetting: configtls.TLSSetting{
							CAFile: "testdata/test_cert.pem",
						},
						Insecure: false,
					},
					Keepalive: nil,
				},
			},
		},
		{
			name: "createSecureExporterWithKeepAlive",
			config: Config{
				ExporterSettings: config.NewExporterSettings(component.NewID(typeStr)),
				GRPCClientSettings: configgrpc.GRPCClientSettings{
					Headers:     nil,
					Endpoint:    "foo.bar",
					Compression: "",
					TLSSetting: configtls.TLSClientSetting{
						TLSSetting: configtls.TLSSetting{
							CAFile: "testdata/test_cert.pem",
						},
						Insecure:   false,
						ServerName: "",
					},
					Keepalive: &configgrpc.KeepaliveClientConfig{
						Time:                0,
						Timeout:             0,
						PermitWithoutStream: false,
					},
				},
			},
		},
		{
			name: "createSecureExporterWithMissingFile",
			config: Config{
				ExporterSettings: config.NewExporterSettings(component.NewID(typeStr)),
				GRPCClientSettings: configgrpc.GRPCClientSettings{
					Headers:     nil,
					Endpoint:    "foo.bar",
					Compression: "",
					TLSSetting: configtls.TLSClientSetting{
						TLSSetting: configtls.TLSSetting{
							CAFile: "testdata/test_cert_missing.pem",
						},
						Insecure: false,
					},
					Keepalive: nil,
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newTracesExporter(&tt.config, componenttest.NewNopExporterCreateSettings())
			assert.NoError(t, err)
			assert.NotNil(t, got)
			t.Cleanup(func() {
				require.NoError(t, got.Shutdown(context.Background()))
			})

			err = got.Start(context.Background(), componenttest.NewNopHost())
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			// This is expected to fail.
			err = got.ConsumeTraces(context.Background(), testdata.GenerateTracesNoLibraries())
			assert.Error(t, err)
		})
	}
}

// CA key and cert
// openssl req -new -nodes -x509 -days 9650 -keyout ca.key -out ca.crt -subj "/C=US/ST=California/L=Mountain View/O=Your Organization/OU=Your Unit/CN=localhost"
// Server key and cert
// openssl genrsa -des3 -out server.key 1024
// openssl req -new -key server.key -out server.csr -subj "/C=US/ST=California/L=Mountain View/O=Your Organization/OU=Your Unit/CN=localhost"
// openssl x509 -req -days 9650 -in server.csr -CA ca.crt -CAkey ca.key -set_serial 01 -out server.crt
// Client key and cert
// openssl genrsa -des3 -out client.key 1024
// openssl req -new -key client.key -out client.csr -subj "/C=US/ST=California/L=Mountain View/O=Your Organization/OU=Your Unit/CN=localhost"
// openssl x509 -req -days 9650 -in client.csr -CA ca.crt -CAkey ca.key -set_serial 01 -out client.crt
// Remove passphrase
// openssl rsa -in server.key -out temp.key && rm server.key && mv temp.key server.key
// openssl rsa -in client.key -out temp.key && rm client.key && mv temp.key client.key
func TestMutualTLS(t *testing.T) {
	caPath := filepath.Join("testdata", "ca.crt")
	serverCertPath := filepath.Join("testdata", "server.crt")
	serverKeyPath := filepath.Join("testdata", "server.key")
	clientCertPath := filepath.Join("testdata", "client.crt")
	clientKeyPath := filepath.Join("testdata", "client.key")

	// start gRPC Jaeger server
	tlsCfgOpts := configtls.TLSServerSetting{
		TLSSetting: configtls.TLSSetting{
			CertFile: serverCertPath,
			KeyFile:  serverKeyPath,
		},
		ClientCAFile: caPath,
	}
	tlsCfg, err := tlsCfgOpts.LoadTLSConfig()
	require.NoError(t, err)
	spanHandler := &mockSpanHandler{}
	server, serverAddr := initializeGRPCTestServer(t, func(server *grpc.Server) {
		api_v2.RegisterCollectorServiceServer(server, spanHandler)
	}, grpc.Creds(credentials.NewTLS(tlsCfg)))
	defer server.GracefulStop()

	// Create gRPC trace exporter
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	// Disable queuing to ensure that we execute the request when calling ConsumeTraces
	// otherwise we will have to wait.
	cfg.QueueSettings.Enabled = false
	cfg.GRPCClientSettings = configgrpc.GRPCClientSettings{
		Endpoint: serverAddr.String(),
		TLSSetting: configtls.TLSClientSetting{
			TLSSetting: configtls.TLSSetting{
				CAFile:   caPath,
				CertFile: clientCertPath,
				KeyFile:  clientKeyPath,
			},
			Insecure:   false,
			ServerName: "localhost",
		},
	}
	exporter, err := factory.CreateTracesExporter(context.Background(), componenttest.NewNopExporterCreateSettings(), cfg)
	require.NoError(t, err)
	err = exporter.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, exporter.Shutdown(context.Background())) })

	traceID := pcommon.TraceID([16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15})
	spanID := pcommon.SpanID([8]byte{0, 1, 2, 3, 4, 5, 6, 7})

	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetTraceID(traceID)
	span.SetSpanID(spanID)

	err = exporter.ConsumeTraces(context.Background(), td)
	require.NoError(t, err)
	requestes := spanHandler.getRequests()
	assert.Equal(t, 1, len(requestes))
	jTraceID, err := model.TraceIDFromBytes(traceID[:])
	require.NoError(t, err)
	require.Len(t, requestes, 1)
	require.Len(t, requestes[0].GetBatch().Spans, 1)
	assert.Equal(t, jTraceID, requestes[0].GetBatch().Spans[0].TraceID)
}

func TestConnectionStateChange(t *testing.T) {
	var state connectivity.State

	wg := sync.WaitGroup{}
	sr := &mockStateReporter{
		state: connectivity.Connecting,
	}
	sender := &protoGRPCSender{
		settings:                  componenttest.NewNopTelemetrySettings(),
		stopCh:                    make(chan struct{}),
		conn:                      sr,
		connStateReporterInterval: 10 * time.Millisecond,
		clientSettings: &configgrpc.GRPCClientSettings{
			Headers:     nil,
			Endpoint:    "foo.bar",
			Compression: "",
			TLSSetting: configtls.TLSClientSetting{
				Insecure: true,
			},
			Keepalive: nil,
		},
	}

	wg.Add(1)
	sender.AddStateChangeCallback(func(c connectivity.State) {
		state = c
		wg.Done()
	})

	done := make(chan struct{})
	go func() {
		sender.startConnectionStatusReporter()
		done <- struct{}{}
	}()

	t.Cleanup(func() {
		// set the stopped flag, and wait for statusReporter to finish and signal back
		sender.stopLock.Lock()
		sender.stopped = true
		sender.stopLock.Unlock()
		<-done
	})

	wg.Wait() // wait for the initial state to be propagated

	// test
	wg.Add(1)
	sr.SetState(connectivity.Ready)

	// verify
	wg.Wait() // wait until we get the state change
	assert.Equal(t, connectivity.Ready, state)
}

func TestConnectionReporterEndsOnStopped(t *testing.T) {
	sr := &mockStateReporter{
		state: connectivity.Connecting,
	}

	sender := &protoGRPCSender{
		settings:                  componenttest.NewNopTelemetrySettings(),
		stopCh:                    make(chan struct{}),
		conn:                      sr,
		connStateReporterInterval: 10 * time.Millisecond,
		clientSettings: &configgrpc.GRPCClientSettings{
			Headers:     nil,
			Endpoint:    "foo.bar",
			Compression: "",
			TLSSetting: configtls.TLSClientSetting{
				Insecure: true,
			},
			Keepalive: nil,
		},
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		sender.startConnectionStatusReporter()
		wg.Done()
	}()

	sender.stopLock.Lock()
	sender.stopped = true
	sender.stopLock.Unlock()

	// if the test finishes, we are good... if it gets blocked, the conn status reporter didn't return when the sender was marked as stopped
	wg.Wait()
}

type mockStateReporter struct {
	state connectivity.State
	mu    sync.RWMutex
}

func (m *mockStateReporter) GetState() connectivity.State {
	m.mu.RLock()
	st := m.state
	m.mu.RUnlock()
	return st
}
func (m *mockStateReporter) SetState(st connectivity.State) {
	m.mu.Lock()
	m.state = st
	m.mu.Unlock()
}

func initializeGRPCTestServer(t *testing.T, beforeServe func(server *grpc.Server), opts ...grpc.ServerOption) (*grpc.Server, net.Addr) {
	server := grpc.NewServer(opts...)
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	beforeServe(server)
	go func() {
		require.NoError(t, server.Serve(lis))
	}()
	return server, lis.Addr()
}

type mockSpanHandler struct {
	mux      sync.Mutex
	requests []*api_v2.PostSpansRequest
}

func (h *mockSpanHandler) getRequests() []*api_v2.PostSpansRequest {
	h.mux.Lock()
	defer h.mux.Unlock()
	return h.requests
}

func (h *mockSpanHandler) PostSpans(_ context.Context, r *api_v2.PostSpansRequest) (*api_v2.PostSpansResponse, error) {
	h.mux.Lock()
	defer h.mux.Unlock()
	h.requests = append(h.requests, r)
	return &api_v2.PostSpansResponse{}, nil
}
