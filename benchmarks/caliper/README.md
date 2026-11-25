# Caliper registration benchmark

This workload measures signed data registration through Caliper's Fabric Gateway connector. A successful request requires a valid Fabric commit. Owner registration and Ed25519 signing occur during initialization, before the timed registration round.

## Install tools

Use Node.js 18.19.0 or later supported by Caliper 0.6.0. From the repository root:

~~~bash
mkdir -p runtime/caliper
cp benchmarks/caliper/package.json runtime/caliper/package.json
npm install --prefix runtime/caliper
runtime/caliper/node_modules/.bin/caliper bind --caliper-bind-sut fabric:2.4 --caliper-bind-cwd "$PWD/runtime/caliper"
~~~

The binding uses Fabric Gateway SDK 1.5.0. The resolved dependency lockfile remains under runtime/caliper.

## Configure and run

Install and build DIS-reconstruct using the [running guide](../../docs/runbook.md). Save this as runtime/caliper/deployment.yaml:

~~~yaml
version: 1
runtime: ../experiments/caliper
network:
  organizations: 5
  peers_per_organization: 1
  orderers: 5
  endorsement_threshold: 3
  gateway_concurrency: 500
  port_base: 31000
  domain: dis-local
  hosts:
    local:
      address: 127.0.0.1
services:
  api_port: 50080
  dns_port: 50530
  redis_port: 50679
~~~

Start and measure from the repository root:

~~~bash
./run.sh plan --config runtime/caliper/deployment.yaml
./run.sh up --config runtime/caliper/deployment.yaml
export DIS_RUNTIME="$PWD/runtime/experiments/caliper"
python3 scripts/benchmark-caliper.py --rates 50,100,150,200,250,300,350,400,450 --duration 30 --repetitions 3 --workers 4 --out runtime/benchmarks/caliper-run
~~~

Four local workers share one Gateway endpoint, each using a separate application owner. The offered rate is the aggregate across workers. Identifiers are unique and requests use the same signed registration fields as the Go workload. The runner generates its network configuration from the deployment's Gateway profile and randomizes round order using a fixed seed.

Run workloads sequentially and choose a new output directory for each run. To compare Gateway limits, use a separate configuration and runtime with gateway_concurrency set to 2500. Keep worker count, rates, round duration, endorsement, block settings, and hardware placement fixed.

Stop the deployment with:

~~~bash
./run.sh down --config runtime/caliper/deployment.yaml
~~~

## Results

The output contains the HTML report, logs, per-request results, environment settings, and summary.json. Per-request files retain transaction IDs, statuses, payloads, and timestamps. The runner checks their counts and business results against the report. Recheck saved results with:

~~~bash
python3 scripts/caliper_results.py runtime/benchmarks/caliper-run
~~~

Caliper 0.6.0 includes successful and failed completions in its throughput column. The summary also reports successful_commit_tps: valid commits divided by the interval from first request creation to last completion, including failed requests in the interval. Record actual send rates, successes, failures, and latency together. Duration-based scheduling can issue extra requests at round boundaries.

Registration is the timed operation. Setup, signing, and DNS verification are separate stages. Measure DNS query throughput and commit-to-DNS publication latency with their corresponding workloads.

Generated configurations, dependencies, and results remain under runtime. Generated Gateway configurations reference private-key paths; retain them within the experiment environment.

## Source-paper comparison

Figure 5 (PDF page 9) of [Data Interoperating Architecture (DIA): Decoupling Data and Applications to Give Back Your Data Ownership](https://ieeexplore.ieee.org/document/10197025) and Figure 12 (PDF pages 31–32) of [Dominate data by yourself: a decentralized scheme for data interoperation when data is decoupled from applications](https://link.springer.com/article/10.1007/s11280-025-01337-z) report five peers, five orderers, majority endorsement, and offered rates from 50 to 450 TPS.

The configuration above follows those topology and rate settings. Its workload is signed data registration, with 30-second rounds, three repetitions, four workers, and Gateway concurrency 500. These are explicit reconstruction settings. For comparisons, retain the operation definition, effective block settings, topology, machine placement, source revision, and measured completion criteria with each result.
