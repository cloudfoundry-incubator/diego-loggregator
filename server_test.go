package loggregator_test

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/go-loggregator/testhelpers/fakes"
	"code.cloudfoundry.org/localip"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type TestServer struct {
	receivers  chan loggregator_v2.Ingress_BatchSenderServer
	port       int
	tlsConfig  *tls.Config
	grpcServer *grpc.Server
}

func NewTestServer(serverCert, serverKey, caCert string) (*TestServer, error) {
	port, err := localip.LocalPort()
	if err != nil {
		return nil, err
	}

	cert, err := tls.LoadX509KeyPair(serverCert, serverKey)
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		ClientAuth:         tls.RequestClientCert,
		InsecureSkipVerify: false,
	}
	caCertBytes, err := ioutil.ReadFile(caCert)
	if err != nil {
		return nil, err
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCertBytes)
	tlsConfig.RootCAs = caCertPool

	return &TestServer{
		tlsConfig: tlsConfig,
		receivers: make(chan loggregator_v2.Ingress_BatchSenderServer),
		port:      int(port),
	}, nil
}

func NewInsecureTestServer() (*TestServer, error) {
	port, err := localip.LocalPort()
	if err != nil {
		return nil, err
	}

	return &TestServer{
		receivers: make(chan loggregator_v2.Ingress_BatchSenderServer),
		port:      int(port),
	}, nil
}

func (t *TestServer) Port() int {
	return t.port
}

func (t *TestServer) Receivers() chan loggregator_v2.Ingress_BatchSenderServer {
	return t.receivers
}

func (t *TestServer) Start() error {
	listener, err := net.Listen("tcp4", fmt.Sprintf("localhost:%d", t.port))
	if err != nil {
		return err
	}

	var opts []grpc.ServerOption
	if t.tlsConfig != nil {
		opts = append(opts, grpc.Creds(credentials.NewTLS(t.tlsConfig)))
	}
	t.grpcServer = grpc.NewServer(opts...)

	senderServer := &fakes.FakeIngressServer{}
	senderServer.BatchSenderStub = func(recv loggregator_v2.Ingress_BatchSenderServer) error {
		t.receivers <- recv
		return nil
	}
	loggregator_v2.RegisterIngressServer(t.grpcServer, senderServer)

	go t.grpcServer.Serve(listener)

	return nil
}

func (t *TestServer) Stop() {
	t.grpcServer.Stop()
}
