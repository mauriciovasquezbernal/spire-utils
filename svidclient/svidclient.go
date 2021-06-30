package main

import (
	"context"
	"fmt"
	"net"

	"github.com/spiffe/go-spiffe/v2/proto/spiffe/workload"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/spire/proto/spire/types"

	//"github.com/spiffe/spire/proto/spire/common"

	"google.golang.org/grpc"
	//"google.golang.org/grpc/health/grpc_health_v1"

	"google.golang.org/grpc/metadata"
)

const (
	SocketPath = "/tmp/spire-agent/public/api.sock"
	//SocketPath = "/tmp/agent.sock"
	//SocketPath = "/run/spire/sockets/agent.sock"
)

func dialer(ctx context.Context, addr string) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, "unix", addr)
}

func UDSDial(socketPath string) (*grpc.ClientConn, error) {
	return grpc.Dial(socketPath,
		grpc.WithInsecure(),
		grpc.WithContextDialer(dialer),
		grpc.WithBlock(),
		grpc.FailOnNonTempDialError(true),
		grpc.WithReturnConnectionError())
}

func NewWorkloadClient(conn *grpc.ClientConn) workload.SpiffeWorkloadAPIClient {
	return workload.NewSpiffeWorkloadAPIClient(conn)
}

// idStringToProto converts a SPIFFE ID from the given string to *types.SPIFFEID
func idStringToProto(id string) (*types.SPIFFEID, error) {
	idType, err := spiffeid.FromString(id)
	if err != nil {
		return nil, err
	}
	return &types.SPIFFEID{
		TrustDomain: idType.TrustDomain().String(),
		Path:        idType.Path(),
	}, nil
}

func printableEntryID(id string) string {
	if id == "" {
		return "(none)"
	}
	return id
}

func getClient() (workload.SpiffeWorkloadAPIClient, error) {
	conn, err := UDSDial(SocketPath)
	if err != nil {
		return nil, fmt.Errorf("dialing: %v", err)
	}

	client := NewWorkloadClient(conn)
	if client == nil {
		fmt.Printf("error creating entry client")
	}

	return client, nil
}

func main() {
	client, err := getClient()
	if err != nil {
		fmt.Printf("error happened: %v\n", err)
		return
	}

	header := metadata.Pairs("workload.spiffe.io", "true")
	ctx := metadata.NewOutgoingContext(context.Background(), header)

	req := workload.X509SVIDRequest{}

	stream, err := client.FetchX509SVID(ctx, &req)
	if err != nil {
		fmt.Printf("error fetching svid: %v\n", err)
		return
	}

	for {
		resp, err := stream.Recv()
		if err != nil {
			fmt.Printf("error fetching svid: %v\n", err)
			return
		}

		for _, svid := range resp.Svids {
			fmt.Printf("Entry ID         : %s\n", printableEntryID(svid.SpiffeId))
			//fmt.Printf("SVID Not after: %s\n", getNoAfter(svid.X509Svid))
			//fmt.Printf("Bundle Not after: %s\n", getNoAfter(svid.Bundle))
		}

		fmt.Println("...")
	}
}
