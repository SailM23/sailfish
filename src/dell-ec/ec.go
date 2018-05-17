// Build tags: only build this for the simulation build. Be sure to note the required blank line after.
// +build ec

package obmc

import (
	"context"
	"sync"
	"time"

	"github.com/spf13/viper"
	"io/ioutil"
	// "github.com/go-yaml/yaml"
	yaml "gopkg.in/yaml.v2"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	"github.com/superchalupa/go-redfish/src/log"
	plugins "github.com/superchalupa/go-redfish/src/ocp"
	"github.com/superchalupa/go-redfish/src/ocp/basicauth"
	"github.com/superchalupa/go-redfish/src/ocp/root"
	"github.com/superchalupa/go-redfish/src/ocp/session"

	attr_prop "github.com/superchalupa/go-redfish/src/dell-resources/attribute-property"
	attr_res "github.com/superchalupa/go-redfish/src/dell-resources/attribute-resource"
	"github.com/superchalupa/go-redfish/src/dell-resources/ec_manager"
	"github.com/superchalupa/go-redfish/src/dell-resources/chassis-iom"
)

type ocp struct {
	rootSvc             *root.Service
	sessionSvc          *session.Service
	basicAuthSvc        *basicauth.Service
	configChangeHandler func()
	logger              log.Logger
}

func (o *ocp) GetSessionSvc() *session.Service     { return o.sessionSvc }
func (o *ocp) GetBasicAuthSvc() *basicauth.Service { return o.basicAuthSvc }
func (o *ocp) ConfigChangeHandler()                { o.configChangeHandler() }

func New(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, viperMu *sync.Mutex, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) *ocp {
	// initial implementation is one BMC, one Chassis, and one System.
	// Yes, this function is somewhat long, however there really isn't any logic here. If we start getting logic, this needs to be split.

	logger = logger.New("module", "ec")
	self := &ocp{
		logger: logger,
	}

	self.rootSvc, _ = root.New()

	self.sessionSvc, _ = session.New(
		session.Root(self.rootSvc),
	)

	self.basicAuthSvc, _ = basicauth.New()

	cmc_integrated_1_svc, _ := ec_manager.New(
		ec_manager.WithUniqueName("CMC.Integrated.1"),
	)

	bmcAttrSvc, _ := attr_res.New(
		attr_res.BaseResource(cmc_integrated_1_svc),
		attr_res.WithURI("/redfish/v1/Managers/CMC.Integrated.1/Attributes"),
		attr_res.WithUniqueName("CMC.Integrated.1.Attributes"),
	)

	bmcAttrProp, _ := attr_prop.New(
		attr_prop.BaseResource(bmcAttrSvc),
		attr_prop.WithAttribute("test", "1", "A", "foo"),
		attr_prop.WithFQDD("CMC.Integrated.1"),
	)

    // one iom for now
    chassis_iom, _ := iom_chassis.New(
		ec_manager.WithUniqueName("CMC.Integrated.1"),
    )

	// VIPER Config:
	// pull the config from the YAML file to populate some static config options
	self.configChangeHandler = func() {
		logger.Info("Re-applying configuration from config file.")
		self.sessionSvc.ApplyOption(plugins.UpdateProperty("session_timeout", cfgMgr.GetInt("session.timeout")))
		for _, k := range []string{"name", "description", "model", "timezone", "version"} {
			cmc_integrated_1_svc.ApplyOption(plugins.UpdateProperty(k, cfgMgr.Get("managers."+cmc_integrated_1_svc.GetUniqueName()+"."+k)))
		}
	}
	self.ConfigChangeHandler()

	cfgMgr.SetDefault("main.dumpConfigChanges.filename", "redfish-changed.yaml")
	cfgMgr.SetDefault("main.dumpConfigChanges.enabled", "true")
	dumpViperConfig := func() {
		viperMu.Lock()
		defer viperMu.Unlock()

		dumpFileName := cfgMgr.GetString("main.dumpConfigChanges.filename")
		enabled := cfgMgr.GetBool("main.dumpConfigChanges.enabled")
		if !enabled {
			return
		}

		// TODO: change this to a streaming write (reduce mem usage)
		var config map[string]interface{}
		cfgMgr.Unmarshal(&config)
		output, _ := yaml.Marshal(config)
		_ = ioutil.WriteFile(dumpFileName, output, 0644)
	}

	self.sessionSvc.AddPropertyObserver("session_timeout", func(newval interface{}) {
		viperMu.Lock()
		cfgMgr.Set("session.timeout", newval.(int))
		viperMu.Unlock()
		dumpViperConfig()
	})

	// register all of the plugins (do this first so we dont get any race
	// conditions if somebody accesses the URIs before these plugins are
	// registered
	domain.RegisterPlugin(func() domain.Plugin { return self.rootSvc })
	domain.RegisterPlugin(func() domain.Plugin { return self.sessionSvc })
	domain.RegisterPlugin(func() domain.Plugin { return self.basicAuthSvc })
	domain.RegisterPlugin(func() domain.Plugin { return cmc_integrated_1_svc })
	domain.RegisterPlugin(func() domain.Plugin { return bmcAttrSvc })
	domain.RegisterPlugin(func() domain.Plugin { return bmcAttrProp })
	domain.RegisterPlugin(func() domain.Plugin { return chassis_iom })

	// and now add everything to the URI tree
	self.rootSvc.AddResource(ctx, ch, eb, ew)
	time.Sleep(1)
	self.sessionSvc.AddResource(ctx, ch, eb, ew)
	self.basicAuthSvc.AddResource(ctx, ch, eb, ew)

	logger.Info("adding cmc resources")
	cmc_integrated_1_svc.AddResource(ctx, ch, eb, ew)
	bmcAttrSvc.AddView(ctx, ch, eb, ew)
	bmcAttrProp.AddView(ctx, ch, eb, ew)
	bmcAttrProp.AddController(ctx, ch, eb, ew)

	chassis_iom.AddView(ctx, ch, eb, ew)
	chassis_iom.AddController(ctx, ch, eb, ew)

	return self
}