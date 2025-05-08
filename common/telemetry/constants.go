package telemetry

const (
	MaxMessageSize                 = 100
	MetricMessageCount             = "hb_in_metric_messages_count"
	MetricsCount                   = "hb_in_metrics_count"
	MaxDeviceNameAndMetricNameSize = 25 + 35
	CommandMessageCount            = "hb_command_messages_count"
	CommandsCount                  = "hb_commands_count"
	ContextDataCallsCount          = "hb_contextual_data_api_calls_count"
	ContextDataSize                = "hb_contextual_data_size_bytes"
	CompletedJobsTime              = "hb_completed_jobs_training_time_mins"
	FailedJobsTime                 = "hb_failed_jobs_training_time_mins"
	CanceledJobsTime               = "hb_canceled_jobs_training_time_mins"
	TerminatedJobsTime             = "hb_terminated_jobs_training_time_mins"
	CompletedJobsCount             = "hb_completed_jobs_count"
	FailedJobsCount                = "hb_failed_jobs_count"
	CanceledJobsCount              = "hb_canceled_jobs_count"
	TerminatedJobsCount            = "hb_terminated_jobs_count"
)
