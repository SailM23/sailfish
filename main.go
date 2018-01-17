package main

import (
	"context"
	"fmt"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"time"

	eh "github.com/looplab/eventhorizon"

	domain "github.com/superchalupa/go-redfish/internal/redfishresource"
	"github.com/superchalupa/go-redfish/plugins/session"
)

func main() {
	log.Println("starting backend")

	domainObjs, _ := NewDomainObjects()
	domainObjs.EventPublisher.AddObserver(&Logger{})
	domain.RegisterRRA(domainObjs.EventBus)

	orighandler := domainObjs.CommandHandler

	// Create a tiny logging middleware for the command handler.
	loggingHandler := eh.CommandHandlerFunc(func(ctx context.Context, cmd eh.Command) error {
		log.Printf("CMD %#v", cmd)
		return orighandler.HandleCommand(ctx, cmd)
	})

	domainObjs.CommandHandler = loggingHandler

	// generate uuid of root object
	rootID := eh.NewUUID()

	// Set up our standard extensions
	AddUserDetails := session.SetupSessionService(context.Background(), rootID, domainObjs.EventWaiter, domainObjs.CommandHandler, domainObjs.EventBus)
	chainAuth := func(u string, p []string) http.Handler {
		// set up a new handler object for each request
		return &RedfishHandler{UserName: u, Privileges: p, d: domainObjs}
	}
	AddUserDetails.OnUserDetails = chainAuth

	// set up some basic stuff
	domainObjs.Tree["/redfish/v1/"] = rootID
	loggingHandler.HandleCommand(
		context.Background(),
		&domain.CreateRedfishResource{
			ID:          rootID,
			ResourceURI: "/redfish/v1/",
			Type:        "#ServiceRoot.v1_0_2.ServiceRoot",
			Context:	 "/redfish/v1/$metadata#ServiceRoot",
            // anybody can access
            Privileges:                          map[string]interface{}{"GET": []string{"Unauthenticated"}},
			Properties: map[string]interface{}{
				"Id":                 "RootService",
				"Name":               "Root Service",
				"RedfishVersion":     "1.0.2",
				"UUID":               "92384634-2938-2342-8820-489239905423",
				"@Redfish.Copyright": "Copyright 2014-2016 Distributed Management Task Force, Inc. (DMTF). For the full DMTF copyright policy, see http://www.dmtf.org/about/policies/copyright.",
			},
		},
	)

	// Handle the API.
	m := mux.NewRouter()

	// per spec: redirect /redfish to /redfish/
	m.Path("/redfish").HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/redfish/", 301) })
	// per spec: hardcoded output for /redfish/ to list versions supported.
	m.Path("/redfish/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("{\n\t\"v1\": \"/redfish/v1/\"\n}\n")) })
	// per spec: redirect /redfish/v1 to /redfish/v1/
	m.Path("/redfish/v1").HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/redfish/v1/", 301) })

	// generic handler for redfish output on most http verbs
	// Note: this works by using the session service to get user details from token to pass up the stack using the embedded struct
	m.PathPrefix("/redfish/v1/").Methods("GET", "PUT", "POST", "PATCH", "DELETE", "HEAD", "OPTIONS").Handler(AddUserDetails)

	// backend command handling
	m.PathPrefix("/api/createresource").Handler(domainObjs.MakeHandler(domain.CreateRedfishResourceCommand))
	m.PathPrefix("/api/removeresource").Handler(domainObjs.MakeHandler(domain.RemoveRedfishResourceCommand))

	// Simple HTTP request logging.
	logger := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func(begin time.Time) {
			log.Println(
				"source", r.RemoteAddr,
				"method", r.Method,
				"url", r.URL,
				"business_logic_time", time.Since(begin),
				"args", fmt.Sprintf("%#v", mux.Vars(r)),
			)
		}(time.Now())
		m.ServeHTTP(w, r)
	})

	s := &http.Server{
		Addr:           ":8080",
		Handler:        logger,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	log.Println(s.ListenAndServe())
}

// Logger is a simple event handler for logging all events.
type Logger struct{}

// Notify implements the Notify method of the EventObserver interface.
func (l *Logger) Notify(ctx context.Context, event eh.Event) {
	log.Printf("EVENT %s", event)
}
