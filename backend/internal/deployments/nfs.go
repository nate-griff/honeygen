package deployments

import (
	"context"
	"fmt"
	"net"

	"github.com/go-git/go-billy/v5/osfs"
	nfs "github.com/willscott/go-nfs"
	nfshelper "github.com/willscott/go-nfs/helpers"
)

func (m *Manager) startNFS(d Deployment, filePath string, srvCtx context.Context, cancel context.CancelFunc) (func(), error) {
	addr := fmt.Sprintf(":%d", d.Port)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("nfs listen on %s: %w", addr, err)
	}

	bfs := osfs.New(filePath)
	handler := nfshelper.NewNullAuthHandler(bfs)
	handler = nfshelper.NewCachingHandler(handler, 1024)

	go func() {
		m.logger.Info("nfs deployment started", "id", d.ID, "addr", addr, "world_model", d.WorldModelID)
		if err := nfs.Serve(listener, handler); err != nil {
			if srvCtx.Err() == nil {
				m.logger.Error("nfs deployment server error", "id", d.ID, "error", err)
			}
		}
		cancel()
	}()

	stopFn := func() {
		_ = listener.Close()
	}

	return stopFn, nil
}
