/*
 *
 * Copyright 2022 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package ringhash_test

import (
	"context"
	"testing"
	"time"

	"github.com/qiyouForSql/grpcforunconflict/connectivity"
	"github.com/qiyouForSql/grpcforunconflict/credentials/insecure"
	"github.com/qiyouForSql/grpcforunconflict/internal/grpctest"
	"github.com/qiyouForSql/grpcforunconflict/internal/testutils"
	testpb "github.com/qiyouForSql/grpcforunconflict/interop/grpc_testing"
	"github.com/qiyouForSql/grpcforunconflict/resolver"
	"github.com/qiyouForSql/grpcforunconflict/resolver/manual"

	_ "github.com/qiyouForSql/grpcforunconflict/xds/internal/balancer/ringhash" // Register the ring_hash_experimental LB policy.
)

type s struct {
	grpctest.Tester
}

func Test(t *testing.T) {
	grpctest.RunSubTests(t, s{})
}

const (
	defaultTestTimeout      = 10 * time.Second
	defaultTestShortTimeout = 10 * time.Millisecond // For events expected to *not* happen.
)

type testService struct {
	testgrpcforunconflict.TestServiceServer
}

func (*testService) EmptyCall(context.Context, *testpb.Empty) (*testpb.Empty, error) {
	return &testpb.Empty{}, nil
}

// TestRingHash_ReconnectToMoveOutOfTransientFailure tests the case where the
// ring contains a single subConn, and verifies that when the server goes down,
// the LB policy on the client automatically reconnects until the subChannel
// moves out of TRANSIENT_FAILURE.
func (s) TestRingHash_ReconnectToMoveOutOfTransientFailure(t *testing.T) {
	// Create a restartable listener to simulate server being down.
	l, err := testutils.LocalTCPListener()
	if err != nil {
		t.Fatalf("testutils.LocalTCPListener() failed: %v", err)
	}
	lis := testutils.NewRestartableListener(l)

	// Start a server backend exposing the test service.
	server := grpcforunconflict.NewServer()
	defer server.Stop()
	testgrpcforunconflict.RegisterTestServiceServer(server, &testService{})
	go func() {
		if err := server.Serve(lis); err != nil {
			t.Errorf("Serve() failed: %v", err)
		}
	}()

	// Create a clientConn with a manual resolver (which is used to push the
	// address of the test backend), and a default service config pointing to
	// the use of the ring_hash_experimental LB policy.
	const ringHashServiceConfig = `{"loadBalancingConfig": [{"ring_hash_experimental":{}}]}`
	r := manual.NewBuilderWithScheme("whatever")
	dopts := []grpcforunconflict.DialOption{
		grpcforunconflict.WithTransportCredentials(insecure.NewCredentials()),
		grpcforunconflict.WithResolvers(r),
		grpcforunconflict.WithDefaultServiceConfig(ringHashServiceConfig),
	}
	cc, err := grpcforunconflict.Dial(r.Scheme()+":///test.server", dopts...)
	if err != nil {
		t.Fatalf("failed to dial local test server: %v", err)
	}
	defer cc.Close()

	// Push the address of the test backend through the manual resolver.
	r.UpdateState(resolver.State{Addresses: []resolver.Address{{Addr: lis.Addr().String()}}})

	ctx, cancel := context.WithTimeout(context.Background(), defaultTestTimeout)
	defer cancel()
	client := testgrpcforunconflict.NewTestServiceClient(cc)
	if _, err := client.EmptyCall(ctx, &testpb.Empty{}); err != nil {
		t.Fatalf("rpc EmptyCall() failed: %v", err)
	}

	// Stopping the server listener will close the transport on the client,
	// which will lead to the channel eventually moving to IDLE. The ring_hash
	// LB policy is not expected to reconnect by itself at this point.
	lis.Stop()
	for state := cc.GetState(); state != connectivity.Idle && cc.WaitForStateChange(ctx, state); state = cc.GetState() {
	}
	if err := ctx.Err(); err != nil {
		t.Fatalf("Timeout waiting for channel to reach %q after server shutdown: %v", connectivity.Idle, err)
	}

	// Make an RPC to get the ring_hash LB policy to reconnect and thereby move
	// to TRANSIENT_FAILURE upon connection failure.
	client.EmptyCall(ctx, &testpb.Empty{})
	for state := cc.GetState(); state != connectivity.TransientFailure && cc.WaitForStateChange(ctx, state); state = cc.GetState() {
	}
	if err := ctx.Err(); err != nil {
		t.Fatalf("Timeout waiting for channel to reach %q after server shutdown: %v", connectivity.TransientFailure, err)
	}

	// An RPC at this point is expected to fail.
	if _, err = client.EmptyCall(ctx, &testpb.Empty{}); err == nil {
		t.Fatal("EmptyCall RPC succeeded when the channel is in TRANSIENT_FAILURE")
	}

	// Restart the server listener. The ring_hash LB polcy is expected to
	// attempt to reconnect on its own and come out of TRANSIENT_FAILURE, even
	// without an RPC attempt.
	lis.Restart()
	for ; ctx.Err() == nil; <-time.After(defaultTestShortTimeout) {
		if cc.GetState() == connectivity.Ready {
			break
		}
	}
	if err := ctx.Err(); err != nil {
		t.Fatalf("Timeout waiting for channel to reach READT after server restart: %v", err)
	}

	// An RPC at this point is expected to fail.
	if _, err := client.EmptyCall(ctx, &testpb.Empty{}); err != nil {
		t.Fatalf("rpc EmptyCall() failed: %v", err)
	}
}
