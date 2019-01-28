package net

import (
	"reflect"
	"testing"
	"time"

	"github.com/mosaicnetworks/babble/src/common"
	"github.com/mosaicnetworks/babble/src/hashgraph"
	"github.com/mosaicnetworks/babble/src/peers"
)

const (
	INMEM = iota
	TCP
	numTestTransports // NOTE: must be last
)

func NewTestTransport(ttype int, addr string, t *testing.T) Transport {
	switch ttype {
	case INMEM:
		_, it := NewInmemTransport(addr)
		return it
	case TCP:
		tt, err := NewTCPTransport(addr, nil, 2, time.Second, common.NewTestLogger(t))
		if err != nil {
			t.Fatal(err)
		}
		return tt
	default:
		panic("Unknown transport type")
	}
}

func TestTransport_StartStop(t *testing.T) {
	for ttype := 0; ttype < numTestTransports; ttype++ {
		trans := NewTestTransport(ttype, "127.0.0.1:0", t)
		if err := trans.Close(); err != nil {
			t.Fatalf("err: %v", err)
		}
	}
}

func TestTransport_Sync(t *testing.T) {
	addr1 := "127.0.0.1:1234"
	addr2 := "127.0.0.1:1235"
	for ttype := 0; ttype < numTestTransports; ttype++ {
		trans1 := NewTestTransport(ttype, addr1, t)
		defer trans1.Close()
		rpcCh := trans1.Consumer()

		// Make the RPC request
		args := SyncRequest{
			FromID: 0,
			Known: map[uint32]int{
				0: 1,
				1: 2,
				2: 3,
			},
		}
		resp := SyncResponse{
			FromID: 1,
			Events: []hashgraph.WireEvent{
				hashgraph.WireEvent{
					Body: hashgraph.WireBody{
						Transactions:         [][]byte(nil),
						SelfParentIndex:      1,
						OtherParentCreatorID: 10,
						OtherParentIndex:     0,
						CreatorID:            9,
					},
				},
			},
			Known: map[uint32]int{
				0: 5,
				1: 5,
				2: 6,
			},
		}

		// Listen for a request
		go func() {
			select {
			case rpc := <-rpcCh:
				// Verify the command
				req := rpc.Command.(*SyncRequest)
				if !reflect.DeepEqual(req, &args) {
					t.Fatalf("command mismatch: %#v %#v", *req, args)
				}
				rpc.Respond(&resp, nil)

			case <-time.After(200 * time.Millisecond):
				t.Fatalf("timeout")
			}
		}()

		// Transport 2 makes outbound request
		trans2 := NewTestTransport(ttype, addr2, t)
		defer trans2.Close()

		if ttype == INMEM {
			itrans1 := trans1.(*InmemTransport)
			itrans2 := trans2.(*InmemTransport)
			itrans1.Connect(addr2, trans2)
			itrans2.Connect(addr1, trans1)
			trans1 = itrans1
			trans2 = itrans2
		}

		var out SyncResponse
		if err := trans2.Sync(trans1.LocalAddr(), &args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Verify the response
		if !reflect.DeepEqual(resp, out) {
			t.Fatalf("command mismatch: %#v %#v", resp, out)
		}
	}
}

func TestTransport_EagerSync(t *testing.T) {
	addr1 := "127.0.0.1:1236"
	addr2 := "127.0.0.1:1237"
	for ttype := 0; ttype < numTestTransports; ttype++ {
		trans1 := NewTestTransport(ttype, addr1, t)
		defer trans1.Close()
		rpcCh := trans1.Consumer()

		// Make the RPC request
		args := EagerSyncRequest{
			FromID: 0,
			Events: []hashgraph.WireEvent{
				hashgraph.WireEvent{
					Body: hashgraph.WireBody{
						Transactions:         [][]byte(nil),
						SelfParentIndex:      1,
						OtherParentCreatorID: 10,
						OtherParentIndex:     0,
						CreatorID:            9,
					},
				},
			},
		}
		resp := EagerSyncResponse{
			FromID:  1,
			Success: true,
		}

		// Listen for a request
		go func() {
			select {
			case rpc := <-rpcCh:
				// Verify the command
				req := rpc.Command.(*EagerSyncRequest)
				if !reflect.DeepEqual(req, &args) {
					t.Fatalf("command mismatch: %#v %#v", *req, args)
				}
				rpc.Respond(&resp, nil)

			case <-time.After(200 * time.Millisecond):
				t.Fatalf("timeout")
			}
		}()

		// Transport 2 makes outbound request
		trans2 := NewTestTransport(ttype, addr2, t)
		defer trans2.Close()

		if ttype == INMEM {
			itrans1 := trans1.(*InmemTransport)
			itrans2 := trans2.(*InmemTransport)
			itrans1.Connect(addr2, trans2)
			itrans2.Connect(addr1, trans1)
			trans1 = itrans1
			trans2 = itrans2
		}

		var out EagerSyncResponse
		if err := trans2.EagerSync(trans1.LocalAddr(), &args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Verify the response
		if !reflect.DeepEqual(resp, out) {
			t.Fatalf("command mismatch: %#v %#v", resp, out)
		}
	}
}

func TestTransport_FastForward(t *testing.T) {
	addr1 := "127.0.0.1:1238"
	addr2 := "127.0.0.1:1239"
	for ttype := 0; ttype < numTestTransports; ttype++ {
		trans1 := NewTestTransport(ttype, addr1, t)
		defer trans1.Close()
		rpcCh := trans1.Consumer()

		//Prepare the response Frame and corresponding Block

		framePeers := []*peers.Peer{
			peers.NewPeer("pub1", "addr1"),
			peers.NewPeer("pub2", "addr2"),
		}

		//Marshalling/Unmarshalling clears private fiels, so we precompute the
		//Marsalled/Unmarshalled objects to compare the expected result to the
		//RPC response.

		frame := &hashgraph.Frame{
			Round: 10,
			Peers: framePeers,
			Roots: map[string]*hashgraph.Root{
				"pub1": hashgraph.NewBaseRoot(framePeers[0].ID()),
				"pub2": hashgraph.NewBaseRoot(framePeers[1].ID()),
			},
			Events: []*hashgraph.Event{
				hashgraph.NewEvent(
					[][]byte{
						[]byte("tx1"),
						[]byte("tx2"),
					},
					[]hashgraph.BlockSignature{
						hashgraph.BlockSignature{
							[]byte("pub1"),
							0,
							"the signature",
						},
					},
					[]string{"pub1", "pub2"},
					[]byte("pub1"),
					4,
				),
			},
		}

		marshalledFrame, err := frame.Marshal()
		if err != nil {
			t.Fatal(err)
		}

		var unmarshalledFrame hashgraph.Frame
		err = unmarshalledFrame.Unmarshal(marshalledFrame)
		if err != nil {
			t.Fatal(err)
		}

		block, err := hashgraph.NewBlockFromFrame(9, frame)
		if err != nil {
			t.Fatal(err)
		}

		marshalledBlock, err := block.Marshal()
		if err != nil {
			t.Fatal(err)
		}

		var unmarshalledBlock hashgraph.Block
		err = unmarshalledBlock.Unmarshal(marshalledBlock)
		if err != nil {
			t.Fatal(err)
		}

		snapshot := []byte("this is the snapshot")

		// Make the RPC request and response

		args := FastForwardRequest{
			FromID: 0,
		}
		resp := FastForwardResponse{
			FromID:   1,
			Block:    unmarshalledBlock,
			Frame:    unmarshalledFrame,
			Snapshot: snapshot,
		}

		// Listen for a request
		go func() {
			select {
			case rpc := <-rpcCh:
				// Verify the command
				req := rpc.Command.(*FastForwardRequest)
				if !reflect.DeepEqual(req, &args) {
					t.Fatalf("command mismatch: %#v %#v", *req, args)
				}
				rpc.Respond(&resp, nil)

			case <-time.After(200 * time.Millisecond):
				t.Fatalf("timeout")
			}
		}()

		// Transport 2 makes outbound request
		trans2 := NewTestTransport(ttype, addr2, t)
		defer trans2.Close()

		if ttype == INMEM {
			itrans1 := trans1.(*InmemTransport)
			itrans2 := trans2.(*InmemTransport)
			itrans1.Connect(addr2, trans2)
			itrans2.Connect(addr1, trans1)
			trans1 = itrans1
			trans2 = itrans2
		}

		var out FastForwardResponse
		if err := trans2.FastForward(trans1.LocalAddr(), &args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Verify the response
		if !reflect.DeepEqual(resp, out) {
			t.Fatalf("ttype %d. Response mismatch: %#v %#v", ttype, resp, out)
		}
	}
}
