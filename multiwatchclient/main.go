package main

import (
	"bufio"
	"context"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	delegationv1 "github.com/spiffe/spire-api-sdk/proto/spire/api/agent/delegation/v1"
	"github.com/spiffe/spire-api-sdk/proto/spire/api/types"

	"google.golang.org/grpc"
)

const (
	SocketPath = "/tmp/admin.sock"
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

// parseSelector parses a CLI string from type:value into a selector type.
// Everything to the right of the first ":" is considered a selector value.
func parseSelector(str string) (*types.Selector, error) {
	parts := strings.SplitAfterN(str, ":", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("selector \"%s\" must be formatted as type:value", str)
	}

	s := &types.Selector{
		// Strip the trailing delimiter
		Type:  strings.TrimSuffix(parts[0], ":"),
		Value: parts[1],
	}
	return s, nil
}

func getClient() (delegationv1.DelegationClient, error) {
	conn, err := UDSDial(SocketPath)
	if err != nil {
		return nil, fmt.Errorf("dialing: %v", err)
	}

	client := delegationv1.NewDelegationClient(conn)
	if client == nil {
		fmt.Printf("error creating entry client")
	}

	return client, nil
}

var idsMap map[uint64]string

func watch(stream delegationv1.Delegation_FetchX509SVIDsClient) {
	//tream, err := client.WatchX509SVIDs(ctx, &delegationv1.WatchX509SVIDsRequest{})
	//f err != nil {
	//	fmt.Printf("error fetching svid: %v\n", err)
	//	return
	//

	for {
		resp, err := stream.Recv()
		if err != nil {
			fmt.Printf("error fetching svid (yeah): %v\n", err)
			return
		}

		selector, ok := idsMap[resp.Id]
		if !ok {
			selector = fmt.Sprintf("NOT FOUND: %d", resp.Id)
			continue
		}

		fmt.Printf("Reply for %s\n", selector)
		for _, svid := range resp.X509Svids {
			fmt.Printf("Entry ID         : %s\n", svid.X509Svid.Id)
			fmt.Printf("Expires at       : %d\n", svid.X509Svid.ExpiresAt)
		}

		fmt.Println("...")
	}
}

func add(stream delegationv1.Delegation_FetchX509SVIDsClient, selector string) {
	sel, err := parseSelector(selector)
	if err != nil {
		fmt.Printf("error parsing selectgor: %s", err)
		return
	}

	id := rand.Uint64()

	req1 := &delegationv1.FetchX509SVIDsRequest{
		Operation: delegationv1.FetchX509SVIDsRequest_ADD,
		Id:        id,
		Selectors: []*types.Selector{
			sel,
		},
	}

	err = stream.Send(req1)
	if err != nil {
		fmt.Printf("error adding watch: %s", err)
		return
	}

	//fmt.Printf("token for %s was %s\n", selector, res.Token)
	idsMap[id] = selector

	fmt.Printf("added with id %d\n", id)
}

func del(stream delegationv1.Delegation_FetchX509SVIDsClient, id uint64) {
	req1 := &delegationv1.FetchX509SVIDsRequest{
		Operation: delegationv1.FetchX509SVIDsRequest_DEL,
		Id:        id,
	}

	err := stream.Send(req1)
	if err != nil {
		fmt.Printf("error deleting watch: %s", err)
		return
	}
}

func main() {
	rand.Seed(time.Now().UTC().UnixNano())

	client, err := getClient()
	if err != nil {
		fmt.Printf("error happened: %v\n", err)
		return
	}

	idsMap = make(map[uint64]string)

	ctx := context.Background()

	stream, err := client.FetchX509SVIDs(ctx)
	if err != nil {
		fmt.Print("error in client.FetchX509SVIDs\n")
		return
	}

	resp, err := stream.Recv()
	if err != nil {
		fmt.Printf("error on stream.Recv %s\n", err)
		return
	}

	if resp.Id != 0 {
		fmt.Printf("server replied with 0 different than 0\n")
		return
	}

	fmt.Printf("server says everything is fine!\n")

	// start watcher
	go watch(stream)

	reader := bufio.NewReader(os.Stdin)

	// delete and remove subscription
	for {
		read_line, _ := reader.ReadString('\n')
		read_line = strings.TrimRight(read_line, "\r\n")
		parts := strings.Split(read_line, " ")

		switch parts[0] {
		case "add":
			add(stream, parts[1])
		case "del":
			i64, _ := strconv.ParseInt(parts[1], 10, 64)
			del(stream, uint64(i64))
		}
	}
}
