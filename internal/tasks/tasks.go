package tasks

import (
	"encoding/json"
	"time"

	"github.com/hibiken/asynq"
)

const (
	TypeLicenseExpire = "license:expire:check"
)

type ExpireLicensePayload struct{}

func NewLicenseExpireTask(opts ...asynq.Option) (*asynq.Task, error) {
	payload := ExpireLicensePayload{}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	uniqueOpt := asynq.Unique(1 * time.Hour)
	allOpts := append(opts, uniqueOpt)

	return asynq.NewTask(TypeLicenseExpire, payloadBytes, allOpts...), nil
}
