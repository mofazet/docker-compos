/*
	Copyright (c) 2020 Docker Inc.

	Permission is hereby granted, free of charge, to any person
	obtaining a copy of this software and associated documentation
	files (the "Software"), to deal in the Software without
	restriction, including without limitation the rights to use, copy,
	modify, merge, publish, distribute, sublicense, and/or sell copies
	of the Software, and to permit persons to whom the Software is
	furnished to do so, subject to the following conditions:

	The above copyright notice and this permission notice shall be
	included in all copies or substantial portions of the Software.

	THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
	EXPRESS OR IMPLIED,
	INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
	FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
	IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT
	HOLDERS BE LIABLE FOR ANY CLAIM,
	DAMAGES OR OTHER LIABILITY,
	WHETHER IN AN ACTION OF CONTRACT,
	TORT OR OTHERWISE,
	ARISING FROM, OUT OF OR IN CONNECTION WITH
	THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
*/

package server

import (
	"context"
	"errors"
	"net"
	"strings"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"

	"github.com/docker/api/client"
	apicontext "github.com/docker/api/context"
	"github.com/docker/api/context/store"
	"github.com/docker/api/server/proxy"
)

// New returns a new GRPC server.
func New() *grpc.Server {
	s := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			unaryMeta,
			unary,
		),
		grpc.StreamInterceptor(stream),
	)
	hs := health.NewServer()
	grpc_health_v1.RegisterHealthServer(s, hs)
	return s
}

//CreateListener creates a listener either on tcp://, or local listener, supporting unix:// for unix socket or npipe:// for named pipes on windows
func CreateListener(address string) (net.Listener, error) {
	if strings.HasPrefix(address, "tcp://") {
		return net.Listen("tcp", strings.TrimPrefix(address, "tcp://"))
	}
	return createLocalListener(address)
}

func unary(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	return grpc_prometheus.UnaryServerInterceptor(ctx, req, info, handler)
}

func stream(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	return grpc_prometheus.StreamServerInterceptor(srv, ss, info, handler)
}

func unaryMeta(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errors.New("missing metadata")
	}

	key := md[apicontext.Key]

	if len(key) == 1 {
		s, err := store.New()
		if err != nil {
			return nil, err
		}
		ctx = store.WithContextStore(ctx, s)

		ctx = apicontext.WithCurrentContext(ctx, key[0])

		c, err := client.New(ctx)
		if err != nil {
			return nil, err
		}
		ctx, err = proxy.WithClient(ctx, c)
		if err != nil {
			return nil, err
		}
	}
	m, err := handler(ctx, req)
	return m, err
}