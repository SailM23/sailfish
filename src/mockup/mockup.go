package mockup

import (
	"context"
	"sync"

	"github.com/spf13/viper"

	eh "github.com/looplab/eventhorizon"

	"github.com/superchalupa/sailfish/src/actionhandler"
	ah "github.com/superchalupa/sailfish/src/actionhandler"
	dell_ec "github.com/superchalupa/sailfish/src/dell-ec"
	"github.com/superchalupa/sailfish/src/dell-resources/registries"
	"github.com/superchalupa/sailfish/src/eventwaiter"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/awesome_mapper2"
	"github.com/superchalupa/sailfish/src/ocp/event"
	"github.com/superchalupa/sailfish/src/ocp/eventservice"
	"github.com/superchalupa/sailfish/src/ocp/session"
	"github.com/superchalupa/sailfish/src/ocp/stdcollections"
	"github.com/superchalupa/sailfish/src/ocp/telemetryservice"
	"github.com/superchalupa/sailfish/src/ocp/testaggregate"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
	"github.com/superchalupa/sailfish/src/stdmeta"
)

type waiter interface {
	Listen(context.Context, func(eh.Event) bool) (*eventwaiter.EventListener, error)
}

func New(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, ch eh.CommandHandler, eb eh.EventBus, d *domain.DomainObjects) {
	logger = logger.New("module", "ec")

	// These three all set up a waiter for the root service to appear, so init root service after.
	actionhandler.Setup(ctx, ch, eb)
	event.Setup(ch, eb)
	domain.StartInjectService(logger, eb)
	actionSvc := ah.StartService(ctx, logger, ch, eb)
	am2Svc, _ := awesome_mapper2.StartService(ctx, logger, eb)
	telemetryservice.Setup(ctx, actionSvc, ch, eb)
	pumpSvc := dell_ec.NewPumpActionSvc(ctx, logger, eb)

	// the package for this is going to change, but this is what makes the various mappers and view functions available
	instantiateSvc := testaggregate.New(ctx, logger, cfgMgr, cfgMgrMu, ch)
	evtSvc := eventservice.New(ctx, cfgMgr, cfgMgrMu, instantiateSvc, actionSvc, ch, eb)
	eventservice.RegisterAggregate(instantiateSvc)
	testaggregate.RegisterWithURI(instantiateSvc)
	testaggregate.RegisterPublishEvents(instantiateSvc, evtSvc)
	testaggregate.RegisterAggregate(instantiateSvc)
	testaggregate.RegisterAM2(instantiateSvc, am2Svc)
	testaggregate.RegisterPumpAction(instantiateSvc, actionSvc, pumpSvc)
	registries.RegisterAggregate(instantiateSvc)
	stdmeta.RegisterFormatters(instantiateSvc, d)
	stdcollections.RegisterAggregate(instantiateSvc)
	session.RegisterAggregate(instantiateSvc)

	// ignore unused for now
	_ = actionSvc

	// common parameters to instantiate that are used almost everywhere
	baseParams := map[string]interface{}{}
	baseParams["rooturi"] = "/redfish/v1"
	modParams := func(newParams map[string]interface{}) map[string]interface{} {
		ret := map[string]interface{}{}
		for k, v := range baseParams {
			ret[k] = v
		}
		for k, v := range newParams {
			ret[k] = v
		}
		return ret
	}

	//*********************************************************************
	//  /redfish/v1
	//*********************************************************************
	_, rootView, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, cfgMgrMu, "rootview", baseParams)
	baseParams["rootid"] = rootView.GetUUID()

	//*********************************************************************
	//  /redfish/v1/testview - a proof of concept test view and example
	//*********************************************************************
	instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, cfgMgrMu, "testview", map[string]interface{}{"rooturi": rootView.GetURI()})

	//*********************************************************************
	//  /redfish/v1/{Managers,Chassis,Systems,Accounts}
	//*********************************************************************
	_, _, _ = instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, cfgMgrMu, "chassis", modParams(map[string]interface{}{"collection_uri": "/redfish/v1/Chassis"}))
	_, _, _ = instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, cfgMgrMu, "systems", modParams(map[string]interface{}{"collection_uri": "/redfish/v1/Systems"}))
	_, _, _ = instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, cfgMgrMu, "managers", modParams(map[string]interface{}{"collection_uri": "/redfish/v1/Managers"}))
	_, accountSvcVw, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, cfgMgrMu, "accountservice", modParams(map[string]interface{}{}))
	baseParams["actsvc_uri"] = accountSvcVw.GetURI()
	baseParams["actsvc_id"] = accountSvcVw.GetUUID()
	_, _, _ = instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, cfgMgrMu, "roles", modParams(map[string]interface{}{"collection_uri": "/redfish/v1/AccountService/Roles"}))
	_, _, _ = instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, cfgMgrMu, "accounts", modParams(map[string]interface{}{"collection_uri": "/redfish/v1/AccountService/Accounts"}))

	//*********************************************************************
	//  Standard redfish roles
	//*********************************************************************
	stdcollections.AddStandardRoles(ctx, rootView.GetUUID(), rootView.GetURI(), ch)

	//*********************************************************************
	// /redfish/v1/Sessions
	//*********************************************************************
	_, sessionSvcVw, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, cfgMgrMu, "sessionservice", baseParams)
	baseParams["sessionsvc_id"] = sessionSvcVw.GetUUID()
	baseParams["sessionsvc_uri"] = sessionSvcVw.GetURI()
	session.SetupSessionService(instantiateSvc, sessionSvcVw, ch, eb)
	instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, cfgMgrMu, "sessioncollection", modParams(map[string]interface{}{"collection_uri": "/redfish/v1/SessionService/Sessions"}))

	//*********************************************************************
	// /redfish/v1/EventService
	// /redfish/v1/TelemetryService
	//*********************************************************************
	evtSvc.StartEventService(ctx, logger, instantiateSvc, baseParams)
	telemetryservice.StartTelemetryService(ctx, logger, rootView)
}
