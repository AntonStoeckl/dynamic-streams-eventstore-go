package userland

import (
	"strconv"

	jsoniter "github.com/json-iterator/go"
)

const SomethingHasHappenedEventTypePrefix = "SomethingHasHappened"

type SomethingHasHappened struct {
	eventType string
	Payload   SomethingHasHappenedPayload
}

type SomethingHasHappenedPayload struct {
	ID              string
	SomeInformation string
}

func SomethingHasHappenedFromPayload(payload SomethingHasHappenedPayload, postfix int) SomethingHasHappened {
	return SomethingHasHappened{
		eventType: SomethingHasHappenedEventTypePrefix + strconv.Itoa(postfix),
		Payload: SomethingHasHappenedPayload{
			ID:              payload.ID,
			SomeInformation: payload.SomeInformation,
		},
	}
}

func SomethingHasHappenedFromJSON(eventJSON []byte) (SomethingHasHappened, error) {
	payload := new(SomethingHasHappenedPayload)
	err := jsoniter.ConfigFastest.Unmarshal(eventJSON, &payload)
	if err != nil {
		return SomethingHasHappened{}, err
	}

	return SomethingHasHappened{
		eventType: SomethingHasHappenedEventTypePrefix,
		Payload: SomethingHasHappenedPayload{
			ID:              payload.ID,
			SomeInformation: payload.SomeInformation,
		},
	}, nil
}

func (bc SomethingHasHappened) EventType() string {
	return bc.eventType
}

func (bc SomethingHasHappened) PayloadToJSON() ([]byte, error) {
	return jsoniter.ConfigFastest.Marshal(bc.Payload)
}
