package lcl

import (
	"context"
	"fmt"

	eh "github.com/looplab/eventhorizon"
	eventpublisher "github.com/looplab/eventhorizon/publisher/local"

	//ah "github.com/superchalupa/sailfish/src/actionhandler"
	"github.com/superchalupa/sailfish/src/eventwaiter"
	"github.com/superchalupa/sailfish/src/log"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
	//mgrCMCIntegrated "github.com/superchalupa/sailfish/src/dell-resources/managers/cmc.integrated"
)

type viewer interface {
	GetUUID() eh.UUID
	GetURI() string
}

type LCLService struct {
	ch eh.CommandHandler
	eb eh.EventBus
	ew *eventwaiter.EventWaiter
}

func New(ch eh.CommandHandler, eb eh.EventBus) *LCLService {
	EventPublisher := eventpublisher.NewEventPublisher()
	eb.AddHandler(eh.MatchAnyEventOf(LogEvent), EventPublisher)
	EventWaiter := eventwaiter.NewEventWaiter(eventwaiter.SetName("LCL Service"), eventwaiter.NoAutoRun)
	EventPublisher.AddObserver(EventWaiter)
	go EventWaiter.Run()

	return &LCLService{
		ch: ch,
		eb: eb,
		ew: EventWaiter,
	}
}

// StartService will create a model, view, and controller for the eventservice, then start a goroutine to publish events
func (l *LCLService) StartService(ctx context.Context, logger log.Logger, rootView viewer) {
	// COLLECTION AGGREGATE to hold Lclog and Faultlist: /redfish/v1/Managers/CMC.Integrated.1/LogServices
	// AGGREGATE: /redfish/v1/Managers/CMC.Integrated.1/LogServices/Lclog
	// COLLECTION AGGREGATE: /redfish/v1/Managers/CMC.Integrated.1/Logs/Lclog.json  <-- lets save this for last
	//		^--- need a new feature: autoexpand - put this into the aggregate. Test the feature in redfish_handler and auto expand
	// COLLECTION MEMBER AGGREGATE: /redfish/v1/Managers/CMC.Integrated.1/Logs/Lclog/66.json
	//
	// SKIP FOR NOW (implement after LCL done) __redfish__v1__Managers__CMC.Integrated.1__Logs__FaultList.json
	lcLogUri := rootView.GetURI() + "/Logs/Lclog"
	lclLogger := logger.New("module", "LCL")

	// Start up goroutine that listens for log-specific events and creates log aggregates
	l.manageLcLogs(ctx, lclLogger, lcLogUri)
}

const MAX_LOGS = 100

// manageLcLogs starts a background process to create new log entreis
func (l *LCLService) manageLcLogs(ctx context.Context, logger log.Logger, logUri string) {

	// set up listener for the delete event
	// INFO: this listener will only ever get
	listener, err := l.ew.Listen(ctx,
		func(event eh.Event) bool {
			t := event.EventType()
			if t == LogEvent {
				if event.Data().(*LogEventData).MessageID == "" {
					return false
				}
				return true
			}
			return false
		},
	)
	if err != nil {
		return
	}

	go func() {
		defer listener.Close()
		lclogs := []eh.UUID{}

		inbox := listener.Inbox()
		for {
			select {
			case event := <-inbox:
				uuid := eh.NewUUID()
				uri := fmt.Sprintf("%s/%s", logUri, uuid)
				logger.Info("Processing logevent", "event", event)
				switch typ := event.EventType(); typ {
				case LogEvent:
					lclogs = append(lclogs, uuid)
					logEntry := event.Data().(*LogEventData)
					l.ch.HandleCommand(
						ctx,
						&domain.CreateRedfishResource{
							ID:          uuid,
							ResourceURI: uri,
							Type:        "#LogServiceCollection.LogServiceCollection",
							Context:     "/redfish/v1/$metadata#LogServiceCollection.LogServiceCollection",
							Privileges: map[string]interface{}{
								"GET":    []string{"ConfigureManager"},
								"POST":   []string{},
								"PUT":    []string{"ConfigureManager"},
								"PATCH":  []string{"ConfigureManager"},
								"DELETE": []string{"ConfigureManager"},
							},
							Properties: map[string]interface{}{
								"Description": logEntry.Description,
								"Name":        logEntry.Name,
								"EntryType":   logEntry.EntryType,
								"Id":          logEntry.Id,
								"MessageArgs": logEntry.MessageArgs,
								"Message":     logEntry.Message,
								"MessageID":   logEntry.MessageID,
								"Category":    logEntry.Category,
								"Severity":    logEntry.Severity,
								"Action":      logEntry.Action,
							}})
				}

			case <-ctx.Done():
				logger.Info("context is done")
				return
			}

			for len(lclogs) > MAX_LOGS {
				logger.Info("too many logs, trimming", "len", len(lclogs))
				toDelete := lclogs[0]
				lclogs = lclogs[1:]
				l.ch.HandleCommand(ctx, &domain.RemoveRedfishResource{ID: toDelete, ResourceURI: fmt.Sprintf("%s/%s", logUri, toDelete)})
			}
		}
	}()

	return
}
