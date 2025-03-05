package infra

import (
	"context"
	"encoding/json"
	"expvar"
	"time"

	"github.com/luizaranda/go-core/pkg/telemetry"
)

func exportedVarPolling(ctx context.Context, tracer telemetry.Client) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			exportedVarPoolHTTP(tracer)
		case <-ctx.Done():
			return
		}
	}
}

type pooledTransportPoolInfo map[string]map[string]int64

func exportedVarPoolHTTP(tracer telemetry.Client) {
	v := expvar.Get("toolkit.http.client.conn_pools")
	if v == nil {
		return
	}

	var info pooledTransportPoolInfo
	if err := json.Unmarshal([]byte(v.String()), &info); err != nil {
		return
	}

	for pool, v := range info {
		for network, conns := range v {
			tracer.Gauge("toolkit.http.client.conn_pool", float64(conns), telemetry.Tags("pool", pool, "network", network))
		}
	}
}
