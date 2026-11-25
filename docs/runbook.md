# Running DIS-reconstruct

## Install and build

Use Linux amd64 with Git, Go 1.23.4 or later, Python 3.9 or later with venv support, a C compiler, and make.

~~~bash
git clone https://github.com/han-ry19/DIS-reconstruct.git
cd DIS-reconstruct
python3 -m venv .venv
. .venv/bin/activate
python -m pip install PyYAML
python scripts/bootstrap.py
go build -o bin/ ./cmd/...
~~~

Bootstrap installs Fabric 2.4.9, CoreDNS 1.12.0 release assets, and Redis 7.2.5 under runtime/tools. The build produces the application commands and CoreDNS with the Redis plugin. Offline archives can be supplied with --archive, --coredns-archive, and --redis-archive; use python scripts/bootstrap.py --help for options.

## Configure and start

Edit [config.yaml](../config.yaml), then run:

~~~bash
./run.sh plan
./run.sh
./run.sh status
~~~

The default configuration uses runtime/configured, four organizations, four peers, four Raft orderers, and three-organization endorsement. The API listens on 127.0.0.1:38080, DNS on 127.0.0.1:38530, and Redis on 127.0.0.1:38679. Startup prints node endpoints, service ports, log paths, and readiness progress.

For another deployment, pass --config PATH to each command. See the [configuration guide](../config/README.md) and [deployment guide](experiment-deployment.md).

## Run a complete workflow

~~~bash
bin/dis-demo -url http://127.0.0.1:38080
bin/dis-sync -fabric-config runtime/configured/fabric/gateway.json \
  -redis 127.0.0.1:38679 -check -verify 127.0.0.1:38530
~~~

The demo registers a user and application, registers data, requests Read access, records the owner's approval, and queries the resulting authorization. Each run generates fresh identifiers. The check command compares Redis and both complete authoritative DNS zones with the committed ledger state. Adjust addresses and paths when using a different configuration.

## Sign and submit a request

~~~bash
bin/dis-cli keygen -out identity.key
bin/dis-cli sign -key identity.key -request request.json > signed-request.json
curl --fail-with-body -H 'Content-Type: application/json' \
  --data-binary @signed-request.json http://127.0.0.1:38080/v1/transactions
~~~

The key generator prints the public key and writes a private key with owner-only permissions. Request fields are defined in the [API schema](../api/openapi.yaml). The request domain must match the deployment. Use a fresh request ID and current timestamp. Exact retries return the original result; changed content with the same actor/request ID is rejected. See [management operations](management-updates.md) for update examples.

## Stop and restart

~~~bash
./run.sh down
./run.sh
~~~

Certificates, ledger data, Redis persistence, checkpoints, and logs remain in the configured runtime directory. Logs are in fabric/ and services/; remote node logs reside under each worker's runtime directory.

## Verify

Run the commands in [system verification](verification.md). Acceptance scripts take their runtime from DIS_RUNTIME. For the default configuration:

~~~bash
export DIS_RUNTIME="$PWD/runtime/configured"
python3 scripts/acceptance.py --restart --all-gateways
python3 scripts/acceptance-events.py
~~~

These checks submit transactions and restart managed components; use a dedicated test deployment.

## Direct component management

For component-level operations, set DIS_RUNTIME to the configured absolute runtime path, then use scripts/network.py and scripts/services.py. The unified launcher sets this variable automatically. Direct managers without DIS_RUNTIME use runtime/ and their own default ports (API 28080, DNS 28530, Redis 28679).

## Development file backend

~~~bash
bin/dis-api -listen 127.0.0.1:8080 -state runtime/development-state.json -domain dis-local
~~~

Run bin/dis-demo -url http://127.0.0.1:8080 in another terminal. This backend runs the business rules against an atomic local state file.

## Troubleshooting

For occupied ports, select available ports in a new configuration. For a state lock, check the process already using that runtime. For rejected timestamps, regenerate and sign the request. For component exits, inspect fabric/ and services/ logs in the configured runtime before restarting.
