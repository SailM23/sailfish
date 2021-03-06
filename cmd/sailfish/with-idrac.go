// +build !skip_idrac

package main

import (
	"context"
	"sync"

	eh "github.com/looplab/eventhorizon"
	"github.com/spf13/viper"

	"github.com/superchalupa/sailfish/src/dell-idrac"

	log "github.com/superchalupa/sailfish/src/log"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

func init() {
	implementations["dell_idrac"] = func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, viperMu *sync.RWMutex, ch eh.CommandHandler, eb eh.EventBus, d *domain.DomainObjects) Implementation {
		return dell_idrac.New(ctx, logger, cfgMgr, viperMu, ch, eb, d)
	}
}
