/*
Copyright 2018 BlackRock, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package nats

import (
	"fmt"
	"time"

	"github.com/blackrock/axis/job"
	natsio "github.com/nats-io/go-nats"
	"go.uber.org/zap"
)

type nats struct {
	job.AbstractSignal
	stop             chan struct{}
	natsSubscription *natsio.Subscription
	msgCh            chan *natsio.Msg
}

// Start listening for NATS signals and send them on the events channel
func (n *nats) Start(events chan job.Event) error {
	n.Log.Info("starting", zap.String("url", n.NATS.URL), zap.String("subject", n.NATS.Subject))
	natsConn, err := natsio.Connect(n.NATS.URL)
	if err != nil {
		return fmt.Errorf("failed to connect to nats cluster url %s. Cause: %+v", n.NATS.URL, err.Error())
	}
	n.natsSubscription, err = natsConn.ChanSubscribe(n.NATS.Subject, n.msgCh)
	if err != nil {
		return fmt.Errorf("failed to subscribe to nats subject %s. Cause: %+v", n.NATS.Subject, err.Error())
	}

	go n.listen(events)
	return nil
}

func (n *nats) listen(events chan job.Event) {
	for {
		select {
		case natsMsg := <-n.msgCh:
			event := &event{
				AbstractEvent: job.AbstractEvent{},
				nats:          n,
				msg:           natsMsg,
				timestamp:     time.Now().UTC(),
			}
			// perform constraint checks
			err := n.CheckConstraints(event.GetTimestamp())
			n.Log.Debug("constraint check", zap.Error(err))
			if err != nil {
				event.SetError(err)
			}
			n.Log.Debug("sending nat event", zap.String("nodeID", event.GetID()))
			events <- event
		case <-n.stop:
			return
		}
	}
}

func (n *nats) Stop() error {
	defer close(n.msgCh)
	n.Log.Info("stopping", zap.String("url", n.NATS.URL), zap.String("subject", n.NATS.Subject))
	n.stop <- struct{}{}
	return n.natsSubscription.Unsubscribe()
}
