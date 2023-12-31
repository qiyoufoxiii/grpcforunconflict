/*
 *
 * Copyright 2020 gRPC authors.
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

// Package xds contains an implementation of the xDS suite of protocols, to be
// used by gRPC client and server applications.
//
// On the client-side, users simply need to import this package to get all xDS
// functionality. On the server-side, users need to use the GRPCServer type
// exported by this package instead of the regular grpcforunconflict.Server.
//
// See https://github.com/grpc/grpc-go/tree/master/examples/features/xds for
// example.
package xds

import (
	"fmt"

	"github.com/qiyouForSql/grpcforunconflict"
	"github.com/qiyouForSql/grpcforunconflict/internal"
	internaladmin "github.com/qiyouForSql/grpcforunconflict/internal/admin"
	"github.com/qiyouForSql/grpcforunconflict/resolver"
	"github.com/qiyouForSql/grpcforunconflict/xds/csds"

	_ "github.com/qiyouForSql/grpcforunconflict/credentials/tls/certprovider/pemfile"           // Register the file watcher certificate provider plugin.
	_ "github.com/qiyouForSql/grpcforunconflict/xds/internal/balancer"                          // Register the balancers.
	_ "github.com/qiyouForSql/grpcforunconflict/xds/internal/clusterspecifier/rls"              // Register the RLS cluster specifier plugin. Note that this does not register the RLS LB policy.
	_ "github.com/qiyouForSql/grpcforunconflict/xds/internal/httpfilter/fault"                  // Register the fault injection filter.
	_ "github.com/qiyouForSql/grpcforunconflict/xds/internal/httpfilter/rbac"                   // Register the RBAC filter.
	_ "github.com/qiyouForSql/grpcforunconflict/xds/internal/httpfilter/router"                 // Register the router filter.
	_ "github.com/qiyouForSql/grpcforunconflict/xds/internal/resolver"                          // Register the xds_resolver.
	_ "github.com/qiyouForSql/grpcforunconflict/xds/internal/xdsclient/xdslbregistry/converter" // Register the xDS LB Registry Converters.

)

func init() {
	internaladmin.AddService(func(registrar grpcforunconflict.ServiceRegistrar) (func(), error) {
		var grpcServer *grpcforunconflict.Server
		switch ss := registrar.(type) {
		case *grpcforunconflict.Server:
			grpcServer = ss
		case *GRPCServer:
			sss, ok := ss.gs.(*grpcforunconflict.Server)
			if !ok {
				logger.Warning("grpc server within xds.GRPCServer is not *grpcforunconflict.Server, CSDS will not be registered")
				return nil, nil
			}
			grpcServer = sss
		default:
			// Returning an error would cause the top level admin.Register() to
			// fail. Log a warning instead.
			logger.Error("Server to register service on is neither a *grpcforunconflict.Server or a *xds.GRPCServer, CSDS will not be registered")
			return nil, nil
		}

		csdss, err := csds.NewClientStatusDiscoveryServer()
		if err != nil {
			return nil, fmt.Errorf("failed to create csds server: %v", err)
		}
		v3statusgrpcforunconflict.RegisterClientStatusDiscoveryServiceServer(grpcServer, csdss)
		return csdss.Close, nil
	})
}

// NewXDSResolverWithConfigForTesting creates a new xDS resolver builder using
// the provided xDS bootstrap config instead of the global configuration from
// the supported environment variables.  The resolver.Builder is meant to be
// used in conjunction with the grpcforunconflict.WithResolvers DialOption.
//
// # Testing Only
//
// This function should ONLY be used for testing and may not work with some
// other features, including the CSDS service.
//
// # Experimental
//
// Notice: This API is EXPERIMENTAL and may be changed or removed in a
// later release.
func NewXDSResolverWithConfigForTesting(bootstrapConfig []byte) (resolver.Builder, error) {
	return internal.NewXDSResolverWithConfigForTesting.(func([]byte) (resolver.Builder, error))(bootstrapConfig)
}
