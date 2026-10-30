# Caliper registration benchmark

This workload measures signed data registration through Caliper's Fabric peer Gateway connector. Successful samples require a valid Fabric commit. Owner registration and Ed25519 request signing occur during workload initialization, outside the timed registration round.

## Install pinned tools

From the repository root on the Linux experiment controller:

~~~bash
mkdir -p runtime/caliper
cp benchmarks/caliper/package.json runtime/caliper/package.json
npm install --prefix runtime/caliper
runtime/caliper/node_modules/.bin/caliper bind --caliper-bind-sut fabric:2.4 --caliper-bind-cwd "$PWD/runtime/caliper"
~~~

Use Node.js 18.19.0 or later supported by Caliper 0.6.0. The binding uses Fabric Gateway SDK 1.5.0. Each installation retains its resolved dependency lockfile under runtime/caliper.

## Deploy and measure

Build and install project dependencies following the running guide. This example isolates the benchmark from other deployments:

~~~bash
export DIS_RUNTIME="$PWD/runtime/experiments/caliper"
python3 scripts/network.py up --topology deploy/topologies/caliper-5org.yaml
python3 scripts/services.py up --api-port 50080 --dns-port 50530 --redis-port 50679
python3 scripts/benchmark-caliper.py --rates 50,300,450 --duration 30 --repetitions 3 --workers 4 --out runtime/benchmarks/caliper-run
~~~

The runner generates a network configuration from the selected deployment's Gateway profile, referencing its TLS certificates and Fabric signing identity. Four workers share one Gateway endpoint and each uses a separate application owner. The offered rate is the aggregate across workers. Data identifiers are unique; no automatic business retry is added. Pre-signed request payload fields match the Go registration benchmark.

The report and logs record actual send rate, successful and failed completions, latency, and throughput using Caliper's definitions. Caliper 0.6.0 counts both successful and failed completions in its throughput column. The validated summary additionally reports successful_commit_tps, calculated as valid commits divided by the interval from the first request creation to the last completion (including failures). Per-worker files retain transaction IDs, statuses, result payloads, and timestamps. After execution, the runner validates their counts and business results against the HTML report and writes summary.json. Existing output can be checked again with python3 scripts/caliper_results.py OUTPUT. The configured rate is a target; Caliper's duration-based scheduling may issue a few extra requests at a round boundary.

Only registration calls are timed. Setup transactions, signing, and later DNS verification are outside that interval. Fabric commit and DNS visibility are separate completion conditions. Run performance workloads sequentially and use a new output directory for every run.

## Gateway concurrency

The default five-organization topology retains Fabric 2.4.9's Gateway concurrency limit of 500. Use deploy/topologies/caliper-5org-gateway2500.yaml with a fresh DIS_RUNTIME to measure the same workload with 2500 concurrent Gateway RPCs per peer. The two topology files otherwise use the same network settings. Stop a deployment before reusing its port range. Keep worker count, offered rates, round duration, endorsement policy, and block settings fixed when comparing these configurations.

## Artifacts

All generated configurations, reports, dependency installations, and measurement samples remain under runtime and are ignored by Git. The network configuration references private key paths; keep generated artifacts within the experiment environment. The repository contains reusable workload code and portable instructions.

After measurements, stop the isolated deployment with services.py down and network.py down under the same DIS_RUNTIME.
