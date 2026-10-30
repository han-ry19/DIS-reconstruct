package fabric

import (
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	"github.com/hyperledger/fabric-gateway/pkg/client"
	"github.com/hyperledger/fabric-gateway/pkg/identity"
	gateway "github.com/hyperledger/fabric-protos-go-apiv2/gateway"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"os"
	"strings"
	"time"
)

type Config struct {
	Endpoint    string `json:"endpoint"`
	ServerName  string `json:"server_name"`
	TLSCert     string `json:"tls_cert"`
	MSPID       string `json:"msp_id"`
	Certificate string `json:"certificate"`
	PrivateKey  string `json:"private_key"`
	Channel     string `json:"channel"`
	Chaincode   string `json:"chaincode"`
}
type Backend struct {
	network    *client.Network
	chaincode  string
	connection *grpc.ClientConn
	gateway    *client.Gateway
	contract   *client.Contract
}

func Open(path string) (*Backend, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	var cfg Config
	if e = json.Unmarshal(b, &cfg); e != nil {
		return nil, e
	}
	ca, e := os.ReadFile(cfg.TLSCert)
	if e != nil {
		return nil, e
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(ca) {
		return nil, errors.New("invalid TLS certificate")
	}
	certPEM, e := os.ReadFile(cfg.Certificate)
	if e != nil {
		return nil, e
	}
	cert, e := identity.CertificateFromPEM(certPEM)
	if e != nil {
		return nil, e
	}
	id, e := identity.NewX509Identity(cfg.MSPID, cert)
	if e != nil {
		return nil, e
	}
	keyPEM, e := os.ReadFile(cfg.PrivateKey)
	if e != nil {
		return nil, e
	}
	key, e := identity.PrivateKeyFromPEM(keyPEM)
	if e != nil {
		return nil, e
	}
	sign, e := identity.NewPrivateKeySign(key)
	if e != nil {
		return nil, e
	}
	conn, e := grpc.Dial(cfg.Endpoint, grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(pool, cfg.ServerName)), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(64<<20)))
	if e != nil {
		return nil, e
	}
	gw, e := client.Connect(id, client.WithSign(sign), client.WithClientConnection(conn), client.WithEvaluateTimeout(15*time.Second), client.WithEndorseTimeout(30*time.Second), client.WithSubmitTimeout(15*time.Second), client.WithCommitStatusTimeout(60*time.Second))
	if e != nil {
		conn.Close()
		return nil, e
	}
	return &Backend{connection: conn, gateway: gw, network: gw.GetNetwork(cfg.Channel), chaincode: cfg.Chaincode, contract: gw.GetNetwork(cfg.Channel).GetContract(cfg.Chaincode)}, nil
}
func (b *Backend) Close()       { b.gateway.Close(); b.connection.Close() }
func (b *Backend) Name() string { return "fabric" }
func (b *Backend) Submit(r domain.Request) (domain.Result, error) {
	raw, e := json.Marshal(r)
	if e != nil {
		return domain.Result{}, e
	}
	payload, e := b.contract.SubmitTransaction("Submit", string(raw))
	if e != nil {
		return domain.Result{}, classify(e)
	}
	var result domain.Result
	e = json.Unmarshal(payload, &result)
	return result, e
}
func (b *Backend) View() (domain.State, error) {
	payload, e := b.contract.EvaluateTransaction("GetSnapshot")
	if e != nil {
		return domain.State{}, e
	}
	var state domain.State
	e = json.Unmarshal(payload, &state)
	return state, e
}
func classify(e error) error {
	var endorsement *client.EndorseError
	if !errors.As(e, &endorsement) {
		return e
	}
	for _, raw := range status.Convert(endorsement).Details() {
		detail, ok := raw.(*gateway.ErrorDetail)
		if !ok {
			continue
		}
		message := detail.Message
		for _, known := range []error{domain.ErrInvalid, domain.ErrUnauthorized, domain.ErrNotFound, domain.ErrConflict} {
			if strings.Contains(message, known.Error()) {
				return fmt.Errorf("%w: chaincode rejected request", known)
			}
		}
	}
	return e
}

// CheckEndorsement verifies discovery and the deployed policy without committing.
func (b *Backend) CheckEndorsement() error {
	proposal, e := b.contract.NewProposal("GetSnapshot")
	if e != nil {
		return e
	}
	_, e = proposal.Endorse()
	return e
}
