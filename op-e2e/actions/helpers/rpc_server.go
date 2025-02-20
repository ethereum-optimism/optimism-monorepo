package helpers

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

type SimpleRPCServer struct {
	handler  *rpc.Server
	listener net.Listener
	apis     []rpc.API
	ipcDir   string
}

func NewSimpleRPCServer() *SimpleRPCServer {
	return &SimpleRPCServer{
		handler: rpc.NewServer(),
	}
}

func (s *SimpleRPCServer) AddAPI(api rpc.API) {
	s.apis = append(s.apis, api)
}

func (s *SimpleRPCServer) Start(t Testing) {
	t.Cleanup(func() {
		require.NoError(t, s.Stop())
	})

	// Register all APIs to the RPC server.
	for _, api := range s.apis {
		if err := s.handler.RegisterName(api.Namespace, api.Service); err != nil {
			require.NoError(t, fmt.Errorf("failed to register API %s: %w", api.Namespace, err))
		}
		t.Logf("registered API namespace %v", api.Namespace)
	}

	// Note: We don't use t.TempDir() here because it includes the test name in the path which can make it longer
	// than the 104 char limit for ipc socket paths.
	ipcDir, err := os.MkdirTemp("", "ipc*")
	require.NoError(t, err, "failed to create temp dir for IPC")
	s.ipcDir = ipcDir
	listener, err := ipcListen(t, s.ipcDir)
	require.NoError(t, err, "failed to start ipc listener")
	s.listener = listener
	go func() {
		err := s.handler.ServeListener(listener)
		if !errors.Is(err, net.ErrClosed) {
			require.NoError(t, err)
		}
	}()
}

func (s *SimpleRPCServer) Connect(t Testing) client.RPC {
	ipc, err := rpc.DialIPC(t.Ctx(), s.listener.Addr().String())
	require.NoError(t, err)
	return client.NewBaseRPCClient(ipc)
}

func (s *SimpleRPCServer) Stop() error {
	if s.listener != nil {
		// Best effort to close, but make sure we go on to remove the file
		_ = s.listener.Close()
	}
	if s.ipcDir != "" {
		return os.RemoveAll(s.ipcDir)
	}
	return nil
}

// ipcListen will create a Unix socket on the given endpoint.
func ipcListen(t Testing, dir string) (net.Listener, error) {
	endpoint := filepath.Join(dir, "ipc")
	l, err := net.Listen("unix", endpoint)
	if err != nil {
		return nil, err
	}
	return l, nil
}
