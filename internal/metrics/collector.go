package metrics

import (
	"github.com/prometheus/client_golang/prometheus"

	"cortex-cc/internal/engine"
	"cortex-cc/internal/models"
	"cortex-cc/internal/store"
)

var (
	descCallsActive = prometheus.NewDesc(
		"cortex_calls_active",
		"Number of currently active calls, partitioned by queue.",
		[]string{"queue"}, nil,
	)
	descCallsWaiting = prometheus.NewDesc(
		"cortex_calls_waiting",
		"Number of calls waiting in queue.",
		[]string{"queue"}, nil,
	)
	descSLABreaches = prometheus.NewDesc(
		"cortex_calls_sla_breached",
		"Number of SLA-breached calls currently in queue (wait > 120s).",
		[]string{"queue"}, nil,
	)
	descAgents = prometheus.NewDesc(
		"cortex_agents_total",
		"Number of agents, partitioned by status.",
		[]string{"status"}, nil,
	)
	descAvgHandleTime = prometheus.NewDesc(
		"cortex_avg_handle_time_seconds",
		"Fleet-wide average handle time across all agents.",
		nil, nil,
	)
	descQAScore = prometheus.NewDesc(
		"cortex_qa_score_avg",
		"Average QA dimension score (1-10) across the last 50 scored calls.",
		[]string{"dimension"}, nil,
	)
)

// Collector implements prometheus.Collector. It pulls live data from the
// engine and store on every scrape — no background goroutine required.
type Collector struct {
	eng   *engine.Engine
	store *store.Store
}

func NewCollector(eng *engine.Engine, st *store.Store) *Collector {
	return &Collector{eng: eng, store: st}
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- descCallsActive
	ch <- descCallsWaiting
	ch <- descSLABreaches
	ch <- descAgents
	ch <- descAvgHandleTime
	ch <- descQAScore
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	c.collectCallMetrics(ch)
	c.collectAgentMetrics(ch)
	c.collectQAMetrics(ch)
}

func (c *Collector) collectCallMetrics(ch chan<- prometheus.Metric) {
	active := map[string]float64{}
	waiting := map[string]float64{}
	breached := map[string]float64{}

	for _, call := range c.eng.GetActiveCalls() {
		switch call.Status {
		case models.CallStatusActive:
			active[call.QueueName]++
		case models.CallStatusQueued:
			waiting[call.QueueName]++
			if call.SLABreached {
				breached[call.QueueName]++
			}
		}
	}

	queues := []string{"Sales", "Billing", "Support"}
	for _, q := range queues {
		ch <- prometheus.MustNewConstMetric(descCallsActive, prometheus.GaugeValue, active[q], q)
		ch <- prometheus.MustNewConstMetric(descCallsWaiting, prometheus.GaugeValue, waiting[q], q)
		ch <- prometheus.MustNewConstMetric(descSLABreaches, prometheus.GaugeValue, breached[q], q)
	}
}

func (c *Collector) collectAgentMetrics(ch chan<- prometheus.Metric) {
	byStatus := map[models.AgentStatus]float64{}
	var totalHandleTime float64
	var agentCount float64

	for _, a := range c.eng.GetAgents() {
		byStatus[a.Status]++
		if a.AvgHandleTime > 0 {
			totalHandleTime += a.AvgHandleTime
			agentCount++
		}
	}

	for status, count := range byStatus {
		ch <- prometheus.MustNewConstMetric(descAgents, prometheus.GaugeValue, count, string(status))
	}

	avg := 0.0
	if agentCount > 0 {
		avg = totalHandleTime / agentCount
	}
	ch <- prometheus.MustNewConstMetric(descAvgHandleTime, prometheus.GaugeValue, avg)
}

func (c *Collector) collectQAMetrics(ch chan<- prometheus.Metric) {
	scores, err := c.store.GetRecentCallScores(50)
	if err != nil || len(scores) == 0 {
		return
	}

	var empathy, resolution, professionalism, overall float64
	n := float64(len(scores))
	for _, s := range scores {
		empathy += float64(s.Empathy)
		resolution += float64(s.Resolution)
		professionalism += float64(s.Professionalism)
		overall += float64(s.Overall)
	}

	ch <- prometheus.MustNewConstMetric(descQAScore, prometheus.GaugeValue, empathy/n, "empathy")
	ch <- prometheus.MustNewConstMetric(descQAScore, prometheus.GaugeValue, resolution/n, "resolution")
	ch <- prometheus.MustNewConstMetric(descQAScore, prometheus.GaugeValue, professionalism/n, "professionalism")
	ch <- prometheus.MustNewConstMetric(descQAScore, prometheus.GaugeValue, overall/n, "overall")
}
