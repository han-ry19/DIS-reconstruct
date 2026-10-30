// Command dis-bench measures valid Fabric completions under a fixed offered load.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	"github.com/han-ry19/DIS-reconstruct/internal/fabric"
	"github.com/hyperledger/fabric-gateway/pkg/client"
)

type sample struct {
	Index        int     `json:"index"`
	Object       string  `json:"object_id"`
	Started      float64 `json:"started_seconds"`
	Finished     float64 `json:"finished_seconds"`
	Latency      float64 `json:"latency_ms"`
	SchedulerLag float64 `json:"scheduler_lag_ms"`
	Status       string  `json:"status"`
	Error        string  `json:"error,omitempty"`
}

func category(err error) string {
	if err == nil {
		return "valid"
	}
	var commit *client.CommitError
	if errors.As(err, &commit) {
		return "commit_" + commit.Code.String()
	}
	var endorse *client.EndorseError
	if errors.As(err, &endorse) {
		return "endorsement_error"
	}
	return "request_error"
}
func quantile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	index := int(float64(len(values)-1) * p)
	return values[index]
}
func statistics(values []float64) map[string]float64 {
	if len(values) == 0 {
		return map[string]float64{}
	}
	sort.Float64s(values)
	total := 0.0
	for _, v := range values {
		total += v
	}
	return map[string]float64{"mean": total / float64(len(values)), "p50": quantile(values, .5), "p95": quantile(values, .95), "p99": quantile(values, .99), "max": values[len(values)-1]}
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	config := flag.String("fabric-config", "runtime/fabric/gateway.json", "Gateway profile")
	operation := flag.String("operation", "data.register", "data.register, identity.register, identity.read, or data.read")
	rate := flag.Int("rate", 50, "Scheduled requests per second")
	duration := flag.Duration("duration", 10*time.Second, "Request scheduling window")
	capacity := flag.Int("max-inflight", 2048, "Maximum simultaneous calls; excess requests are recorded as client_limited")
	output := flag.String("out", "runtime/benchmark", "Result directory")
	flag.Parse()
	if *rate < 1 || *duration <= 0 || *capacity < 1 {
		return fmt.Errorf("rate, duration, and max-inflight must be positive")
	}
	if *operation != "data.register" && *operation != "identity.register" && *operation != "identity.read" && *operation != "data.read" {
		return fmt.Errorf("unsupported operation")
	}
	if err := os.MkdirAll(*output, 0755); err != nil {
		return err
	}
	backend, err := fabric.Open(*config)
	if err != nil {
		return err
	}
	defer backend.Close()
	state, err := backend.View()
	if err != nil {
		return err
	}
	var nonce [8]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return err
	}
	prefix := "bench-" + hex.EncodeToString(nonce[:])
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	owner := prefix + ".uid"
	setup := domain.Request{Domain: state.Domain, Operation: "identity.register", Actor: owner, RequestID: prefix + "-setup", Timestamp: time.Now().Unix(), Parameters: domain.Parameters{IdentityID: owner, PublicKey: base64.StdEncoding.EncodeToString(pub), Kind: "user"}}
	if err = domain.Sign(&setup, key); err != nil {
		return err
	}
	if _, err = backend.Submit(setup); err != nil {
		return fmt.Errorf("setup: %w", err)
	}
	total := int(float64(*rate) * duration.Seconds())
	if total < 1 {
		return fmt.Errorf("rate and duration must schedule at least one request")
	}
	existingData := []string{}
	for id := range state.Data {
		existingData = append(existingData, id)
	}
	sort.Strings(existingData)
	if *operation == "data.read" && len(existingData) == 0 {
		return fmt.Errorf("data.read requires registered data")
	}
	requests := make([]domain.Request, total)
	dataSignature := base64.StdEncoding.EncodeToString(ed25519.Sign(key, []byte("benchmark data payload")))
	for i := range requests {
		label := fmt.Sprintf("%s-%d", prefix, i)
		request := domain.Request{Domain: state.Domain, Operation: *operation, Actor: owner, RequestID: label, Timestamp: time.Now().Unix()}
		if *operation == "identity.register" {
			request.Actor = label + ".uid"
			request.Parameters = domain.Parameters{IdentityID: request.Actor, PublicKey: base64.StdEncoding.EncodeToString(pub), Kind: "user"}
		} else {
			request.Parameters = domain.Parameters{DataID: label + ".data", Address: "https://data.example/benchmark/item", DataSignature: dataSignature, Profile: &domain.DataProfile{Label: "benchmark", Format: "text/plain", Description: "Identifier registration workload"}}
		}
		if *operation == "data.read" {
			request.Parameters.DataID = existingData[i%len(existingData)]
		}
		if err = domain.Sign(&request, key); err != nil {
			return err
		}
		requests[i] = request
	}
	samples := make([]sample, total)
	semaphore := make(chan struct{}, *capacity)
	var workers sync.WaitGroup
	start := time.Now()
	for i, request := range requests {
		due := start.Add(time.Duration(float64(i) / float64(*rate) * float64(time.Second)))
		if delay := time.Until(due); delay > 0 {
			time.Sleep(delay)
		}
		samples[i] = sample{Index: i, Object: request.Parameters.DataID, Status: "client_limited", SchedulerLag: float64(time.Since(due)) / float64(time.Millisecond)}
		if *operation == "identity.register" {
			samples[i].Object = request.Actor
		}
		select {
		case semaphore <- struct{}{}:
		default:
			continue
		}
		workers.Add(1)
		go func(i int, request domain.Request, due time.Time) {
			defer workers.Done()
			defer func() { <-semaphore }()
			callStart := time.Now()
			var err error
			if *operation == "identity.read" {
				_, err = backend.GetIdentity(owner)
			} else if *operation == "data.read" {
				_, err = backend.GetData(request.Parameters.DataID)
			} else {
				_, err = backend.Submit(request)
			}
			end := time.Now()
			s := sample{Index: i, Object: samples[i].Object, Started: callStart.Sub(start).Seconds(), Finished: end.Sub(start).Seconds(), Latency: float64(end.Sub(callStart)) / float64(time.Millisecond), SchedulerLag: float64(callStart.Sub(due)) / float64(time.Millisecond), Status: category(err)}
			if err != nil {
				s.Error = err.Error()
			}
			samples[i] = s
		}(i, request, due)
	}
	if delay := time.Until(start.Add(*duration)); delay > 0 {
		time.Sleep(delay)
	}
	workers.Wait()
	elapsed := time.Since(start).Seconds()
	raw, err := os.Create(filepath.Join(*output, "requests.jsonl"))
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(raw)
	counts := map[string]int{}
	latencies, lags := []float64{}, []float64{}
	validWindow := 0
	successfulIDs := []string{}
	for _, s := range samples {
		if err = encoder.Encode(s); err != nil {
			raw.Close()
			return err
		}
		counts[s.Status]++
		lags = append(lags, s.SchedulerLag)
		if s.Status == "valid" {
			latencies = append(latencies, s.Latency)
			successfulIDs = append(successfulIDs, s.Object)
			if s.Finished <= duration.Seconds() {
				validWindow++
			}
		}
	}
	if err = raw.Close(); err != nil {
		return err
	}
	after, err := backend.View()
	if err != nil {
		return err
	}
	report := map[string]any{"operation": *operation, "offered_tps": *rate, "schedule_seconds": duration.Seconds(), "elapsed_seconds": elapsed, "scheduled": total, "status_counts": counts, "success_rate": float64(counts["valid"]) / float64(total), "valid_throughput_tps": float64(counts["valid"]) / elapsed, "valid_completions_during_schedule_tps": float64(validWindow) / duration.Seconds(), "valid_latency_ms": statistics(latencies), "scheduler_lag_ms": statistics(lags), "max_inflight": *capacity, "sequence_before": state.Sequence, "sequence_after": after.Sequence, "seed_identity": owner, "successful_ids": successfulIDs, "started_at": start.UTC().Format(time.RFC3339Nano)}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(*output, "summary.json"), encoded, 0644); err != nil {
		return err
	}
	delete(report, "successful_ids")
	return json.NewEncoder(os.Stdout).Encode(report)
}
