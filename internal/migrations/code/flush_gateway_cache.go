package code

import (
	"context"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/brocaar/lorawan"
	"github.com/mxc-foundation/lpwan-server/internal/storage"
)

// FlushGatewayCache flushes the gateway cache.
func FlushGatewayCache(db sqlx.Ext) error {
	var ids []lorawan.EUI64

	err := sqlx.Select(db, &ids, `
		select
			gateway_id
		from
			gateway
	`)
	if err != nil {
		return errors.Wrap(err, "select gateway ids error")
	}

	for _, id := range ids {
		if err := storage.FlushGatewayCache(context.Background(), storage.RedisPool(), id); err != nil {
			log.WithError(err).Error("flush gateway cache error")
		}
	}

	return nil
}
