package engine

import (
	"context"
	"log"
	"time"

	"github.com/abhiram/cortex-cc/internal/models"
)

var callerNames = []string{
	"Michael Johnson", "Sophia Williams", "David Brown", "Olivia Jones",
	"Daniel Davis", "Emma Wilson", "James Anderson", "Ava Taylor",
	"Christopher Martin", "Isabella Thomas", "Matthew Harris", "Mia Jackson",
	"Andrew White", "Charlotte Lewis", "Joshua Clark", "Amelia Walker",
}

var callerPrefixes = []string{
	"416", "647", "905", "613", "514", "604", "403", "780",
}

func (e *Engine) runCallGenerator(ctx context.Context) {
	t := time.NewTicker(callGenInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			e.generateCall()
		}
	}
}

func (e *Engine) generateCall() {
	e.mu.Lock()
	defer e.mu.Unlock()

	queue := queues[randN(len(queues))]
	name := callerNames[randN(len(callerNames))]
	prefix := callerPrefixes[randN(len(callerPrefixes))]
	number := prefix + "-" + randDigits(3) + "-" + randDigits(4)

	c := &models.Call{
		ID:           e.newCallID(),
		CallerNumber: number,
		CallerName:   name,
		QueueName:    queue,
		Status:       models.CallStatusQueued,
		Sentiment:    randFloat(0.2, 0.8), // start neutral-to-positive
		StartedAt:    time.Now(),
	}

	e.calls[c.ID] = c
	if err := e.store.UpsertCall(c); err != nil {
		log.Printf("engine: store call: %v", err)
	}
	e.emit("call_queued", c)
	log.Printf("engine: new call %s from %s → %s queue", c.ID, c.CallerName, c.QueueName)
}
