# Beat dashboard

Import [beat.json](beat.json) into Grafana, then select your Prometheus **Data
source** and **Service**. The dashboard uses built-in panels only; it was tested
with Grafana 13.2.2 and Prometheus 3.14.0. No local datasource UID or demo service
is selected in the example.

## Metrics and filters

The dashboard expects classic Prometheus histograms from `otelbeat.MakeHandler`:

- `job_run_duration_seconds_{bucket,sum,count}` with `phase` and `outcome`.
- `job_run_processed_{bucket,sum,count}` with `outcome`.
- `beat_run_lateness_seconds_{bucket,sum,count}` with `mode`.
- `beat_missed_total` with `mode`.

Configure your export pipeline to expose `service_name`. Optional filters use
`deployment_environment_name`, `k8s_cluster_name`, `k8s_namespace_name` and scrape
`instance`. All matches missing optional labels, so local execution works without
Kubernetes. Keep independent replica streams distinct before aggregation.

These names assume underscore escaping with unit/type suffixes. Native
histograms or a different naming strategy require query changes.

## Reading the panels

The eight panels cover completed iterations, processed items, recovered panics,
item throughput, work and ErrorHandler error rates, execution duration, missed
points and start lateness. They do not duplicate runtime or queue metrics.

Top-level counts use the selected dashboard range. Rates and quantiles use the
separate **Rate / quantile window**, default 15m. Choose a window covering at
least four scrape intervals and several execution periods. **Duration phase**
selects total, work or error_handler; count attempts using phase=total only.

- ErrorHandler has its own error-rate denominator. Canceled and unknown outcomes
  are excluded. An idle window has no rate, rather than a healthy 0%.
- Counts based on increase are estimates; the first unobserved increment cannot
  be recovered. Processed items include reported partial progress, not unique IDs.
- Quantiles are bucket estimates. A blocked attempt produces no completion data.
- Missed points arrive with the next completed attempt and may be lost at shutdown.

The link to Job / CronJob works when that optional dashboard is also imported
with its original UID. This dashboard does not configure exporters or alerts.
