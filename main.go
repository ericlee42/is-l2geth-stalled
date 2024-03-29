package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"
)

type Block struct {
	Number   string `json:"number"`
	LastSeen time.Time
}

func GetRemote(ctx context.Context, endpoint string) (*Block, error) {
	data, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": rand.Int63(),
		"method": "eth_getBlockByNumber", "params": []any{"latest", false}})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Add("content-type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	type jsonrpcError struct {
		Message string `json:"message"`
	}

	var jsonrpcMsg struct {
		Error  *jsonrpcError   `json:"error,omitempty"`
		Result json.RawMessage `json:"result,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&jsonrpcMsg); err != nil {
		return nil, err
	}

	if jsonrpcMsg.Error != nil {
		return nil, fmt.Errorf("jsonrpc error: %s", jsonrpcMsg.Error.Message)
	}

	var block Block
	if err := json.Unmarshal(jsonrpcMsg.Result, &block); err != nil {
		return nil, err
	}

	block.LastSeen = time.Now().UTC()
	return &block, nil
}

func GetLocal(file string) (*Block, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var block Block
	if err := json.Unmarshal(data, &block); err != nil {
		return nil, err
	}
	return &block, nil
}

func SaveLocal(block *Block, file string) error {
	fd, err := os.Create(file)
	if err != nil {
		return err
	}
	defer fd.Close()

	return json.NewEncoder(fd).Encode(block)
}

func main() {
	var (
		FilePath string
		RPC      string

		Timeout         time.Duration
		StalledDuration time.Duration
	)

	flag.StringVar(&FilePath, "file", "/tmp/is-l2geth-stalled.json", "an ephemeral file path")
	flag.DurationVar(&Timeout, "timeout", time.Second*3, "the timeout to send rpc request")
	flag.StringVar(&RPC, "rpc", "http://localhost:8545", "geth rpc endpoint")
	flag.DurationVar(&StalledDuration, "duration", 120*time.Second, "duration to check if the l2geth is stalled")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()

	local, err := GetLocal(FilePath)
	if err != nil {
		log.Fatalln("failed to get local file", err)
	}

	remote, err := GetRemote(ctx, RPC)
	if err != nil {
		log.Fatalln("failed to get block from rpc", err)
	}

	if local != nil && remote.Number == local.Number && remote.LastSeen.Sub(local.LastSeen) > StalledDuration {
		log.Fatalf("geth is stalled at %s in the past %s", remote.Number, StalledDuration)
	}

	if local == nil || remote.Number != local.Number {
		if err := SaveLocal(remote, FilePath); err != nil {
			log.Fatalln("failed to save the block file", err)
		}
	}
}
