package kb

import (
	"log"
	"time"

	"github.com/google/uuid"

	"cortex-cc/internal/models"
	"cortex-cc/internal/store"
)

// SeedIfEmpty inserts sample KB articles when the table is empty.
// Safe to call on every startup — no-op if articles already exist.
func SeedIfEmpty(st *store.Store) {
	n, err := st.KBCount()
	if err != nil || n > 0 {
		return
	}

	articles := []struct {
		title   string
		content string
		tags    []string
	}{
		{
			title: "Billing Dispute Resolution Policy",
			content: `When a customer reports a billing dispute, follow these steps:
1. Verify the customer's identity and account number.
2. Pull the last 3 billing cycles and compare charges.
3. If the overcharge is under $50, agents are authorized to issue a credit immediately.
4. For overcharges $50-$200, a supervisor must approve the credit within 24 hours.
5. For overcharges over $200, escalate to the Billing Disputes team (ext. 4100).
6. Document the dispute reason in the call notes before ending the call.
Resolution target: disputes resolved or escalated within 10 minutes of reporting.`,
			tags: []string{"billing", "disputes", "credits", "policy"},
		},
		{
			title: "Refund Processing Procedure",
			content: `Refund eligibility and processing:
- Refunds are available within 30 days of charge for unused services.
- Agent-authorized refunds: up to $100 per transaction.
- Manager-authorized refunds: $100-$500.
- Director approval required: over $500.
Processing steps:
1. Confirm eligibility (within 30-day window, unused service).
2. Select "Refund" in the billing system, enter amount and reason code.
3. Refunds to credit card: 5-7 business days.
4. Refunds to bank account: 3-5 business days.
5. Provide the customer with a confirmation number.
Note: Refunds for cancelled services follow the Cancellation Policy.`,
			tags: []string{"refund", "billing", "processing"},
		},
		{
			title: "Escalation Matrix",
			content: `Use this matrix to determine when and how to escalate a call:

LEVEL 1 — Agent can handle:
- General billing questions
- Service plan changes
- Password resets, basic troubleshooting

LEVEL 2 — Supervisor required:
- Customer threatening to cancel after 3+ years
- Disputes over $50
- Legal threats or mentions of regulatory bodies
- Customer has been on hold more than 20 minutes

LEVEL 3 — Specialist team:
- Billing: ext. 4100 (Billing Disputes)
- Technical: ext. 4200 (Tier 2 Support)
- Legal: ext. 4500 (Compliance Team)
- Media/PR threats: ext. 4600 (Communications)

Always document the reason for escalation and the name of the person you transferred to.`,
			tags: []string{"escalation", "supervisor", "procedure"},
		},
		{
			title: "SLA Definitions and Breach Remedies",
			content: `Service Level Agreements (SLAs):

Queue wait time SLA: Calls should be answered within 120 seconds.
- Breach remedy: $5 account credit for residential, $25 for business accounts.

First-call resolution (FCR) target: 75% of issues resolved on first contact.
- If not resolved: follow-up within 24 hours is mandatory.

Agent handle time target: average 4 minutes per call.
- Calls exceeding 15 minutes require supervisor check-in.

Escalation SLA: escalations must be acknowledged within 5 minutes.

When an SLA breach occurs:
1. Apologize to the customer for the wait.
2. Apply the appropriate account credit automatically using code SLA-BREACH.
3. Document the breach in the call notes.`,
			tags: []string{"sla", "breach", "credit", "policy"},
		},
		{
			title: "Premium Support Plan — Agent FAQ",
			content: `Premium Support Plan features:
- 24/7 priority phone and chat support
- Dedicated support line: 1-800-555-PREM
- Average wait time < 2 minutes (SLA guaranteed)
- On-site support for Enterprise tier (within 4 hours in metro areas)
- Monthly account review call included

Pricing (current as of Q1 2026):
- Individual: $19.99/month
- Business (up to 10 seats): $79.99/month
- Enterprise (unlimited): $299.99/month

How to upgrade a customer:
1. Check if they are eligible (active account, good standing).
2. Use promo code UPGRADE10 for 10% off the first 3 months.
3. Process in billing system under "Plan Changes > Premium Upgrade".
4. Confirmation email sent automatically within 5 minutes.`,
			tags: []string{"premium", "plan", "pricing", "upgrade"},
		},
		{
			title: "Tier 1 Technical Troubleshooting Guide",
			content: `Standard troubleshooting steps for common issues:

Internet connectivity:
1. Ask customer to restart modem/router (unplug 30s, replug).
2. Check for outages in their area (use Outage Tracker tool).
3. If no outage: run remote line test from your console.
4. If line test fails: schedule technician visit.

Service not working after plan change:
1. Confirm change was processed (check billing system).
2. Ask customer to clear cache or restart device.
3. If still failing: trigger service reprovision (Provisioning tab).

App or portal login issues:
1. Verify email address on account.
2. Use "Send Password Reset" in customer account panel.
3. If locked out (5+ failed attempts): unlock from Security tab.

Escalate to Tier 2 (ext. 4200) if:
- Remote line test fails 3 times
- Issue persists after reprovisioning
- Hardware failure suspected`,
			tags: []string{"technical", "troubleshooting", "support", "tier1"},
		},
		{
			title: "Call Recording and Data Privacy Policy",
			content: `All calls are recorded for quality and training purposes.

Consent requirements:
- Automated disclosure plays at call start: "This call may be recorded."
- No additional agent consent required for inbound calls.
- For outbound calls: agent must verbally state recording notice before proceeding.

Customer data requests:
- Customers may request a copy of their call recording within 12 months.
- Submit requests to: privacy@cortexcc.com or ext. 4700.
- Turnaround: 10 business days.

Data retention:
- Call recordings: 12 months.
- Transcripts: 24 months.
- Billing records: 7 years (regulatory requirement).

Do NOT:
- Share call recordings without customer consent or legal subpoena.
- Access another agent's call recordings without supervisor authorization.
- Store customer PII (SSN, full card number) in call notes.`,
			tags: []string{"privacy", "recording", "compliance", "data"},
		},
		{
			title: "Customer Satisfaction Follow-Up Process",
			content: `Post-call survey process:
- Automated CSAT survey sent via SMS/email within 1 hour of call end.
- Survey asks: overall satisfaction (1-5), issue resolved (yes/no), likelihood to recommend (NPS 0-10).

Agent-triggered follow-up:
- For high-value customers (Premium plan, >3 years tenure): offer a personal follow-up call.
- Use the "Schedule Callback" feature in your console.
- Callbacks must happen within 48 hours.

Low CSAT handling:
- Score of 1-2: flag for supervisor review automatically.
- Supervisor must call back within 24 hours.
- Document recovery action in customer account notes.

Incentives for positive CSAT:
- Agents maintaining >4.2 CSAT average qualify for monthly bonus.
- Team leaderboard updated weekly — accessible on the intranet portal.`,
			tags: []string{"csat", "survey", "satisfaction", "follow-up"},
		},
		{
			title: "Account Cancellation Policy",
			content: `Before processing a cancellation:
1. Identify the cancellation reason (required field).
2. Offer a retention package based on reason:
   - Price: offer 20% discount for 3 months (code RETAIN20).
   - Moving: check if service is available at new address.
   - Competitor: offer loyalty credit ($25) and feature comparison.
   - Not satisfied: escalate to Retention Specialist (ext. 4300).
3. If customer still wants to cancel after retention attempt, proceed.

Processing cancellation:
1. Set end date (must be end of current billing period — no mid-cycle cancellations).
2. Confirm any outstanding balance or prorated refund.
3. Send confirmation email with cancellation number.
4. Final bill sent within 30 days.

Cooling-off period:
- Customers can reverse cancellation within 14 days at no penalty.
- Use "Reactivate Account" in billing system.`,
			tags: []string{"cancellation", "retention", "policy"},
		},
		{
			title: "After-Hours and Emergency Support",
			content: `After-hours support hours: 10pm – 6am local time, weekdays.
                      Weekend hours: 8pm – 8am.

After-hours channels:
- Phone: routed to on-call team automatically (same main number).
- Chat: AI chatbot handles Tier 1; escalates to on-call agent if needed.
- Self-service portal: available 24/7 at portal.cortexcc.com.

On-call escalation:
- On-call agent contact: available in the on-call schedule on the intranet.
- For critical outages (>100 customers affected): page the NOC at ext. 9911.
- For billing emergencies: on-call billing manager paged automatically for charges >$1000.

What counts as an emergency:
- Complete service outage for a business account.
- Security breach or suspected account compromise.
- Legal or regulatory deadline (court orders, etc.).

Non-emergencies should be routed to the next business day queue with a callback scheduled.`,
			tags: []string{"after-hours", "emergency", "on-call", "support"},
		},
	}

	now := time.Now()
	for _, a := range articles {
		art := &models.KBArticle{
			ID:        uuid.NewString(),
			Title:     a.title,
			Content:   a.content,
			Tags:      a.tags,
			CreatedAt: now,
		}
		if err := st.InsertKBArticle(art); err != nil {
			log.Printf("kb: seed error for %q: %v", a.title, err)
		}
	}
	log.Printf("kb: seeded %d articles", len(articles))
}
