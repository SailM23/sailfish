// Copyright (c) 2014 - Max Ekman <max@looplab.se>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package domain

import (
	eh "github.com/superchalupa/eventhorizon"
)

const (
	// OdataCreatedEvent is when an Odata is created.
	OdataCreatedEvent eh.EventType = "OdataCreated"

	// OdataAcceptedEvent is when an Odata has been accepted by the guest.
	OdataPropertyChangedEvent eh.EventType = "OdataPropertyChanged"
)

func init() {
	eh.RegisterEventData(OdataCreatedEvent, func() eh.EventData {
		return &OdataCreatedData{}
	})
	eh.RegisterEventData(OdataPropertyChangedEvent, func() eh.EventData {
		return &OdataPropertyChangedData{}
	})
}

// OdataCreatedData is the event data for when an invite has been created.
type OdataCreatedData struct {
	id string
}

type OdataPropertyChangedData struct {
	PropertyName  string
	PropertyValue interface{}
}
