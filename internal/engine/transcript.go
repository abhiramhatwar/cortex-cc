package engine

import (
	"context"
	"log"
	"time"

	"github.com/abhiram/cortex-cc/internal/models"
	"github.com/google/uuid"
)

var agentLines = map[string][]string{
	"Sales": {
		"Thank you for calling Sales, how can I help you today?",
		"I'd be happy to walk you through our current plans.",
		"That's a great question. Let me pull up your account.",
		"We have a promotion running this month that might interest you.",
		"I can offer you a 15% discount if you upgrade today.",
		"Let me transfer you to our specialist for that.",
		"Is there anything else I can help you with?",
	},
	"Billing": {
		"Thank you for calling Billing, how can I assist you?",
		"I can see your account here. What seems to be the issue?",
		"I understand your frustration, let me look into that charge.",
		"That payment was processed on the 3rd. Can you confirm your bank?",
		"I'm going to issue a credit to your account right away.",
		"Your new balance after the adjustment will be shown within 24 hours.",
		"Is there anything else regarding your bill I can help with?",
	},
	"Support": {
		"Thank you for calling Support, what issue are you experiencing?",
		"Let me run a diagnostic on your connection.",
		"Can you confirm your device model for me?",
		"I see there's an outage in your area that should be resolved shortly.",
		"Let's try restarting the device together.",
		"Your ticket number is SR-48291. We'll follow up by email.",
		"Is the issue resolved now or should I escalate this?",
	},
}

var customerLines = map[string][]string{
	"Sales": {
		"Hi, I'm calling about upgrading my plan.",
		"I saw an ad for a new bundle, can you tell me more?",
		"How much would it cost to add another line?",
		"I've been a customer for 5 years, can I get a better deal?",
		"I'm thinking of switching unless you can match this offer.",
		"What's the contract length for the business plan?",
		"Okay, let me think about it and call back.",
	},
	"Billing": {
		"Hi, I have a question about a charge on my bill.",
		"I was charged twice this month, I need that fixed.",
		"Why is my bill $40 more than last month?",
		"I cancelled that service in January, why am I still being charged?",
		"I already paid this online, can you check?",
		"This is the third time I'm calling about this issue.",
		"Fine, but I expect to see that credit applied.",
	},
	"Support": {
		"Hi, my phone line has been down since this morning.",
		"I can't make outbound calls, everything goes to a busy tone.",
		"The internet keeps dropping every hour or so.",
		"I've already restarted it three times, it's not helping.",
		"This is affecting my business, I need this fixed urgently.",
		"When will the outage be resolved?",
		"Okay, I'll try that. Give me a second.",
	},
}

func (e *Engine) runTranscriptGenerator(ctx context.Context) {
	t := time.NewTicker(transcriptInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			e.generateTranscriptLines()
		}
	}
}

func (e *Engine) generateTranscriptLines() {
	e.mu.RLock()
	activeCalls := make([]*models.Call, 0)
	for _, c := range e.calls {
		if c.Status == models.CallStatusActive {
			cp := *c
			activeCalls = append(activeCalls, &cp)
		}
	}
	e.mu.RUnlock()

	for _, c := range activeCalls {
		speaker := "agent"
		var line string
		if randN(2) == 0 {
			lines := agentLines[c.QueueName]
			if len(lines) == 0 {
				continue
			}
			line = lines[randN(len(lines))]
		} else {
			speaker = "customer"
			lines := customerLines[c.QueueName]
			if len(lines) == 0 {
				continue
			}
			line = lines[randN(len(lines))]
		}

		t := &models.Transcript{
			ID:        uuid.NewString(),
			CallID:    c.ID,
			Speaker:   speaker,
			Text:      line,
			Timestamp: time.Now(),
		}
		if err := e.store.InsertTranscript(t); err != nil {
			log.Printf("engine: insert transcript: %v", err)
			continue
		}
		e.emit("transcript_line", t)

		// Customer lines trigger both sentiment scoring and agent assist suggestions.
		// Both run in goroutines so they never block the transcript tick.
		if speaker == "customer" {
			callID := c.ID
			go e.UpdateSentiment(callID, line)

			if e.OnCustomerLine != nil {
				callCopy := *c
				go e.OnCustomerLine(&callCopy, line)
			}
		}
	}
}
