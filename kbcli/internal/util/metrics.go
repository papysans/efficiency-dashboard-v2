package util

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	metricsNamespace = "kbcli"
)

var (
	// kbcli_command_runs_total 命令执行总次数
	// labels: command (import/import-conv/import-repo/import-org/efficiency-v2), status (success/fail)
	commandRunsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Name:      "command_runs_total",
			Help:      "Total number of command runs, labeled by command and status",
		},
		[]string{"command", "status"},
	)

	// kbcli_command_duration_seconds 命令执行耗时
	// labels: command
	commandDurationSeconds = promauto.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace:  metricsNamespace,
			Name:       "command_duration_seconds",
			Help:       "Duration of command execution in seconds",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"command"},
	)

	// kbcli_command_records_total 处理的记录数
	// labels: command, result (success/fail/skip)
	commandRecordsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Name:      "command_records_total",
			Help:      "Total number of records processed, labeled by command and result",
		},
		[]string{"command", "result"},
	)

	// kbcli_command_last_run_timestamp 最后一次命令执行的 UNIX 时间戳
	// labels: command
	commandLastRunTimestamp = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "command_last_run_timestamp",
			Help:      "UNIX timestamp of the last command run",
		},
		[]string{"command"},
	)

	// kbcli_command_last_status 最后一次命令执行的状态 (1=success, 0=fail)
	// labels: command
	commandLastStatus = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "command_last_status",
			Help:      "Status of the last command run: 1=success, 0=fail",
		},
		[]string{"command"},
	)
)

// RecordCommandRun 记录一次命令执行的指标
func RecordCommandRun(command string, startTime time.Time, successCount, failCount, skipCount int, runErr error) {
	elapsed := time.Since(startTime).Seconds()
	commandDurationSeconds.WithLabelValues(command).Observe(elapsed)
	commandLastRunTimestamp.WithLabelValues(command).Set(float64(time.Now().Unix()))

	if runErr != nil {
		commandRunsTotal.WithLabelValues(command, "fail").Inc()
		commandLastStatus.WithLabelValues(command).Set(0)
	} else {
		commandRunsTotal.WithLabelValues(command, "success").Inc()
		commandLastStatus.WithLabelValues(command).Set(1)
	}

	if successCount > 0 {
		commandRecordsTotal.WithLabelValues(command, "success").Add(float64(successCount))
	}
	if failCount > 0 {
		commandRecordsTotal.WithLabelValues(command, "fail").Add(float64(failCount))
	}
	if skipCount > 0 {
		commandRecordsTotal.WithLabelValues(command, "skip").Add(float64(skipCount))
	}
}
